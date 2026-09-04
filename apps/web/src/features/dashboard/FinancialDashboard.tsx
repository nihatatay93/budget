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
import { CategoryLabel } from "../../components/CategoryAppearance";
import { type Currency, formatMoney } from "../../lib/currency";
import { categoryName, t } from "../../lib/i18n";

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
      setRangeError(t("Choose a start date that is on or before the end date."));
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
          <p className="eyebrow">{t("Ledger-derived overview")}</p>
          <h2 id="financial-overview-heading">{t("Financial overview")}</h2>
          <p className="projection-caption">
            {t("Posted figures are authoritative. Pending activity is shown separately.")}
          </p>
        </div>
        <form className="projection-range" onSubmit={applyRange}>
          <label>
            {t("From")}
            <input
              aria-label={t("Projection start date")}
              type="date"
              value={fromDate}
              onChange={(event) => setFromDate(event.target.value)}
            />
          </label>
          <label>
            {t("To")}
            <input
              aria-label={t("Projection end date")}
              type="date"
              value={toDate}
              onChange={(event) => setToDate(event.target.value)}
            />
          </label>
          <button className="secondary-button" type="submit">{t("Apply range")}</button>
          {range ? (
            <button className="text-button" type="button" onClick={resetRange}>
              {t("Current month")}
            </button>
          ) : null}
        </form>
      </div>
      {rangeError ? <p className="form-error" role="alert">{rangeError}</p> : null}
      {query.isLoading ? <LoadingState label={t("Loading financial report")} rows={6} /> : null}
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
          label={t("Balance")}
          amounts={projection.summary.balance_base_minor}
          currency={currency}
          pendingLabel={t("Pending delta")}
        />
        <SummaryCard
          label={t("Income")}
          amounts={projection.summary.income_base_minor}
          currency={currency}
          pendingLabel={t("Pending income")}
        />
        <SummaryCard
          label={t("Spending")}
          amounts={projection.summary.spending_base_minor}
          currency={currency}
          pendingLabel={t("Pending spending")}
        />
      </div>
      <IncomeSpendingComparison projection={projection} />
      <ReportHelp workspaceId={workspaceId} />
      <div className="projection-detail-grid">
        <section className="projection-detail" aria-labelledby="projection-accounts-heading">
          <div className="projection-detail-heading">
            <div>
              <p className="eyebrow">{t("Cumulative through period end")}</p>
              <h3 id="projection-accounts-heading">{t("Account balances")}</h3>
            </div>
            <Link to={`/workspaces/${workspaceId}/accounts`}>{t("Manage accounts")}</Link>
          </div>
          {projection.accounts.length === 0 ? (
            <EmptyState
              compact
              description={t("Create an account before recording financial activity.")}
              icon="accounts"
              title={t("No accounts yet")}
            />
          ) : (
            <div className="projection-list">
              {projection.accounts.map((account) => (
                <article className="projection-row" key={account.id}>
                  <div>
                    <strong>{account.name}</strong>
                    <small>
                      {t(`account.type.${account.type}`)}
                      {account.archived_at ? ` · ${t("Archived")}` : ""}
                    </small>
                  </div>
                  <div className="projection-amount">
                    <strong>{formatMoney(account.native_balance_minor.posted, account.currency)}</strong>
                    {account.native_balance_minor.pending !== 0 ? (
                      <small>
                        {t("{amount} pending · {projected} projected", { amount: signedMoney(account.native_balance_minor.pending, account.currency), projected: formatMoney(account.native_balance_minor.projected, account.currency) })}
                      </small>
                    ) : null}
                    {account.currency !== currency ? (
                      <small>{t("{amount} base", { amount: formatMoney(account.base_balance_minor.posted, currency) })}</small>
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
              <p className="eyebrow">{t("Selected period")}</p>
              <h3 id="projection-categories-heading">{t("Category activity")}</h3>
            </div>
            <Link to={`/workspaces/${workspaceId}/transactions`}>{t("Review transactions")}</Link>
          </div>
          <CategoryGroup
            categories={expenseCategories}
            currency={currency}
            empty={t("No net expense activity in this period.")}
            label={t("Spending")}
          />
          <CategoryGroup
            categories={incomeCategories}
            currency={currency}
            empty={t("No net income activity in this period.")}
            label={t("Income")}
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
        <p className="eyebrow">{t("Selected period")}</p>
        <h3 id="report-comparison-heading">{t("Income and spending")}</h3>
        <p>{t("Posted allocations only. Transfers and pending activity stay outside these totals.")}</p>
      </div>
      <div className="report-comparison-bars">
        <ReportBar amount={income} currency={currency} label={t("Income")} maximum={maximum} tone="income" />
        <ReportBar amount={spending} currency={currency} label={t("Spending")} maximum={maximum} tone="spending" />
      </div>
      <div className="report-net-result">
        <StatusBadge tone={net >= 0 ? "positive" : "warning"}>{t("Net cash flow")}</StatusBadge>
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
      {pending !== 0 ? <small>{t("{amount} pending", { amount: signedMoney(pending, currency) })}</small> : null}
    </div>
  );
}

function ReportHelp({ workspaceId }: { workspaceId: string }) {
  return (
    <InlineNotice
      action={<Link to={`/workspaces/${workspaceId}/budget`}>{t("Review monthly plan")}</Link>}
      title={t("Reports explain what happened")}
    >
      <p>{t("Use Budget to plan category targets; this view stays focused on ledger-derived results.")}</p>
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
      <small>{t("Posted")}</small>
      <div>
        <span>{pendingLabel}</span>
        <b>{signedMoney(amounts.pending, currency)}</b>
      </div>
      <div>
        <span>{t("Projected total")}</span>
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
            <strong><CategoryLabel colorKey={category.color_key} iconType={category.icon_type} iconValue={category.icon_value ?? category.icon} name={categoryName(category)} /></strong>
            <small>
              {categories.some((candidate) => candidate.parent_id === category.id)
                ? t("Includes subcategories")
                : t("Category total")}
              {category.archived_at ? ` · ${t("Archived")}` : ""}
            </small>
          </div>
          <div className="projection-category-value">
            <CategoryVisual
              amount={category.rolled_up_base_minor.posted}
              currency={currency}
              label={`${categoryName(category)} posted ${label.toLowerCase()}`}
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
  const message = error instanceof APIError ? error.message : t("The financial overview could not be loaded.");
  return (
    <div className="projection-error" role="alert">
      <p>{message}</p>
      <button className="secondary-button" type="button" onClick={retry}>{t("Try again")}</button>
    </div>
  );
}

function signedMoney(amount: number, currency: Currency): string {
  const formatted = formatMoney(amount, currency);
  return amount > 0 && Number.isSafeInteger(amount) ? `+${formatted}` : formatted;
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
