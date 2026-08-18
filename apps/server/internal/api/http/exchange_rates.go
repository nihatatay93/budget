package httpapi

import (
	"context"
	"errors"

	openapi_types "github.com/oapi-codegen/runtime/types"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/exchange"
)

func (s *server) ListExchangeRates(
	ctx context.Context,
	request openapi.ListExchangeRatesRequestObject,
) (openapi.ListExchangeRatesResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	// A nil service means the operator did not enable rate fetching. That is a supported
	// configuration, not a fault, so it produces the same 503 as a provider outage.
	if s.ExchangeRates == nil {
		return exchangeRatesUnavailable(requestID), nil
	}
	rates, err := s.ExchangeRates.Rates(ctx, request.WorkspaceId.String(), principal.User.ID)
	switch {
	case errors.Is(err, exchange.ErrUnavailable):
		return exchangeRatesUnavailable(requestID), nil
	case err != nil:
		return nil, err
	}
	response := make([]openapi.ExchangeRate, 0, len(rates))
	for _, rate := range rates {
		response = append(response, openapi.ExchangeRate{
			BaseCurrency:  openapi.Currency(rate.Base),
			QuoteCurrency: openapi.Currency(rate.Quote),
			// Sent as a decimal string so the published precision survives JSON transport.
			Rate:     rate.Rate.FloatString(exchangeRateDecimalPlaces),
			RateDate: openapi_types.Date{Time: rate.Date},
		})
	}
	return openapi.ListExchangeRates200JSONResponse{
		Body:    openapi.ExchangeRateListResponse{Rates: response},
		Headers: openapi.ListExchangeRates200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

// exchangeRateDecimalPlaces matches the precision the cache preserves.
const exchangeRateDecimalPlaces = 12

func exchangeRatesUnavailable(requestID string) openapi.ListExchangeRates503JSONResponse {
	return openapi.ListExchangeRates503JSONResponse{
		Body: errorBody(
			requestID,
			"exchange_rates_unavailable",
			"Currency conversion is unavailable.",
		),
		Headers: openapi.ListExchangeRates503ResponseHeaders{XRequestID: &requestID},
	}
}
