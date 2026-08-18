package exchange

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/money"
)

var testNow = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return testNow }

type repositoryStub struct {
	rates []Rate
	err   error
	saved []Rate
}

func (s *repositoryStub) LatestRates(context.Context, money.Currency) ([]Rate, error) {
	return s.rates, s.err
}

func (s *repositoryStub) SaveRates(_ context.Context, rates []Rate) error {
	s.saved = append(s.saved, rates...)
	return nil
}

type providerStub struct {
	rates  []Rate
	err    error
	calls  int
	called bool
}

func (s *providerStub) LatestRates(
	context.Context, money.Currency, []money.Currency,
) ([]Rate, error) {
	s.calls++
	s.called = true
	return s.rates, s.err
}

func rateAt(quote money.Currency, value string, date time.Time) Rate {
	parsed, _ := new(big.Rat).SetString(value)
	return Rate{Base: money.TRY, Quote: quote, Rate: parsed, Date: date, FetchedAt: date}
}

func freshRates() []Rate {
	return []Rate{
		rateAt(money.USD, "0.02088", testNow),
		rateAt(money.EUR, "0.01801", testNow),
	}
}

func newTestService(repository Repository, provider Provider) *Service {
	return NewService(repository, provider, fixedClock{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestRatesServesFreshCacheWithoutCallingProvider(t *testing.T) {
	provider := &providerStub{}
	service := newTestService(&repositoryStub{rates: freshRates()}, provider)

	rates, err := service.Rates(context.Background(), money.TRY)
	if err != nil {
		t.Fatalf("Rates() error = %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("Rates() returned %d rates, want 2", len(rates))
	}
	if provider.called {
		t.Fatal("Rates() contacted the provider despite a fresh cache")
	}
}

func TestRatesRefreshesAndCachesWhenStale(t *testing.T) {
	stale := []Rate{rateAt(money.USD, "0.02", testNow.Add(-72*time.Hour))}
	repository := &repositoryStub{rates: stale}
	provider := &providerStub{rates: freshRates()}
	service := newTestService(repository, provider)

	rates, err := service.Rates(context.Background(), money.TRY)
	if err != nil {
		t.Fatalf("Rates() error = %v", err)
	}
	if len(rates) != 2 || !provider.called {
		t.Fatalf("Rates() = %d rates, provider called = %v", len(rates), provider.called)
	}
	if len(repository.saved) != 2 {
		t.Fatalf("Rates() cached %d rates, want 2", len(repository.saved))
	}
}

// A missing quote must trigger a refresh even when the cached row is recent, otherwise a
// partially populated cache would permanently hide one currency.
func TestRatesRefreshesWhenAQuoteIsMissing(t *testing.T) {
	partial := []Rate{rateAt(money.USD, "0.02088", testNow)}
	provider := &providerStub{rates: freshRates()}
	service := newTestService(&repositoryStub{rates: partial}, provider)

	if _, err := service.Rates(context.Background(), money.TRY); err != nil {
		t.Fatalf("Rates() error = %v", err)
	}
	if !provider.called {
		t.Fatal("Rates() served an incomplete cache without refreshing")
	}
}

func TestRatesRejectsIncompleteProviderResponse(t *testing.T) {
	provider := &providerStub{rates: []Rate{rateAt(money.USD, "0.02088", testNow)}}
	service := newTestService(&repositoryStub{}, provider)

	if _, err := service.Rates(context.Background(), money.TRY); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Rates() error = %v, want ErrUnavailable", err)
	}
}

func TestRatesUsesFetchTimeRatherThanPublicationDateForFreshness(t *testing.T) {
	weekendPublication := testNow.Add(-72 * time.Hour)
	rates := []Rate{
		rateAt(money.USD, "0.02088", weekendPublication),
		rateAt(money.EUR, "0.01801", weekendPublication),
	}
	for index := range rates {
		rates[index].FetchedAt = testNow.Add(-time.Hour)
	}
	provider := &providerStub{}
	service := newTestService(&repositoryStub{rates: rates}, provider)

	if _, err := service.Rates(context.Background(), money.TRY); err != nil {
		t.Fatalf("Rates() error = %v", err)
	}
	if provider.called {
		t.Fatal("Rates() refreshed a recently fetched weekend rate")
	}
}

// Degradation is the point of the design: a provider outage must not fail the request.
func TestRatesFallsBackToStaleCacheWhenProviderFails(t *testing.T) {
	stale := []Rate{
		rateAt(money.USD, "0.02", testNow.Add(-72*time.Hour)),
		rateAt(money.EUR, "0.018", testNow.Add(-72*time.Hour)),
	}
	service := newTestService(
		&repositoryStub{rates: stale},
		&providerStub{err: errors.New("network unreachable")},
	)

	rates, err := service.Rates(context.Background(), money.TRY)
	if err != nil {
		t.Fatalf("Rates() error = %v, want stale cache", err)
	}
	if len(rates) != 2 {
		t.Fatalf("Rates() returned %d rates, want the stale pair", len(rates))
	}
}

func TestRatesReportsUnavailableWithNoCacheAndFailingProvider(t *testing.T) {
	service := newTestService(
		&repositoryStub{},
		&providerStub{err: errors.New("network unreachable")},
	)

	if _, err := service.Rates(context.Background(), money.TRY); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Rates() error = %v, want ErrUnavailable", err)
	}
}

// A failing cache read must not stop a provider fetch from serving the request.
func TestRatesSurvivesRepositoryFailure(t *testing.T) {
	provider := &providerStub{rates: freshRates()}
	service := newTestService(&repositoryStub{err: errors.New("database down")}, provider)

	rates, err := service.Rates(context.Background(), money.TRY)
	if err != nil || len(rates) != 2 {
		t.Fatalf("Rates() = %d rates, error = %v", len(rates), err)
	}
}

func TestRatesRejectsUnsupportedBase(t *testing.T) {
	service := newTestService(&repositoryStub{rates: freshRates()}, &providerStub{})
	if _, err := service.Rates(context.Background(), money.Currency("XYZ")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Rates() error = %v, want ErrUnavailable", err)
	}
}

func TestRateConvertMatchesPublishedExample(t *testing.T) {
	rate := rateAt(money.USD, "0.02088", testNow)
	// 50,000.00 TRY, the figure ADR 0005 uses.
	converted, err := rate.Convert(5000000)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if converted != 104400 {
		t.Fatalf("Convert() = %d, want 104400", converted)
	}
}
