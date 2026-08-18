// Package exchange provides display-only currency conversion.
//
// Rates served here are never used to derive transaction_entries.base_amount_minor. That
// value is booked at the transaction date's rate by a separate mechanism, because
// recomputing stored base amounts at today's rate would violate invariant 12 of
// docs/domain-model.md. See docs/decisions/0005-supported-currencies-and-display-conversion.md.
package exchange

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/nihatatay93/budget/internal/money"
)

// ErrUnavailable reports that no rate could be served. Callers treat this as "show no
// converted figure", never as a request failure.
var ErrUnavailable = errors.New("exchange rates are unavailable")

// Rate is one published conversion factor. Date is the provider's publication date, which
// can be older than today because reference rates are published on working days only.
type Rate struct {
	Base      money.Currency
	Quote     money.Currency
	Rate      *big.Rat
	Date      time.Time
	FetchedAt time.Time
}

// Convert applies the rate to an amount in the base currency's minor units.
func (r Rate) Convert(amountMinor int64) (int64, error) {
	return money.Convert(amountMinor, r.Rate)
}

// Provider fetches rates from outside the application.
type Provider interface {
	LatestRates(ctx context.Context, base money.Currency, quotes []money.Currency) ([]Rate, error)
}

// Repository caches fetched rates so normal operation does not depend on provider uptime.
type Repository interface {
	LatestRates(ctx context.Context, base money.Currency) ([]Rate, error)
	SaveRates(ctx context.Context, rates []Rate) error
}

// Clock exists so freshness logic is testable without waiting for real time to pass.
type Clock interface {
	Now() time.Time
}

// Service serves rates from cache, refreshing from the provider when the cache is stale.
type Service struct {
	repository Repository
	provider   Provider
	clock      Clock
	logger     *slog.Logger
	// freshness bounds how long a cache fetch is reused before a refresh is attempted.
	// Publication dates can legitimately be several days old over weekends and holidays, so
	// freshness is measured from the fetch time rather than the provider's rate date.
	freshness time.Duration
}

func NewService(
	repository Repository,
	provider Provider,
	clock Clock,
	logger *slog.Logger,
) *Service {
	return &Service{
		repository: repository,
		provider:   provider,
		clock:      clock,
		logger:     logger,
		freshness:  24 * time.Hour,
	}
}

// Rates returns conversions from base into every other supported currency.
//
// It never returns a transport or provider error: a caller that cannot show a converted
// figure should still render the page. Failures degrade to ErrUnavailable and are logged.
func (s *Service) Rates(ctx context.Context, base money.Currency) ([]Rate, error) {
	if s == nil || !base.Valid() {
		return nil, ErrUnavailable
	}
	cached, err := s.repository.LatestRates(ctx, base)
	if err != nil {
		s.logger.WarnContext(ctx, "read cached exchange rates", "error", err, "base", base)
		cached = nil
	}
	if s.fresh(base, cached) {
		return cached, nil
	}

	fetched, err := s.provider.LatestRates(ctx, base, quotesFor(base))
	if err != nil {
		s.logger.WarnContext(ctx, "fetch exchange rates", "error", err, "base", base)
		// Stale cache beats nothing: the response carries its own date so a client can show
		// how old the figure is.
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, ErrUnavailable
	}
	if !complete(base, fetched) {
		s.logger.WarnContext(ctx, "fetch exchange rates", "error", "incomplete provider response", "base", base)
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, ErrUnavailable
	}
	fetchedAt := s.clock.Now()
	for index := range fetched {
		fetched[index].FetchedAt = fetchedAt
	}
	if err := s.repository.SaveRates(ctx, fetched); err != nil {
		s.logger.WarnContext(ctx, "cache exchange rates", "error", err, "base", base)
	}
	return fetched, nil
}

// fresh reports whether every supported quote is present and recent enough to serve without
// contacting the provider.
func (s *Service) fresh(base money.Currency, rates []Rate) bool {
	if len(rates) == 0 {
		return false
	}
	cutoff := s.clock.Now().Add(-s.freshness)
	for _, rate := range rates {
		if rate.FetchedAt.IsZero() || rate.FetchedAt.Before(cutoff) {
			return false
		}
	}
	return complete(base, rates)
}

func complete(base money.Currency, rates []Rate) bool {
	wanted := quotesFor(base)
	if len(rates) != len(wanted) {
		return false
	}
	seen := make(map[money.Currency]bool, len(rates))
	for _, rate := range rates {
		if rate.Base != base || rate.Quote == base || !rate.Quote.Valid() || rate.Rate == nil ||
			rate.Rate.Sign() <= 0 || rate.Date.IsZero() || seen[rate.Quote] {
			return false
		}
		seen[rate.Quote] = true
	}
	for _, quote := range wanted {
		if !seen[quote] {
			return false
		}
	}
	return true
}

// quotesFor returns every supported currency except the base.
func quotesFor(base money.Currency) []money.Currency {
	supported := money.Supported()
	quotes := make([]money.Currency, 0, len(supported))
	for _, currency := range supported {
		if currency != base {
			quotes = append(quotes, currency)
		}
	}
	return quotes
}

// ParseRate converts a provider's decimal string into an exact rational.
func ParseRate(value string) (*big.Rat, error) {
	rate, ok := new(big.Rat).SetString(value)
	if !ok || rate.Sign() <= 0 {
		return nil, fmt.Errorf("%w: unusable rate %q", ErrUnavailable, value)
	}
	return rate, nil
}
