package exchange

import (
	"context"
	"errors"
	"time"

	"github.com/nihatatay93/budget/internal/money"
)

var ErrHistoricalRateUnavailable = errors.New("historical exchange rate is unavailable")

type HistoricalProvider interface {
	HistoricalRates(
		context.Context, time.Time, money.Currency, []money.Currency,
	) ([]Rate, error)
}

// BookingService resolves a transaction-date rate. It intentionally never reads the latest
// display-rate cache: a missing historical lookup must fail rather than silently book at an
// unrelated date.
type BookingService struct {
	provider   HistoricalProvider
	repository Repository
	clock      Clock
}

func NewBookingService(
	provider HistoricalProvider,
	repository Repository,
	clock Clock,
) *BookingService {
	return &BookingService{provider: provider, repository: repository, clock: clock}
}

func (s *BookingService) BaseAmount(
	ctx context.Context,
	date time.Time,
	from, to money.Currency,
	amountMinor int64,
) (int64, error) {
	if s == nil || s.provider == nil || !from.Valid() || !to.Valid() || from == to ||
		date.IsZero() || date.After(s.clock.Now()) {
		return 0, ErrHistoricalRateUnavailable
	}
	rates, err := s.provider.HistoricalRates(ctx, date, from, []money.Currency{to})
	if err != nil || len(rates) != 1 || rates[0].Base != from || rates[0].Quote != to ||
		rates[0].Date.After(date) {
		return 0, ErrHistoricalRateUnavailable
	}
	rates[0].FetchedAt = s.clock.Now()
	converted, err := rates[0].Convert(amountMinor)
	if err != nil {
		return 0, ErrHistoricalRateUnavailable
	}
	// Cache the provider response for diagnostics and future reuse, but a cache failure must
	// not discard a rate that was successfully resolved for the current atomic write.
	_ = s.repository.SaveRates(ctx, rates)
	return converted, nil
}
