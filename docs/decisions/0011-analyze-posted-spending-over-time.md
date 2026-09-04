# ADR 0011: Analyze Posted Spending Over Time

## Status

Accepted

## Context

[ADR 0007](0007-derive-financial-projections-from-the-ledger.md) defines one period's
position: balances through a cutoff, plus income, spending, and category activity inside an
inclusive window, with pending activity kept explicitly separate.

That answers *where the money is*. It does not answer *how the money behaves*: which
categories consume it, when during a week or month it leaves, whether a category is growing
or shrinking, and how the current period compares with the one before it. Answering those
questions from the projection endpoint would mean a client requesting many windows and
re-deriving totals, distributions, and comparisons in presentation code — separately in web
and iOS, and inconsistently between them.

The ledger already holds every input. What was missing was one accepted interpretation of
time bucketing, comparison windows, and behavioural orientation.

## Decision

Spending analysis is a second derived read model in its own `analysis` capability, exposed
as `GET /v1/workspaces/{workspaceId}/spending-analysis`. It does not persist any total.

### Posted only

Every figure the analysis publishes is posted. Pending activity is excluded entirely rather
than reported alongside, which is the opposite choice from ADR 0007 and is deliberate:
reporting describes a position, including what has not settled; analysis describes settled
behaviour. A distribution that mixes settled and unsettled activity describes neither.

Soft-deleted transactions never contribute. Transfers carry no allocations under
[ADR 0003](0003-reconcile-transaction-entries-and-allocations.md), so allocation-derived
figures exclude them by construction. The one entry-derived breakdown, per-account activity,
excludes `kind = 'transfer'` explicitly, so moving money between owned accounts never reads
as spending.

### Analysis window and comparison window

The window is an optional pair of inclusive ISO calendar dates, with the same validation as
ADR 0007. When neither is supplied the server uses the trailing twelve workspace-local
months, anchored to the first day of a month so month buckets are whole.

Every response also carries a comparison window of exactly the same length, ending the day
before the analysis window begins. Every "versus previous" figure is computed against that
window inside the same query, so a period-over-period reading is always like-for-like and
never depends on a client issuing a second request with a window it chose itself.

### Time buckets

A `granularity` of `day`, `week`, or `month` sets the series bucket width. Weeks start on
Monday, matching ISO. When granularity is omitted the server chooses from the window length,
targeting a readable number of points rather than a fixed count.

The series tiles the whole window: buckets are contiguous, non-overlapping, clamped to the
window at both ends, and present even when empty, so a quiet stretch reads as a flat line
rather than as missing data.

Because buckets are generated per calendar step rather than per row of activity, an analysis
costs what its window spans, not what happened inside it — an empty workspace queried across
a century still produces a bucket a day. The window is therefore bounded at ten years, and
the resolved series at 750 buckets. The second bound is what stops a decade being requested
one day at a time; the first also bounds the equal-length comparison window read alongside
it. A server-chosen granularity is always one the server will accept, so the bounds are
reachable only by asking for them explicitly.

Granularity is a closed enum validated at the transport boundary *and* in the domain. Unlike
other enums this value becomes a SQL bucket width, so an unrecognized member is refused
before it travels inward rather than being treated as merely unreachable.

### Reporting orientation

Values use the same orientation as ADR 0007: positive spending means money left the
workspace, and income keeps its ledger sign. Orientation is applied in the domain; SQL
returns ledger-signed values throughout.

Two figures need care beyond a sign flip. A category's largest reading is its most negative
allocation for an expense category and its most positive for an income one, because a refund
sits at the opposite end of each. And a window whose only expense activity was a refund has
no largest charge at all, so that figure is reported as zero rather than as a negative
charge that never happened.

### Response contents

One request returns, from one snapshot:

- the analysis window, the comparison window, the resolved granularity, the workspace
  timezone, and the base currency
- window totals with their comparison-window counterparts, transaction counts, the largest
  single posted expense, and how many days carried spending
- the contiguous time series of income, spending, net, and transaction count per bucket
- per-category direct and rolled-up totals with comparison-window counterparts, transaction
  counts, largest allocation, and first and last activity dates
- sparse per-category, per-bucket activity, so a category's own trend shares the series axis
- ISO weekday and per-day distributions
- a ranked payee list
- per-account inflow and outflow

Category rollups follow ADR 0007: direct and rolled-up values are both present, and a client
summing a level must use one or the other, never both.

All of it observes one consistent repeatable-read PostgreSQL snapshot, so no two panels of
one response can disagree. Every query is workspace-scoped, and any workspace member may
read the analysis.

## Rationale

Splitting `analysis` from `reporting` follows the question being asked rather than the table
being read. The two capabilities share a ledger but not a policy: one keeps pending activity
visible because a position is incomplete without it, the other excludes it because behaviour
is only meaningful once settled. Merging them would force one of those policies onto the
other.

Returning the comparison window from the server, rather than letting clients request a second
period, is what makes "up 22% on the previous period" mean the same thing everywhere. The
same argument applies to bucketing: a client that groups days into weeks itself will
eventually disagree with another client about where a week starts.

Sending distributions and rankings pre-aggregated keeps a year of activity to one small
response and keeps financial arithmetic out of presentation code, consistent with ADR 0007.

## Consequences

- A single response requires several aggregate queries; they must share one read snapshot.
- The domain needs the workspace timezone and a clock to resolve the default trailing window
  and its comparison window deterministically.
- The comparison window is always read, so an analysis query touches roughly twice its
  window's activity.
- Windows beyond ten years, and granularities that would exceed the bucket bound, are refused
  with 400 rather than served slowly. A client that wants a longer view asks for a coarser
  bucket, which is the reading that span supports anyway.
- The payee ranking is bounded and is a leaderboard, not an export.
- Payee analysis is only as good as the payee field; transactions recorded without one are
  counted in every other breakdown but absent from that list.
- Per-account activity is entry-derived and therefore attributes a split transaction to the
  account it moved through, not proportionally across its categories.
- Adding a granularity member means changing the contract, the domain enum, and both
  contract-mirror tests, which is intentional friction around a value that reaches SQL.
- Forecasting, recurring-transaction detection, budget-pace projection, and anomaly
  detection remain deferred.

## Alternatives Considered

### Extend the financial-projection endpoint

Reporting would have to serve two incompatible pending policies and grow a bucketing concept
it does not otherwise need. The endpoint would become the union of two questions and answer
neither cleanly.

### Have clients request one window per bucket

A twelve-month daily view would mean hundreds of requests, and every client would re-derive
its own totals, comparisons, and week boundaries. Cross-client disagreement would be a matter
of time.

### Aggregate in the client from the transaction list

This moves financial arithmetic into presentation code, requires downloading a full year of
transactions, and duplicates the sign, rollup, and transfer rules in every client — the same
reasoning ADR 0007 rejected.

### Include pending activity alongside posted, as reporting does

It doubles the size of every breakdown to answer a question analysis does not ask. A weekday
distribution or a period-over-period comparison built partly from unsettled activity
describes behaviour that has not happened yet.

### Persist or materialize the aggregates

The current scale does not justify invalidation and repair complexity. Derived SQL remains
the source of truth, and any future cache must be reproducible from the ledger.
