# ADR 0010: Deliver Invitations Over SMTP

## Status

Accepted

## Context

A workspace invitation is authorized by a single-use token. The repository stores only its
hash, so the moment of creation is the one opportunity to put the token in front of the
person being invited.

Until now the token was disclosed to the inviter, who passed it on by whatever means they
had. That works, but it asks the inviter to move a credential by hand and gives the recipient
no context about who invited them or when the code expires.

Email is the obvious delivery channel and the one the architecture already anticipated. It
also introduces the first outbound credential the application holds, and the first
user-controlled text that reaches a protocol with in-band headers.

## Decision

Send invitation email over SMTP, disabled by default and enabled through configuration.

### Delivery is best effort

A delivery failure never fails the request that created the invitation. The invitation is
already stored and its token is still returned to the inviter, so a bounced or misconfigured
relay degrades to exactly the share-it-yourself flow that runs when email is switched off.
Failing the request would discard a valid invitation over a transport problem.

The API therefore continues to disclose the acceptance token at creation whether or not email
is configured, and the clients continue to display it.

### The token is not a link

The token travels in the message body as a code to paste, not as a URL. A link would place
the credential in browser history, in the referrer of whatever page the recipient opened
next, and in the logs of anything that followed a redirect.

### The transport is always encrypted

Port 465 uses TLS from the first byte and any other port negotiates STARTTLS. A relay
offering neither is refused rather than downgraded, because the connection carries both the
account credential and the invitation token.

### Headers are validated, not escaped

The inviter's display name and the workspace name reach the subject line, and both are
user-controlled. A value containing a line break is rejected rather than repaired, because a
repaired value silently sends a message the caller did not intend, while a rejected one
surfaces as a delivery failure that is already non-fatal.

### Delivery is synchronous

The send happens inline, bounded by a timeout. A self-hosted workspace invites people rarely,
and a queue would add infrastructure the project does not otherwise need. If invitation
volume ever makes this visible, a PostgreSQL-backed outbox is the next step rather than a
broker.

## Rationale

Optional and best-effort keeps the feature from becoming a way for the product to fail. The
security choices follow from what the message carries: the token is a bearer credential with
a short life, so it is kept out of URLs and off plaintext connections, and the headers that
frame it are built from validated input.

## Consequences

- The deployment gains an outbound SMTP dependency when enabled, and holds a relay credential
  in configuration.
- An operator who enables SMTP and misconfigures it sees invitations that still work, with a
  warning in the logs, rather than invitation creation breaking.
- Providers that require the envelope sender to match the authenticated account are
  accommodated by defaulting the from address to the username.
- Message content is plain text. HTML would need a second body part and a sanitization story
  for the same user-controlled values.
- Bounce handling, delivery receipts, and retries are not implemented; a failed send is
  logged and the inviter still holds the token.

## Alternatives Considered

### Fail invitation creation when delivery fails

This makes delivery observable, but it throws away a stored invitation because a relay was
briefly unreachable, and it makes the workspace's collaboration flow depend on an optional
component.

### Send a link containing the token

A one-click link is a better experience, but it puts the credential in browser history and
referrers. A code to paste keeps the credential in the message.

### Queue delivery through a background worker

Correct at volume and unnecessary here. It adds a table, a worker loop, and retry semantics
for a message a self-hoster sends a handful of times.

### Use a transactional email API instead of SMTP

A provider API would give delivery reporting, but it makes the product depend on a specific
vendor and an account, which conflicts with keeping self-hosting free of mandatory external
services. SMTP works with any relay, including one the operator runs.
