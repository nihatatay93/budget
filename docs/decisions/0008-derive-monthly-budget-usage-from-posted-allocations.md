# ADR 0008: Derive Monthly Budget Usage from Posted Allocations

## Status

Accepted

## Context

The initial schema reserved budget and budget-item tables, but it did not settle how a month
is identified, which categories can be planned, how refunds and category descendants affect
usage, or whether unused amounts carry forward. Those choices must be identical in the Go
domain, PostgreSQL queries, web application, and iOS application.

Budgets are plans, while transaction allocations remain the authoritative record of actual
income and spending. Persisting mutable usage counters would duplicate that ledger truth and
create repair and invalidation problems.

## Decision

### Monthly identity and boundaries

The MVP supports one budget per workspace calendar month. A month is represented externally
as `YYYY-MM` and stored as its first Gregorian calendar date in `starts_on`. The pair
`(workspace_id, starts_on)` is unique, and `starts_on` must be the first day of its month.

The budget window is the complete half-open month:

```text
[starts_on, starts_on + 1 calendar month)
```

The workspace IANA timezone determines the current month when a client asks for the default
month. Transaction dates are business `DATE` values, so usage compares them directly and
does not convert them through UTC. A result repeats the month, timezone, and base currency.

Each budget may have a user-facing name. It has no independent archive or active lifecycle in
the MVP: a month either has a budget or it does not.

### Planned amounts and category eligibility

Each budget item assigns a strictly positive `amount_base_minor` to one expense category.
The amount is in the workspace base currency and is not converted from an account currency.
Income categories cannot be budget items in the initial spending-budget model.

A budget may target a parent or leaf expense category, but it cannot contain both an ancestor
and any of that ancestor's descendants. This prevents planned totals and usage from counting
the same category branch more than once. Category reparenting that would introduce such an
overlap is rejected.

### Allocation-derived usage

Usage is derived at read time from transaction allocations. A budget item's usage includes
allocations to its category and every descendant during the budget month, subject to:

```text
transaction.status = posted
transaction.deleted_at IS NULL
transaction_date inside the budget month
```

The expense reporting orientation from ADR 0007 is reused:

```text
used_base_minor      = -SUM(signed expense allocations)
remaining_base_minor = planned_base_minor - used_base_minor
```

Therefore purchases increase usage, while positive refunds reduce it. Usage and remaining
amounts are not clamped: a net refund may produce negative usage and remaining may exceed the
plan; overspending may produce a negative remaining amount. Pending transactions do not
affect budget usage. Transfers have no allocations and therefore never affect budgets.

Budget totals sum item plans and item usage exactly once. The no-overlapping-branches rule
makes that sum unambiguous.

### Archived categories

Archiving a category does not erase an existing budget item or its historical usage. Existing
items remain readable and may be retained, edited, or removed. An archived category cannot be
newly added to a budget. Descendants and historical allocations remain part of a retained
item's rollup even when a category is archived.

### Rollover

Rollover is not supported in the MVP. Every month starts with only that month's explicit
planned amounts; unused or overspent amounts do not alter another month. The existing schema
flag is fixed to `false` and omitted from the external contract. Adding rollover later
requires a separate decision covering carry direction, refunds, edits to prior months, and
recomputation.

### Write contract and authorization

The monthly budget and its complete item set are replaced atomically. Reads are available to
every workspace member. Owners, administrators, and members may create or replace a budget;
viewers are read-only. Every operation verifies membership against the workspace in its path.

## Rationale

A calendar-month identity matches the product roadmap and avoids overlapping active-budget
rules. Full replacement is a small, deterministic aggregate write and prevents partially
updated plans. Reusing signed posted allocations gives budgets exactly the same refund and
category meaning as financial reporting.

Restricting items to non-overlapping expense branches makes both individual progress and the
workspace total understandable. Deferring rollover avoids turning one monthly read into a
dependency chain across all prior budgets before the product has validated that behavior.

## Consequences

- Budget usage is always reproducible from the ledger and is never stored as mutable truth.
- Persistence must enforce unique first-of-month budgets, positive base-currency item amounts,
  expense-category eligibility, workspace isolation, and non-overlapping category branches.
- The service must distinguish an archived category already referenced by a budget from a new
  archived-category assignment.
- Posted future-dated transactions inside the selected month contribute; pending transactions
  do not.
- Clients must display negative remaining amounts, negative net usage, and refund-expanded
  remaining amounts without silently clamping them.
- Rollover controls and cross-month carry queries are intentionally absent.

## Alternatives Considered

### Store mutable spent amounts

This would require every transaction edit, deletion, status change, and category reparent to
repair budget counters. Allocation-derived reads remain simpler and auditable.

### Allow overlapping parent and child items

Summing their rollups would double-count usage, while selecting an implicit winner would make
totals surprising. The MVP rejects overlap instead.

### Include pending allocations in usage

Pending activity is useful as a projection but is not authoritative spending. Phase 6 usage
remains posted-only; a future explicit pending-budget projection can be added separately.

### Enable rollover immediately

Carry rules introduce dependencies on previous months and ambiguous behavior after refunds or
historical edits. The MVP validates monthly planning before accepting that complexity.
