# Architecture Decision Records

This directory is **public-facing** and should be included in public release snapshots.

This directory records architectural and domain decisions that are important enough that a
future contributor or coding agent might otherwise "simplify" or replace them without
understanding the reason behind them.

## Why ADRs

The project is expected to be developed with both human contributors and coding agents.

Important decisions should therefore live in the repository rather than only in:

- chat history
- pull-request discussion
- contributor memory
- coding-agent session history

An ADR makes the repository itself the source of truth.

Because day-to-day development happens privately and public Git history is intentionally
curated, ADRs are especially important: accepted reasoning should remain visible even when
intermediate implementation commits are not.

## Repository Publication Decision

The project uses:

```text
budget-dev   PRIVATE canonical development repository
budget       PUBLIC curated release repository
```

The public repository contains stable release snapshots. It should not expose private agent
instructions, Codex configuration, implementation plans, abandoned experiments, or
work-in-progress history. Public architecture/domain documentation remains part of the
release because it helps users understand and maintain the software.

## Format

Use sequential names:

```text
0001-embed-and-apply-postgresql-migrations.md
0002-separate-cookie-and-bearer-session-transports.md
0003-reconcile-transaction-entries-and-allocations.md
...
```

Suggested template:

```md
# ADR NNNN: Title

## Status

Proposed | Accepted | Superseded | Rejected

## Context

What problem or constraint caused this decision?

## Decision

What did we choose?

## Rationale

Why did we choose it?

## Consequences

What becomes easier, harder, or intentionally constrained?

## Alternatives Considered

What serious alternatives were evaluated?
```

Keep ADRs concise. They are not implementation plans.

## Recorded ADRs

- [ADR 0001: Embed and Apply PostgreSQL Migrations](0001-embed-and-apply-postgresql-migrations.md)
- [ADR 0002: Separate Cookie and Bearer Session
  Transports](0002-separate-cookie-and-bearer-session-transports.md)
- [ADR 0003: Reconcile Transaction Entries and
  Allocations](0003-reconcile-transaction-entries-and-allocations.md)
- [ADR 0004: Generate OpenAPI Transport
  Boundaries](0004-generate-openapi-transport-boundaries.md)
- [ADR 0005: Supported Currencies and Display
  Conversion](0005-supported-currencies-and-display-conversion.md)
- [ADR 0006: Book Historical Base
  Amounts](0006-book-historical-base-amounts.md)
- [ADR 0007: Derive Financial Projections from the
  Ledger](0007-derive-financial-projections-from-the-ledger.md)
- [ADR 0008: Derive Monthly Budget Usage from Posted
  Allocations](0008-derive-monthly-budget-usage-from-posted-allocations.md)
- [ADR 0009: Protect Workspace Collaboration and
  Ownership](0009-protect-workspace-collaboration-and-ownership.md)
- [ADR 0010: Deliver Invitations Over SMTP](0010-deliver-invitations-over-smtp.md)
- [ADR 0011: Analyze Posted Spending Over
  Time](0011-analyze-posted-spending-over-time.md)

## Initial Accepted Decisions

### 0. Private Development + Public Release Mirror

Canonical development happens in `budget-dev`, a private repository. Stable validated source
snapshots are exported to `budget`, the public repository. Public history should contain one
clean release commit per published version where practical.

Consequences:

- internal development history and agent configuration remain private
- public history stays easy to inspect
- releases require a controlled export process
- a public release must be traceable to one private source revision
- changing this publication model should require a dedicated ADR

The following decisions are already accepted in the current design and are documented in
the project architecture/domain files. They can be promoted into individual ADR files as
the repository is scaffolded. New material decisions are recorded directly as individual
ADRs above. The numeric labels in this summary are not ADR identifiers.

### 1. Go Backend

Use Go for the backend instead of Java/Spring Boot.

Primary reason:

- lightweight self-hosting and deployment
- explicit architecture
- small runtime footprint
- strong fit for a modular monolith

Java remains technically viable but is not the current project direction.

### 2. PostgreSQL as the Primary Datastore

PostgreSQL is the system of record and the only mandatory infrastructure service beyond the
application itself.

Do not add extra infrastructure until a concrete requirement justifies it.

### 3. Explicit SQL with pgx + sqlc

Use explicit PostgreSQL queries and generated type-safe Go bindings rather than adopting a
general-purpose ORM as the default persistence model.

### 4. Monorepo with Runnable Applications under `apps/`

Use:

```text
apps/server
apps/web
apps/ios
```

with shared repository-level API/database/docs/deployment assets.

This is inspired by the structural clarity of projects such as Claro while remaining
idiomatic for Go, React, and Swift.

### 5. Modular Monolith

The backend starts as one deployable Go application with domain-oriented internal packages.

Do not split services by domain unless future scale/organizational requirements make the
trade-off worthwhile.

### 6. Workspace-Centered Ownership

Financial data belongs to a workspace.

Users access workspace data through membership.

This supports:

- personal workspaces
- family/shared workspaces
- multiple workspaces per user

without later migrating a user-owned financial schema.

### 7. Transaction / Entry / Allocation Model

A transaction represents an economic event.

Transaction entries represent account effects.

Transaction allocations represent category/budget effects.

This decision exists to correctly model:

- transfers
- credit cards
- credit-card payments
- split expenses
- refunds
- multi-currency
- opening balance adjustments

while keeping the product simpler than a formal double-entry accounting application.

### 8. Balances Are Derived

`accounts.balance` is not authoritative state.

Account balances are derived from posted transaction entries.

Snapshots/caches may be added later for performance but are not the system of record.

### 9. Money Uses Integer Minor Units

Never use floating-point values for stored money.

Use integer minor units and explicit currency metadata.

### 10. OpenAPI Is the Shared API Contract

The backend, React web application, and SwiftUI iOS application share one OpenAPI contract.

### 11. Native iOS

The iOS application is written in Swift/SwiftUI rather than React Native.

### 12. Self-Hosting Is a Product Requirement

The project must remain practical to deploy using Docker Compose without mandatory
cloud-provider services.

Preferred essential runtime:

```text
application
postgres
```

A reverse proxy such as Caddy can provide TLS/routing.

### 13. Avoid Premature Infrastructure

The initial system intentionally does not require:

```text
Redis
Kafka
NATS
Elasticsearch
Kubernetes
microservices
```

Adding one of these requires a documented requirement and should normally produce a new ADR.

## When to Add an ADR

Create an ADR when a change would materially affect:

- domain semantics
- security boundaries
- database ownership/invariants
- persistence approach
- API contract strategy
- deployment topology
- framework choices
- mandatory infrastructure
- authentication architecture
- multi-currency/accounting behavior

Routine implementation choices do not need ADRs.

Changes to the private/public repository model, export mechanism, public contribution policy,
or release-history strategy should also be captured in an ADR.

## Superseding a Decision

Do not rewrite historical ADRs to pretend an old decision was never made.

Instead:

1. mark the old ADR as `Superseded`
2. create a new ADR
3. link the two
4. explain why the context changed
