package httpapi

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/exchange"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

type fakeExchangeRateService struct {
	rates []exchange.Rate
	err   error
	call  resourceListCall
}

func (s *fakeExchangeRateService) Rates(
	_ context.Context, workspaceID, userID string,
) ([]exchange.Rate, error) {
	s.call = resourceListCall{workspaceID, userID, false}
	return s.rates, s.err
}

func exchangeRateTestRouter(t *testing.T, rates exchangeRateService) http.Handler {
	t.Helper()
	services := testServices()
	services.ExchangeRates = rates
	return testRouter(t, services)
}

func getExchangeRates(t *testing.T, router http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	return performJSON(
		t, router, http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/exchange-rates", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
}

func TestListExchangeRatesReturnsRatesWithPublicationDate(t *testing.T) {
	published := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	rate, _ := new(big.Rat).SetString("0.02088")
	service := &fakeExchangeRateService{rates: []exchange.Rate{
		{Base: money.TRY, Quote: money.USD, Rate: rate, Date: published},
	}}
	response := getExchangeRates(t, exchangeRateTestRouter(t, service))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.call != (resourceListCall{testWorkspaceID, testUserID, false}) {
		t.Fatalf("Rates() call = %#v", service.call)
	}
	var body struct {
		Rates []struct {
			BaseCurrency  string `json:"base_currency"`
			QuoteCurrency string `json:"quote_currency"`
			Rate          string `json:"rate"`
			RateDate      string `json:"rate_date"`
		} `json:"rates"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Rates) != 1 {
		t.Fatalf("rates = %#v", body.Rates)
	}
	got := body.Rates[0]
	// The rate travels as a decimal string so no precision is lost in JSON.
	if got.Rate != "0.020880000000" || got.BaseCurrency != "TRY" || got.QuoteCurrency != "USD" {
		t.Fatalf("rate = %#v", got)
	}
	if got.RateDate != "2026-08-17" {
		t.Fatalf("rate_date = %q, want the publication date", got.RateDate)
	}
}

// A deployment that never enabled rate fetching must still serve every other endpoint, so a
// disabled service reports unavailable rather than failing the request.
//
// The service is supplied exactly as app.go supplies it when the feature is off: a typed nil
// rather than a literal nil interface. Passing a literal nil here would exercise a case
// production never produces and would hide a panic on the default configuration.
func TestListExchangeRatesReportsUnavailableWhenDisabled(t *testing.T) {
	response := getExchangeRates(t, exchangeRateTestRouter(t, disabledExchangeRates()))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "exchange_rates_unavailable" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

func TestListExchangeRatesReportsUnavailableWhenProviderFails(t *testing.T) {
	service := &fakeExchangeRateService{err: exchange.ErrUnavailable}
	response := getExchangeRates(t, exchangeRateTestRouter(t, service))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
}

func TestListExchangeRatesEnforcesWorkspaceMembership(t *testing.T) {
	service := &fakeExchangeRateService{err: workspace.ErrForbidden}
	response := getExchangeRates(t, exchangeRateTestRouter(t, service))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
}

func TestListExchangeRatesRequiresAuthentication(t *testing.T) {
	router := exchangeRateTestRouter(t, &fakeExchangeRateService{})
	response := performJSON(
		t, router, http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/exchange-rates", "", nil,
	)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}
