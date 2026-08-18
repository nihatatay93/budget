-- name: LatestExchangeRates :many
-- One row per quote currency, each the most recently published rate for that pair.
SELECT DISTINCT ON (quote_currency)
    rate_date,
    base_currency,
    quote_currency,
    rate,
    fetched_at
FROM exchange_rates
WHERE base_currency = $1
ORDER BY quote_currency, rate_date DESC;

-- name: UpsertExchangeRate :exec
INSERT INTO exchange_rates (
    rate_date, base_currency, quote_currency, rate
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (rate_date, base_currency, quote_currency)
DO UPDATE SET rate = EXCLUDED.rate, fetched_at = now();
