# Contributing to Budget

Budget is in early development. Please discuss material domain or architecture changes
before implementing them and read the source-of-truth documents under `docs/` first.

## Prerequisites

- Go 1.26
- Node.js 24 with npm
- Docker with Docker Compose
- PostgreSQL 18, normally through Docker
- Xcode 16 or newer for iOS development

## Setup

```bash
cp .env.example .env
docker compose up -d postgres
make generate
make check
```

Run the two applications separately while developing, so each reloads on its own:

```bash
# Terminal 1: API on :8080, serving a placeholder UI
cd apps/server && go run ./cmd/server

# Terminal 2: web application on :5173, proxying the API
cd apps/web && npm run dev
```

`.env.example` sets `BUDGET_PUBLIC_ORIGIN=http://localhost:5173` for exactly this split. The
server embeds a development placeholder rather than the built application; a production image
embeds the real build, and a build tag keeps the two from being confused.

For iOS, open `apps/ios/Budget.xcodeproj`. The generated API package builds from
`api/openapi.yaml` through a SwiftPM plugin, so a contract change reaches the app on the next
build.

## Changing the API contract

`api/openapi.yaml` is the source of truth for all three clients. After editing it:

```bash
make generate      # regenerates Go, TypeScript, and the iOS package input
make check
```

Generated code is committed, and `make generate-check` fails if it is stale.

## Validation

```bash
make test
make test-integration
make check
make ios-check
make release-check
```

`make test` does not require Docker. Integration and release targets do. `make ios-check`
uses `/Applications/Xcode.app` by default; set `DEVELOPER_DIR` when selecting another Xcode
installation for a single command.

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| `make test-integration` fails to start a container | Docker is not running. Unit tests via `make test` need no Docker. |
| `make ios-check` cannot find a simulator | Set `IOS_TEST_DESTINATION` to a simulator `xcrun simctl list devices` reports. |
| Writes return 403 in the browser | `BUDGET_PUBLIC_ORIGIN` does not match the origin you are using. |
| `make generate-check` fails | Generated code is stale; run `make generate` and commit the result. |
| `npm ci` fails with `EACCES` on `~/.npm/_cacache` | npm was once run under `sudo`, leaving root-owned files in the cache. Fix with `sudo chown -R "$(id -un)" ~/.npm`. This blocks `make check`, `make release-check`, and the release export, though not CI, which starts from a clean runner. |

## Expectations

Changes to persistence require a forward-safe Goose migration. Changes to the external API
require an OpenAPI update. Domain changes require tests for the affected financial
invariants and, where appropriate, an ADR.
