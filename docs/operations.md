# Operations

## Status

Accepted operational guidance for self-hosted deployments.

This document is **public-facing** and should be included in public release snapshots.

## Runtime shape

A deployment is one application container and one PostgreSQL container, optionally behind a
reverse proxy that terminates TLS:

```text
Internet -> Caddy -> Budget -> PostgreSQL
```

The application serves the API and the web interface from the same origin. It applies pending
database migrations at startup and begins listening only after they succeed, so a failed
migration is visible as a container that does not become ready rather than as an application
running against a schema it does not expect.

## Health and readiness

```text
GET /healthz   the process is running
GET /readyz    the process can reach PostgreSQL
```

Use `/healthz` for a liveness check and `/readyz` for a readiness check. `/readyz` answers
`503` while the database is unreachable, which is the correct signal to stop routing traffic
to an instance without restarting it. Neither endpoint requires authentication and neither
discloses anything beyond the status.

## Request protection

Credential endpoints — login, registration, and invitation acceptance — are throttled per
client. Ordinary authenticated reads are not: throttling them would degrade the product
without defending anything, because a signed-in user listing their accounts is not a guessing
attempt.

```text
BUDGET_AUTH_RATE_LIMIT   attempts per minute per client (0 disables)
BUDGET_AUTH_RATE_BURST   attempts allowed back to back
BUDGET_TRUSTED_PROXIES   networks whose X-Forwarded-For is believed
```

**Set `BUDGET_TRUSTED_PROXIES` when running behind a reverse proxy.** Without it every request
appears to come from the proxy, so all clients share one bucket. With it, the client is taken
from the forwarded header. The header is ignored when the immediate peer is not listed,
because believing it from an untrusted peer would let any caller reset their own limit by
inventing an address.

The limiter is in-process. Running several application instances gives a limit per instance,
which is weaker but never wrong.

Request bodies on the API are capped at 64 KiB.

## Backups

PostgreSQL holds everything. The application keeps no state on disk beyond its configuration,
so a database backup is a complete backup.

```bash
# Back up
docker compose exec -T postgres pg_dump -U budget -Fc budget > budget-$(date +%F).dump

# Restore into an empty database
docker compose exec -T postgres pg_restore -U budget -d budget --clean --if-exists \
  < budget-2026-08-18.dump
```

Take a backup **before every upgrade**. Migrations are forward-only: the application never
rolls one back on its own, and reverting to an older image against a newer schema is not
supported.

Verify a backup by restoring it into a scratch database and starting the application against
it. A backup that has never been restored is a hypothesis.

## Upgrading

1. Take a backup.
2. Pull the new image.
3. Start it. Pending migrations apply during startup.
4. Watch `/readyz` until it answers `200`.
5. Open the web interface and confirm that Overview loads for an existing workspace. At compact
   width the primary destinations appear in the bottom navigation; at wider widths they appear
   in the workspace sidebar.

If startup fails, the container exits and the previous schema is untouched only if no
migration committed. Check the logs before retrying: a migration that failed partway is
recorded by Goose, and the safe recovery is to restore the backup rather than to re-run.

## Failure modes

| Symptom | Likely cause | What happens | What to do |
| --- | --- | --- | --- |
| `/readyz` returns 503 | PostgreSQL unreachable | Requests needing data fail; the process stays up | Check the database container and credentials |
| Container exits at startup | Migration failed | No traffic is served against a half-applied schema | Read the logs; restore the backup if a migration partly applied |
| Invitations create but no email arrives | SMTP disabled, wrong credentials, or relay unreachable | Invitation still created; token returned to the inviter | Check the warning in the logs; the inviter can share the code directly |
| Currency conversion missing | Rate fetching disabled or provider unreachable | Amounts render in their own currency | Optional feature; enable or ignore |
| `429` on sign-in | Rate limit reached | Credential attempts refused for a minute | Expected under repeated failures; raise the limit or set trusted proxies if it fires for normal use |
| Everyone shares one rate-limit bucket | Behind a proxy without `BUDGET_TRUSTED_PROXIES` | One user's failures throttle others | Set the trusted proxy networks |

Both optional features — invitation email and exchange rates — are designed so that a failure
degrades the feature rather than the product. Neither can take the application down.

## Logs

Logs are structured JSON on stdout. Every request carries a request identifier, echoed in the
`X-Request-ID` response header and in error bodies, so a user-reported failure can be found
directly.

Credentials are never logged: not passwords, session tokens, invitation tokens, or SMTP
passwords. A delivery failure logs the transport error and the invitation identifier, never
the token.

## Security headers

The application sets a content security policy, `X-Content-Type-Options`, `Referrer-Policy`,
and `Permissions-Policy` on every response. It adds `Strict-Transport-Security` only when
configured for a TLS deployment, because pinning a browser to HTTPS from a plain-HTTP
development server would make the application unreachable there.

The policy is strict because the application loads nothing from a third party: no external
scripts, fonts, images, or API calls from the browser. Exchange rates are fetched by the
server, never the client. Adding a third-party asset would require relaxing it, which is
worth resisting.
