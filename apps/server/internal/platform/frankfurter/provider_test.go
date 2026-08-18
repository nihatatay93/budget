package frankfurter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/money"
)

// livePayload is a verbatim response captured from https://api.frankfurter.dev on
// 2026-08-17, so the parser is pinned to the shape the real provider actually serves.
const livePayload = `{"amount":1.0,"base":"TRY","date":"2026-08-17","rates":{"EUR":0.01801,"USD":0.02088}}`

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(server.URL, 5*time.Second)
}

func TestLatestRatesParsesLivePayload(t *testing.T) {
	var requested string
	provider := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(livePayload))
	})

	rates, err := provider.LatestRates(
		context.Background(), money.TRY, []money.Currency{money.USD, money.EUR},
	)
	if err != nil {
		t.Fatalf("LatestRates() error = %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("LatestRates() returned %d rates, want 2", len(rates))
	}
	if rates[0].Quote != money.USD || rates[1].Quote != money.EUR {
		t.Fatalf("LatestRates() quote order = %s, %s, want request order USD, EUR", rates[0].Quote, rates[1].Quote)
	}
	if requested != "base=TRY&symbols=USD%2CEUR" {
		t.Fatalf("request query = %q", requested)
	}

	byQuote := map[money.Currency]string{}
	for _, rate := range rates {
		if rate.Base != money.TRY {
			t.Fatalf("rate base = %s, want TRY", rate.Base)
		}
		if got := rate.Date.Format(time.DateOnly); got != "2026-08-17" {
			t.Fatalf("rate date = %s, want the payload's publication date", got)
		}
		byQuote[rate.Quote] = rate.Rate.FloatString(5)
	}
	if byQuote[money.USD] != "0.02088" || byQuote[money.EUR] != "0.01801" {
		t.Fatalf("parsed rates = %v, want the exact published decimals", byQuote)
	}
}

// The published decimal must survive as an exact rational: 50,000.00 TRY is exactly
// 1,044.00 USD at 0.02088, and a float64 round trip would not land on it reliably.
func TestLatestRatesPreservesExactDecimal(t *testing.T) {
	provider := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(livePayload))
	})

	rates, err := provider.LatestRates(context.Background(), money.TRY, []money.Currency{money.USD})
	if err != nil {
		t.Fatalf("LatestRates() error = %v", err)
	}
	for _, rate := range rates {
		if rate.Quote != money.USD {
			continue
		}
		converted, err := rate.Convert(5000000)
		if err != nil || converted != 104400 {
			t.Fatalf("Convert() = %d, err = %v, want 104400", converted, err)
		}
	}
}

func TestLatestRatesIgnoresUnsupportedSymbols(t *testing.T) {
	provider := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"base":"TRY","date":"2026-08-17","rates":{"USD":0.02088,"GBP":0.0155}}`))
	})

	rates, err := provider.LatestRates(context.Background(), money.TRY, []money.Currency{money.USD})
	if err != nil {
		t.Fatalf("LatestRates() error = %v", err)
	}
	if len(rates) != 1 || rates[0].Quote != money.USD {
		t.Fatalf("LatestRates() = %+v, want only the supported symbol", rates)
	}
}

func TestLatestRatesRejectsBadResponses(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		payload string
	}{
		{name: "server error", status: http.StatusInternalServerError, payload: `{}`},
		{name: "malformed json", status: http.StatusOK, payload: `not json`},
		{name: "trailing json", status: http.StatusOK, payload: `{"base":"TRY","date":"2026-08-17","rates":{"USD":0.02}} {}`},
		{name: "unparseable date", status: http.StatusOK, payload: `{"base":"TRY","date":"today","rates":{"USD":0.02}}`},
		{name: "base mismatch", status: http.StatusOK, payload: `{"base":"EUR","date":"2026-08-17","rates":{"USD":0.02}}`},
		{name: "missing quote", status: http.StatusOK, payload: `{"base":"TRY","date":"2026-08-17","rates":{"EUR":0.018}}`},
		{name: "zero rate", status: http.StatusOK, payload: `{"base":"TRY","date":"2026-08-17","rates":{"USD":0}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.payload))
			})
			if _, err := provider.LatestRates(
				context.Background(), money.TRY, []money.Currency{money.USD},
			); err == nil {
				t.Fatal("LatestRates() error = nil, want a failure the service can degrade from")
			}
		})
	}
}

func TestHistoricalRatesUsesBusinessDatePath(t *testing.T) {
	var path string
	provider := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"base":"USD","date":"2026-08-14","rates":{"TRY":49.25}}`))
	})
	rates, err := provider.HistoricalRates(
		context.Background(), time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
		money.USD, []money.Currency{money.TRY},
	)
	if err != nil {
		t.Fatalf("HistoricalRates() error = %v", err)
	}
	if path != "/v1/2026-08-16" || len(rates) != 1 || rates[0].Date.Format(time.DateOnly) != "2026-08-14" {
		t.Fatalf("path = %q, rates = %+v", path, rates)
	}
}
