# Configuration Reference

## Status

Accepted configuration surface.

This document is **public-facing** and should be included in public release snapshots.

Budget is configured entirely through environment variables. `.env.example` is the template;
copy it to `.env` and edit. A self-hoster should not have to read source code to understand a
setting, so every variable the application reads is listed here.

## Required

| Variable | Description |
| --- | --- |
| `BUDGET_DATABASE_URL` | PostgreSQL connection string. The only setting with no default. |

Docker Compose builds this from `POSTGRES_USER`, `POSTGRES_PASSWORD`, and `POSTGRES_DB`, so a
Compose deployment sets those instead.

## Application

| Variable | Default | Description |
| --- | --- | --- |
| `BUDGET_ENV` | `development` | Free-form environment label used in logs. |
| `BUDGET_HTTP_ADDR` | `:8080` | Address the server listens on inside the container. |
| `BUDGET_HTTP_PORT` | `8080` | Host port Compose publishes. Change it if 8080 is in use. |
| `BUDGET_PUBLIC_ORIGIN` | `http://localhost:5173` | The origin browsers use. Must match exactly, including scheme and port. |
| `BUDGET_SESSION_TTL` | `720h` | Server-side session lifetime. |
| `BUDGET_SESSION_COOKIE_SECURE` | auto | `true` marks the session cookie `Secure` and enables HSTS. Defaults to `false` only for loopback origins. |

`BUDGET_PUBLIC_ORIGIN` is a security control, not cosmetic: unsafe cookie-authenticated
requests must come from it, so a wrong value shows up as sign-in appearing to work while every
subsequent write is refused.

## Request protection

| Variable | Default | Description |
| --- | --- | --- |
| `BUDGET_AUTH_RATE_LIMIT` | `20` | Credential attempts per minute per client. `0` disables. |
| `BUDGET_AUTH_RATE_BURST` | `10` | Attempts allowed back to back. |
| `BUDGET_TRUSTED_PROXIES` | empty | Networks whose `X-Forwarded-For` is believed. Addresses or CIDRs, comma separated. |

Set `BUDGET_TRUSTED_PROXIES` when running behind a reverse proxy, or every client shares one
bucket. See [Operations](operations.md#request-protection).

## Invitation email (optional)

Disabled by default. When off, an invitation's one-time code is shown to the inviter to pass
on directly.

| Variable | Default | Description |
| --- | --- | --- |
| `BUDGET_SMTP_ENABLED` | `false` | Enables outbound invitation email. |
| `BUDGET_SMTP_HOST` | — | Relay hostname. Required when enabled. |
| `BUDGET_SMTP_PORT` | `587` | `465` uses TLS immediately; anything else negotiates STARTTLS. |
| `BUDGET_SMTP_USERNAME` | — | Account used to authenticate. Required when enabled. |
| `BUDGET_SMTP_PASSWORD` | — | Password or app password. Required when enabled. |
| `BUDGET_SMTP_FROM_ADDRESS` | username | Envelope sender. Most providers require it to match the account. |
| `BUDGET_SMTP_FROM_NAME` | `Budget` | Display name on the message. |
| `BUDGET_SMTP_TIMEOUT` | `15s` | Per-delivery timeout. |

A relay that offers neither TLS nor STARTTLS is refused rather than used, because the
connection carries both the account credential and the invitation token.

### Gmail

Set `BUDGET_SMTP_HOST=smtp.gmail.com` and `BUDGET_SMTP_PORT=587`, use the full Gmail address
as the username, and generate an **App Password** at
<https://myaccount.google.com/apppasswords> for the password. Gmail rejects the normal account
password for SMTP, and app passwords require 2-Step Verification. Leave
`BUDGET_SMTP_FROM_ADDRESS` empty so mail is sent as the authenticated account.

## Currency conversion (optional)

Disabled by default. When off, amounts render in their own currency and the application makes
no outbound requests.

| Variable | Default | Description |
| --- | --- | --- |
| `BUDGET_EXCHANGE_RATES_ENABLED` | `false` | Enables display currency conversion. |
| `BUDGET_EXCHANGE_RATES_BASE_URL` | `https://api.frankfurter.dev` | Rate provider. Must be `https`. |
| `BUDGET_EXCHANGE_RATES_TIMEOUT` | `10s` | Per-request timeout. |

Rates are cached in PostgreSQL and used for display only. They never alter stored amounts:
see [ADR 0005](decisions/0005-supported-currencies-and-display-conversion.md).

## Validation behaviour

Optional features validate their settings only when enabled, so a deployment that never turns
on SMTP cannot fail to start because of an SMTP value. When a feature is enabled and
misconfigured, startup fails with a message naming the variable, rather than starting and
failing later at the point of use.
