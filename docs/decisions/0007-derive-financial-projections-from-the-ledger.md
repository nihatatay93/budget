# ADR 0007: Derive Financial Projections from the Ledger

## Status

Accepted

## Context

The dashboard needs account balances, income, spending, category reporting, and an explicit
view of pending activity. The ledger already stores the authoritative inputs: account effects
in transaction entries and reporting effects in allocations. It does not yet define one
precise interpretation for reporting windows, category signs and hierarchy, pending values,
or multi-currency totals.

Leaving those decisions to individual SQL queries or clients would allow the Go server, web
application, and iOS application to show different totals for the same workspace.

## Decision

Financial projections are derived read models. The MVP does not persist dashboard totals,
category totals, or projected balances as authoritative state.

### Reporting window

A projection query accepts `from_date` and `to_date` as an optional pair of inclusive ISO
calendar dates. Supplying only one date is invalid, and `from_date` must not follow
`to_date`.

When neither date is supplied, the server uses the first day of the current month through
the workspace's current local date. The workspace's stored IANA timezone determines that
current date. Explicit transaction dates are PostgreSQL `DATE` values and are compared
directly; they are not converted through UTC.

The result reports its effective date range, workspace timezone, and workspace base currency
so every client labels the same period.

### Posted and pending values

Soft-deleted transactions never contribute. Posted transactions produce authoritative
figures. Pending transactions use the same calculations but remain separate:

```text
projected = posted + pending
```

`projected` means the arithmetic result if the included pending transactions were posted. It
is not a forecast and does not infer recurring or future transactions.

### Account balances

Account balances are cumulative through `to_date`; `from_date` does not reset them:

```text
posted native balance = SUM(posted entries.amount_minor through to_date)
pending native delta  = SUM(pending entries.amount_minor through to_date)
projected native balance = posted native balance + pending native delta
```

The same calculation over `base_amount_minor` supplies stable workspace-base reporting
values. A workspace base balance is the sum of those base balances. It is historical booked
value, not a current-rate revaluation.

Projection results include archived accounts because archival must not erase balances or
history. Clients may visually separate archived accounts, but may not silently omit them
from authoritative workspace totals.

### Income, spending, and categories

Period activity comes from allocations whose transaction dates fall inside the inclusive
window. Category kind selects the reporting bucket; allocation sign remains independent of
kind:

```text
income   =  SUM(income-category allocation.amount_base_minor)
spending = -SUM(expense-category allocation.amount_base_minor)
```

Thus an expense purchase increases displayed spending, while a positive refund reduces it.
An income reversal reduces displayed income. Neither value is clamped to zero or converted
to an absolute value, so a period may truthfully show net-negative spending or income.

Transfers have no allocations and therefore contribute neither income nor spending.
Unallocated adjustments affect account balances only. An adjustment with allocations affects
category reporting according to those allocations.

Category projections expose both direct values and rolled-up values that include the
category and all descendants. Workspace summaries sum direct values exactly once; they never
sum every rolled-up row. Archived categories and their ancestors remain in results when
needed to explain historical or pending activity.

### Query contract and consistency

The domain-facing projection result contains:

- the effective period, timezone, and base currency
- posted, pending, and projected income and spending in base minor units
- per-account posted balances, pending deltas, and projected balances in both native and
  base minor units
- per-category direct and rolled-up posted, pending, and projected values in base minor units

Category values use reporting orientation: positive expense values mean net spending and
positive income values mean net income. The category kind remains present so clients do not
infer meaning from sign alone.

All parts of one projection response must observe one consistent PostgreSQL snapshot. Every
query is scoped by workspace, and any workspace member may read projections. The transport
shape remains defined by OpenAPI when the projection endpoint is introduced.

Latest display-conversion rates never feed these calculations. A client may optionally show
a converted presentation value afterward under ADR 0005, but the projection's base-currency
minor-unit values remain authoritative.

## Rationale

Entries are the only reliable source for account effects, while allocations are the only
reliable source for income, spending, categories, and later budgets. Keeping posted and
pending columns separate prevents a projected value from being mistaken for settled money.

Inclusive calendar dates match the existing transaction filter language and how users
describe a reporting period. Using the workspace timezone only to resolve an omitted current
period avoids converting date-only financial facts through technical timestamps.

Server-side normalization and hierarchy rollups give web and iOS identical answers and keep
financial arithmetic out of presentation code.

## Consequences

- Reporting queries must filter out soft-deleted transactions and distinguish `posted` from
  `pending` explicitly.
- Account balance queries use all qualifying history through `to_date`, while activity
  queries also apply `from_date`.
- Projection repositories need recursive category rollups and must avoid double-counting
  parent and child rows in workspace summaries.
- The projection service needs the workspace timezone and a clock to resolve the default
  month-to-date window deterministically and testably.
- A multi-query response must run in one consistent read snapshot or use one statement that
  provides equivalent consistency.
- Archived accounts and categories remain visible where needed to explain financial history.
- Materialized totals, current-rate account revaluation, cash-flow forecasting, and recurring
  projections remain deferred.

## Alternatives Considered

### Derive all figures from account entries

This would make transfers look like income and spending and would discard the category intent
captured by allocations.

### Merge pending activity into authoritative totals

This produces simpler fields but makes unsettled activity indistinguishable from posted
financial history.

### Recalculate historical values with today's exchange rate

This would make old reports change over time and violates the historical-booking decision.

### Normalize category values in each client

Web and iOS could negate expense allocations and build hierarchy rollups independently, but
that duplicates financial logic and makes cross-client disagreement likely.

### Persist or cache dashboard totals now

The current scale does not justify invalidation and repair complexity. Derived SQL remains
the source of truth; a future cache must be replaceable and reproducible from the ledger.
