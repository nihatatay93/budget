# Budget

Budget is an open-source, self-hostable personal finance application.

The product combines:

- a Go modular-monolith backend
- a React and TypeScript web application
- a native SwiftUI iOS application
- PostgreSQL as the only mandatory infrastructure dependency

The project is pre-alpha. Its accepted architecture and financial model are documented in:

- [Architecture](docs/architecture.md)
- [Technology stack](docs/tech-stack.md)
- [Domain model](docs/domain-model.md)
- [Architecture decisions](docs/decisions/README.md)
- [Configuration reference](docs/configuration.md)
- [Operations](docs/operations.md)

## Development

Canonical development occurs in a private repository. Stable, validated source snapshots
are published to the public `budget` repository.

The intended local workflow is:

```bash
cp .env.example .env
docker compose up -d postgres
make check
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contributor setup and validation commands.

## Self-hosting

One application container and PostgreSQL. The image contains the Go API and the compiled
React application, served from the same origin.

```bash
git clone https://github.com/nihatatay93/budget.git
cd budget
cp .env.example .env
docker compose up -d
```

Then open <http://localhost:8080> and register. The first account creates its own workspace.

Before exposing it beyond your machine, set these in `.env`:

- `POSTGRES_PASSWORD` — the default is a placeholder.
- `BUDGET_PUBLIC_ORIGIN` — the exact origin browsers will use, including scheme and port.
  Writes are refused from any other origin, so a wrong value looks like sign-in working and
  every save failing.
- `BUDGET_SESSION_COOKIE_SECURE=true` once TLS terminates in front.
- `BUDGET_TRUSTED_PROXIES` if a reverse proxy sits in front, or all clients share one
  rate-limit bucket.

`BUDGET_HTTP_PORT` changes the published port when 8080 is taken.

For TLS, `docker compose --profile proxy up -d` adds Caddy on ports 80 and 443; set
`BUDGET_HOSTNAME` to your domain.

Two features are optional and off by default, and the application runs fully without either:
invitation email over SMTP, and display currency conversion. Enabling neither means no
outbound network connections at all.

- [Configuration reference](docs/configuration.md) — every setting
- [Operations](docs/operations.md) — health checks, backups, upgrades, failure modes

## License

Budget is available under the [MIT License](LICENSE).
