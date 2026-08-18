# ADR 0005: Supported Currencies and Display Conversion

## Status

Accepted

## Context

Account currency and workspace base currency were accepted as any string matching
`^[A-Z]{3}$`. That pattern accepts codes that do not exist, so a typo produces a stored
currency that no exchange-rate provider or formatter can interpret, and nothing in the
system can later repair it because the value is embedded in financial history.

Separately, users want to read a workspace figure in another currency: a 50,000 TRY monthly
budget shown as its US dollar equivalent. `docs/domain-model.md` deferred the exchange-rate
model and provider, and invariant 12 requires that historical reporting must not change
merely because today's rate changed.

Those two needs use exchange rates for different purposes and must not share a mechanism.

## Decision

### Supported currency set

The product supports exactly:

```text
TRY
USD
EUR
```

The set is enforced in the OpenAPI contract as an enum, in Go through the `money` package,
and in PostgreSQL through `CHECK` constraints on `workspaces.base_currency` and
`accounts.currency`. Adding a currency is a deliberate change: one enum entry and one
migration.

All three currencies use two minor-unit decimal places, so the currency-exponent metadata
utility contemplated in `docs/domain-model.md` remains deferred. Adding a currency with a
different exponent requires that utility first.

Workspace base currency remains immutable after creation.

### Display conversion

A display conversion renders an amount that is already final in the workspace base currency
into another supported currency, for viewing only.

```text
stored:    50000.00 TRY   (authoritative)
displayed:  1044.00 USD   (derived, rate of 2026-08-17)
```

Rules:

- display conversion never writes to the ledger and is never persisted on a financial row
- the rate date is shown wherever a converted figure appears
- conversion runs on integer minor units through an exact rational rate with a single
  rounding step; no floating-point arithmetic participates
- a converted figure is never used as an input to a further calculation

### Historical booking rate

When a transaction affects an account whose currency differs from the workspace base
currency, `transaction_entries.base_amount_minor` must be computed at the transaction date's
rate and stored permanently, exactly as `docs/domain-model.md` already requires. This is a
separate mechanism from display conversion and is deferred until transaction write behaviour
is implemented.

Display conversion must never be used to derive `base_amount_minor`.

### Rate provider

Rates come from Frankfurter (`https://api.frankfurter.dev`), which publishes European
Central Bank reference rates, requires no API key, and offers both latest and historical
endpoints. The historical endpoint is what the deferred booking-rate work will use.

The adapter deliberately uses Frankfurter's frozen `/v1` ECB contract. Its `/v2` contract
blends multiple providers and therefore has different accounting semantics; moving to it is
a future decision, not an incidental endpoint upgrade.

Provider behaviour that the implementation must respect:

- rates are published once per working day, so a response on a weekend or holiday carries an
  earlier date; the response `date` field is the rate's authority, not the request time
- responses are cached in a PostgreSQL `exchange_rates` table keyed by rate date, base, and
  quote, so normal operation does not depend on provider availability

### Self-hosting posture

Outbound rate fetching is disabled by default and enabled through configuration. When it is
disabled, unreachable, or failing, the application behaves exactly as it does without the
feature: amounts render in their own currency and no converted figure appears. A rate
failure must never fail a request that would otherwise succeed.

## Rationale

Constraining the currency set removes a class of unrepairable data errors while the project
has no production data, and keeps the two-decimal assumption honest rather than accidental.

Separating display conversion from historical booking protects invariant 12. The failure
mode being avoided is concrete: if a stored `base_amount_minor` were ever recomputed at
today's rate, last year's spending reports would change every day.

Frankfurter needs no account, no key, and no commercial relationship, so it does not
compromise self-hostability. Defaulting it off keeps a self-hosted finance application from
making outbound requests that its operator did not ask for.

## Consequences

- Adding a currency requires a migration, an enum change, and client regeneration.
- The first narrowing migration leaves its checks `NOT VALID` if legacy rows use another
  currency. Those rows remain stored and are not rewritten; an operator must normalize them
  before the checks can be validated and current clients can use those records.
- A currency whose minor unit is not two decimal places cannot be added until currency
  exponent metadata exists.
- The application gains an optional outbound network dependency, which must be documented
  for self-hosters and must remain optional.
- Cached rates make the display feature resilient to provider downtime but mean a displayed
  rate can be older than today; the rate date must always be visible.
- Converted figures are presentation output only. Any future feature that needs a converted
  value as accounting input must use the historical booking mechanism instead.

## Alternatives Considered

### Keep free-text ISO 4217 codes with a validation library

A full ISO 4217 list avoids typos but reintroduces the currency-exponent problem across
currencies the product cannot yet format correctly, and implies multi-currency support that
the transaction model does not yet provide.

### Convert in the browser

Calling the provider directly from the web client removes backend work but duplicates the
logic in the iOS client, exposes each user's browser to a third party, provides no shared
cache, and produces nothing reusable for the historical booking rate.

### Store a converted amount alongside each figure

Persisting display conversions would make reporting trivially fast but would either freeze a
rate that users expect to be current or require rewriting stored financial rows as rates
move, which is precisely what invariant 12 forbids.

### Enable rate fetching by default

A better first-run experience, but it makes a self-hosted finance application contact an
external service before its operator has chosen to allow it.
