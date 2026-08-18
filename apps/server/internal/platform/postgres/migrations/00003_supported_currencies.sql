-- +goose Up

-- Narrow currency columns from "any three uppercase letters" to the supported set. The old
-- pattern accepted codes that do not exist, and a bad code cannot be repaired later because
-- it is embedded in financial history. NOT VALID preserves any pre-existing rows outside the
-- new set during an upgrade while still enforcing the narrower rule for new and changed rows.
-- See docs/decisions/0005-supported-currencies-and-display-conversion.md.

ALTER TABLE workspaces
    DROP CONSTRAINT workspaces_base_currency_check,
    ADD CONSTRAINT workspaces_base_currency_check
        CHECK (base_currency IN ('TRY', 'USD', 'EUR')) NOT VALID;

ALTER TABLE accounts
    DROP CONSTRAINT accounts_currency_check,
    ADD CONSTRAINT accounts_currency_check
        CHECK (currency IN ('TRY', 'USD', 'EUR')) NOT VALID;

-- Cached exchange rates for display conversion only. Rates are never written to a financial
-- row: transaction_entries.base_amount_minor is booked at the transaction date's rate by a
-- separate mechanism that does not read this table.
--
-- rate_date is the provider's publication date, not the fetch time. European Central Bank
-- rates are published once per working day, so a row fetched on a Sunday carries Friday's
-- date, and clients display that date alongside any converted figure.
CREATE TABLE exchange_rates (
    rate_date DATE NOT NULL,
    base_currency TEXT NOT NULL CHECK (base_currency IN ('TRY', 'USD', 'EUR')),
    quote_currency TEXT NOT NULL CHECK (quote_currency IN ('TRY', 'USD', 'EUR')),
    rate NUMERIC NOT NULL CHECK (rate > 0),
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (rate_date, base_currency, quote_currency),
    CHECK (base_currency <> quote_currency)
);

CREATE INDEX exchange_rates_lookup_idx
    ON exchange_rates (base_currency, quote_currency, rate_date DESC);

-- +goose Down

DROP TABLE exchange_rates;

ALTER TABLE accounts
    DROP CONSTRAINT accounts_currency_check,
    ADD CONSTRAINT accounts_currency_check
        CHECK (currency ~ '^[A-Z]{3}$');

ALTER TABLE workspaces
    DROP CONSTRAINT workspaces_base_currency_check,
    ADD CONSTRAINT workspaces_base_currency_check
        CHECK (base_currency ~ '^[A-Z]{3}$');
