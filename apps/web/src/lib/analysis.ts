import type {
  AnalysisGranularity,
  SpendingAnalysis,
  SpendingAnalysisRange,
} from "../api/client";
import { t } from "./i18n";

type AnalysisCategory = SpendingAnalysis["categories"][number];

/** A calendar day in ISO form. Every analysis boundary travels as one of these. */
export type ISODate = string;

const MILLISECONDS_PER_DAY = 86_400_000;

/** Comfortably beyond the longest window the server will resolve. */
const MAX_CALENDAR_WEEKS = 600;

/**
 * Dates are handled as UTC calendar days rather than local instants. The server has already
 * resolved the window in the workspace timezone, so re-interpreting these strings locally
 * would shift a boundary by a day for anyone east or west of the workspace.
 */
function parseISODate(value: ISODate): Date {
  const [year, month, day] = value.split("-").map(Number);
  return new Date(Date.UTC(year, (month ?? 1) - 1, day ?? 1));
}

function formatISODate(value: Date): ISODate {
  return value.toISOString().slice(0, 10);
}

function addDays(value: ISODate, days: number): ISODate {
  return formatISODate(new Date(parseISODate(value).getTime() + days * MILLISECONDS_PER_DAY));
}

/** ISO weekday, where 1 is Monday and 7 is Sunday, matching the analysis contract. */
export function isoWeekday(value: ISODate): number {
  return parseISODate(value).getUTCDay() || 7;
}

/** Today in the workspace timezone, so a range preset means the same day the server means. */
export function workspaceToday(timezone: string, now = new Date()): ISODate {
  try {
    return new Intl.DateTimeFormat("en-CA", {
      timeZone: timezone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(now);
  } catch {
    // An unusable workspace timezone is reported by the API; keep a working local fallback.
    return formatISODate(new Date(now.getTime() - now.getTimezoneOffset() * 60_000));
  }
}

export type RangePresetKey =
  | "this-month"
  | "last-month"
  | "last-3-months"
  | "last-6-months"
  | "year-to-date"
  | "last-12-months";

export type RangePreset = {
  key: RangePresetKey;
  label: string;
  range: SpendingAnalysisRange;
};

function startOfMonth(value: ISODate): ISODate {
  return `${value.slice(0, 7)}-01`;
}

function shiftMonths(value: ISODate, months: number): ISODate {
  const date = parseISODate(value);
  return formatISODate(
    new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + months, 1)),
  );
}

function endOfMonth(value: ISODate): ISODate {
  return addDays(shiftMonths(value, 1), -1);
}

/**
 * Presets are inclusive windows anchored on the workspace's today. Each carries the bucket
 * width that reads well at that span so switching ranges never leaves an unreadable chart.
 */
export function rangePresets(today: ISODate): RangePreset[] {
  const monthStart = startOfMonth(today);
  const previousMonthStart = shiftMonths(monthStart, -1);
  return [
    {
      key: "this-month",
      label: "This month",
      range: { fromDate: monthStart, toDate: today, granularity: "day" },
    },
    {
      key: "last-month",
      label: "Last month",
      range: {
        fromDate: previousMonthStart,
        toDate: endOfMonth(previousMonthStart),
        granularity: "day",
      },
    },
    {
      key: "last-3-months",
      label: "Last 3 months",
      range: { fromDate: shiftMonths(monthStart, -2), toDate: today, granularity: "week" },
    },
    {
      key: "last-6-months",
      label: "Last 6 months",
      range: { fromDate: shiftMonths(monthStart, -5), toDate: today, granularity: "month" },
    },
    {
      key: "year-to-date",
      label: "Year to date",
      range: { fromDate: `${today.slice(0, 4)}-01-01`, toDate: today, granularity: "month" },
    },
    {
      key: "last-12-months",
      label: "Last 12 months",
      range: { fromDate: shiftMonths(monthStart, -11), toDate: today, granularity: "month" },
    },
  ];
}

export const analysisGranularities: AnalysisGranularity[] = ["day", "week", "month"];

export function granularityLabel(granularity: AnalysisGranularity): string {
  return t(`analysis.granularity.${granularity}`);
}

/**
 * The same bucket width reads as an adjective on a control ("Weekly") and as a noun in a
 * sentence ("one week of posted activity"). Prose needs the second form.
 */
export function granularityNoun(granularity: AnalysisGranularity): string {
  return t(`analysis.bucket.${granularity}`);
}

/** Counts read as prose, and "1 transactions" is the giveaway that they were not. */
export function transactionCountLabel(count: number): string {
  return t(count === 1 ? "{count} transaction" : "{count} transactions", {
    count: String(count),
  });
}

/**
 * Relative movement against the comparison window. A previous window of zero has no ratio
 * to report: growth from nothing is unbounded, and rendering it as a percentage would invent
 * precision the data does not have.
 */
export function deltaRatio(current: number, previous: number): number | null {
  if (previous === 0) return null;
  return (current - previous) / Math.abs(previous);
}

export function formatPercent(ratio: number, locale?: string): string {
  return new Intl.NumberFormat(locale, {
    style: "percent",
    maximumFractionDigits: ratio >= 1 || ratio <= -1 ? 0 : 1,
    signDisplay: "exceptZero",
  }).format(ratio);
}

/** Money spent per day of the window, which makes windows of different lengths comparable. */
export function averagePerDay(totalMinor: number, dayCount: number): number {
  if (dayCount <= 0) return 0;
  return Math.round(totalMinor / dayCount);
}

export function averagePerTransaction(totalMinor: number, transactionCount: number): number {
  if (transactionCount <= 0) return 0;
  return Math.round(totalMinor / transactionCount);
}

/**
 * The share of income that was not spent. Reported only when income exists, because a
 * savings rate against no income is not a meaningful figure.
 */
export function savingsRate(incomeMinor: number, spendingMinor: number): number | null {
  if (incomeMinor <= 0) return null;
  return (incomeMinor - spendingMinor) / incomeMinor;
}

export type CategoryNode = {
  category: AnalysisCategory;
  children: AnalysisCategory[];
  amountMinor: number;
  comparisonMinor: number;
  transactionCount: number;
  share: number;
};

/**
 * Top-level categories carry rolled-up totals, so a workspace that organizes spending into
 * subcategories still reads as a small number of meaningful groups. Children ride along for
 * progressive disclosure rather than competing with their parent in the ranking.
 */
export function categoryBreakdown(
  analysis: SpendingAnalysis,
  kind: "expense" | "income",
): CategoryNode[] {
  const inKind = analysis.categories.filter((category) => category.kind === kind);
  const roots = inKind.filter((category) => !category.parent_id);
  const nodes = roots
    .map((category) => ({
      category,
      children: inKind
        .filter((candidate) => candidate.parent_id === category.id)
        .filter((candidate) => candidate.rolled_up_base_minor !== 0)
        .sort((left, right) => right.rolled_up_base_minor - left.rolled_up_base_minor),
      amountMinor: category.rolled_up_base_minor,
      comparisonMinor: category.comparison_rolled_up_base_minor,
      transactionCount: category.rolled_up_transaction_count,
      share: 0,
    }))
    .filter((node) => node.amountMinor !== 0)
    .sort((left, right) => right.amountMinor - left.amountMinor);
  const total = nodes.reduce((sum, node) => sum + Math.max(node.amountMinor, 0), 0);
  return nodes.map((node) => ({
    ...node,
    share: total > 0 ? Math.max(node.amountMinor, 0) / total : 0,
  }));
}

/** Bucketed activity for one category, aligned to the analysis series so charts share an axis. */
export function categorySeries(
  analysis: SpendingAnalysis,
  categoryIds: string[],
): number[] {
  const wanted = new Set(categoryIds);
  const byDate = new Map<ISODate, number>();
  for (const point of analysis.category_series) {
    if (!wanted.has(point.category_id)) continue;
    byDate.set(point.start_date, (byDate.get(point.start_date) ?? 0) + point.base_minor);
  }
  // Points are anchored to the bucket the server truncated to, which can precede a clamped
  // first bucket start. Fold anything before the window into the opening bucket.
  return analysis.series.map((bucket, index) => {
    let total = byDate.get(bucket.start_date) ?? 0;
    if (index === 0) {
      for (const [date, amount] of byDate) {
        if (date < bucket.start_date) total += amount;
      }
    }
    return total;
  });
}

/** Every category in one subtree, so a parent's trend includes the children it rolls up. */
export function categorySubtreeIds(
  analysis: SpendingAnalysis,
  categoryId: string,
): string[] {
  const ids = [categoryId];
  for (let index = 0; index < ids.length; index += 1) {
    for (const candidate of analysis.categories) {
      if (candidate.parent_id === ids[index] && !ids.includes(candidate.id)) {
        ids.push(candidate.id);
      }
    }
  }
  return ids;
}

export type WeekdayReading = {
  weekday: number;
  spendingMinor: number;
  transactionCount: number;
  share: number;
};

/** All seven days, including the quiet ones, so the shape of a week is visible at a glance. */
export function weekdayReadings(analysis: SpendingAnalysis): WeekdayReading[] {
  const byWeekday = new Map(analysis.weekdays.map((value) => [value.weekday, value]));
  const readings = Array.from({ length: 7 }, (_, index) => {
    const weekday = index + 1;
    const value = byWeekday.get(weekday);
    return {
      weekday,
      spendingMinor: Math.max(value?.spending_base_minor ?? 0, 0),
      transactionCount: value?.transaction_count ?? 0,
      share: 0,
    };
  });
  const peak = Math.max(...readings.map((reading) => reading.spendingMinor), 0);
  return readings.map((reading) => ({
    ...reading,
    share: peak > 0 ? reading.spendingMinor / peak : 0,
  }));
}

export type CalendarCell = {
  date: ISODate;
  spendingMinor: number;
  transactionCount: number;
  intensity: number;
};

export type CalendarWeek = { key: string; cells: (CalendarCell | null)[] };

/**
 * A Monday-aligned grid over the window. Leading and trailing gaps are null so the columns
 * stay aligned to weekdays instead of sliding by the window's start day.
 */
export function calendarWeeks(analysis: SpendingAnalysis): CalendarWeek[] {
  const byDate = new Map(analysis.days.map((day) => [day.date, day]));
  const peak = Math.max(
    ...analysis.days.map((day) => Math.max(day.spending_base_minor, 0)),
    0,
  );
  const { from_date: from, to_date: to } = analysis.period;
  const weeks: CalendarWeek[] = [];
  let cursor = addDays(from, -(isoWeekday(from) - 1));
  // The server bounds the window, so the cap is only a floor under a malformed period: it
  // keeps a bad response from spinning here instead of failing visibly.
  while (cursor <= to && weeks.length < MAX_CALENDAR_WEEKS) {
    const cells: (CalendarCell | null)[] = [];
    for (let offset = 0; offset < 7; offset += 1) {
      const date = addDays(cursor, offset);
      if (date < from || date > to) {
        cells.push(null);
        continue;
      }
      const day = byDate.get(date);
      const spendingMinor = Math.max(day?.spending_base_minor ?? 0, 0);
      cells.push({
        date,
        spendingMinor,
        transactionCount: day?.transaction_count ?? 0,
        intensity: peak > 0 ? spendingMinor / peak : 0,
      });
    }
    weeks.push({ key: cursor, cells });
    cursor = addDays(cursor, 7);
  }
  return weeks;
}

export type Insight = {
  id: string;
  tone: "danger" | "neutral" | "positive" | "warning";
  title: string;
  detail: string;
};

type InsightContext = {
  analysis: SpendingAnalysis;
  formatAmount: (amountMinor: number) => string;
  formatCategory: (category: AnalysisCategory) => string;
  formatWeekday: (weekday: number) => string;
  formatBucket: (startDate: ISODate) => string;
};

/**
 * Reading a chart is work. These sentences state the few things the numbers already prove,
 * so nothing here is a projection or a recommendation — only what the window contains.
 */
export function analysisInsights(context: InsightContext): Insight[] {
  const { analysis, formatAmount, formatCategory, formatWeekday, formatBucket } = context;
  const insights: Insight[] = [];
  const { totals } = analysis;

  const spendingDelta = deltaRatio(
    totals.spending_base_minor,
    totals.comparison_spending_base_minor,
  );
  if (spendingDelta !== null && Math.abs(spendingDelta) >= 0.05) {
    const rising = spendingDelta > 0;
    insights.push({
      id: "momentum",
      tone: rising ? "warning" : "positive",
      title: rising ? t("Spending is up") : t("Spending is down"),
      detail: t(
        rising
          ? "You spent {percent} more than the previous {days} days, {amount} in total."
          : "You spent {percent} less than the previous {days} days, {amount} in total.",
        {
          percent: formatPercent(Math.abs(spendingDelta)).replace("+", ""),
          days: String(totals.day_count),
          amount: formatAmount(totals.spending_base_minor),
        },
      ),
    });
  }

  const expenses = categoryBreakdown(analysis, "expense");
  const mover = expenses
    .filter((node) => node.comparisonMinor > 0)
    .map((node) => ({ node, ratio: deltaRatio(node.amountMinor, node.comparisonMinor) ?? 0 }))
    .filter((entry) => Math.abs(entry.ratio) >= 0.15)
    .sort((left, right) =>
      Math.abs(right.ratio * right.node.amountMinor) -
      Math.abs(left.ratio * left.node.amountMinor))
    .at(0);
  if (mover) {
    const rising = mover.ratio > 0;
    insights.push({
      id: "mover",
      tone: rising ? "warning" : "positive",
      title: t(rising ? "{category} grew the most" : "{category} fell the most", {
        category: formatCategory(mover.node.category),
      }),
      detail: t("{amount} this period against {comparison} in the previous one.", {
        amount: formatAmount(mover.node.amountMinor),
        comparison: formatAmount(mover.node.comparisonMinor),
      }),
    });
  }

  const leader = expenses.at(0);
  if (leader && leader.share > 0) {
    insights.push({
      id: "concentration",
      tone: leader.share >= 0.4 ? "warning" : "neutral",
      title: t("{category} leads your spending", {
        category: formatCategory(leader.category),
      }),
      detail: t("{percent} of everything you spent, across {transactions}.", {
        percent: formatPercent(leader.share).replace("+", ""),
        transactions: transactionCountLabel(leader.transactionCount),
      }),
    });
  }

  const busiestWeekday = weekdayReadings(analysis)
    .filter((reading) => reading.spendingMinor > 0)
    .sort((left, right) => right.spendingMinor - left.spendingMinor)
    .at(0);
  if (busiestWeekday) {
    insights.push({
      id: "rhythm",
      tone: "neutral",
      title: t("{weekday} is your heaviest day", {
        weekday: formatWeekday(busiestWeekday.weekday),
      }),
      detail: t("{amount} spent on {weekday}s in this period.", {
        amount: formatAmount(busiestWeekday.spendingMinor),
        weekday: formatWeekday(busiestWeekday.weekday),
      }),
    });
  }

  const peakBucket = [...analysis.series]
    .sort((left, right) => right.spending_base_minor - left.spending_base_minor)
    .at(0);
  if (peakBucket && peakBucket.spending_base_minor > 0) {
    insights.push({
      id: "peak",
      tone: "neutral",
      title: t("{bucket} was the peak", { bucket: formatBucket(peakBucket.start_date) }),
      detail: t("{amount} across {transactions}.", {
        amount: formatAmount(peakBucket.spending_base_minor),
        transactions: transactionCountLabel(peakBucket.transaction_count),
      }),
    });
  }

  const rate = savingsRate(totals.income_base_minor, totals.spending_base_minor);
  if (rate !== null) {
    insights.push({
      id: "savings",
      tone: rate >= 0.2 ? "positive" : rate >= 0 ? "neutral" : "danger",
      title: rate >= 0 ? t("You kept {percent} of income", {
        percent: formatPercent(rate).replace("+", ""),
      }) : t("You spent more than you earned"),
      detail: t("{income} earned against {spending} spent.", {
        income: formatAmount(totals.income_base_minor),
        spending: formatAmount(totals.spending_base_minor),
      }),
    });
  }

  return insights;
}
