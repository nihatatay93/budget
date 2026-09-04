# Architecture

## Status

Accepted initial architecture.

This document is **public-facing** and should be included in public release snapshots.

## Repository Publication Model

The project uses two repositories with different responsibilities.

### `budget-dev` — private canonical development repository

Contains the real development history and may include feature branches, work-in-progress
commits, experiments, Codex/agent configuration, implementation plans, and internal release
tooling.

### `budget` — public curated release repository

Contains validated source snapshots intended for self-hosting, stable version tags, public
documentation, and source inspection. The public repository is **an output of the development
process**, not the canonical day-to-day development repository.

Public history should stay intentionally simple:

```text
Budget v0.1.0
    |
Budget v0.2.0
    |
Budget v0.2.1
```

A public release commit represents one complete validated application state.

## Product Shape

Budget is a self-hostable personal finance product with three runnable applications:

```text
apps/
├── server/
├── web/
└── ios/
```

The backend is a **modular monolith**. The repository is a monorepo because the backend,
web client, iOS client, API contract, database migrations, documentation, and deployment
configuration are parts of one product and should evolve together.

## Repository Layout

Target development layout:

```text
budget-dev/
├── .github/
│   └── workflows/
│
├── .codex/                 # private, optional
├── AGENTS.md               # private
│
├── apps/
│   ├── server/
│   │   ├── cmd/
│   │   │   └── server/
│   │   │       └── main.go
│   │   ├── internal/
│   │   │   ├── app/
│   │   │   ├── auth/
│   │   │   ├── workspace/
│   │   │   ├── account/
│   │   │   ├── transaction/
│   │   │   ├── category/
│   │   │   ├── budget/
│   │   │   ├── reporting/
│   │   │   ├── analysis/
│   │   │   ├── api/
│   │   │   ├── webui/
│   │   │   ├── platform/
│   │   │   │   └── postgres/
│   │   │   │       └── migrations/
│   │   │   └── config/
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   ├── web/
│   │   ├── src/
│   │   ├── package.json
│   │   └── vite.config.ts
│   │
│   └── ios/
│       ├── BudgetApp/
│       ├── BudgetAppTests/
│       └── BudgetApp.xcodeproj
│
├── api/
│   └── openapi.yaml
│
├── database/
│   ├── queries/
│   └── sqlc.yaml
│
├── docker/
│   └── Dockerfile
│
├── docs/
│   ├── architecture.md
│   ├── domain-model.md
│   ├── tech-stack.md
│   ├── decisions/
│   └── internal/           # private, optional
│
├── plans/                  # private, optional
├── scripts/
├── .env.example
├── .gitattributes
├── docker-compose.yml
├── Makefile
├── CONTRIBUTING.md
├── SECURITY.md
└── README.md
```

The exact layout may evolve, but the boundaries described here should remain recognizable.
Capabilities are created when their behavior is implemented rather than kept as empty
packages. This applies to `recurring`, to a dedicated `user` package should identity
behavior outgrow `auth`, and to web feature and component directories.


## Public Release Snapshot

The public repository is produced from a filtered snapshot of the private repository.
Private development artifacts must be excluded. A recommended `.gitattributes` baseline is:

```gitattributes
AGENTS.md export-ignore
.codex/ export-ignore
docs/internal/ export-ignore
plans/ export-ignore
```

Public CI workflows remain in the exported snapshot. Private-only GitHub workflows must be
named `private-*.yml` or `private-*.yaml` and excluded through those file patterns instead of
applying `export-ignore` to the entire `.github/` tree. Release validation fails if a private
workflow survives the export.

Conceptually:

```text
PRIVATE DEVELOPMENT REPO

A -- B -- C -- D -- E -- F -- G
                         |
                         +-- release source revision
                                  |
                            export + validate
                                  |
                                  v
PUBLIC RELEASE REPO

v0.1.0 -------- v0.2.0 -------- v0.3.0
```

The public repository does not need the internal commit graph. The export mechanism may use
`git archive` plus a controlled synchronization step or equivalent deterministic tooling.

Requirements:

- exported content is reproducible
- private-only files are excluded
- tests and builds run before publication
- each release is traceable to its private source revision
- public release tags correspond to the snapshot commits

## Inspiration

The repository and publication structure is inspired by projects such as Claro that treat:

- runnable applications
- reusable/domain logic
- database assets
- deployment
- documentation

as explicit architectural concerns.

We are copying the principle, not the TypeScript-specific folder structure.

In particular, Go domain code should use idiomatic Go packages rather than recreating a
JavaScript monorepo `packages/core`, `packages/adapters`, etc. layout without need.

## Runtime Architecture

Preferred production topology:

```text
                     Internet
                        |
                        v
                     Caddy
                        |
                        v
             +-----------------------+
             |      Go Server        |
             |                       |
             |  REST API             |
             |  React SPA            |
             |  Background jobs      |
             +-----------+-----------+
                         |
                         v
                   PostgreSQL

SwiftUI iOS
     |
     +------------- HTTPS -----------> Go Server
```

The goal is a deployment experience close to:

```bash
git clone <repository>
cp .env.example .env
docker compose up -d
```

The project must remain deployable on:

- a VPS
- a home server
- a cloud VM
- common container hosts

without requiring one specific cloud provider.

## Backend Architecture

The server is a modular monolith organized primarily by domain capability.

Preferred direction:

```text
apps/server/internal/
├── app/
├── auth/
├── workspace/
├── account/
├── transaction/
├── category/
├── budget/
├── reporting/
├── analysis/
├── api/
├── webui/
├── platform/
└── config/
```

The `app` package is the composition root for explicit dependency assembly. The `webui`
package owns the embedded production SPA. Recurring behavior remains deferred until its
feature work begins.

A domain package may contain files such as:

```text
transaction/
├── model.go
├── service.go
├── repository.go
├── errors.go
└── service_test.go
```

Do not default to a global technical-layer layout such as:

```text
controllers/
services/
repositories/
models/
```

because that scatters one domain capability across the entire codebase.

### Dependency Direction

Desired conceptual flow:

```text
HTTP / external input
        |
        v
application/domain behavior
        |
        v
repository/interfaces
        |
        v
PostgreSQL / infrastructure
```

HTTP concerns should not define business rules.

PostgreSQL implementation details should not leak into core domain behavior where they can
reasonably be isolated.

## Platform / Adapters

Infrastructure concerns belong in explicitly named packages, for example:

```text
internal/platform/
├── postgres/
├── mail/
├── crypto/
└── clock/
```

Examples:

- PostgreSQL repositories
- SMTP delivery
- password hashing
- token generation
- external exchange-rate clients

The domain should depend on small interfaces where this improves testability and
separation.

Avoid interface abstraction merely for abstraction's sake.

The initial HTTP transport uses the Go standard library `net/http` router and middleware of
the form `func(http.Handler) http.Handler`. A third-party router is introduced only if a
concrete routing requirement materially improves on the standard library.

## API Architecture

The public HTTP API is defined through:

```text
api/openapi.yaml
```

The API is workspace-aware.

OpenAPI-generated code is restricted to transport boundaries. The Go server uses generated
strict `net/http` interfaces and wire models, while its domain packages remain independent.
The web and iOS clients similarly keep generated types behind small handwritten API
boundaries. See [ADR 0004](decisions/0004-generate-openapi-transport-boundaries.md).

Every API response carries `X-Request-ID`. Error responses use the shared OpenAPI
`ErrorResponse` envelope with a stable machine-readable code, a safe human-readable message,
the same request ID, and optional structured details. Internal errors are logged with their
request ID without exposing implementation details to clients.

Typical route shape:

```text
/v1/auth/...
/v1/invitations/accept
/v1/workspaces
/v1/workspaces/{workspaceId}/members
/v1/workspaces/{workspaceId}/invitations
/v1/workspaces/{workspaceId}/accounts
/v1/workspaces/{workspaceId}/transactions
/v1/workspaces/{workspaceId}/categories
/v1/workspaces/{workspaceId}/financial-projection
/v1/workspaces/{workspaceId}/spending-analysis
/v1/workspaces/{workspaceId}/budgets
```

Every workspace-scoped endpoint must verify that the authenticated user is an authorized
member of the requested workspace.

Do not rely only on client-side workspace selection.

Account and category setup uses collection and item resources beneath the workspace path.
`GET` requests are available to every workspace member, while mutations require a role other
than `viewer`. `DELETE` on these resources is intentionally archival, not hard deletion;
list operations omit archived records unless `include_archived=true` is requested. Domain
services enforce these policies before calling workspace-scoped repositories, and database
constraints protect structural invariants from non-HTTP write paths.

Transactions are exposed as complete aggregates beneath the workspace path. Create and
replacement writes validate and persist the transaction, entries, and allocations in one
database transaction; item deletion is a recoverable soft delete. Reads are available to all
workspace members and mutations use the same non-viewer management boundary as account and
category setup.

Financial projections are workspace-scoped derived read models owned by the `reporting`
capability. They calculate balances from entries and income, spending, and category totals
from allocations without persisting authoritative dashboard totals. Posted figures remain
separate from pending projections, and all parts of one response observe a consistent
PostgreSQL snapshot. Projection reads are available to every workspace member. See
[ADR 0007](decisions/0007-derive-financial-projections-from-the-ledger.md).

Spending analysis is a second derived read model, owned by the `analysis` capability and
kept separate from `reporting` because it answers a different question. Reporting states one
period's position, including what has not settled yet; analysis describes settled behaviour
over time and compares it with the equal-length window immediately before. Every analysis
figure is therefore posted-only, and one request returns the time series, the category
breakdown, the weekday and daily distributions, the payee ranking, and per-account activity
from a single consistent snapshot so no two panels can disagree. Bucket width is a bounded
enum the transport and the domain both validate, because it becomes a SQL bucket width.
Analysis reads are available to every workspace member. Both the web and iOS clients consume
this one endpoint and apply the same window, ranking, and comparison rules, so a period reads
identically on either. See [ADR 0011](decisions/0011-analyze-posted-spending-over-time.md).

Collaboration administration uses a stricter role boundary than ordinary financial writes.
Every active member may read the member roster. Owners manage all permitted invitation roles
and memberships; administrators manage only member/viewer invitations and memberships.
Invitations are seven-day bearer capabilities and cannot grant ownership. Membership removal
revokes access without deleting historical actor identity, and the final active owner cannot
be demoted, removed, or leave. See
[ADR 0009](decisions/0009-protect-workspace-collaboration-and-ownership.md).

## Web Architecture

The web application is a React SPA used primarily for authenticated personal-finance
workflows.

Possible feature-oriented structure:

```text
apps/web/src/
├── app/
├── components/          # presentation shared across features
├── features/
│   ├── accounts/
│   ├── budgets/
│   ├── categories/
│   ├── dashboard/
│   └── transactions/
├── pages/               # routed screens that compose feature panels
├── api/
└── lib/
```

A feature owns its own queries, mutations, and form state. Pages stay thin: they resolve the
workspace and compose panels. Directories are created when a feature exists, not in advance.

The generated OpenAPI client/types should live behind a small API boundary rather than
being imported arbitrarily throughout the entire UI.

For production, Vite writes generated assets into the server's `internal/webui` area so the
Go binary can embed them without crossing a Go module boundary. Generated production assets
are not source of truth and remain ignored. The build must distinguish a development
fallback from a production asset bundle so a production image cannot silently ship only a
placeholder UI.

## iOS Architecture

The iOS application is native SwiftUI.

It uses the same backend API and the same OpenAPI contract as the web application.

The local `apps/ios/BudgetAPI` Swift package owns Apple's build plugin, OpenAPIRuntime, and
URLSession transport dependencies. `make generate-api` copies the authoritative
`api/openapi.yaml` into that package as a checked input; the plugin generates Swift source
only as build output. The app target imports the package through its handwritten `APIClient`
boundary and does not expose generated wire types to SwiftUI views.

Initial concerns:

```text
iOS
├── authentication/session
├── workspace selection
├── accounts
├── transactions
├── categories
├── budgets
└── dashboard
```

Credentials/session material must be stored in Keychain rather than UserDefaults.
The self-hosted server address may be stored in UserDefaults because it is configuration,
not credential material. Native authentication always requests bearer sessions.

Use a normal checked-in `.xcodeproj` with Xcode synchronized folder groups. Do not add a
project generator unless native project maintenance develops a concrete need for one.

## Database Architecture

PostgreSQL is the source of truth.

The database is intentionally responsible for enforcing important invariants where
practical:

- foreign keys
- uniqueness
- workspace relationships
- valid money/currency fields
- membership constraints
- archival/deletion rules where appropriate

The domain model is described in `docs/domain-model.md`.

SQL queries and `sqlc.yaml` remain in the repository-level `database/` directory. Goose SQL
migrations live under `apps/server/internal/platform/postgres/migrations/` so the Go module
can embed them. The application applies pending migrations before it starts serving
requests, using a PostgreSQL session lock to serialize concurrent application instances.
See [ADR 0001](decisions/0001-embed-and-apply-postgresql-migrations.md).

## Background Work

The initial project does not require a separate worker service or queue infrastructure.

Background work may run in the Go application when appropriate, for example:

- recurring transaction generation
- invitation email delivery
- lightweight maintenance jobs

If reliability requirements later demand a job queue, prefer a PostgreSQL-backed approach
before introducing a separate infrastructure dependency.

## Deployment Principles

### Production

Prefer one application image that contains:

- Go backend
- built React application

and connects to PostgreSQL.

The production build compiles the React SPA before compiling the Go binary that embeds it.
Application startup completes database migrations successfully before the HTTP listener is
made ready.

The multi-stage Dockerfile is the sole producer of the SPA assets embedded in a release
binary. Host/worktree `internal/webui/dist` output is excluded from the Docker build context
so a release image cannot consume stale locally generated assets.

### Development

Development may run:

- Go server
- Vite dev server
- PostgreSQL

as separate processes/containers for faster feedback.

### Configuration

Runtime configuration should be environment based and documented in `.env.example`.

Self-hosters should not need to inspect source code to understand mandatory configuration.

## Security Boundaries

Workspace isolation is a core security boundary.

A request such as:

```text
GET /v1/workspaces/{workspaceId}/transactions/{transactionId}
```

must prove:

1. the caller has an authenticated session
2. the user belongs to `{workspaceId}`
3. the requested transaction belongs to `{workspaceId}`

Queries should be written in a workspace-aware form rather than fetching by global entity
ID and checking ownership casually afterward.

Web sessions use secure cookies and unsafe cookie-authenticated requests must pass CSRF
checks based on an explicitly configured allowed origin, with Fetch Metadata as defense in
depth. Native iOS requests use an explicit `Authorization: Bearer` session transport and do
not rely on browser cookies. Cookie and bearer sessions share the sessions table but retain
their transport kind so policy can distinguish them.
See [ADR 0002](decisions/0002-separate-cookie-and-bearer-session-transports.md).

## Architectural Non-Goals

Initial architecture intentionally avoids:

- microservices
- distributed event buses
- separate caching infrastructure
- service meshes
- Kubernetes
- mandatory cloud-specific services
- premature domain abstractions

The architecture should remain boring, explicit, and maintainable until real scale or
product requirements justify additional complexity.
