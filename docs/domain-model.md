# Domain Model

## Status

Accepted initial domain direction.

This document is **public-facing** and should be included in public release snapshots. The
private/public repository workflow must not create a second version of the domain model; the
public repository contains a validated snapshot of the same product source.

This document defines the main financial concepts and invariants. It is intentionally more
important than the initial API shape because API and persistence design should follow the
domain rather than drive it.

## Core Principle

The model separates:

```text
Transaction = economic event
Transaction Entry = effect on an account
Transaction Allocation = effect on a category/budget
```

Conceptually:

```text
                 Transaction
                 /         \
                /           \
               v             v
      Transaction Entry   Allocation
               |             |
               v             v
            Account       Category
```

This allows correct handling of:

- expenses
- income
- transfers
- credit-card purchases
- credit-card payments
- split transactions
- refunds
- opening balances
- future multi-currency behavior

without forcing the project into full formal accounting software.

## Workspace as the Ownership Boundary

Financial data belongs to a workspace, not directly to a user.

```text
User
  |
  v
Workspace Member
  |
  v
Workspace
  |
  +-- Accounts
  +-- Categories
  +-- Transactions
  +-- Budgets
```

A user can belong to more than one workspace.

Example:

```text
Nihat
├── Personal
└── Atay Family

Gökçem
├── Personal
└── Atay Family
```

This model must exist from the beginning. Migrating a mature single-user model to shared
workspaces later is unnecessarily risky.

---

# Identity

## User

Initial conceptual fields:

```text
users
-----
id
email
password_hash
display_name
created_at
updated_at
```

Rules:

- email is unique
- registration creates the user, initial workspace, owner membership, protected
  Uncategorized Income/Expense categories, and first session atomically
- passwords accept 15 through 128 Unicode characters and are stored only as Argon2id hashes
- financial preferences should generally not live on the user if they vary per workspace
- externally visible identifiers should use UUIDs

## Session

Conceptual fields:

```text
sessions
--------
id
user_id
token_hash
transport
expires_at
created_at
last_used_at
```

Rules:

- generate cryptographically random opaque tokens
- store only a hash of the session token
- distinguish `cookie` and `bearer` transport policy in the session record
- sessions expire after 30 days by default; deployments may configure the lifetime
- deleting/revoking the session invalidates it immediately

---

# Workspace

## Workspace

Conceptual fields:

```text
workspaces
----------
id
name
base_currency
timezone
created_by
created_at
updated_at
```

### Base Currency

The workspace has one reporting currency.

Example:

```text
Atay Family
base_currency = TRY
timezone = Europe/Istanbul
```

Individual accounts may later have different currencies, but dashboard/reporting needs a
stable workspace reporting currency.

## Workspace Member

Many-to-many relationship:

```text
workspace_members
-----------------
workspace_id
user_id
role
joined_at
removed_at
```

Initial role model:

```text
owner
admin
member
viewer
```

An active membership has `removed_at IS NULL`. Removing a member revokes access without
deleting the row because transactions and invitations retain workspace-member actor
references. Reaccepting an invitation may reactivate that same row with the invitation's
role and a new `joined_at` value.

For workspace financial setup, all members may read accounts and categories. Owners,
administrators, and members may create, update, and archive them; viewers are read-only.
Every operation still verifies membership against the workspace in the request path.

Collaboration administration uses a narrower boundary than ordinary financial writes:

| Action | Owner | Admin | Member | Viewer |
| --- | --- | --- | --- | --- |
| List active members | yes | yes | yes | yes |
| List pending invitations | yes | yes | no | no |
| Invite an admin | yes | no | no | no |
| Invite a member or viewer | yes | yes | no | no |
| Change any active member's role | yes | no | no | no |
| Change a member/viewer between those roles | yes | yes | no | no |
| Remove another member | any role, subject to last-owner protection | member/viewer only | no | no |
| Leave the workspace | yes, unless the last owner | yes | yes | yes |

Administrators cannot appoint, change, or remove owners or other administrators. An owner
may promote an existing active member to owner, but invitations cannot grant the owner role.
The workspace creator has no permanent privilege beyond their active owner membership.

Every workspace must have at least one active owner. A role change, removal, or departure
that would remove the last active owner is rejected. This invariant must be enforced under
concurrent writes by PostgreSQL as well as by the domain service.

Recommended primary/unique boundary:

```text
(workspace_id, user_id)
```

## Workspace Invitation

Conceptual fields:

```text
workspace_invitations
---------------------
id
workspace_id
email
role
token_hash
invited_by
expires_at
accepted_at
accepted_by
revoked_at
created_at
```

Rules:

- generate a 256-bit random token, store only its SHA-256 hash, and return the raw token only
  once when an authorized owner or administrator creates or replaces the invitation
- invitations expire exactly seven days after creation; expiration is based on an absolute
  timestamp rather than the workspace calendar timezone
- an invitation is pending only while it is unaccepted, unrevoked, and unexpired; acceptance
  and revocation are terminal states and tokens are never reused
- initial invitations may grant `admin`, `member`, or `viewer`, but never `owner`; appointing
  an owner requires a separate owner-authorized role change on an active membership
- owners may invite or revoke any permitted invitation role; administrators are limited to
  `member` and `viewer`
- normalize invitation email addresses using the same trim-and-lowercase policy as user
  authentication
- at most one pending invitation exists for a normalized email in a workspace; creating
  another atomically revokes the old invitation, rotates the token, and replaces its role
  and expiry
- reject creation when the normalized email already belongs to an active workspace member;
  expired and revoked invitations do not block a replacement, and different workspaces are
  independent
- acceptance requires an authenticated user and atomically creates or reactivates only the
  membership in the invitation's workspace with the invitation's role
- the email is a delivery label, not an identity proof: because the MVP does not verify email
  ownership, possession of the high-entropy invitation token authorizes any authenticated
  user to accept it
- if the accepting user is already an active member, reject without consuming the invitation
  so an invitation cannot silently change an existing role; a retry by the same user after a
  successful acceptance returns the accepted membership idempotently
- concurrent acceptance has one winner; a different user cannot reuse an accepted token
- invitation listing never returns token material, and raw tokens must not be logged
- invitation behavior must not allow joining a different workspace than the invitation
  targets

The MVP does not require an outbound email service. The authorized inviter receives a
one-time shareable acceptance link and may deliver it out of band. Adding automatic email
delivery later does not change the bearer-token acceptance policy unless verified email
ownership is introduced explicitly.

---

# Account

## Account

Conceptual fields:

```text
accounts
--------
id
workspace_id
name
type
currency
institution_name
archived_at
created_at
updated_at
```

Potential account types:

```text
bank
cash
credit_card
savings
investment
other
```

The MVP can expose a smaller subset.

### Currency Immutability

Once an account has financial history, its currency must not be changed casually.

Changing an account from TRY to USD would invalidate the meaning of historical
`amount_minor` values.

The application rejects this change after any transaction entry references the account,
and PostgreSQL enforces the same rule as a backstop. Other account metadata may still be
corrected. Removing an account through the API sets `archived_at`; it does not delete the
account or its history.

### Balance

Do not use:

```text
accounts.balance
```

as the authoritative balance.

Authoritative balance is derived from posted transaction entries:

```text
Account Balance = SUM(transaction_entries.amount_minor)
```

A future cached/snapshot balance is allowed for performance, but it remains derived data.

---

# Money Representation

Never store financial amounts as floating-point values.

Use integer minor units.

Examples:

```text
TRY 1,250.50 -> 125050
USD 10.99    -> 1099
```

Conceptual representation:

```text
amount_minor BIGINT
currency     ISO currency code
```

## Supported Currencies

The product supports exactly `TRY`, `USD`, and `EUR`. The set is enforced in the OpenAPI
contract, in Go, and by PostgreSQL `CHECK` constraints on `workspaces.base_currency` and
`accounts.currency`. Workspace base currency is immutable after creation.

All three use two minor-unit decimal places, so the currency metadata utility described
below is not required yet. Adding a currency with a different exponent requires building that
utility first, because formatting and conversion would otherwise be wrong for it.

Currency exponent/formatting rules will require a small currency metadata utility when the
supported set grows beyond currencies that use exactly two decimal places.

See [ADR 0005](decisions/0005-supported-currencies-and-display-conversion.md).

---

# Transaction

## Transaction Aggregate

Conceptual fields:

```text
transactions
------------
id
workspace_id
kind
status
transaction_date
payee
description
notes
source
created_by
updated_by
created_at
updated_at
deleted_at
```

Potential `kind` values:

```text
standard
transfer
adjustment
```

Do not require `expense` and `income` to be transaction kinds. Direction can be inferred
from signed entries/allocations.

Potential `status` values:

```text
pending
posted
```

Reconciliation is a structural property of the transaction aggregate and applies to both
`pending` and `posted` transactions. Pending is not a draft state: a persisted pending
transaction still contains a valid, complete set of entries and allocations.

Authoritative account balances, budgets, dashboard income/spending, and category reporting
include only `posted` transactions. A client may display pending totals separately as
projected information, but pending activity must not change authoritative reported values.

Potential `source` values:

```text
manual
recurring
import
api
```

## Transaction Date

Use a business/reporting date such as:

```text
transaction_date DATE
```

for financial reporting.

Keep:

```text
created_at TIMESTAMPTZ
updated_at TIMESTAMPTZ
```

for audit/technical timestamps.

This avoids month-boundary behavior being accidentally altered by UTC conversion.

---

# Transaction Entry

An entry represents how a transaction affects one account.

Conceptual fields:

```text
transaction_entries
-------------------
id
transaction_id
account_id
amount_minor
base_amount_minor
created_at
```

Use signed amounts:

```text
positive = account gains value
negative = account loses value
```

## Expense Example

Restaurant purchase: TRY 350.

```text
Transaction
  "Dinner"

Entry
  Checking Account    -35000
```

## Income Example

Salary: TRY 100,000.

```text
Transaction
  "Salary"

Entry
  Checking Account    +10000000
```

## Transfer Example

Move TRY 10,000 from checking to savings.

```text
Transaction
  kind = transfer

Entries
  Checking   -1000000
  Savings    +1000000
```

Workspace net worth change:

```text
0
```

Income:

```text
0
```

Spending:

```text
0
```

A transfer must never be counted as income or expense simply because two accounts changed.

Transfer invariants:

- allocations must be empty
- entry `base_amount_minor` values must sum to zero

Fees or other externally reportable effects must be represented explicitly rather than
hidden inside a transfer that appears net-zero.

---

# Credit Cards

A credit card is an account.

A purchase makes the credit-card account more negative.

Example: TRY 1,000 restaurant purchase.

```text
Entry
  Credit Card    -100000

Allocation
  Restaurants    -100000
```

Later, paying the card from a checking account is a transfer:

```text
Checking       -100000
Credit Card    +100000
```

The credit-card payment must not create another expense. The expense occurred when the
purchase was recorded.

---

# Category

Conceptual fields:

```text
categories
----------
id
workspace_id
parent_id
name
kind
system_key
icon
archived_at
created_at
updated_at
```

Potential `kind`:

```text
expense
income
```

`system_key` is null for ordinary categories. Protected categories use stable values such
as `uncategorized_expense` and `uncategorized_income`, unique within a workspace.

Category `kind` classifies the category's reporting purpose; it does not constrain allocation
sign. An expense category accepts negative purchases and positive refunds. An income
category accepts positive income and negative reversals.

Hierarchical categories should be supported from the beginning.

Example:

```text
Expense
├── Food
│   ├── Groceries
│   └── Restaurants
├── Transport
│   ├── Fuel
│   └── Taxi
└── Bills

Income
├── Salary
└── Other
```

The UI may initially limit nesting depth even if the persistence model is recursive.

Category hierarchy rules:

- a parent must be active, belong to the same workspace, and have the same `kind`
- a category cannot be its own ancestor
- a category with active children cannot be archived until its children are archived
- `kind` cannot change while the category has children or transaction allocations
- API removal archives a category rather than deleting it

Each workspace has protected system categories for uncategorized expense and income. Their
structural and reporting fields are immutable, and they cannot be archived or deleted. When
a user does not choose a category for a standard transaction, its allocation uses the
appropriate protected uncategorized category so spending and income totals remain complete.

---

# Transaction Allocation

An allocation answers:

> What was this money for?

Conceptual fields:

```text
transaction_allocations
-----------------------
id
transaction_id
category_id
amount_base_minor
created_at
```

Use signed amounts:

```text
negative = expense
positive = income/refund
```

## Allocation Reconciliation

Allocation rules are defined by transaction kind:

```text
transfer
  allocations are empty
  SUM(entries.base_amount_minor) = 0

standard
  one or more allocations are required
  SUM(allocations.amount_base_minor) = SUM(entries.base_amount_minor)

adjustment
  allocations may be empty
  when present, SUM(allocations.amount_base_minor) = SUM(entries.base_amount_minor)
```

An uncategorized standard transaction is represented by a normal allocation to the
workspace's protected Uncategorized expense or income category; it is not represented by
an empty allocation set.

The domain service validates these rules before persistence. PostgreSQL also enforces the
cross-table invariant with deferred constraint triggers covering entries, allocations, and
relevant transaction-kind changes, so direct SQL writes cannot leave a committed aggregate
in an invalid state.

See [ADR 0003](decisions/0003-reconcile-transaction-entries-and-allocations.md).

## Split Transaction Example

A TRY 1,500 supermarket purchase:

```text
Account Entry
  Credit Card    -150000

Allocations
  Groceries      -100000
  Household       -50000
```

This is one transaction with one account movement and multiple budget/category effects.

## Refund Example

A TRY 100 restaurant refund:

```text
Account Entry
  Checking       +10000

Allocation
  Restaurants    +10000
```

Monthly restaurant spending naturally decreases by TRY 100.

---

# Why Entries and Allocations Are Separate

These answer different questions.

Entries:

> Which account gained or lost value?

Allocations:

> Which spending/income category should reporting and budgets use?

Example:

```text
TRY 500 restaurant purchase

Account effect:
  Credit Card    -500

Budget effect:
  Restaurants    -500
```

Keeping these concepts separate prevents transfers and credit-card payments from corrupting
budget reporting.

---

# Multi-Currency

Multi-currency should be anticipated in the model even if the first UI is simple.

Workspace:

```text
base_currency = TRY
```

Account:

```text
currency = USD
```

An entry may hold:

```text
amount_minor
base_amount_minor
```

Where:

- `amount_minor` is the amount in the account's native currency
- `base_amount_minor` is the historical value in workspace reporting currency

Historical base amounts should be stored at transaction time rather than recalculated using
today's FX rate, otherwise old spending reports change as exchange rates move.

## Display Conversion Is Not Booking

Two different uses of exchange rates must not share a mechanism.

```text
display conversion    presentation only, today's rate, never stored
historical booking    base_amount_minor, transaction-date rate, stored forever
```

A display conversion renders a figure that is already final in the workspace base currency
into another supported currency so a user can read it, for example showing a 50,000 TRY
budget as its US dollar equivalent. It is never persisted on a financial row, never feeds a
further calculation, and always appears alongside the date of the rate used.

Historical booking is the mechanism described above: when an entry's account currency differs
from the workspace base currency, `base_amount_minor` is computed at the transaction date's
rate and stored permanently. Display conversion must never be used to derive it, because
recomputing stored base amounts at today's rate would violate invariant 12.

Rates come from Frankfurter, cached in PostgreSQL. Rate fetching is optional and disabled by
default; when it is off or unavailable, amounts simply render in their own currency.

See [ADR 0005](decisions/0005-supported-currencies-and-display-conversion.md).

Transaction writes now book the historical base amount according to
[ADR 0006](decisions/0006-book-historical-base-amounts.md): same-currency amounts are derived,
while foreign-currency entries use an explicit base amount or the optional historical-rate
provider. A failed lookup never leaves a partial aggregate.

---

# Opening Balances and Adjustments

Do not use a mutable `initial_balance` field as financial truth.

Create an adjustment transaction.

Example:

```text
Transaction
  kind = adjustment
  description = "Opening balance"

Entry
  Checking    +5000000
```

This keeps the ledger complete and reconciliable.

---

# Financial Projections and Reporting

Financial projections are derived read models rather than stored financial facts. The ledger
remains authoritative:

```text
account balances              <- transaction entries
income, spending, categories  <- transaction allocations
```

## Reporting Period

A reporting period uses inclusive `from_date` and `to_date` calendar dates. Both are supplied
together or both are omitted. When omitted, the effective period is month-to-date in the
workspace timezone: the first day of the current local month through the current local date.

`transaction_date` is already a business `DATE`, so explicit ranges compare it directly and
do not convert it through UTC. Responses repeat the effective range, workspace timezone, and
base currency.

## Posted and Pending Projections

Soft-deleted transactions are always excluded. Posted values are authoritative. Pending
values are calculated separately, and a projected value is their explicit arithmetic sum:

```text
projected = posted + pending
```

This is a pending projection, not a forecast. It does not infer future or recurring activity.

## Account Projection

Balances are cumulative through `to_date`; they do not reset at `from_date`:

```text
posted native balance    = SUM(posted entry.amount_minor through to_date)
pending native delta     = SUM(pending entry.amount_minor through to_date)
projected native balance = posted native balance + pending native delta
```

Equivalent sums of `base_amount_minor` provide the account and workspace values in workspace
base currency. These are historically booked reporting values, not revaluations using today's
display rate.

Archived accounts remain part of projection results and authoritative workspace totals.
Archival changes organization and editing behavior; it does not erase a balance.

## Income and Spending Projection

Income and spending use allocations inside the reporting period:

```text
income   =  SUM(income-category allocation.amount_base_minor)
spending = -SUM(expense-category allocation.amount_base_minor)
```

The sign is not clamped or converted to an absolute value. A positive refund allocated to an
expense category reduces spending and can produce net-negative spending for a period. A
negative reversal allocated to an income category similarly reduces income.

Transfers have no allocations, so they never contribute. An adjustment without allocations
changes balances only; an allocated adjustment participates in reporting through its
categories.

## Category Projection

Each category exposes direct activity and rolled-up activity including all descendants.
Posted, pending, and projected values remain separate. Values use reporting orientation:

```text
expense category  positive = net spending, negative = net refund
income category   positive = net income,   negative = net reversal
```

Workspace income and spending totals sum direct allocation effects exactly once. Summing
every parent rollup would double-count descendants and is invalid. Archived categories and
their ancestors remain available when needed to explain historical or pending activity.

All figures in one projection response observe one consistent PostgreSQL snapshot and are
scoped to one verified workspace membership. See
[ADR 0007](decisions/0007-derive-financial-projections-from-the-ledger.md).

---

# Budget

Budgets are monthly spending plans. Actual usage remains derived from the transaction ledger;
it is never written back as mutable budget state.

## Monthly Budget

Conceptual fields:

```text
budgets
-------
id
workspace_id
name
starts_on
created_at
updated_at
```

There is at most one budget per workspace calendar month. `starts_on` is the first day of
that month, and the effective window is `[starts_on, starts_on + 1 month)`. The workspace
timezone determines the default current month, while transaction `DATE` values compare
directly to the calendar boundaries.

## Budget Item

Conceptual fields:

```text
budget_items
------------
id
budget_id
category_id
amount_base_minor
```

Example:

```text
Monthly Budget

Groceries       TRY 15,000
Restaurants     TRY  8,000
Entertainment   TRY  5,000
Shopping        TRY 10,000
```

Item amounts are strictly positive workspace-base-currency minor units. The initial model
accepts expense categories only. A budget may plan a parent or a descendant category, but not
both in the same month; category reparenting must not introduce such an overlap.

Actual spending is not written into the budget table. Usage rolls up the item's category and
all descendants and includes only posted, non-deleted transaction allocations inside the
complete budget month:

```text
Used      = -SUM(signed expense allocations)
Remaining = Budgeted - Used
```

Refunds therefore reduce usage. Used and remaining values are not clamped; net refunds and
overspending remain visible. Pending transactions and transfers do not affect usage.

Archived categories already present in a budget remain visible and continue to explain
historical usage. They may be retained, edited, or removed, but cannot be newly assigned.

Rollover is deliberately disabled for the MVP. Every month uses only its explicit planned
amounts. See [ADR 0008](decisions/0008-derive-monthly-budget-usage-from-posted-allocations.md).

---

# Recurring Transactions

Recurring behavior is deferred from the first database milestone but the conceptual model
should fit the existing transaction aggregate.

Potential model:

```text
recurring_rules
---------------
id
workspace_id
name
frequency
interval
starts_on
ends_on
next_run_on
auto_post
active
created_by
created_at
updated_at
```

Initial frequencies may include:

```text
daily
weekly
monthly
yearly
```

A recurring rule should generate a normal transaction.

Do not create a parallel "recurring money" model that bypasses normal entries and
allocations.

A future template may use:

```text
recurring_rule_entries
recurring_rule_allocations
```

to mirror a normal transaction.

---

# Deletion and Archival

Recommended behavior:

```text
Account      -> archive
Category     -> archive
Transaction -> soft delete
```

Accounts/categories may be referenced by historical financial data and therefore should not
normally be hard deleted.

Transactions should initially support `deleted_at` so accidental removal is recoverable and
shared-workspace behavior remains auditable.

---

# Workspace Isolation

Workspace isolation is both a domain rule and a security rule.

Every financial entity must be scoped to a workspace directly or through constraints that
make workspace ownership unambiguous.

Avoid queries conceptually equivalent to:

```sql
SELECT * FROM transactions WHERE id = $1;
```

Prefer workspace-aware access:

```sql
SELECT *
FROM transactions
WHERE id = $1
  AND workspace_id = $2;
```

Where practical, PostgreSQL foreign-key/unique constraints should also prevent a transaction
in Workspace A from referencing an account/category in Workspace B.

---

# Audit Fields

Shared workspaces make actor information useful.

Transactions should preserve:

```text
created_by
updated_by
```

where appropriate.

A full activity/audit log can be added later:

```text
activity_log
------------
id
workspace_id
actor_user_id
action
entity_type
entity_id
metadata
created_at
```

It is not required for the initial MVP.

---

# Identifiers

Use UUIDs for externally visible domain entities.

Preferred direction:

```text
UUIDv7
```

because time-ordered identifiers provide useful index/locality characteristics while
remaining non-sequential from an API consumer's perspective.

Generate UUIDv7 identifiers in Go using `github.com/google/uuid` when the service needs identifiers
before inserting a multi-row aggregate. PostgreSQL 18 columns also use `DEFAULT uuidv7()` as
a backstop for valid direct inserts. UUIDv7 values expose approximate creation time and must
not be treated as secrets or authorization capabilities.

---

# Initial Database Scope

The first real schema should likely include:

## Authentication

```text
users
sessions
```

## Workspace

```text
workspaces
workspace_members
workspace_invitations
```

## Finance

```text
accounts
categories
transactions
transaction_entries
transaction_allocations
```

## Budget

```text
budgets
budget_items
```

Total:

```text
12 tables
```

This is enough for a capable initial product without prematurely implementing every future
feature.

---

# Deferred Features

Do not include these in the first persistence milestone unless product requirements change:

```text
bank_connections
bank_credentials
transaction_import_rules
attachments
receipts
tags
investment_portfolios
currency_rate_provider
notifications
full_activity_log
recurring_transactions
```

The core schema should make these possible later without requiring a rewrite of the primary
transaction model.

---

# MVP Financial Flows

The initial domain should support:

```text
Register / Login

Create Workspace
Invite Member

Create Account
├── Bank
├── Cash
└── Credit Card

Add Transaction
├── Expense
├── Income
├── Transfer
└── Split Transaction

Categories

Dashboard
├── Income
├── Spending
├── Spending by Category
└── Account Balances

Budgets
└── Monthly Category Limits
```

---

# Core Financial Invariants

Implementation and tests must preserve these invariants:

1. Monetary values are never represented as floats.
2. Every financial resource belongs to exactly one workspace context.
3. A user must be a member of a workspace before accessing its resources.
4. Account balances are derived from posted transaction entries.
5. Transfers do not contribute to income or spending.
6. Credit-card payments are transfers, not expenses.
7. Category/budget reporting uses allocations, not raw account entries.
8. Standard transaction allocations must reconcile exactly with entry base amounts;
   uncategorized standard transactions use protected Uncategorized allocations.
9. Transfers have no allocations and their entry base amounts sum to zero.
10. Account currency cannot be casually changed after financial history exists.
11. Archived accounts/categories remain usable for historical reporting.
12. Historical multi-currency reporting must not change merely because today's FX rate
    changed.
13. Recurring rules, when introduced, create normal transactions rather than bypassing the
    transaction model.
14. Pending and posted transactions obey the same structural reconciliation rules, while
    authoritative financial reporting includes only posted transactions.
15. Allocation sign is independent of category kind so refunds and reversals remain valid.
