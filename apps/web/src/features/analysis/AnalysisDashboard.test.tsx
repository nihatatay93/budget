import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SpendingAnalysis } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
import { AnalysisDashboard } from "./AnalysisDashboard";

const workspace = {
  id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
  name: "Personal",
  base_currency: "TRY" as const,
  timezone: "Europe/Istanbul",
};

const foodId = "0198b7ae-5e93-72da-b7aa-cd015d4bb77a";
const restaurantsId = "0198b7ae-5e93-72da-b7aa-cd015d4bb77b";
const transportId = "0198b7ae-5e93-72da-b7aa-cd015d4bb77c";
const salaryId = "0198b7ae-5e93-72da-b7aa-cd015d4bb77d";
const accountId = "0198b7ae-5e93-72d9-ab00-32b0861a3f37";

const analysis: SpendingAnalysis = {
  period: {
    from_date: "2026-03-01",
    to_date: "2026-08-25",
    comparison_from_date: "2025-09-04",
    comparison_to_date: "2026-02-28",
    granularity: "month",
    timezone: "Europe/Istanbul",
    base_currency: "TRY",
  },
  totals: {
    income_base_minor: 900000,
    spending_base_minor: 600000,
    net_base_minor: 300000,
    comparison_income_base_minor: 800000,
    comparison_spending_base_minor: 400000,
    comparison_net_base_minor: 400000,
    transaction_count: 48,
    spending_transaction_count: 40,
    largest_spending_base_minor: 125000,
    spending_day_count: 30,
    day_count: 178,
  },
  series: [
    {
      start_date: "2026-03-01", end_date: "2026-03-31",
      income_base_minor: 300000, spending_base_minor: 250000, net_base_minor: 50000,
      transaction_count: 18,
    },
    {
      start_date: "2026-04-01", end_date: "2026-04-30",
      income_base_minor: 300000, spending_base_minor: 150000, net_base_minor: 150000,
      transaction_count: 14,
    },
    {
      start_date: "2026-05-01", end_date: "2026-05-31",
      income_base_minor: 300000, spending_base_minor: 200000, net_base_minor: 100000,
      transaction_count: 16,
    },
  ],
  categories: [
    {
      id: foodId, name: "Food", kind: "expense", icon_type: "system", icon_value: "utensils",
      color_key: "orange", direct_base_minor: 100000, rolled_up_base_minor: 400000,
      comparison_direct_base_minor: 100000, comparison_rolled_up_base_minor: 200000,
      transaction_count: 8, rolled_up_transaction_count: 28, largest_base_minor: 125000,
    },
    {
      id: restaurantsId, parent_id: foodId, name: "Restaurants", kind: "expense",
      icon_type: "system", icon_value: "utensils", color_key: "amber",
      direct_base_minor: 300000, rolled_up_base_minor: 300000,
      comparison_direct_base_minor: 100000, comparison_rolled_up_base_minor: 100000,
      transaction_count: 20, rolled_up_transaction_count: 20, largest_base_minor: 125000,
    },
    {
      id: transportId, name: "Transport", kind: "expense", icon_type: "system",
      icon_value: "car", color_key: "blue", direct_base_minor: 200000,
      rolled_up_base_minor: 200000, comparison_direct_base_minor: 200000,
      comparison_rolled_up_base_minor: 200000, transaction_count: 12,
      rolled_up_transaction_count: 12, largest_base_minor: 30000,
    },
    {
      id: salaryId, name: "Salary", kind: "income", icon_type: "system", icon_value: "wallet",
      color_key: "green", direct_base_minor: 900000, rolled_up_base_minor: 900000,
      comparison_direct_base_minor: 800000, comparison_rolled_up_base_minor: 800000,
      transaction_count: 6, rolled_up_transaction_count: 6, largest_base_minor: 150000,
    },
  ],
  category_series: [
    { category_id: restaurantsId, start_date: "2026-03-01", base_minor: 150000 },
    { category_id: foodId, start_date: "2026-04-01", base_minor: 100000 },
    { category_id: restaurantsId, start_date: "2026-05-01", base_minor: 150000 },
    { category_id: transportId, start_date: "2026-03-01", base_minor: 200000 },
  ],
  weekdays: [
    { weekday: 1, income_base_minor: 0, spending_base_minor: 100000, transaction_count: 8 },
    { weekday: 6, income_base_minor: 0, spending_base_minor: 400000, transaction_count: 22 },
  ],
  days: [
    { date: "2026-03-02", income_base_minor: 0, spending_base_minor: 100000, transaction_count: 4 },
    { date: "2026-03-07", income_base_minor: 0, spending_base_minor: 400000, transaction_count: 10 },
  ],
  payees: [
    {
      payee: "Migros", spending_base_minor: 220000, income_base_minor: 0,
      transaction_count: 14, first_date: "2026-03-02", last_date: "2026-08-20",
    },
    {
      payee: "Shell", spending_base_minor: 90000, income_base_minor: 0,
      transaction_count: 1, first_date: "2026-03-05", last_date: "2026-08-11",
    },
  ],
  accounts: [{
    id: accountId, name: "Checking", type: "bank", currency: "TRY",
    outflow_base_minor: 600000, inflow_base_minor: 900000, transaction_count: 48,
  }],
};

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-25T09:00:00Z"));
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("AnalysisDashboard", () => {
  it("answers where the money went, when, and how the trend moved", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse(analysis))));
    const { container } = renderDashboard();

    const spent = (await screen.findByText("Total spent")).closest("article");
    expect(spent).not.toBeNull();
    expect(spent).toHaveTextContent(/6,000\.00/);
    // Spending 50% above the comparison window is unfavourable, unlike income growth.
    expect(within(spent!).getByText(/\+50%/)).toBeInTheDocument();

    const net = screen.getAllByText("Net")
      .map((element) => element.closest("article.analysis-metric-card"))
      .find(Boolean);
    expect(net).toHaveTextContent(/3,000\.00/);
    expect(net).toHaveTextContent("33.3% of income kept");

    // Top-level categories carry their subtree, so Food outranks Transport on 4,000 not 1,000.
    const spending = screen.getByRole("heading", { name: "Spending by category" })
      .closest("section");
    expect(spending).not.toBeNull();
    const rows = within(spending!).getAllByRole("listitem");
    expect(rows[0]).toHaveTextContent("Food");
    expect(rows[0]).toHaveTextContent(/4,000\.00/);
    expect(rows[1]).toHaveTextContent("Transport");

    expect(screen.getByRole("img", { name: "Spending by category" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /Spending and income by month/ })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Daily spending intensity" })).toBeInTheDocument();

    expect(screen.getByText("Migros")).toBeInTheDocument();
    // A count that reads "1 transactions" is the tell that the phrase was assembled blindly.
    expect(screen.getByText(/^1 transaction ·/)).toBeInTheDocument();
    expect(screen.getByText("Checking")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Income by category" })).toBeInTheDocument();

    await expectNoAccessibilityViolations(container);
  });

  it("requests the trailing six months anchored on the workspace day by default", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(jsonResponse(analysis)));
    vi.stubGlobal("fetch", fetchMock);
    renderDashboard();
    await screen.findByText("Total spent");

    expect(fetchMock).toHaveBeenCalledWith(
      `/v1/workspaces/${workspace.id}/spending-analysis?from_date=2026-03-01&to_date=2026-08-25`,
      expect.any(Object),
    );
  });

  it("re-requests the window when a preset or bucket width changes", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(jsonResponse(analysis)));
    vi.stubGlobal("fetch", fetchMock);
    renderDashboard();
    await screen.findByText("Total spent");

    fireEvent.click(screen.getByRole("button", { name: "This month" }));
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        `/v1/workspaces/${workspace.id}/spending-analysis?from_date=2026-08-01&to_date=2026-08-25`,
        expect.any(Object),
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "Weekly" }));
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        `/v1/workspaces/${workspace.id}/spending-analysis?from_date=2026-08-01&to_date=2026-08-25&granularity=week`,
        expect.any(Object),
      );
    });
  });

  it("rejects an inverted custom range before making another request", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(jsonResponse(analysis)));
    vi.stubGlobal("fetch", fetchMock);
    renderDashboard();
    await screen.findByText("Total spent");

    fireEvent.change(screen.getByLabelText("Analysis start date"), {
      target: { value: "2026-08-20" },
    });
    fireEvent.change(screen.getByLabelText("Analysis end date"), {
      target: { value: "2026-08-01" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply range" }));

    expect(screen.getByRole("alert")).toHaveTextContent("on or before the end date");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("reveals subcategories on demand rather than competing with their parent", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse(analysis))));
    renderDashboard();
    await screen.findByText("Total spent");

    expect(screen.queryByText("Restaurants")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show 1 subcategories" }));
    expect(screen.getByText("Restaurants")).toBeInTheDocument();
  });

  it("invites a first transaction instead of drawing empty charts", async () => {
    const empty: SpendingAnalysis = {
      ...analysis,
      totals: { ...analysis.totals, transaction_count: 0, spending_base_minor: 0 },
      series: [], categories: [], category_series: [], weekdays: [], days: [],
      payees: [], accounts: [],
    };
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse(empty))));
    renderDashboard();

    expect(await screen.findByText("Nothing to analyze yet")).toBeInTheDocument();
    expect(screen.queryByText("Total spent")).not.toBeInTheDocument();
  });

  it("offers a retry when the analysis cannot be loaded", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse(
      { error: { code: "internal_error", message: "Internal server error." } }, 500,
    ))));
    renderDashboard();

    expect(await screen.findByText("Analysis unavailable")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
  });
});

function renderDashboard() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <AnalysisDashboard workspace={workspace} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
