import { useQuery } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Link } from "react-router-dom";

import {
  APIError,
  type FinancialProjection,
  type FinancialProjectionRange,
  financialProjectionQueryKey,
  getFinancialProjection,
} from "../../api/client";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  MoneyAmount,
  StatusBadge,
} from "../../components/Presentation";
import { type Currency, formatMoney } from "../../lib/currency";

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
          <button className="secondary-button" type="submit">Apply range</button>
          {range ? (
            <button className="text-button" type="button" onClick={resetRange}>
              Current month
            </button>
          ) : null}
        </form>
      </div>
      {rangeError ? <p className="form-error" role="alert">{rangeError}</p> : null}
      {query.isLoading ? <LoadingState label="Loading financial report" rows={6} /> : null}
      {query.isError ? <ProjectionError error={query.error} retry={() => query.refetch()} /> : null}
      {query.data ? (
        <ProjectionContent projection={query.data} workspaceId={workspace.id} />
      ) : null}
    </section>
  );
}

function ProjectionContent({
  projection,
  workspaceId,
}: {
  projection: FinancialProjection;
  workspaceId: string;
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
      <IncomeSpendingComparison projection={projection} />
      <ReportHelp workspaceId={workspaceId} />
      <div className="projection-detail-grid">
        <section className="projection-detail" aria-labelledby="projection-accounts-heading">
          <div className="projection-detail-heading">
            <div>
              <p className="eyebrow">Cumulative through period end</p>
              <h3 id="projection-accounts-heading">Account balances</h3>
            </div>
            <Link to={`/workspaces/${workspaceId}/accounts`}>Manage accounts</Link>
          </div>
          {projection.accounts.length === 0 ? (
            <EmptyState
              compact
              description="Create an account before recording financial activity."
              icon="accounts"
              title="No accounts yet"
            />
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
            <Link to={`/workspaces/${workspaceId}/transactions`}>Review transactions</Link>
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

function IncomeSpendingComparison({ projection }: { projection: FinancialProjection }) {
  const currency = projection.period.base_currency;
  const income = projection.summary.income_base_minor.posted;
  const spending = projection.summary.spending_base_minor.posted;
  const maximum = Math.max(Math.abs(income), Math.abs(spending), 1);
  const net = income - spending;
  return (
    <section className="report-comparison" aria-labelledby="report-comparison-heading">
      <div>
        <p className="eyebrow">Selected period</p>
        <h3 id="report-comparison-heading">Income and spending</h3>
        <p>Posted allocations only. Transfers and pending activity stay outside these totals.</p>
      </div>
      <div className="report-comparison-bars">
        <ReportBar amount={income} currency={currency} label="Income" maximum={maximum} tone="income" />
        <ReportBar amount={spending} currency={currency} label="Spending" maximum={maximum} tone="spending" />
      </div>
      <div className="report-net-result">
        <StatusBadge tone={net >= 0 ? "positive" : "warning"}>Net cash flow</StatusBadge>
        <strong><MoneyAmount amount={net} currency={currency} signed /></strong>
      </div>
    </section>
  );
}

function ReportBar({ amount, currency, label, maximum, tone }: {
  amount: number;
  currency: Currency;
  label: string;
  maximum: number;
  tone: "income" | "spending";
}) {
  const width = Math.abs(amount) / maximum * 100;
  return (
    <div className="report-bar-row">
      <div>
        <span>{label}</span>
        <strong><MoneyAmount amount={amount} currency={currency} /></strong>
      </div>
      <div
        aria-label={`${label}: ${formatMoney(amount, currency)}`}
        className="report-bar-track"
        role="img"
      >
        <span className={`report-bar-fill report-bar-${tone}`} style={{ width: `${width}%` }} />
      </div>
    </div>
  );
}

function CategoryVisual({
  amount,
  currency,
  label,
  maximum,
}: {
  amount: number;
  currency: Currency;
  label: string;
  maximum: number;
}) {
  return (
    <div
      aria-label={`${label}: ${formatMoney(amount, currency)}`}
      className="category-activity-track"
      role="img"
    >
      <span style={{ width: `${maximum > 0 ? Math.abs(amount) / maximum * 100 : 0}%` }} />
    </div>
  );
}

function ReportReadout({
  amount,
  currency,
  pending,
}: {
  amount: number;
  currency: Currency;
  pending: number;
}) {
  return (
    <div className="projection-amount">
      <strong>{formatMoney(amount, currency)}</strong>
      {pending !== 0 ? <small>{signedMoney(pending, currency)} pending</small> : null}
    </div>
  );
}

function ReportHelp({ workspaceId }: { workspaceId: string }) {
  return (
    <InlineNotice
      action={<Link to={`/workspaces/${workspaceId}/budget`}>Review monthly plan</Link>}
      title="Reports explain what happened"
    >
      <p>Use Budget to plan category targets; this view stays focused on ledger-derived results.</p>
    </InlineNotice>
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
      <strong><MoneyAmount amount={amounts.posted} currency={currency} /></strong>
      <small>Posted</small>
      <div>
        <span>{pendingLabel}</span>
        <b>{signedMoney(amounts.pending, currency)}</b>
      </div>
      <div>
        <span>Projected total</span>
        <b><MoneyAmount amount={amounts.projected} currency={currency} /></b>
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
  const maximum = Math.max(
    ...active.map((category) => Math.abs(category.rolled_up_base_minor.posted)),
    1,
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
          <div className="projection-category-value">
            <CategoryVisual
              amount={category.rolled_up_base_minor.posted}
              currency={currency}
              label={`${category.name} posted ${label.toLowerCase()}`}
              maximum={maximum}
            />
            <ReportReadout
              amount={category.rolled_up_base_minor.posted}
              currency={currency}
              pending={category.rolled_up_base_minor.pending}
            />
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
