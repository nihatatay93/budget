import { describe, expect, it } from "vitest";

import type { SpendingAnalysis } from "../api/client";
import {
  analysisInsights,
  averagePerDay,
  calendarWeeks,
  categoryBreakdown,
  categorySeries,
  categorySubtreeIds,
  deltaRatio,
  isoWeekday,
  rangePresets,
  savingsRate,
  transactionCountLabel,
  weekdayReadings,
  workspaceToday,
} from "./analysis";

const parentId = "0198b7ae-5e93-72da-b7aa-cd015d4bb77a";
const childId = "0198b7ae-5e93-72da-b7aa-cd015d4bb77b";
const otherId = "0198b7ae-5e93-72da-b7aa-cd015d4bb77c";

function analysisFixture(overrides: Partial<SpendingAnalysis> = {}): SpendingAnalysis {
  return {
    period: {
      from_date: "2026-08-01",
      to_date: "2026-08-14",
      comparison_from_date: "2026-07-18",
      comparison_to_date: "2026-07-31",
      granularity: "week",
      timezone: "Europe/Istanbul",
      base_currency: "TRY",
    },
    totals: {
      income_base_minor: 500000,
      spending_base_minor: 300000,
      net_base_minor: 200000,
      comparison_income_base_minor: 500000,
      comparison_spending_base_minor: 200000,
      comparison_net_base_minor: 300000,
      transaction_count: 20,
      spending_transaction_count: 16,
      largest_spending_base_minor: 90000,
      spending_day_count: 7,
      day_count: 14,
    },
    series: [
      {
        start_date: "2026-08-01",
        end_date: "2026-08-02",
        income_base_minor: 500000,
        spending_base_minor: 100000,
        net_base_minor: 400000,
        transaction_count: 6,
      },
      {
        start_date: "2026-08-03",
        end_date: "2026-08-09",
        income_base_minor: 0,
        spending_base_minor: 180000,
        net_base_minor: -180000,
        transaction_count: 10,
      },
      {
        start_date: "2026-08-10",
        end_date: "2026-08-14",
        income_base_minor: 0,
        spending_base_minor: 20000,
        net_base_minor: -20000,
        transaction_count: 4,
      },
    ],
    categories: [
      {
        id: parentId,
        name: "Food",
        kind: "expense",
        icon_type: "system",
        icon_value: "utensils",
        color_key: "orange",
        direct_base_minor: 40000,
        rolled_up_base_minor: 240000,
        comparison_direct_base_minor: 40000,
        comparison_rolled_up_base_minor: 120000,
        transaction_count: 4,
        rolled_up_transaction_count: 14,
        largest_base_minor: 90000,
      },
      {
        id: childId,
        parent_id: parentId,
        name: "Restaurants",
        kind: "expense",
        icon_type: "system",
        icon_value: "utensils",
        color_key: "amber",
        direct_base_minor: 200000,
        rolled_up_base_minor: 200000,
        comparison_direct_base_minor: 80000,
        comparison_rolled_up_base_minor: 80000,
        transaction_count: 10,
        rolled_up_transaction_count: 10,
        largest_base_minor: 90000,
      },
      {
        id: otherId,
        name: "Transport",
        kind: "expense",
        icon_type: "system",
        icon_value: "car",
        color_key: "blue",
        direct_base_minor: 60000,
        rolled_up_base_minor: 60000,
        comparison_direct_base_minor: 80000,
        comparison_rolled_up_base_minor: 80000,
        transaction_count: 2,
        rolled_up_transaction_count: 2,
        largest_base_minor: 40000,
      },
    ],
    category_series: [
      { category_id: parentId, start_date: "2026-08-03", base_minor: 40000 },
      { category_id: childId, start_date: "2026-08-03", base_minor: 140000 },
      { category_id: childId, start_date: "2026-08-10", base_minor: 60000 },
    ],
    weekdays: [
      { weekday: 1, income_base_minor: 0, spending_base_minor: 40000, transaction_count: 3 },
      { weekday: 6, income_base_minor: 0, spending_base_minor: 200000, transaction_count: 8 },
    ],
    days: [
      { date: "2026-08-01", income_base_minor: 500000, spending_base_minor: 100000, transaction_count: 6 },
      { date: "2026-08-08", income_base_minor: 0, spending_base_minor: 200000, transaction_count: 8 },
    ],
    payees: [],
    accounts: [],
    ...overrides,
  };
}

describe("analysis dates", () => {
  it("reads today in the workspace timezone rather than the browser's", () => {
    // 22:30 UTC is already the next day in Istanbul, which is the day the server would use.
    const instant = new Date("2026-08-25T22:30:00Z");
    expect(workspaceToday("Europe/Istanbul", instant)).toBe("2026-08-26");
    expect(workspaceToday("America/Los_Angeles", instant)).toBe("2026-08-25");
  });

  it("numbers weekdays the way the analysis contract does", () => {
    expect(isoWeekday("2026-08-03")).toBe(1);
    expect(isoWeekday("2026-08-09")).toBe(7);
  });

  it("anchors presets on the workspace day and picks a readable bucket width", () => {
    const presets = rangePresets("2026-08-25");
    const byKey = Object.fromEntries(presets.map((preset) => [preset.key, preset.range]));
    expect(byKey["this-month"]).toEqual({
      fromDate: "2026-08-01", toDate: "2026-08-25", granularity: "day",
    });
    expect(byKey["last-month"]).toEqual({
      fromDate: "2026-07-01", toDate: "2026-07-31", granularity: "day",
    });
    expect(byKey["last-12-months"]).toEqual({
      fromDate: "2025-09-01", toDate: "2026-08-25", granularity: "month",
    });
    expect(byKey["year-to-date"].fromDate).toBe("2026-01-01");
  });
});

describe("derived readings", () => {
  it("reports no ratio when there is nothing to compare against", () => {
    expect(deltaRatio(100, 0)).toBeNull();
    expect(deltaRatio(150, 100)).toBeCloseTo(0.5);
    expect(deltaRatio(50, 100)).toBeCloseTo(-0.5);
  });

  it("declines to report a savings rate without income", () => {
    expect(savingsRate(0, 5000)).toBeNull();
    expect(savingsRate(10000, 2500)).toBeCloseTo(0.75);
  });

  it("uses the singular form for a single transaction", () => {
    expect(transactionCountLabel(1)).toBe("1 transaction");
    expect(transactionCountLabel(0)).toBe("0 transactions");
    expect(transactionCountLabel(14)).toBe("14 transactions");
  });

  it("averages spending across the whole window, not only its active days", () => {
    expect(averagePerDay(300000, 14)).toBe(21429);
    expect(averagePerDay(300000, 0)).toBe(0);
  });
});

describe("categoryBreakdown", () => {
  it("ranks top-level categories by their rolled-up totals", () => {
    const nodes = categoryBreakdown(analysisFixture(), "expense");
    expect(nodes.map((node) => node.category.id)).toEqual([parentId, otherId]);
    expect(nodes[0].amountMinor).toBe(240000);
    expect(nodes[0].transactionCount).toBe(14);
    expect(nodes[0].children.map((child) => child.id)).toEqual([childId]);
  });

  it("computes shares against the ranked total so they sum to one", () => {
    const nodes = categoryBreakdown(analysisFixture(), "expense");
    const total = nodes.reduce((sum, node) => sum + node.share, 0);
    expect(total).toBeCloseTo(1);
    expect(nodes[0].share).toBeCloseTo(240000 / 300000);
  });

  it("omits categories with no activity in the window", () => {
    expect(categoryBreakdown(analysisFixture(), "income")).toEqual([]);
  });
});

describe("categorySeries", () => {
  it("includes a subtree so a parent's trend matches its rolled-up total", () => {
    const analysis = analysisFixture();
    const ids = categorySubtreeIds(analysis, parentId);
    expect(ids).toEqual([parentId, childId]);
    const values = categorySeries(analysis, ids);
    expect(values).toEqual([0, 180000, 60000]);
    expect(values.reduce((sum, value) => sum + value, 0)).toBe(240000);
  });

  // Month and week anchors precede a window that starts mid-bucket. Dropping those points
  // would silently understate the first bucket.
  it("folds activity anchored before the window into the opening bucket", () => {
    const analysis = analysisFixture({
      category_series: [
        { category_id: childId, start_date: "2026-07-27", base_minor: 25000 },
        { category_id: childId, start_date: "2026-08-03", base_minor: 140000 },
      ],
    });
    expect(categorySeries(analysis, [childId])).toEqual([25000, 140000, 0]);
  });
});

describe("weekdayReadings", () => {
  it("returns all seven days so quiet ones stay visible", () => {
    const readings = weekdayReadings(analysisFixture());
    expect(readings).toHaveLength(7);
    expect(readings.map((reading) => reading.weekday)).toEqual([1, 2, 3, 4, 5, 6, 7]);
    expect(readings[5].share).toBe(1);
    expect(readings[0].share).toBeCloseTo(0.2);
    expect(readings[1].spendingMinor).toBe(0);
  });
});

describe("calendarWeeks", () => {
  it("aligns the grid to Monday and leaves days outside the window empty", () => {
    const weeks = calendarWeeks(analysisFixture());
    // 2026-08-01 is a Saturday, so the first week has five leading gaps.
    expect(weeks[0].cells.slice(0, 5).every((cell) => cell === null)).toBe(true);
    expect(weeks[0].cells[5]?.date).toBe("2026-08-01");
    expect(weeks.at(-1)?.cells.at(-1)).toBeNull();
    const dates = weeks.flatMap((week) => week.cells.filter(Boolean).map((cell) => cell!.date));
    expect(dates).toHaveLength(14);
    expect(dates[0]).toBe("2026-08-01");
    expect(dates.at(-1)).toBe("2026-08-14");
  });

  it("scales intensity against the heaviest day in the window", () => {
    const weeks = calendarWeeks(analysisFixture());
    const cells = weeks.flatMap((week) => week.cells).filter(Boolean);
    const heaviest = cells.find((cell) => cell!.date === "2026-08-08");
    const lighter = cells.find((cell) => cell!.date === "2026-08-01");
    const quiet = cells.find((cell) => cell!.date === "2026-08-02");
    expect(heaviest?.intensity).toBe(1);
    expect(lighter?.intensity).toBeCloseTo(0.5);
    expect(quiet?.intensity).toBe(0);
  });
});

describe("analysisInsights", () => {
  const context = {
    formatAmount: (value: number) => `${(value / 100).toFixed(2)} TRY`,
    formatCategory: (category: { name: string }) => category.name,
    formatWeekday: (weekday: number) => `day ${weekday}`,
    formatBucket: (startDate: string) => startDate,
  };

  it("states the movements the window actually proves", () => {
    const insights = analysisInsights({ analysis: analysisFixture(), ...context });
    const byId = Object.fromEntries(insights.map((insight) => [insight.id, insight]));

    expect(byId.momentum.tone).toBe("warning");
    expect(byId.momentum.detail).toContain("50%");
    expect(byId.mover.title).toContain("Food");
    expect(byId.concentration.title).toContain("Food");
    expect(byId.rhythm.title).toContain("day 6");
    expect(byId.peak.title).toContain("2026-08-03");
    expect(byId.savings.tone).toBe("positive");
  });

  it("stays quiet about momentum when the change is negligible", () => {
    const analysis = analysisFixture();
    analysis.totals.comparison_spending_base_minor = analysis.totals.spending_base_minor;
    const ids = analysisInsights({ analysis, ...context }).map((insight) => insight.id);
    expect(ids).not.toContain("momentum");
  });

  it("reports overspending rather than a negative savings rate", () => {
    const analysis = analysisFixture();
    analysis.totals.spending_base_minor = 600000;
    analysis.totals.net_base_minor = -100000;
    const savings = analysisInsights({ analysis, ...context })
      .find((insight) => insight.id === "savings");
    expect(savings?.tone).toBe("danger");
    expect(savings?.title).toBe("You spent more than you earned");
  });
});
