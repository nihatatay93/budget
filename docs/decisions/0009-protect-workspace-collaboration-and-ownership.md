# ADR 0009: Protect Workspace Collaboration and Ownership

## Status

Accepted

## Context

The initial domain model reserved workspace memberships, four roles, and invitation records,
but left invitation acceptance, duplicate handling, role administration, and removal rules
open. These operations cross a security boundary: they grant or revoke access to all
financial data in a workspace.

The MVP has unique login email addresses but does not verify that a user controls an email
inbox. It also must remain self-hostable without requiring an outbound email provider.
Consequently, an invitation email address cannot safely serve as identity proof, and member
removal cannot simply delete rows that historical transactions reference as actors.

## Decision

### Invitations are expiring bearer capabilities

An invitation uses a cryptographically random 256-bit token, stores only its SHA-256 hash,
and expires seven days after creation. The raw token is returned once to the authorized
inviter as a shareable link and is never returned by list operations or written to logs.
Automatic email delivery is optional and deferred.

Acceptance requires an authenticated user but does not require that user's login email to
match the invitation email. The email is normalized like a login address and identifies the
intended recipient for delivery and duplicate detection; possession of the token is the
actual capability. A successful acceptance atomically creates or reactivates membership in
only the invitation's workspace.

An invitation can be pending, accepted, revoked, or expired. Accepted and revoked states are
terminal. Acceptance records the user and is idempotent only for that same user; concurrent
acceptance by another user cannot succeed.

There is at most one pending invitation per normalized email and workspace. Creating another
revokes and replaces the prior invitation, including its token, role, and expiry. Creating an
invitation for an email already associated with an active member is rejected. If an accepting
user already has an active membership, acceptance is rejected without consuming the token
and never changes that existing role.

### Owner appointment is not delegated through a link

Invitations may grant `admin`, `member`, or `viewer`, but never `owner`. An owner appoints a
second owner through an explicit role change after that user is an active member.

Owners may invite any permitted role, revoke any pending invitation, change any active
member's role, and remove any other active member, subject to last-owner protection.
Administrators may invite or revoke only `member` and `viewer` invitations and may change or
remove only members and viewers. Members and viewers cannot administer collaboration.
Every active member may list the active roster and may leave voluntarily.

Every workspace must retain at least one active owner. Demoting, removing, or departing as
the last owner is rejected. The domain layer checks the rule for useful errors, and
PostgreSQL enforces it under concurrent writes.

### Removal preserves historical actor identity

Membership removal is soft: it records `removed_at` and immediately excludes the user from
authorization and active member lists. The row remains so transactions and invitations keep
valid actor references. A later accepted invitation may reactivate the same membership row
with the newly invited role.

The workspace's `created_by` value is historical metadata. It does not grant access or
override the current active role.

## Rationale

Treating the link as the capability is honest about the MVP's identity guarantees and keeps
self-hosting independent of an email provider. A short lifetime, one-time token disclosure,
hash-only storage, rotation on replacement, and single-user acceptance limit exposure.

Separating owner promotion from invitation acceptance makes the highest privilege an
intentional in-workspace action. The role matrix lets administrators handle ordinary sharing
without allowing them to take ownership or administer their peers. Soft removal preserves
financial audit references while revoking access immediately.

## Consequences

- Invitation creation must disclose that the link can be used by any authenticated person
  who receives it.
- The invitation schema needs acceptance/revocation actor state and uniqueness for one
  pending normalized email per workspace.
- Membership reads and authorization must consistently exclude removed rows.
- Reinviting a removed user reuses the membership identity rather than inserting a duplicate.
- Role and removal writes need a transaction-safe last-owner database invariant.
- The API must distinguish replacement, expiry, revocation, already-member conflicts, and
  idempotent acceptance without exposing token hashes.
- Verified-email-bound invitations would be a new identity policy and require a superseding
  decision rather than a silent acceptance change.

## Alternatives Considered

### Require the accepting user's email to match

Without email verification, matching two stored strings does not prove inbox control and can
give a false sense of security. It also makes out-of-band invitations brittle if the recipient
uses a different login address.

### Allow invitations to grant owner

This makes possession of a forwarded link sufficient to gain the highest workspace
privilege. Explicit promotion after joining is a clearer and safer ceremony.

### Hard-delete removed memberships

Historical transactions and invitations use membership actor references. Deletion would
either fail, erase provenance, or require weakening those integrity constraints.

### Require an email delivery service

Mandatory SMTP or a hosted provider would add deployment configuration and failure modes
without improving identity assurance while email ownership remains unverified.
