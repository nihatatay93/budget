package exchange

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/money"
)

type historicalProviderStub struct {
	rates []Rate
	err   error
}

func (s historicalProviderStub) HistoricalRates(
	context.Context, time.Time, money.Currency, []money.Currency,
) ([]Rate, error) {
	return s.rates, s.err
}

func TestBookingServiceConvertsAndCachesHistoricalRate(t *testing.T) {
	rate, _ := new(big.Rat).SetString("49.25")
	repository := &repositoryStub{}
	service := NewBookingService(
		historicalProviderStub{rates: []Rate{{
			Base: money.USD, Quote: money.TRY, Rate: rate,
			Date: testNow.Add(-24 * time.Hour),
		}}},
		repository, fixedClock{},
	)
	got, err := service.BaseAmount(
		context.Background(), testNow, money.USD, money.TRY, 100,
	)
	if err != nil || got != 4925 {
		t.Fatalf("BaseAmount() = %d, error = %v", got, err)
	}
	if len(repository.saved) != 1 || repository.saved[0].FetchedAt != testNow {
		t.Fatalf("saved rates = %+v", repository.saved)
	}
}

func TestBookingServiceRejectsMissingFutureAndProviderFailure(t *testing.T) {
	service := NewBookingService(
		historicalProviderStub{err: errors.New("offline")}, &repositoryStub{}, fixedClock{},
	)
	for _, date := range []time.Time{testNow, testNow.Add(24 * time.Hour)} {
		if _, err := service.BaseAmount(
			context.Background(), date, money.USD, money.TRY, 100,
		); !errors.Is(err, ErrHistoricalRateUnavailable) {
			t.Fatalf("BaseAmount(%s) error = %v", date, err)
		}
	}
}
