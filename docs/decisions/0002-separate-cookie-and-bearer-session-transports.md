# ADR 0002: Separate Cookie and Bearer Session Transports

## Status

Accepted

## Context

The React web application and native iOS application use the same application-owned session
system, but their safe transport mechanisms differ.

Browsers automatically attach cookies, which creates a cross-site request forgery risk for
state-changing requests. Native iOS clients can attach an explicit bearer token and store it
in Keychain. The original architecture specified secure cookies and Keychain storage but did
not define CSRF enforcement or distinguish session transport policy.

## Decision

Use opaque random session tokens whose hashes are stored in one `sessions` table. Each
session records whether its transport is `cookie` or `bearer`.

Initial tokens contain 256 random bits and are persisted only as SHA-256 hashes. Sessions
expire after 30 days by default, configurable through `BUDGET_SESSION_TTL`. Cookie sessions
use browser-session cookies; closing the browser may end local access earlier, while the
database expiry remains the server-side upper bound.

For web cookie sessions:

- deliver the token only through a `Secure`, `HttpOnly`, `SameSite=Lax` cookie
- do not expose the raw token to browser JavaScript in an authentication response
- require unsafe requests to match an explicitly configured allowed origin
- use Fetch Metadata, especially `Sec-Fetch-Site`, as defense in depth
- use Origin/Referer validation as the fallback when Fetch Metadata is unavailable
- apply the same origin check when registration or login issues a cookie session, preventing
  login-CSRF/session-swapping attacks
- keep CORS restrictive

For native iOS bearer sessions:

- return the opaque token through the native authentication flow
- store it in Keychain
- send it explicitly with `Authorization: Bearer`
- do not apply browser-cookie CSRF checks to bearer-authenticated requests

Origin validation uses trusted public-origin configuration rather than blindly trusting the
request `Host` header. Session lookup, expiry, hashing, revocation, and workspace membership
checks remain shared.

## Rationale

The decision uses the transport best suited to each client while preserving one
self-hostable authentication system. Explicitly recording transport prevents a web flow from
accidentally returning a cookie token as a bearer credential and allows expiration or
rotation policy to diverge later without separate session stores.

## Consequences

- Authentication endpoints must know which transport they are issuing.
- Unsafe cookie-authenticated endpoints require centralized CSRF middleware.
- Reverse-proxy deployments must configure the canonical public origin correctly.
- Security tests must cover rejected cross-site cookie requests and accepted bearer requests.
- SameSite cookies are defense in depth and are not the sole CSRF control.

## Alternatives Considered

### Synchronizer CSRF tokens

This is a valid browser defense but adds token issuance and frontend state. Strict origin
validation plus Fetch Metadata is sufficient for the initial same-origin product; the
decision can be revisited if deployment or browser requirements change.

### Cookies for both web and iOS

This complicates native credential handling and gives up the explicit bearer transport
available to the iOS client.

### Separate session tables

Separate tables duplicate token hashing, expiry, and revocation behavior without providing
an initial security benefit over a required transport discriminator.
