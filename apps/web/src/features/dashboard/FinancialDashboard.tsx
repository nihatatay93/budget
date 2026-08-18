import { useQuery } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  APIError,
  type FinancialProjection,
  type FinancialProjectionRange,
  type MonthlyBudget,
  financialProjectionQueryKey,
  getFinancialProjection,
  getMonthlyBudget,
  monthlyBudgetQueryKey,
} from "../../api/client";
import { type Currency, formatMoney } from "../../lib/currency";
import { monthLabel, workspaceMonth } from "../../lib/month";

type Workspace = {
  id: string;
  name: string;
  base_currency: Currency;
  timezone: string;
};

export function FinancialDashboard({ workspace }: { workspace: Workspace }) {
  const [range, setRange] = useState<FinancialProjectionRange>();
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [rangeError, setRangeError] = useState("");
  const query = useQuery({
    queryKey: financialProjectionQueryKey(workspace.id, range),
    queryFn: () => getFinancialProjection(workspace.id, range),
  });
  const currentMonth = workspaceMonth(workspace.timezone);
  const budget = useQuery({
    queryKey: monthlyBudgetQueryKey(workspace.id, currentMonth),
    queryFn: () => getMonthlyBudget(workspace.id, currentMonth),
    retry: false,
  });

  function applyRange(event: FormEvent) {
    event.preventDefault();
    if (!fromDate || !toDate || fromDate > toDate) {
      setRangeError("Choose a start date that is on or before the end date.");
      return;
    }
    setRangeError("");
    setRange({ fromDate, toDate });
  }

  function resetRange() {
    setFromDate("");
    setToDate("");
    setRangeError("");
    setRange(undefined);
  }

  return (
    <section className="financial-dashboard" aria-labelledby="financial-overview-heading">
      <div className="projection-heading">
        <div>
          <p className="eyebrow">Ledger-derived overview</p>
          <h2 id="financial-overview-heading">Financial overview</h2>
          <p className="projection-caption">
            Posted figures are authoritative. Pending activity is shown separately.
          </p>
        </div>
        <form className="projection-range" onSubmit={applyRange}>
          <label>
            From
            <input
              aria-label="Projection start date"
              type="date"
              value={fromDate}
              onChange={(event) => setFromDate(event.target.value)}
            />
          </label>
          <label>
            To
            <input
              aria-label="Projection end date"
              type="date"
              value={toDate}
              onChange={(event) => setToDate(event.target.value)}
            />
          </label>
          <button className="secondary-button" type="submit">Apply</button>
          {range ? (
            <button className="text-button" type="button" onClick={resetRange}>
              Month to date
            </button>
          ) : null}
        </form>
      </div>
      {rangeError ? <p className="form-error" role="alert">{rangeError}</p> : null}
      {query.isLoading ? <p className="resource-state">Loading financial overview…</p> : null}
      {query.isError ? <ProjectionError error={query.error} retry={() => query.refetch()} /> : null}
      {query.data ? (
        <ProjectionContent
          projection={query.data}
          budget={budget.data}
          budgetMissing={budget.error instanceof APIError && budget.error.status === 404}
        />
      ) : null}
    </section>
  );
}

function ProjectionContent({
  projection,
  budget,
  budgetMissing,
}: {
  projection: FinancialProjection;
  budget?: MonthlyBudget;
  budgetMissing: boolean;
}) {
  const currency = projection.period.base_currency;
  const expenseCategories = projection.categories.filter((category) => category.kind === "expense");
  const incomeCategories = projection.categories.filter((category) => category.kind === "income");

  return (
    <>
      <p className="projection-period">
        {formatDate(projection.period.from_date)}–{formatDate(projection.period.to_date)} ·{" "}
        {projection.period.timezone} · {currency}
      </p>
      <div className="projection-summary-grid">
        <SummaryCard
          label="Balance"
          amounts={projection.summary.balance_base_minor}
          currency={currency}
          pendingLabel="Pending delta"
        />
        <SummaryCard
          label="Income"
          amounts={projection.summary.income_base_minor}
          currency={currency}
          pendingLabel="Pending income"
        />
        <SummaryCard
          label="Spending"
          amounts={projection.summary.spending_base_minor}
          currency={currency}
          pendingLabel="Pending spending"
        />
      </div>
      {budget ? <CurrentBudgetProgress budget={budget} /> : null}
      {budgetMissing ? (
        <div className="current-budget-empty">
          <div>
            <strong>No plan for the current month</strong>
            <small>Set category targets to compare posted spending with your plan.</small>
          </div>
          <a href="#monthly-budget-heading">Create monthly budget</a>
        </div>
      ) : null}
      <div className="projection-detail-grid">
        <section className="projection-detail" aria-labelledby="projection-accounts-heading">
          <div className="projection-detail-heading">
            <div>
              <p className="eyebrow">Cumulative through period end</p>
              <h3 id="projection-accounts-heading">Account balances</h3>
            </div>
            <a href="#accounts-heading">Manage accounts</a>
          </div>
          {projection.accounts.length === 0 ? (
            <p className="resource-state">No accounts yet.</p>
          ) : (
            <div className="projection-list">
              {projection.accounts.map((account) => (
                <article className="projection-row" key={account.id}>
                  <div>
                    <strong>{account.name}</strong>
                    <small>
                      {account.type.replaceAll("_", " ")}
                      {account.archived_at ? " · archived" : ""}
                    </small>
                  </div>
                  <div className="projection-amount">
                    <strong>{formatMoney(account.native_balance_minor.posted, account.currency)}</strong>
                    {account.native_balance_minor.pending !== 0 ? (
                      <small>
                        {signedMoney(account.native_balance_minor.pending, account.currency)} pending ·{" "}
                        {formatMoney(account.native_balance_minor.projected, account.currency)} projected
                      </small>
                    ) : null}
                    {account.currency !== currency ? (
                      <small>{formatMoney(account.base_balance_minor.posted, currency)} base</small>
                    ) : null}
                  </div>
                </article>
              ))}
            </div>
          )}
        </section>
        <section className="projection-detail" aria-labelledby="projection-categories-heading">
          <div className="projection-detail-heading">
            <div>
              <p className="eyebrow">Selected period</p>
              <h3 id="projection-categories-heading">Category activity</h3>
            </div>
            <a href="#transactions-heading">Review transactions</a>
          </div>
          <CategoryGroup
            categories={expenseCategories}
            currency={currency}
            empty="No net expense activity in this period."
            label="Spending"
          />
          <CategoryGroup
            categories={incomeCategories}
            currency={currency}
            empty="No net income activity in this period."
            label="Income"
          />
        </section>
      </div>
    </>
  );
}

function CurrentBudgetProgress({ budget }: { budget: MonthlyBudget }) {
  const progress = budget.planned_base_minor > 0
    ? Math.max(0, Math.min(100, budget.used_base_minor / budget.planned_base_minor * 100))
    : 0;
  return (
    <section className="current-budget-progress" aria-labelledby="current-budget-progress-heading">
      <div>
        <p className="eyebrow">{monthLabel(budget.month)} plan</p>
        <h3 id="current-budget-progress-heading">{budget.name}</h3>
        <small>Posted category allocations only</small>
      </div>
      <div className="current-budget-meter">
        <div>
          <strong>{formatMoney(budget.used_base_minor, budget.base_currency)} used</strong>
          <span>{formatMoney(budget.remaining_base_minor, budget.base_currency)} remaining</span>
        </div>
        <progress aria-label="Current monthly budget usage" max={100} value={progress} />
        <span>of {formatMoney(budget.planned_base_minor, budget.base_currency)} planned</span>
      </div>
      <a href="#monthly-budget-heading">Review budget</a>
    </section>
  );
}

function SummaryCard({
  label,
  amounts,
  currency,
  pendingLabel,
}: {
  label: string;
  amounts: FinancialProjection["summary"]["balance_base_minor"];
  currency: Currency;
  pendingLabel: string;
}) {
  return (
    <article className="projection-summary-card">
      <span>{label}</span>
      <strong>{formatMoney(amounts.posted, currency)}</strong>
      <small>Posted</small>
      <div>
        <span>{pendingLabel}</span>
        <b>{signedMoney(amounts.pending, currency)}</b>
      </div>
      <div>
        <span>Projected total</span>
        <b>{formatMoney(amounts.projected, currency)}</b>
      </div>
    </article>
  );
}

function CategoryGroup({
  categories,
  currency,
  empty,
  label,
}: {
  categories: FinancialProjection["categories"];
  currency: Currency;
  empty: string;
  label: string;
}) {
  const active = categories.filter(
    (category) => category.rolled_up_base_minor.posted !== 0 || category.rolled_up_base_minor.pending !== 0,
  );
  return (
    <div className="category-report">
      <h4>{label}</h4>
      {active.length === 0 ? <p className="resource-state">{empty}</p> : null}
      {active.map((category) => (
        <article className="projection-row" key={category.id}>
          <div style={{ paddingInlineStart: `${categoryDepth(category, categories) * 1.1}rem` }}>
            <strong>{category.icon ? `${category.icon} ` : ""}{category.name}</strong>
            <small>
              {categories.some((candidate) => candidate.parent_id === category.id)
                ? "Includes subcategories"
                : "Category total"}
              {category.archived_at ? " · archived" : ""}
            </small>
          </div>
          <div className="projection-amount">
            <strong>{formatMoney(category.rolled_up_base_minor.posted, currency)}</strong>
            {category.rolled_up_base_minor.pending !== 0 ? (
              <small>{signedMoney(category.rolled_up_base_minor.pending, currency)} pending</small>
            ) : null}
          </div>
        </article>
      ))}
    </div>
  );
}

function ProjectionError({ error, retry }: { error: Error; retry: () => void }) {
  const message = error instanceof APIError ? error.message : "The financial overview could not be loaded.";
  return (
    <div className="projection-error" role="alert">
      <p>{message}</p>
      <button className="secondary-button" type="button" onClick={retry}>Try again</button>
    </div>
  );
}

function signedMoney(amount: number, currency: Currency): string {
  const formatted = formatMoney(amount, currency);
  return amount > 0 && formatted !== "Amount unavailable" ? `+${formatted}` : formatted;
}

function formatDate(value: string): string {
  const [year, month, day] = value.split("-").map(Number);
  if (!year || !month || !day) return value;
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric" })
    .format(new Date(Date.UTC(year, month - 1, day)));
}

function categoryDepth(
  category: FinancialProjection["categories"][number],
  categories: FinancialProjection["categories"],
): number {
  const byId = new Map(categories.map((candidate) => [candidate.id, candidate]));
  const visited = new Set([category.id]);
  let parentId = category.parent_id;
  let depth = 0;
  while (parentId && !visited.has(parentId) && depth < categories.length) {
    visited.add(parentId);
    depth += 1;
    parentId = byId.get(parentId)?.parent_id;
  }
  return depth;
}
