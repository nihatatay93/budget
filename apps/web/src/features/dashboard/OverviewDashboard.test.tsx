import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { FinancialProjection, MonthlyBudget, SessionResponse, Transaction } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
import { OverviewDashboard } from "./OverviewDashboard";

type Workspace = SessionResponse["workspaces"][number];

const workspace: Workspace = {
  id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
  name: "Personal",
  base_currency: "TRY",
  timezone: "Europe/Istanbul",
  role: "owner",
};

const projection: FinancialProjection = {
  period: {
    from_date: "2026-08-01",
    to_date: "2026-08-23",
    timezone: workspace.timezone,
    base_currency: workspace.base_currency,
  },
  summary: {
    balance_base_minor: { posted: 1285000, pending: -24500, projected: 1260500 },
    income_base_minor: { posted: 2200000, pending: 0, projected: 2200000 },
    spending_base_minor: { posted: 915000, pending: 24500, projected: 939500 },
  },
  accounts: [{
    id: "account-1",
    name: "Everyday",
    type: "bank",
    currency: "TRY",
    native_balance_minor: { posted: 1285000, pending: -24500, projected: 1260500 },
    base_balance_minor: { posted: 1285000, pending: -24500, projected: 1260500 },
  }],
  categories: [{
    id: "category-1",
    name: "Food",
    kind: "expense",
    icon: "🍲",
    direct_base_minor: { posted: 280000, pending: 24500, projected: 304500 },
    rolled_up_base_minor: { posted: 280000, pending: 24500, projected: 304500 },
  }],
};

const budget: MonthlyBudget = {
  id: "budget-1",
  workspace_id: workspace.id,
  name: "August plan",
  month: "2026-08",
  timezone: workspace.timezone,
  base_currency: workspace.base_currency,
  planned_base_minor: 1500000,
  used_base_minor: 600000,
  remaining_base_minor: 900000,
  items: [{
    id: "budget-item-1",
    category_id: "category-1",
    category_name: "Food",
    category_icon: "🍲",
    planned_base_minor: 500000,
    used_base_minor: 280000,
    remaining_base_minor: 220000,
  }],
  created_at: "2026-08-01T00:00:00Z",
  updated_at: "2026-08-01T00:00:00Z",
};

const transactions: Transaction[] = [
  transaction({ id: "expense", date: "2026-08-22", payee: "Market", amount: -18500 }),
  transaction({ id: "income", date: "2026-08-23", payee: "Salary", amount: 2200000 }),
  transaction({ id: "transfer", date: "2026-08-21", kind: "transfer", payee: "Savings move", amount: 0 }),
];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("OverviewDashboard", () => {
  it("shows a populated, accessible home view with current context and quick actions", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const path = String(input);
      if (path.includes("financial-projection")) return Promise.resolve(jsonResponse(projection));
      if (path.includes("/budgets?")) return Promise.resolve(jsonResponse(budget));
      if (path.endsWith("/transactions")) return Promise.resolve(jsonResponse({ transactions }));
      return Promise.resolve(jsonResponse({}, 404));
    }));
    const { container } = renderOverview(workspace, true);

    expect(screen.getByRole("status", { name: "Loading financial overview" })).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Monthly budget" })).toBeInTheDocument();
    expect(screen.getByText("Salary")).toBeInTheDocument();
    expect(screen.getByText("Savings move")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Add transaction/ })).toHaveAttribute(
      "href",
      `/workspaces/${workspace.id}/transactions`,
    );
    expect(screen.getByRole("progressbar", { name: "Current monthly budget usage" })).toHaveValue(40);
    expect(screen.getByRole("heading", { name: "Top spending" })).toBeInTheDocument();
    await expectNoAccessibilityViolations(container);
  });

  it("keeps empty and read-only states useful", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const path = String(input);
      if (path.includes("financial-projection")) {
        return Promise.resolve(jsonResponse({ ...projection, accounts: [], categories: [] }));
      }
      if (path.includes("/budgets?")) return Promise.resolve(jsonResponse({}, 404));
      if (path.endsWith("/transactions")) return Promise.resolve(jsonResponse({ transactions: [] }));
      return Promise.resolve(jsonResponse({}, 404));
    }));
    renderOverview({ ...workspace, role: "viewer" }, false);

    expect(await screen.findByText("No plan for this month")).toBeInTheDocument();
    expect(screen.getByText("No transactions yet")).toBeInTheDocument();
    expect(screen.getByText("No accounts yet")).toBeInTheDocument();
    expect(screen.getByText("No posted spending yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Review transactions/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Add transaction/ })).not.toBeInTheDocument();
  });

  it("surfaces projection errors and retries the authoritative query", async () => {
    let projectionAttempts = 0;
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const path = String(input);
      if (path.includes("financial-projection")) {
        projectionAttempts += 1;
        return Promise.resolve(projectionAttempts === 1
          ? jsonResponse({ error: { code: "internal", message: "Projection failed", request_id: "test" } }, 500)
          : jsonResponse(projection));
      }
      if (path.includes("/budgets?")) return Promise.resolve(jsonResponse({}, 404));
      if (path.endsWith("/transactions")) return Promise.resolve(jsonResponse({ transactions: [] }));
      return Promise.resolve(jsonResponse({}, 404));
    }));
    renderOverview(workspace, true);

    expect(await screen.findByRole("alert")).toHaveTextContent("Projection failed");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Posted balance")).toBeInTheDocument();
    expect(projectionAttempts).toBe(2);
  });
});

function renderOverview(value: Workspace, canManage: boolean) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <OverviewDashboard canManage={canManage} workspace={value} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function transaction({
  amount,
  date,
  id,
  kind = "standard",
  payee,
}: {
  amount: number;
  date: string;
  id: string;
  kind?: Transaction["kind"];
  payee: string;
}): Transaction {
  return {
    id,
    workspace_id: workspace.id,
    kind,
    status: "posted",
    transaction_date: date,
    payee,
    source: "manual",
    created_by: "user-1",
    updated_by: "user-1",
    created_at: `${date}T12:00:00Z`,
    updated_at: `${date}T12:00:00Z`,
    entries: kind === "transfer"
      ? [
          { account_id: "account-1", amount_minor: -10000, base_amount_minor: -10000 },
          { account_id: "account-2", amount_minor: 10000, base_amount_minor: 10000 },
        ]
      : [{ account_id: "account-1", amount_minor: amount, base_amount_minor: amount }],
    allocations: [],
  };
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
