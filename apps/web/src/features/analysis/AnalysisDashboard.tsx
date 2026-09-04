import { useQuery } from "@tanstack/react-query";
import { type FormEvent, type ReactNode, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import {
  APIError,
  type AnalysisGranularity,
  type SpendingAnalysis,
  type SpendingAnalysisRange,
  getSpendingAnalysis,
  spendingAnalysisQueryKey,
} from "../../api/client";
import {
  CategoryLabel,
  categoryAccentColor,
} from "../../components/CategoryAppearance";
import {
  DonutChart,
  HeatmapGrid,
  MeterBar,
  Sparkline,
  TrendChart,
  type TrendSeries,
} from "../../components/Charts";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  MoneyAmount,
  StatusBadge,
  SurfaceCard,
} from "../../components/Presentation";
import {
  type CategoryNode,
  type ISODate,
  type RangePresetKey,
  analysisGranularities,
  analysisInsights,
  averagePerDay,
  averagePerTransaction,
  calendarWeeks,
  categoryBreakdown,
  categorySeries,
  categorySubtreeIds,
  deltaRatio,
  formatPercent,
  granularityLabel,
  granularityNoun,
  rangePresets,
  savingsRate,
  transactionCountLabel,
  weekdayReadings,
  workspaceToday,
} from "../../lib/analysis";
import { type Currency, formatMoney } from "../../lib/currency";
import { categoryName, t } from "../../lib/i18n";

type Workspace = {
  id: string;
  name: string;
  base_currency: Currency;
  timezone: string;
};

const TOP_CATEGORY_LIMIT = 8;

export function AnalysisDashboard({ workspace }: { workspace: Workspace }) {
  const today = workspaceToday(workspace.timezone);
  const presets = useMemo(() => rangePresets(today), [today]);
  const [presetKey, setPresetKey] = useState<RangePresetKey | "custom">("last-6-months");
  const [granularity, setGranularity] = useState<AnalysisGranularity | "auto">("auto");
  const [customFrom, setCustomFrom] = useState("");
  const [customTo, setCustomTo] = useState("");
  const [customRange, setCustomRange] = useState<SpendingAnalysisRange>();
  const [rangeError, setRangeError] = useState("");

  const selectedPreset = presets.find((preset) => preset.key === presetKey);
  const baseRange = presetKey === "custom" ? customRange : selectedPreset?.range;
  const range: SpendingAnalysisRange | undefined = baseRange
    ? {
      fromDate: baseRange.fromDate,
      toDate: baseRange.toDate,
      ...(granularity === "auto" ? {} : { granularity }),
    }
    : undefined;

  const query = useQuery({
    queryKey: spendingAnalysisQueryKey(workspace.id, range),
    queryFn: () => getSpendingAnalysis(workspace.id, range),
  });

  function applyCustomRange(event: FormEvent) {
    event.preventDefault();
    if (!customFrom || !customTo || customFrom > customTo) {
      setRangeError(t("Choose a start date that is on or before the end date."));
      return;
    }
    setRangeError("");
    setCustomRange({ fromDate: customFrom, toDate: customTo });
    setPresetKey("custom");
  }

  return (
    <section className="analysis-dashboard" aria-labelledby="analysis-heading">
      <SurfaceCard className="analysis-controls">
        <div className="analysis-controls-intro">
          <p className="eyebrow">{t("Posted activity only")}</p>
          <h2 id="analysis-heading">{t("Where your money went")}</h2>
          <p>
            {t("Every figure below is settled activity in your base currency. Transfers between your own accounts are never counted as spending.")}
          </p>
        </div>
        <div className="analysis-control-groups">
          <div
            aria-label={t("Analysis period")}
            className="segmented-control"
            role="group"
          >
            {presets.map((preset) => (
              <button
                aria-pressed={presetKey === preset.key}
                className={`segmented-option${presetKey === preset.key ? " active" : ""}`}
                key={preset.key}
                onClick={() => { setPresetKey(preset.key); setRangeError(""); }}
                type="button"
              >
                {t(preset.label)}
              </button>
            ))}
          </div>
          <div
            aria-label={t("Time bucket")}
            className="segmented-control segmented-control-quiet"
            role="group"
          >
            <button
              aria-pressed={granularity === "auto"}
              className={`segmented-option${granularity === "auto" ? " active" : ""}`}
              onClick={() => setGranularity("auto")}
              type="button"
            >
              {t("Auto")}
            </button>
            {analysisGranularities.map((option) => (
              <button
                aria-pressed={granularity === option}
                className={`segmented-option${granularity === option ? " active" : ""}`}
                key={option}
                onClick={() => setGranularity(option)}
                type="button"
              >
                {granularityLabel(option)}
              </button>
            ))}
          </div>
        </div>
        <form className="analysis-custom-range" onSubmit={applyCustomRange}>
          <label>
            {t("From")}
            <input
              aria-label={t("Analysis start date")}
              onChange={(event) => setCustomFrom(event.target.value)}
              type="date"
              value={customFrom}
            />
          </label>
          <label>
            {t("To")}
            <input
              aria-label={t("Analysis end date")}
              onChange={(event) => setCustomTo(event.target.value)}
              type="date"
              value={customTo}
            />
          </label>
          <button className="secondary-button" type="submit">{t("Apply range")}</button>
        </form>
        {rangeError ? <p className="form-error" role="alert">{rangeError}</p> : null}
      </SurfaceCard>

      {query.isPending ? (
        <div className="analysis-loading">
          <LoadingState label={t("Loading spending analysis")} rows={5} />
          <div className="analysis-loading-grid">
            <LoadingState label={t("Loading category breakdown")} rows={4} />
            <LoadingState label={t("Loading spending rhythm")} rows={4} />
          </div>
        </div>
      ) : null}

      {query.isError ? (
        <InlineNotice
          action={(
            <button className="secondary-button" onClick={() => void query.refetch()} type="button">
              {t("Try again")}
            </button>
          )}
          title={t("Analysis unavailable")}
          tone="danger"
        >
          {query.error instanceof APIError
            ? query.error.message
            : t("Your spending analysis could not be loaded.")}
        </InlineNotice>
      ) : null}

      {query.data ? (
        <AnalysisContent analysis={query.data} workspaceId={workspace.id} />
      ) : null}
    </section>
  );
}

function AnalysisContent({
  analysis,
  workspaceId,
}: {
  analysis: SpendingAnalysis;
  workspaceId: string;
}) {
  const expenses = categoryBreakdown(analysis, "expense");
  const income = categoryBreakdown(analysis, "income");
  const hasActivity = analysis.totals.transaction_count > 0;

  if (!hasActivity) {
    return (
      <EmptyState
        action={<Link to={`/workspaces/${workspaceId}/transactions`}>{t("Record a transaction")}</Link>}
        description={t("Once this period has posted activity, its trend, categories, and rhythm appear here.")}
        icon="chart"
        title={t("Nothing to analyze yet")}
      />
    );
  }

  return (
    <>
      <PeriodSummary analysis={analysis} />
      <TrendPanel analysis={analysis} />
      <InsightStrip analysis={analysis} />
      <div className="analysis-split">
        <CategoryPanel
          analysis={analysis}
          heading={t("Spending by category")}
          nodes={expenses}
          total={analysis.totals.spending_base_minor}
          workspaceId={workspaceId}
        />
        <RhythmPanel analysis={analysis} />
      </div>
      <div className="analysis-split">
        <PayeePanel analysis={analysis} />
        <AccountPanel analysis={analysis} workspaceId={workspaceId} />
      </div>
      {income.length > 0 ? (
        <CategoryPanel
          analysis={analysis}
          heading={t("Income by category")}
          nodes={income}
          total={analysis.totals.income_base_minor}
          workspaceId={workspaceId}
        />
      ) : null}
      <InlineNotice
        action={<Link to={`/workspaces/${workspaceId}/reports`}>{t("Open reports")}</Link>}
        title={t("Analysis explains the pattern")}
      >
        <p>
          {t("Reports show balances and pending activity for one period. Use Budget to set the targets these results are measured against.")}
        </p>
      </InlineNotice>
      <p className="analysis-footnote">
        {t("Comparison window: {from} – {to}", {
          from: formatDate(analysis.period.comparison_from_date),
          to: formatDate(analysis.period.comparison_to_date),
        })}
      </p>
    </>
  );
}

function PeriodSummary({ analysis }: { analysis: SpendingAnalysis }) {
  const { totals, period } = analysis;
  const currency = period.base_currency;
  const rate = savingsRate(totals.income_base_minor, totals.spending_base_minor);
  return (
    <section className="analysis-summary" aria-labelledby="analysis-summary-heading">
      <article className="analysis-headline">
        <div className="analysis-headline-top">
          <div>
            <p>{t("Total spent")}</p>
            <span>{formatDate(period.from_date)} – {formatDate(period.to_date)}</span>
          </div>
          <StatusBadge tone="positive">{currency}</StatusBadge>
        </div>
        <h3 id="analysis-summary-heading">
          <MoneyAmount amount={totals.spending_base_minor} currency={currency} emphasis="hero" />
        </h3>
        <DeltaChip
          current={totals.spending_base_minor}
          currency={currency}
          invert
          previous={totals.comparison_spending_base_minor}
        />
      </article>
      <div className="analysis-metric-grid">
        <MetricCard
          currency={currency}
          detail={t("across {transactions}", {
            transactions: transactionCountLabel(Number(totals.transaction_count)),
          })}
          label={t("Income")}
          value={totals.income_base_minor}
        >
          <DeltaChip
            current={totals.income_base_minor}
            currency={currency}
            previous={totals.comparison_income_base_minor}
          />
        </MetricCard>
        <MetricCard
          currency={currency}
          detail={rate === null
            ? t("No income in this period")
            : t("{percent} of income kept", { percent: formatPercent(rate).replace("+", "") })}
          label={t("Net")}
          value={totals.net_base_minor}
        >
          <DeltaChip
            current={totals.net_base_minor}
            currency={currency}
            previous={totals.comparison_net_base_minor}
          />
        </MetricCard>
        <MetricCard
          currency={currency}
          detail={t("{count} of {total} days had spending", {
            count: String(totals.spending_day_count),
            total: String(totals.day_count),
          })}
          label={t("Average per day")}
          value={averagePerDay(totals.spending_base_minor, Number(totals.day_count))}
        />
        <MetricCard
          currency={currency}
          detail={t("Largest single charge {amount}", {
            amount: formatMoney(totals.largest_spending_base_minor, currency),
          })}
          label={t("Average per transaction")}
          value={averagePerTransaction(
            totals.spending_base_minor,
            Number(totals.spending_transaction_count),
          )}
        />
      </div>
    </section>
  );
}

function MetricCard({
  children,
  currency,
  detail,
  label,
  value,
}: {
  children?: ReactNode;
  currency: Currency;
  detail: string;
  label: string;
  value: number;
}) {
  return (
    <article className="analysis-metric-card">
      <p>{label}</p>
      <strong><MoneyAmount amount={value} currency={currency} /></strong>
      <small>{detail}</small>
      {children}
    </article>
  );
}

/**
 * Spending going up is not the same kind of news as income going up, so the tone is chosen
 * by what the figure means rather than by its arithmetic sign.
 */
function DeltaChip({
  current,
  currency,
  invert = false,
  previous,
}: {
  current: number;
  currency: Currency;
  invert?: boolean;
  previous: number;
}) {
  const ratio = deltaRatio(current, previous);
  if (ratio === null) {
    return (
      <span className="delta-chip delta-chip-neutral">
        {t("No comparable activity before this period")}
      </span>
    );
  }
  const favourable = invert ? ratio <= 0 : ratio >= 0;
  const tone = Math.abs(ratio) < 0.01 ? "neutral" : favourable ? "positive" : "warning";
  return (
    <span className={`delta-chip delta-chip-${tone}`}>
      {formatPercent(ratio)}
      <small>{t("vs {amount} before", { amount: formatMoney(previous, currency) })}</small>
    </span>
  );
}

function TrendPanel({ analysis }: { analysis: SpendingAnalysis }) {
  const currency = analysis.period.base_currency;
  const [visible, setVisible] = useState<Record<string, boolean>>({
    spending: true, income: true, net: false,
  });
  const definitions: TrendSeries[] = [
    {
      key: "spending",
      label: t("Spending"),
      color: "var(--chart-spending)",
      kind: "area",
      values: analysis.series.map((bucket) => bucket.spending_base_minor),
    },
    {
      key: "income",
      label: t("Income"),
      color: "var(--chart-income)",
      kind: "line",
      values: analysis.series.map((bucket) => bucket.income_base_minor),
    },
    {
      key: "net",
      label: t("Net"),
      color: "var(--chart-net)",
      kind: "line",
      values: analysis.series.map((bucket) => bucket.net_base_minor),
    },
  ];
  const series = definitions.filter((entry) => visible[entry.key]);

  return (
    <SurfaceCard className="analysis-trend" labelledBy="analysis-trend-heading">
      <div className="analysis-panel-heading">
        <div>
          <p className="eyebrow">{t("Over time")}</p>
          <h3 id="analysis-trend-heading">{t("Spending trend")}</h3>
          <p>{t("Each point is one {granularity} of posted activity.", {
            granularity: granularityNoun(analysis.period.granularity),
          })}</p>
        </div>
        <div aria-label={t("Chart series")} className="chart-legend" role="group">
          {definitions.map((entry) => (
            <button
              aria-pressed={visible[entry.key]}
              className={`chart-legend-toggle${visible[entry.key] ? " active" : ""}`}
              key={entry.key}
              onClick={() => setVisible((current) => ({
                ...current, [entry.key]: !current[entry.key],
              }))}
              type="button"
            >
              <i aria-hidden="true" style={{ background: entry.color }} />
              {entry.label}
            </button>
          ))}
        </div>
      </div>
      <TrendChart
        ariaLabel={t("Spending and income by {granularity}", {
          granularity: granularityNoun(analysis.period.granularity),
        })}
        formatValue={(value) => formatMoney(value, currency)}
        labels={analysis.series.map((bucket) =>
          bucketLabel(bucket.start_date, analysis.period.granularity))}
        series={series}
      />
    </SurfaceCard>
  );
}

function InsightStrip({ analysis }: { analysis: SpendingAnalysis }) {
  const currency = analysis.period.base_currency;
  const insights = analysisInsights({
    analysis,
    formatAmount: (value) => formatMoney(value, currency),
    formatCategory: (category) => categoryName(category),
    formatWeekday: weekdayName,
    formatBucket: (startDate) => bucketLabel(startDate, analysis.period.granularity),
  });
  if (insights.length === 0) return null;
  return (
    <section aria-label={t("What stands out")} className="insight-strip">
      {insights.map((insight) => (
        <article className={`insight-card insight-card-${insight.tone}`} key={insight.id}>
          <strong>{insight.title}</strong>
          <p>{insight.detail}</p>
        </article>
      ))}
    </section>
  );
}

function CategoryPanel({
  analysis,
  heading,
  nodes,
  total,
  workspaceId,
}: {
  analysis: SpendingAnalysis;
  heading: string;
  nodes: CategoryNode[];
  total: number;
  workspaceId: string;
}) {
  const currency = analysis.period.base_currency;
  const [expanded, setExpanded] = useState<string | null>(null);
  const ranked = nodes.slice(0, TOP_CATEGORY_LIMIT);
  const remainder = nodes.slice(TOP_CATEGORY_LIMIT);
  const remainderTotal = remainder.reduce((sum, node) => sum + node.amountMinor, 0);
  const headingId = `analysis-category-${heading.replace(/\W+/g, "-").toLowerCase()}`;
  const slices = ranked.map((node) => ({
    id: node.category.id,
    label: categoryName(node.category),
    value: node.amountMinor,
    color: categoryAccentColor(node.category.color_key),
  }));
  if (remainderTotal > 0) {
    slices.push({
      id: "other", label: t("Everything else"), value: remainderTotal,
      color: "var(--border-strong)",
    });
  }

  return (
    <SurfaceCard className="analysis-categories" labelledBy={headingId}>
      <div className="analysis-panel-heading">
        <div>
          <p className="eyebrow">{t("Composition")}</p>
          <h3 id={headingId}>{heading}</h3>
        </div>
        <Link to={`/workspaces/${workspaceId}/categories`}>{t("Manage categories")}</Link>
      </div>
      {nodes.length === 0 ? (
        <p className="resource-state">{t("No category activity in this period.")}</p>
      ) : (
        <>
          <DonutChart
            ariaLabel={heading}
            centerLabel={t("Total")}
            centerValue={compactMoney(total, currency)}
            formatValue={(value) => formatMoney(value, currency)}
            slices={slices}
          />
          <ol className="category-ranking">
            {ranked.map((node, index) => (
              <CategoryRow
                analysis={analysis}
                currency={currency}
                expanded={expanded === node.category.id}
                key={node.category.id}
                node={node}
                onToggle={() => setExpanded(
                  expanded === node.category.id ? null : node.category.id,
                )}
                rank={index + 1}
              />
            ))}
          </ol>
          {remainder.length > 0 ? (
            <p className="category-ranking-remainder">
              {t("{count} more categories account for {amount}.", {
                count: String(remainder.length),
                amount: formatMoney(remainderTotal, currency),
              })}
            </p>
          ) : null}
        </>
      )}
    </SurfaceCard>
  );
}

function CategoryRow({
  analysis,
  currency,
  expanded,
  node,
  onToggle,
  rank,
}: {
  analysis: SpendingAnalysis;
  currency: Currency;
  expanded: boolean;
  node: CategoryNode;
  onToggle: () => void;
  rank: number;
}) {
  const accent = categoryAccentColor(node.category.color_key);
  const trend = useMemo(
    () => categorySeries(analysis, categorySubtreeIds(analysis, node.category.id)),
    [analysis, node.category.id],
  );
  const ratio = deltaRatio(node.amountMinor, node.comparisonMinor);
  const name = categoryName(node.category);

  return (
    <li className="category-ranking-row">
      <div className="category-ranking-main">
        <span className="category-ranking-rank" aria-hidden="true">{rank}</span>
        <div className="category-ranking-identity">
          <CategoryLabel
            colorKey={node.category.color_key}
            iconType={node.category.icon_type}
            iconValue={node.category.icon_value}
            name={name}
          />
          <small>
            {transactionCountLabel(node.transactionCount)}
            {node.category.largest_base_minor > 0
              ? ` · ${t("largest {amount}", { amount: formatMoney(node.category.largest_base_minor, currency) })}`
              : ""}
          </small>
        </div>
        <div className="category-ranking-figures">
          <strong><MoneyAmount amount={node.amountMinor} currency={currency} /></strong>
          <small>{formatPercent(node.share).replace("+", "")}</small>
        </div>
      </div>
      <div className="category-ranking-visual">
        <MeterBar
          color={accent}
          label={t("{category} share of spending: {percent}", {
            category: name,
            percent: formatPercent(node.share).replace("+", ""),
          })}
          share={node.share}
        />
        <Sparkline
          ariaLabel={t("{category} trend across the period", { category: name })}
          color={accent}
          values={trend}
        />
        {ratio === null ? (
          <span className="delta-inline delta-inline-neutral">{t("New")}</span>
        ) : (
          <span className={`delta-inline delta-inline-${ratio > 0.01 ? "up" : ratio < -0.01 ? "down" : "flat"}`}>
            {formatPercent(ratio)}
          </span>
        )}
      </div>
      {node.children.length > 0 ? (
        <>
          <button
            aria-expanded={expanded}
            className="text-button category-ranking-expand"
            onClick={onToggle}
            type="button"
          >
            {expanded
              ? t("Hide subcategories")
              : t("Show {count} subcategories", { count: String(node.children.length) })}
          </button>
          {expanded ? (
            <ul className="category-subcategories">
              {node.children.map((child) => (
                <li key={child.id}>
                  <CategoryLabel
                    colorKey={child.color_key}
                    iconType={child.icon_type}
                    iconValue={child.icon_value}
                    name={categoryName(child)}
                  />
                  <MoneyAmount amount={child.rolled_up_base_minor} currency={currency} />
                </li>
              ))}
            </ul>
          ) : null}
        </>
      ) : null}
    </li>
  );
}

function RhythmPanel({ analysis }: { analysis: SpendingAnalysis }) {
  const currency = analysis.period.base_currency;
  const readings = weekdayReadings(analysis);
  const weeks = calendarWeeks(analysis);
  const busiest = [...readings].sort((left, right) => right.spendingMinor - left.spendingMinor).at(0);

  return (
    <SurfaceCard className="analysis-rhythm" labelledBy="analysis-rhythm-heading">
      <div className="analysis-panel-heading">
        <div>
          <p className="eyebrow">{t("When it happens")}</p>
          <h3 id="analysis-rhythm-heading">{t("Spending rhythm")}</h3>
          <p>
            {busiest && busiest.spendingMinor > 0
              ? t("Most money leaves on {weekday}.", { weekday: weekdayName(busiest.weekday) })
              : t("No posted spending in this period.")}
          </p>
        </div>
      </div>
      <ul className="weekday-chart">
        {readings.map((reading) => (
          <li key={reading.weekday}>
            <span className="weekday-chart-label">{weekdayName(reading.weekday, "short")}</span>
            <MeterBar
              label={t("{weekday}: {amount} across {transactions}", {
                weekday: weekdayName(reading.weekday),
                amount: formatMoney(reading.spendingMinor, currency),
                transactions: transactionCountLabel(reading.transactionCount),
              })}
              share={reading.share}
            />
            <span className="weekday-chart-value">
              {compactMoney(reading.spendingMinor, currency)}
            </span>
          </li>
        ))}
      </ul>
      <div className="analysis-calendar">
        <p className="analysis-subheading">{t("Daily intensity")}</p>
        <HeatmapGrid
          ariaLabel={t("Daily spending intensity")}
          columns={weeks.map((week) => ({
            key: week.key,
            cells: week.cells.map((cell) => (cell
              ? {
                date: cell.date,
                intensity: cell.intensity,
                label: t("{date}: {amount} across {transactions}", {
                  date: formatDate(cell.date),
                  amount: formatMoney(cell.spendingMinor, currency),
                  transactions: transactionCountLabel(cell.transactionCount),
                }),
              }
              : null)),
          }))}
          weekdayLabels={[1, 2, 3, 4, 5, 6, 7].map((weekday) => weekdayName(weekday, "narrow"))}
        />
        <p className="analysis-legend">
          <span>{t("Quiet")}</span>
          <i aria-hidden="true" className="analysis-legend-scale" />
          <span>{t("Heavy")}</span>
        </p>
      </div>
    </SurfaceCard>
  );
}

function PayeePanel({ analysis }: { analysis: SpendingAnalysis }) {
  const currency = analysis.period.base_currency;
  const payees = analysis.payees.filter((payee) => payee.spending_base_minor > 0).slice(0, 10);
  const peak = Math.max(...payees.map((payee) => payee.spending_base_minor), 1);

  return (
    <SurfaceCard className="analysis-payees" labelledBy="analysis-payees-heading">
      <div className="analysis-panel-heading">
        <div>
          <p className="eyebrow">{t("Who you paid")}</p>
          <h3 id="analysis-payees-heading">{t("Top payees")}</h3>
        </div>
      </div>
      {payees.length === 0 ? (
        <EmptyState
          compact
          description={t("Add a payee when recording a transaction to see who you pay most.")}
          icon="receipt"
          title={t("No payees recorded")}
        />
      ) : (
        <ol className="payee-list">
          {payees.map((payee) => (
            <li key={payee.payee}>
              <div>
                <strong>{payee.payee}</strong>
                <small>
                  {transactionCountLabel(Number(payee.transaction_count))}
                  {" · "}
                  {t("last {date}", { date: formatDate(payee.last_date) })}
                </small>
              </div>
              <MeterBar
                label={t("{payee}: {amount}", {
                  payee: payee.payee,
                  amount: formatMoney(payee.spending_base_minor, currency),
                })}
                share={payee.spending_base_minor / peak}
              />
              <MoneyAmount amount={payee.spending_base_minor} currency={currency} />
            </li>
          ))}
        </ol>
      )}
    </SurfaceCard>
  );
}

function AccountPanel({
  analysis,
  workspaceId,
}: {
  analysis: SpendingAnalysis;
  workspaceId: string;
}) {
  const currency = analysis.period.base_currency;
  const accounts = [...analysis.accounts]
    .sort((left, right) => right.outflow_base_minor - left.outflow_base_minor);
  const peak = Math.max(...accounts.map((account) => account.outflow_base_minor), 1);

  return (
    <SurfaceCard className="analysis-accounts" labelledBy="analysis-accounts-heading">
      <div className="analysis-panel-heading">
        <div>
          <p className="eyebrow">{t("Where it moved")}</p>
          <h3 id="analysis-accounts-heading">{t("Account activity")}</h3>
          <p>{t("Money in and out of each account, excluding transfers between your own accounts.")}</p>
        </div>
        <Link to={`/workspaces/${workspaceId}/accounts`}>{t("Manage accounts")}</Link>
      </div>
      {accounts.length === 0 ? (
        <p className="resource-state">{t("No account activity in this period.")}</p>
      ) : (
        <ul className="account-activity-list">
          {accounts.map((account) => (
            <li key={account.id}>
              <div>
                <strong>{account.name}</strong>
                <small>
                  {t(`account.type.${account.type}`)}
                  {account.archived_at ? ` · ${t("Archived")}` : ""}
                </small>
              </div>
              <MeterBar
                label={t("{account}: {amount} out", {
                  account: account.name,
                  amount: formatMoney(account.outflow_base_minor, currency),
                })}
                share={account.outflow_base_minor / peak}
              />
              <div className="account-activity-figures">
                <strong><MoneyAmount amount={account.outflow_base_minor} currency={currency} /></strong>
                <small>{t("{amount} in", {
                  amount: formatMoney(account.inflow_base_minor, currency),
                })}</small>
              </div>
            </li>
          ))}
        </ul>
      )}
    </SurfaceCard>
  );
}

/** A bucket reads by its width: a day needs its date, a month only needs its name. */
function bucketLabel(startDate: ISODate, granularity: AnalysisGranularity): string {
  const date = new Date(`${startDate}T00:00:00Z`);
  if (granularity === "month") {
    // A two-digit year here would read as a day: "Aug 26" for August 2026.
    return new Intl.DateTimeFormat(undefined, {
      month: "short", year: "numeric", timeZone: "UTC",
    }).format(date);
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short", day: "numeric", timeZone: "UTC",
  }).format(date);
}

function formatDate(value: ISODate): string {
  return new Intl.DateTimeFormat(undefined, {
    month: "short", day: "numeric", year: "numeric", timeZone: "UTC",
  }).format(new Date(`${value}T00:00:00Z`));
}

function weekdayName(
  weekday: number,
  format: "long" | "narrow" | "short" = "long",
): string {
  // 2024-01-01 is a Monday, which turns an ISO weekday into a date the formatter can name.
  return new Intl.DateTimeFormat(undefined, { weekday: format, timeZone: "UTC" })
    .format(new Date(Date.UTC(2024, 0, weekday)));
}

/** Axis and tile figures need to fit; the exact amount stays available in text nearby. */
function compactMoney(amountMinor: number, currency: Currency): string {
  if (!Number.isSafeInteger(amountMinor)) return formatMoney(amountMinor, currency);
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(amountMinor / 100);
}
