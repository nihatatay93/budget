# ADR 0006: Book Historical Base Amounts

## Status

Accepted

## Context

Every transaction entry stores both the amount in its account currency and the permanent
historical amount in the workspace base currency. Display conversion cannot supply that
value because its latest rate would make old reports change over time. At the same time,
outbound exchange-rate access is optional for self-hosters and must not be required for
ordinary same-currency bookkeeping.

## Decision

Transaction writes book `base_amount_minor` once as part of the aggregate write:

- when the account and workspace currencies match, the server derives
  `base_amount_minor = amount_minor` and rejects a conflicting supplied value
- for a foreign-currency account, a caller may provide an explicit historical
  `base_amount_minor`; a non-zero explicit amount must have the same sign as `amount_minor`
- when that explicit value is absent and rate fetching is enabled, the server resolves the
  account-to-workspace rate for `transaction_date`, converts with exact decimal arithmetic,
  and stores the rounded minor-unit result
- when neither an explicit value nor a historical provider is available, the write fails
  without persisting any part of the aggregate

The provider's returned publication date may precede the transaction date on a weekend or
holiday but must never follow it. Latest display rates are never consulted for booking.

Updating an aggregate replaces its entries and allocations atomically and therefore books
the replacement values supplied or resolved by that update. Previously stored base amounts
are never recomputed merely because rates later change.

## Rationale

The explicit-value path preserves offline and privacy-focused self-hosting. The provider path
offers a convenient default without moving provider calls into web or iOS clients. Deriving
same-currency values server-side removes a redundant input and prevents disagreement between
native amounts and base amounts.

## Consequences

- Foreign-currency writes without an explicit base amount depend on the optional historical
  provider and may report that booking rates are unavailable.
- Clients must display and edit the stored base amount when a manual foreign-currency value
  is used.
- The transaction repository persists the complete aggregate in one database transaction.
- A future richer audit trail may record the selected rate and provider metadata, but the
  booked integer base amount remains authoritative.

## Alternatives Considered

### Require the provider for every foreign-currency write

Convenient, but it would make core bookkeeping unavailable during provider downtime and
would conflict with the project's optional-outbound-network posture.

### Require callers to calculate every base amount

Fully offline, but it duplicates rate retrieval and exact rounding in every client and makes
same-currency writes needlessly error-prone.

### Recalculate base amounts during reporting

Rejected because historical reports would change as exchange rates move.
