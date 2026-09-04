import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import {
  APIError,
  type FinancialProjection,
  type MonthlyBudget,
  type SessionResponse,
  type Transaction,
  financialProjectionQueryKey,
  getFinancialProjection,
  getMonthlyBudget,
  listTransactions,
  monthlyBudgetQueryKey,
  transactionsQueryKey,
} from "../../api/client";
import { AppIcon } from "../../components/ExperiencePrimitives";
import { CategoryLabel } from "../../components/CategoryAppearance";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  MoneyAmount,
  ProgressMeter,
  StatusBadge,
  SurfaceCard,
} from "../../components/Presentation";
import { type Currency, formatMoney } from "../../lib/currency";
import { categoryName, t } from "../../lib/i18n";
import { monthLabel, workspaceMonth } from "../../lib/month";

type Workspace = SessionResponse["workspaces"][number];

export function OverviewDashboard({
  canManage,
  workspace,
}: {
  canManage: boolean;
  workspace: Workspace;
}) {
  const projection = useQuery({
    queryKey: financialProjectionQueryKey(workspace.id),
    queryFn: () => getFinancialProjection(workspace.id),
  });
  const currentMonth = workspaceMonth(workspace.timezone);
  const budget = useQuery({
    queryKey: monthlyBudgetQueryKey(workspace.id, currentMonth),
    queryFn: () => getMonthlyBudget(workspace.id, currentMonth),
    retry: false,
  });
  const transactions = useQuery({
    queryKey: transactionsQueryKey(workspace.id),
    queryFn: () => listTransactions(workspace.id),
  });

  if (projection.isPending) {
    return (
      <div className="overview-dashboard" aria-label={t("Loading overview")}>
        <LoadingState label={t("Loading financial overview")} rows={4} />
        <div className="overview-loading-grid">
          <LoadingState label={t("Loading monthly plan")} rows={3} />
          <LoadingState label={t("Loading recent activity")} rows={3} />
        </div>
      </div>
    );
  }

  if (projection.isError) {
    return (
      <InlineNotice
        action={<button className="secondary-button" onClick={() => void projection.refetch()} type="button">{t("Try again")}</button>}
        title={t("Overview unavailable")}
        tone="danger"
      >
        {projection.error instanceof APIError
          ? projection.error.message
          : t("Your financial overview could not be loaded.")}
      </InlineNotice>
    );
  }

  return (
    <div className="overview-dashboard">
      <OverviewSummary projection={projection.data} />
      <QuickActions canManage={canManage} workspaceId={workspace.id} />
      <div className="overview-main-grid">
        <BudgetSnapshot
          budget={budget.data}
          error={budget.error}
          isPending={budget.isPending}
          month={currentMonth}
          workspaceId={workspace.id}
        />
        <RecentActivity
          currency={workspace.base_currency}
          error={transactions.error}
          isPending={transactions.isPending}
          transactions={transactions.data ?? []}
          workspaceId={workspace.id}
        />
      </div>
      <div className="overview-detail-grid">
        <AccountHighlights projection={projection.data} workspaceId={workspace.id} />
        <CategoryHighlights projection={projection.data} workspaceId={workspace.id} />
      </div>
    </div>
  );
}

function OverviewSummary({ projection }: { projection: FinancialProjection }) {
  const { summary, period } = projection;
  const pending = summary.balance_base_minor.pending;
  return (
    <section className="overview-summary" aria-labelledby="overview-position-heading">
      <article className="overview-balance-card">
        <div className="overview-card-heading">
          <div>
            <p>{t("Posted balance")}</p>
            <span>{formatDateRange(period.from_date, period.to_date)}</span>
          </div>
          <StatusBadge tone="positive">{period.base_currency}</StatusBadge>
        </div>
        <h2 id="overview-position-heading">
          <MoneyAmount amount={summary.balance_base_minor.posted} currency={period.base_currency} emphasis="hero" />
        </h2>
        <div className="overview-balance-context">
          <span>{t("Pending delta")}</span>
          <MoneyAmount amount={pending} currency={period.base_currency} signed />
          <span>{t("Projected")}</span>
          <MoneyAmount amount={summary.balance_base_minor.projected} currency={period.base_currency} />
        </div>
      </article>
      <SummaryTile
        amount={summary.income_base_minor.posted}
        currency={period.base_currency}
        label={t("Income")}
        pending={summary.income_base_minor.pending}
        tone="income"
      />
      <SummaryTile
        amount={summary.spending_base_minor.posted}
        currency={period.base_currency}
        label={t("Spending")}
        pending={summary.spending_base_minor.pending}
        tone="spending"
      />
    </section>
  );
}

function SummaryTile({
  amount,
  currency,
  label,
  pending,
  tone,
}: {
  amount: number;
  currency: Currency;
  label: string;
  pending: number;
  tone: "income" | "spending";
}) {
  return (
    <article className={`overview-summary-tile overview-summary-${tone}`}>
      <div>
        <span>{label}</span>
        <span aria-hidden="true" className="overview-summary-icon">
          <AppIcon name={tone === "income" ? "chart" : "transactions"} />
        </span>
      </div>
      <MoneyAmount amount={amount} currency={currency} emphasis="hero" />
      <small>
        {pending === 0 ? t("No pending activity") : t("{amount} pending", { amount: formatMoney(pending, currency) })}
      </small>
    </article>
  );
}

function QuickActions({ canManage, workspaceId }: { canManage: boolean; workspaceId: string }) {
  const actions = canManage
    ? [
        { to: "transactions", icon: "transactions" as const, label: "Add transaction" },
        { to: "budget", icon: "budget" as const, label: "Plan this month" },
        { to: "accounts", icon: "accounts" as const, label: "Add account" },
      ]
    : [
        { to: "transactions", icon: "transactions" as const, label: "Review transactions" },
        { to: "budget", icon: "budget" as const, label: "View budget" },
        { to: "accounts", icon: "accounts" as const, label: "View accounts" },
      ];
  return (
    <nav className="overview-quick-actions" aria-label={t("Overview actions")}>
      {actions.map((action) => (
        <Link key={action.to} to={`/workspaces/${workspaceId}/${action.to}`}>
          <span><AppIcon name={action.icon} /></span>{t(action.label)}
        </Link>
      ))}
    </nav>
  );
}

function BudgetSnapshot({
  budget,
  error,
  isPending,
  month,
  workspaceId,
}: {
  budget?: MonthlyBudget;
  error: Error | null;
  isPending: boolean;
  month: string;
  workspaceId: string;
}) {
  const missing = error instanceof APIError && error.status === 404;
  return (
    <SurfaceCard className="overview-panel" labelledBy="overview-budget-heading">
      <PanelHeading
        eyebrow={t("{month} plan", { month: monthLabel(month) })}
        link={t("Open budget")}
        linkTo={`/workspaces/${workspaceId}/budget`}
        title={t("Monthly budget")}
        titleId="overview-budget-heading"
      />
      {isPending ? <LoadingState label={t("Loading monthly budget")} rows={3} /> : null}
      {error && !missing ? <InlineNotice tone="danger">{error.message}</InlineNotice> : null}
      {missing ? (
        <EmptyState
          action={<Link to={`/workspaces/${workspaceId}/budget`}>{t("Create plan")}</Link>}
          description={t("Set category targets to compare your posted spending with a plan.")}
          icon="budget"
          title={t("No plan for this month")}
        />
      ) : null}
      {budget ? <BudgetContent budget={budget} /> : null}
    </SurfaceCard>
  );
}

function BudgetContent({ budget }: { budget: MonthlyBudget }) {
  const percent = budget.planned_base_minor > 0
    ? budget.used_base_minor / budget.planned_base_minor * 100
    : 0;
  const tone = budget.remaining_base_minor < 0 ? "danger" as const : "positive" as const;
  return (
    <div className="overview-budget-content">
      <div className="overview-budget-total">
        <div>
          <span>{t("Used")}</span>
          <MoneyAmount amount={budget.used_base_minor} currency={budget.base_currency} />
        </div>
        <div>
          <span>{t("Remaining")}</span>
          <MoneyAmount amount={budget.remaining_base_minor} currency={budget.base_currency} />
        </div>
      </div>
      <ProgressMeter label={t("Current monthly budget usage")} tone={tone} value={percent} />
      <p>{t("of {amount} planned", { amount: formatMoney(budget.planned_base_minor, budget.base_currency) })}</p>
      <div className="overview-budget-items">
        {budget.items.slice(0, 3).map((item) => (
          <div key={item.id}>
            <CategoryLabel colorKey={item.category_color_key} iconType={item.category_icon_type} iconValue={item.category_icon_value ?? item.category_icon} name={categoryName({ name: item.category_name, predefined_key: item.category_predefined_key })} />
            <strong>{formatMoney(item.used_base_minor, budget.base_currency)}</strong>
          </div>
        ))}
      </div>
    </div>
  );
}

function RecentActivity({
  currency,
  error,
  isPending,
  transactions,
  workspaceId,
}: {
  currency: Currency;
  error: Error | null;
  isPending: boolean;
  transactions: Transaction[];
  workspaceId: string;
}) {
  const recent = [...transactions]
    .sort((left, right) => right.transaction_date.localeCompare(left.transaction_date))
    .slice(0, 5);
  return (
    <SurfaceCard className="overview-panel" labelledBy="overview-activity-heading">
      <PanelHeading
        eyebrow={t("Latest ledger activity")}
        link={t("View all")}
        linkTo={`/workspaces/${workspaceId}/transactions`}
        title={t("Recent transactions")}
        titleId="overview-activity-heading"
      />
      {isPending ? <LoadingState label={t("Loading recent transactions")} rows={4} /> : null}
      {error ? <InlineNotice tone="danger">{error.message}</InlineNotice> : null}
      {!isPending && !error && recent.length === 0 ? (
        <EmptyState compact icon="transactions" title={t("No transactions yet")} />
      ) : null}
      <div className="overview-transaction-list">
        {recent.map((transaction) => (
          <article key={transaction.id}>
            <span aria-hidden="true" className={`transaction-direction transaction-direction-${transactionDirection(transaction)}`}>
              <AppIcon name={transaction.kind === "transfer" ? "accounts" : "transactions"} />
            </span>
            <div>
              <strong>{transaction.payee ?? transaction.description ?? transactionLabel(transaction.kind)}</strong>
              <small>{formatDate(transaction.transaction_date)} · {t(transaction.status === "pending" ? "Pending" : "Posted")}</small>
            </div>
            <div className="overview-transaction-amount">
              {transaction.kind === "transfer" ? (
                <span>{t("Transfer")}</span>
              ) : (
                <MoneyAmount amount={transactionTotal(transaction)} currency={currency} signed />
              )}
            </div>
          </article>
        ))}
      </div>
    </SurfaceCard>
  );
}

function AccountHighlights({ projection, workspaceId }: { projection: FinancialProjection; workspaceId: string }) {
  const accounts = [...projection.accounts]
    .sort((left, right) => Math.abs(right.base_balance_minor.posted) - Math.abs(left.base_balance_minor.posted))
    .slice(0, 4);
  return (
    <SurfaceCard className="overview-panel" labelledBy="overview-accounts-heading">
      <PanelHeading
        eyebrow={t("Cumulative balances")}
        link={t("Manage")}
        linkTo={`/workspaces/${workspaceId}/accounts`}
        title={t("Accounts")}
        titleId="overview-accounts-heading"
      />
      {accounts.length === 0 ? <EmptyState compact icon="accounts" title={t("No accounts yet")} /> : null}
      <div className="overview-highlight-list">
        {accounts.map((account) => (
          <article key={account.id}>
            <div><strong>{account.name}</strong><small>{t(`account.type.${account.type}`)}</small></div>
            <div>
              <MoneyAmount amount={account.native_balance_minor.posted} currency={account.currency} />
              {account.currency !== projection.period.base_currency ? (
                <small>{t("{amount} base", { amount: formatMoney(account.base_balance_minor.posted, projection.period.base_currency) })}</small>
              ) : null}
            </div>
          </article>
        ))}
      </div>
    </SurfaceCard>
  );
}

function CategoryHighlights({ projection, workspaceId }: { projection: FinancialProjection; workspaceId: string }) {
  const categories = projection.categories
    .filter((category) => category.kind === "expense" && category.rolled_up_base_minor.posted !== 0)
    .sort((left, right) => right.rolled_up_base_minor.posted - left.rolled_up_base_minor.posted)
    .slice(0, 4);
  const max = Math.max(1, ...categories.map((category) => Math.max(0, category.rolled_up_base_minor.posted)));
  return (
    <SurfaceCard className="overview-panel" labelledBy="overview-categories-heading">
      <PanelHeading
        eyebrow={t("This period")}
        link={t("Open reports")}
        linkTo={`/workspaces/${workspaceId}/reports`}
        title={t("Top spending")}
        titleId="overview-categories-heading"
      />
      {categories.length === 0 ? <EmptyState compact icon="categories" title={t("No posted spending yet")} /> : null}
      <div className="overview-category-list">
        {categories.map((category) => (
          <article key={category.id}>
            <div>
              <CategoryLabel colorKey={category.color_key} iconType={category.icon_type} iconValue={category.icon_value ?? category.icon} name={categoryName(category)} />
              <MoneyAmount amount={category.rolled_up_base_minor.posted} currency={projection.period.base_currency} />
            </div>
            <ProgressMeter
              label={t("{category} relative spending", { category: categoryName(category) })}
              max={max}
              value={Math.max(0, category.rolled_up_base_minor.posted)}
            />
          </article>
        ))}
      </div>
    </SurfaceCard>
  );
}

function PanelHeading({
  eyebrow,
  link,
  linkTo,
  title,
  titleId,
}: {
  eyebrow: string;
  link: string;
  linkTo: string;
  title: string;
  titleId: string;
}) {
  return (
    <header className="overview-panel-heading">
      <div><p className="eyebrow">{eyebrow}</p><h2 id={titleId}>{title}</h2></div>
      <Link to={linkTo}>{link}</Link>
    </header>
  );
}

function transactionTotal(transaction: Transaction): number {
  let total = 0;
  for (const entry of transaction.entries) {
    const result = total + entry.base_amount_minor;
    if (!Number.isSafeInteger(result)) return 0;
    total = result;
  }
  return total;
}

function transactionDirection(transaction: Transaction) {
  if (transaction.kind === "transfer") return "transfer";
  const total = transactionTotal(transaction);
  return total > 0 ? "income" : total < 0 ? "expense" : "neutral";
}

function transactionLabel(kind: Transaction["kind"]) {
  return kind === "standard" ? t("Transaction") : kind === "transfer" ? t("Transfer") : t("Adjustment");
}

function formatDate(value: string) {
  const [year, month, day] = value.split("-").map(Number);
  if (!year || !month || !day) return value;
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" })
    .format(new Date(Date.UTC(year, month - 1, day)));
}

function formatDateRange(from: string, to: string) {
  return `${formatDate(from)}–${formatDate(to)}`;
}
