# ADR 0001: Embed and Apply PostgreSQL Migrations

## Status

Accepted

## Context

Budget should remain simple to self-host and should start through `docker compose up`
without requiring an operator to install a separate migration CLI or keep migration files
beside the application binary.

The Go server module lives at `apps/server`. Go embed patterns are relative to the package
containing the directive, cannot contain `..`, and cannot embed files outside the module.
Keeping canonical migrations at repository-level `database/migrations` would therefore
prevent the server from embedding them directly.

Applying migrations during application startup also creates a concurrency concern when
more than one application instance starts against the same database.

## Decision

Use Goose SQL migrations stored at:

```text
apps/server/internal/platform/postgres/migrations/
```

The PostgreSQL adapter embeds these migrations in the Go binary. `database/sqlc.yaml` uses
the migration directory as its schema source, while application queries remain under
`database/queries/`.

On startup, the application:

1. connects to PostgreSQL
2. acquires a PostgreSQL-backed Goose session lock
3. applies all pending upward migrations
4. fails startup if migration application fails
5. begins serving requests only after migration success

Automatic rollback is not part of application startup. Rollback is an explicit operator
action.

## Rationale

This produces a self-contained application binary, supports the intended Docker Compose
experience, keeps migration execution coupled to the application version, and avoids a
mandatory runtime CLI or sidecar. A PostgreSQL session lock prevents concurrent instances
from racing migration execution without introducing additional infrastructure.

## Consequences

- The documented repository responsibility for migrations moves from root `database/` to
  the server PostgreSQL adapter.
- The application needs migration privileges during startup unless a future deployment mode
  separates migration and runtime credentials.
- A migration failure makes the application unavailable rather than serving against an
  incompatible schema.
- Forward migrations require particular care because application startup applies them
  automatically.
- sqlc configuration crosses directories to read the canonical migration schema.

## Alternatives Considered

### Keep migrations under `database/migrations` and load them from disk

This avoids moving the directory but requires migration files to be deployed beside the
binary and configured through a path. It weakens the single-binary deployment property.

### Run a separate Goose CLI or migration container

This provides explicit operational separation but adds a required deployment step and makes
the initial self-hosting experience more complex.

### Copy migrations into the Go module during build

This preserves the old repository layout but creates two locations or a generated-copy step
whose synchronization must be verified. Keeping one canonical embedded source is simpler.
