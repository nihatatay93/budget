# Technology Stack

## Status

Accepted initial technology direction.

This document is **public-facing** and should be included in public release snapshots.
Private Codex/agent tooling is development infrastructure, not part of the product stack.

The project prioritizes:

- self-hosting
- low operational complexity
- explicit data modeling
- portability across hosting providers
- native iOS experience
- a clean open-source consumer experience
- reproducible public release snapshots

## Backend

### Language

**Go 1.26**

Go is the preferred backend language for this project.

Reasons:

- simple deployment as a compiled binary
- low runtime overhead
- good fit for a small self-hosted service
- explicit control over dependencies and application structure
- straightforward concurrency model for background tasks
- strong standard library

Java/Spring Boot would also be technically suitable, but Go better matches the project's
goal of a lightweight, easily deployable self-hosted application.

### HTTP Layer

Preferred starting point:

- Go standard library `net/http`, including method/wildcard `ServeMux` patterns
- middleware composed as `func(http.Handler) http.Handler`

Do not add `chi` or a larger web framework unless the project develops a routing requirement
that the standard library does not reasonably satisfy.

### API Contract

**OpenAPI**

The repository should keep the API contract in:

```text
api/openapi.yaml
```

OpenAPI is the shared contract between:

- Go backend
- React web application
- Swift iOS application

Generate strict standard-library server interfaces and transport models with `oapi-codegen`.
Generate TypeScript API types with `openapi-typescript`. Use Apple's Swift OpenAPI Generator
build plugin for the native client once its first API workflow is connected. Generated
representations remain behind application-specific transport/client boundaries and do not
define domain behavior.

Selected tooling:

- Go: `oapi-codegen`
- Swift: Swift OpenAPI Generator
- TypeScript: `openapi-typescript`

### PostgreSQL Access

Preferred stack:

- `pgx`
- `sqlc`

Rationale:

- keep SQL explicit
- retain direct access to PostgreSQL capabilities
- generate type-safe Go code from reviewed SQL
- avoid ORM behavior obscuring financial queries

### Database Migrations

Use **Goose** SQL migrations.

Migration files live in:

```text
apps/server/internal/platform/postgres/migrations/
```

They are embedded in the Go binary and applied before the application begins serving
requests. Concurrent application instances must serialize migration execution with a
PostgreSQL session lock. Runtime rollback is not automatic; rollback remains an explicit
operator action.

`database/sqlc.yaml` reads the embedded migration directory as its schema source while SQL
queries remain under `database/queries/`.

### Exchange Rates

Display currency conversion uses **Frankfurter** (`https://api.frankfurter.dev`), which
serves European Central Bank reference rates without an API key.

Rate fetching is **disabled by default** and enabled through configuration. Responses are
cached in PostgreSQL, and a provider failure must never fail a request that would otherwise
succeed. When the feature is off or unavailable, amounts render in their own currency with no
converted figure.

Supported currencies are `TRY`, `USD`, and `EUR`. See
[ADR 0005](decisions/0005-supported-currencies-and-display-conversion.md).

### Invitation Email

Workspace invitations are delivered over **SMTP**, using the Go standard library rather than a
provider SDK so any relay works, including one the operator runs.

Delivery is **disabled by default** and is best effort: a failure never fails invitation
creation, because the acceptance token is still returned to the inviter to share directly.
The connection is always encrypted, and the token is sent as a code to paste rather than a
link, which would place it in browser history and referrers. See
[ADR 0010](decisions/0010-deliver-invitations-over-smtp.md).

## Database

**PostgreSQL 18**

PostgreSQL is the system of record for:

- users and sessions
- workspace membership
- accounts
- transactions
- category allocations
- budgets
- recurring rules
- future import/reconciliation metadata

PostgreSQL should be the only mandatory infrastructure dependency in the initial product.

Do not add Redis, Kafka, NATS, Elasticsearch, or a separate queue unless a real requirement
justifies the operational cost.

## Web Application

### Core

- React 19
- TypeScript
- Vite
- React Router
- TanStack Query
- npm with a committed `package-lock.json`

The web application is an authenticated dashboard. Server-side rendering is not an
initial requirement, so a React SPA is preferred over a full-stack React framework.

npm is the initial package manager because the repository has one JavaScript application
and does not need a JavaScript workspace tool. Revisit this only if the repository develops
multiple shared JavaScript packages.

### UI

Preferred direction:

- Tailwind CSS
- shadcn/ui

A different component library may be chosen later, but the UI stack should remain
replaceable and should not dictate backend/domain architecture.

### State

Use TanStack Query for server state.

Keep local UI state local. Avoid introducing a global client-state library unless the
application develops state-management needs that React primitives cannot reasonably cover.

## iOS Application

### Core

- Swift 6
- SwiftUI
- Swift Concurrency (`async`/`await`)
- URLSession or generated OpenAPI transport
- Keychain for credentials/session material

The iOS application should remain native rather than using React Native.

### API Client

Prefer generating API types/client code from the same OpenAPI contract used by the web and
backend.

## Authentication

Authentication is application-owned and self-hostable.

Initial preferred direction:

- email/password authentication
- password hashes stored securely
- opaque random session tokens
- only token hashes stored server-side
- web session delivered with `Secure`, `HttpOnly`, `SameSite=Lax` cookies
- unsafe cookie-authenticated requests restricted to explicitly allowed origins, with Fetch
  Metadata checks as defense in depth
- iOS sessions sent explicitly with `Authorization: Bearer` and stored in Keychain
- session records retain a cookie/bearer transport kind even though both use the same
  sessions table

The initial concrete policy is:

- hash passwords with Argon2id using 19 MiB memory, 2 iterations, parallelism 1, a random
  16-byte salt, and a 32-byte key, stored in PHC format
- accept passwords from 15 through 128 Unicode characters without composition rules or
  silent truncation
- generate 256-bit random session tokens, persist only their SHA-256 hashes, and expire
  server-side session records after 30 days by default (`BUDGET_SESSION_TTL`)
- keep the web cookie as a browser-session cookie while enforcing the server-side expiry;
  set `Secure` outside explicit local development, `HttpOnly`, `SameSite=Lax`, `Path=/`, and
  no `Domain` attribute
- use generic login failures and perform password-hash work even when an email is unknown to
  reduce account-enumeration timing differences

Web authentication responses must not expose raw cookie-session tokens to browser
JavaScript. Bearer-authenticated requests do not use the cookie CSRF path. CORS remains
restrictive and public-origin comparison must use trusted configuration rather than an
unvalidated request host. Origin/Referer validation is the fallback when Fetch Metadata is
unavailable.

Do not make Auth0, Firebase, Clerk, Supabase Auth, or another SaaS identity provider a
mandatory dependency.

External identity providers may be added later as optional integrations.

## Deployment

### Containers

- Docker
- Docker Compose

Self-hosting should aim for:

```text
application
postgres
```

as the essential runtime services.

A reverse proxy such as **Caddy** may be included for:

- TLS
- hostname routing
- simple self-hosted deployment

### Web Serving

Preferred production direction:

1. build the React application
2. write generated assets into the server's internal `webui` asset area
3. embed and serve the built SPA from the Go application
4. expose API and web UI from the same application origin

This keeps the production topology simple.

Development may run web/backend as separate processes for fast iteration.

## CI

**GitHub Actions**

The public CI workflow is part of public release snapshots. Private-only workflows are
named `private-*.yml` or `private-*.yaml` and excluded by pattern rather than excluding the
entire `.github/` directory.

Initial CI responsibilities:

- Go formatting/lint checks
- Go tests
- web type checking
- web tests
- web production build
- migration validation where practical
- OpenAPI validation
- Docker image build
- iOS checks when the runner/tooling setup is practical

## Testing

### Backend

- Go standard `testing`
- focused domain/unit tests
- PostgreSQL 18 integration tests using `testcontainers-go`
- explicit tests for financial invariants

Unit tests must remain runnable without Docker. Integration tests are an explicit test
target, require a Docker-compatible runtime, and reuse containers at a sensible suite or
package boundary rather than starting one container for every test. CI and release
validation run both unit and integration targets.

`make release-check` is the authoritative release-validation entry point. It includes unit
and integration tests, generated-source/API validation, and the production Docker build.
The Docker build, not a prior host-side Vite build, produces the SPA embedded in the release
binary.

### Web

- Vitest
- React Testing Library where appropriate

### iOS

- Swift Testing and/or XCTest
- checked-in `.xcodeproj` using Xcode synchronized folder groups

`make ios-check` runs the `BudgetAPI` package tests through the selected Xcode toolchain and
then builds the application for a generic iOS Simulator destination.


## Development and Publication Model

Canonical development happens in a private repository:

```text
budget-dev
```

Stable source snapshots are exported to the public repository:

```text
budget
```

The public repository contains the deployable product source and public documentation, but
not private agent instructions, internal plans, experiments, or work-in-progress history.

The technology stack is intentionally independent from this publication workflow: a public
release must build and run without any private development tooling. Each public release
should be reproducible from a specific private source revision.

## Explicit Non-Goals for the Initial Stack

The initial architecture does not require:

- microservices
- Kubernetes
- Redis
- Kafka
- NATS
- Elasticsearch/OpenSearch
- event sourcing
- CQRS infrastructure
- cloud-provider-specific managed services
- mandatory external authentication SaaS

These are not prohibited forever; they require a concrete need before adoption.
