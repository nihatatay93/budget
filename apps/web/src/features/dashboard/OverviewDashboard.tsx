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
      <div className="overview-dashboard" aria-label="Loading overview">
        <LoadingState label="Loading financial overview" rows={4} />
        <div className="overview-loading-grid">
          <LoadingState label="Loading monthly plan" rows={3} />
          <LoadingState label="Loading recent activity" rows={3} />
        </div>
      </div>
    );
  }

  if (projection.isError) {
    return (
      <InlineNotice
        action={<button className="secondary-button" onClick={() => void projection.refetch()} type="button">Try again</button>}
        title="Overview unavailable"
        tone="danger"
      >
        {projection.error instanceof APIError
          ? projection.error.message
          : "Your financial overview could not be loaded."}
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
            <p>Posted balance</p>
            <span>{formatDateRange(period.from_date, period.to_date)}</span>
          </div>
          <StatusBadge tone="positive">{period.base_currency}</StatusBadge>
        </div>
        <h2 id="overview-position-heading">
          <MoneyAmount amount={summary.balance_base_minor.posted} currency={period.base_currency} emphasis="hero" />
        </h2>
        <div className="overview-balance-context">
          <span>Pending delta</span>
          <MoneyAmount amount={pending} currency={period.base_currency} signed />
          <span>Projected</span>
          <MoneyAmount amount={summary.balance_base_minor.projected} currency={period.base_currency} />
        </div>
      </article>
      <SummaryTile
        amount={summary.income_base_minor.posted}
        currency={period.base_currency}
        label="Income"
        pending={summary.income_base_minor.pending}
        tone="income"
      />
      <SummaryTile
        amount={summary.spending_base_minor.posted}
        currency={period.base_currency}
        label="Spending"
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
        {pending === 0 ? "No pending activity" : `${formatMoney(pending, currency)} pending`}
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
    <nav className="overview-quick-actions" aria-label="Overview actions">
      {actions.map((action) => (
        <Link key={action.to} to={`/workspaces/${workspaceId}/${action.to}`}>
          <span><AppIcon name={action.icon} /></span>{action.label}
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
        eyebrow={`${monthLabel(month)} plan`}
        link="Open budget"
        linkTo={`/workspaces/${workspaceId}/budget`}
        title="Monthly budget"
        titleId="overview-budget-heading"
      />
      {isPending ? <LoadingState label="Loading monthly budget" rows={3} /> : null}
      {error && !missing ? <InlineNotice tone="danger">{error.message}</InlineNotice> : null}
      {missing ? (
        <EmptyState
          action={<Link to={`/workspaces/${workspaceId}/budget`}>Create plan</Link>}
          description="Set category targets to compare your posted spending with a plan."
          icon="budget"
          title="No plan for this month"
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
          <span>Used</span>
          <MoneyAmount amount={budget.used_base_minor} currency={budget.base_currency} />
        </div>
        <div>
          <span>Remaining</span>
          <MoneyAmount amount={budget.remaining_base_minor} currency={budget.base_currency} />
        </div>
      </div>
      <ProgressMeter label="Current monthly budget usage" tone={tone} value={percent} />
      <p>of {formatMoney(budget.planned_base_minor, budget.base_currency)} planned</p>
      <div className="overview-budget-items">
        {budget.items.slice(0, 3).map((item) => (
          <div key={item.id}>
            <span>{item.category_icon ? `${item.category_icon} ` : ""}{item.category_name}</span>
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
        eyebrow="Latest ledger activity"
        link="View all"
        linkTo={`/workspaces/${workspaceId}/transactions`}
        title="Recent transactions"
        titleId="overview-activity-heading"
      />
      {isPending ? <LoadingState label="Loading recent transactions" rows={4} /> : null}
      {error ? <InlineNotice tone="danger">{error.message}</InlineNotice> : null}
      {!isPending && !error && recent.length === 0 ? (
        <EmptyState compact icon="transactions" title="No transactions yet" />
      ) : null}
      <div className="overview-transaction-list">
        {recent.map((transaction) => (
          <article key={transaction.id}>
            <span aria-hidden="true" className={`transaction-direction transaction-direction-${transactionDirection(transaction)}`}>
              <AppIcon name={transaction.kind === "transfer" ? "accounts" : "transactions"} />
            </span>
            <div>
              <strong>{transaction.payee ?? transaction.description ?? transactionLabel(transaction.kind)}</strong>
              <small>{formatDate(transaction.transaction_date)} · {transaction.status}</small>
            </div>
            <div className="overview-transaction-amount">
              {transaction.kind === "transfer" ? (
                <span>Transfer</span>
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
        eyebrow="Cumulative balances"
        link="Manage"
        linkTo={`/workspaces/${workspaceId}/accounts`}
        title="Accounts"
        titleId="overview-accounts-heading"
      />
      {accounts.length === 0 ? <EmptyState compact icon="accounts" title="No accounts yet" /> : null}
      <div className="overview-highlight-list">
        {accounts.map((account) => (
          <article key={account.id}>
            <div><strong>{account.name}</strong><small>{account.type.replaceAll("_", " ")}</small></div>
            <div>
              <MoneyAmount amount={account.native_balance_minor.posted} currency={account.currency} />
              {account.currency !== projection.period.base_currency ? (
                <small>{formatMoney(account.base_balance_minor.posted, projection.period.base_currency)} base</small>
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
        eyebrow="This period"
        link="Open reports"
        linkTo={`/workspaces/${workspaceId}/reports`}
        title="Top spending"
        titleId="overview-categories-heading"
      />
      {categories.length === 0 ? <EmptyState compact icon="categories" title="No posted spending yet" /> : null}
      <div className="overview-category-list">
        {categories.map((category) => (
          <article key={category.id}>
            <div>
              <span>{category.icon ? `${category.icon} ` : ""}{category.name}</span>
              <MoneyAmount amount={category.rolled_up_base_minor.posted} currency={projection.period.base_currency} />
            </div>
            <ProgressMeter
              label={`${category.name} relative spending`}
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
  return kind === "standard" ? "Transaction" : kind === "transfer" ? "Transfer" : "Adjustment";
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
