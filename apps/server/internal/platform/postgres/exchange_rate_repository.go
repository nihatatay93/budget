package postgres

import (
	"context"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/exchange"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
)

// rateDecimalPlaces bounds how much of a rate survives the round trip through NUMERIC.
// Reference rates are published with roughly five decimal places, so this is exact for real
// provider data and only rounds a rate that was already more precise than the source.
const rateDecimalPlaces = 12

type ExchangeRateRepository struct {
	pool *pgxpool.Pool
}

func NewExchangeRateRepository(pool *pgxpool.Pool) *ExchangeRateRepository {
	return &ExchangeRateRepository{pool: pool}
}

func (r *ExchangeRateRepository) LatestRates(
	ctx context.Context,
	base money.Currency,
) ([]exchange.Rate, error) {
	rows, err := sqlc.New(r.pool).LatestExchangeRates(ctx, base.String())
	if err != nil {
		return nil, fmt.Errorf("list exchange rates: %w", err)
	}
	rates := make([]exchange.Rate, 0, len(rows))
	for _, row := range rows {
		baseCurrency, baseOK := money.Parse(row.BaseCurrency)
		quote, quoteOK := money.Parse(row.QuoteCurrency)
		if !baseOK || !quoteOK {
			// A currency retired from the supported set leaves rows behind; skip them rather
			// than failing a read that the caller only needs for display.
			continue
		}
		rate, err := ratFromNumeric(row.Rate)
		if err != nil {
			return nil, err
		}
		rates = append(rates, exchange.Rate{
			Base: baseCurrency, Quote: quote, Rate: rate, Date: row.RateDate.Time,
			FetchedAt: row.FetchedAt.Time,
		})
	}
	return rates, nil
}

func (r *ExchangeRateRepository) SaveRates(ctx context.Context, rates []exchange.Rate) error {
	if len(rates) == 0 {
		return nil
	}
	queries := sqlc.New(r.pool)
	for _, rate := range rates {
		numeric, err := numericFromRat(rate.Rate)
		if err != nil {
			return err
		}
		if err := queries.UpsertExchangeRate(ctx, sqlc.UpsertExchangeRateParams{
			RateDate:      pgtype.Date{Time: rate.Date, Valid: true},
			BaseCurrency:  rate.Base.String(),
			QuoteCurrency: rate.Quote.String(),
			Rate:          numeric,
		}); err != nil {
			return fmt.Errorf("save exchange rate %s/%s: %w", rate.Base, rate.Quote, err)
		}
	}
	return nil
}

// numericFromRat renders the rate as a fixed-point decimal string so PostgreSQL stores the
// published value rather than a binary float approximation of it.
func numericFromRat(rate *big.Rat) (pgtype.Numeric, error) {
	if rate == nil || rate.Sign() <= 0 {
		return pgtype.Numeric{}, money.ErrInvalidRate
	}
	var numeric pgtype.Numeric
	if err := numeric.Scan(rate.FloatString(rateDecimalPlaces)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("encode exchange rate: %w", err)
	}
	return numeric, nil
}

func ratFromNumeric(value pgtype.Numeric) (*big.Rat, error) {
	if !value.Valid || value.NaN {
		return nil, money.ErrInvalidRate
	}
	encoded, err := value.Value()
	if err != nil {
		return nil, fmt.Errorf("decode exchange rate: %w", err)
	}
	text, ok := encoded.(string)
	if !ok {
		return nil, money.ErrInvalidRate
	}
	rate, ok := new(big.Rat).SetString(text)
	if !ok || rate.Sign() <= 0 {
		return nil, money.ErrInvalidRate
	}
	return rate, nil
}
