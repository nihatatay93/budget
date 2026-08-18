# ADR 0003: Reconcile Transaction Entries and Allocations

## Status

Accepted

## Context

Transaction entries describe account effects, while allocations describe category and
budget effects. The domain model previously required split allocations to reconcile but
deferred the exact rule to the schema.

A single rule allowing allocations to be either empty or equal to entry base amounts would
leave two reporting gaps:

- an empty standard transaction allocation would disappear from allocation-based spending
  and income totals
- a transfer could contain offsetting positive and negative allocations that sum to zero
  while corrupting category reports

The invariant crosses transaction, entry, allocation, and category rows, so a simple SQL
`CHECK` constraint cannot enforce it completely.

## Decision

Apply reconciliation rules by transaction kind.

For `transfer` transactions:

```text
allocations are empty
SUM(entries.base_amount_minor) = 0
```

For `standard` transactions:

```text
one or more allocations are required
SUM(allocations.amount_base_minor) = SUM(entries.base_amount_minor)
```

Each workspace has protected system Uncategorized expense and income categories. A standard
transaction for which the user chooses no category receives a normal allocation to the
appropriate protected category. It never uses an empty allocation set.

For `adjustment` transactions:

```text
allocations may be empty
when present:
SUM(allocations.amount_base_minor) = SUM(entries.base_amount_minor)
```

Domain behavior validates the complete aggregate before persistence. PostgreSQL independently
enforces the committed cross-table state through deferred constraint triggers on entries,
allocations, and relevant transaction-kind changes.

These reconciliation rules apply regardless of whether transaction status is `pending` or
`posted`. Pending is not a partially valid draft state. Authoritative balances, budgets,
income, spending, and category reports filter to `posted`; pending projections may be shown
separately.

Category kind does not constrain allocation sign. Positive refunds may allocate to expense
categories, and negative reversals may allocate to income categories.

## Rationale

These rules make account, category, budget, and dashboard totals explainable from the same
transaction aggregate. Transfers cannot leak into income or spending. Uncategorized activity
remains visible without creating a separate reporting path that bypasses allocations.

## Consequences

- Workspace creation must create protected Uncategorized expense and income categories.
- Protected categories need stable system identity and cannot be archived or repurposed.
- Aggregate writes must occur in a database transaction so deferred validation sees the
  complete entry/allocation set.
- Direct SQL writes that leave an invalid aggregate fail no later than transaction commit.
- Deferred row-level triggers may run more than once for a multi-row aggregate; integration
  tests must cover inserts, updates, deletes, and reordered writes.
- Transfer fees or other reportable external effects must be modeled explicitly rather than
  hidden inside a net-zero transfer.
- Domain and PostgreSQL integration tests must explicitly cover positive refunds allocated
  to expense categories and negative reversals allocated to income categories.

## Alternatives Considered

### Permit empty standard allocations

This makes uncategorized entry easy but causes allocation-based total spending and income to
omit activity unless reporting introduces a second derivation path.

### Allow transfer allocations that sum to zero

This satisfies arithmetic reconciliation while still allowing offsetting category effects
that incorrectly influence category and budget reports.

### Enforce reconciliation only in Go

Application validation is necessary for good errors, but it does not protect against direct
SQL writes, maintenance scripts, or future persistence paths.

### Enforce reconciliation only in PostgreSQL

Database enforcement protects stored state but would produce late commit-time errors without
domain-level validation and would make business behavior harder to test independently.
