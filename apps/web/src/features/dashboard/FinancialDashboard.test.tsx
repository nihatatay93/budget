import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { FinancialProjection } from "../../api/client";
import { FinancialDashboard } from "./FinancialDashboard";

const workspace = {
  id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
  name: "Personal",
  base_currency: "TRY" as const,
  timezone: "Europe/Istanbul",
};

const projection: FinancialProjection = {
  period: {
    from_date: "2026-08-01",
    to_date: "2026-08-18",
    timezone: "Europe/Istanbul",
    base_currency: "TRY",
  },
  summary: {
    balance_base_minor: { posted: 125000, pending: -5000, projected: 120000 },
    income_base_minor: { posted: 200000, pending: 0, projected: 200000 },
    spending_base_minor: { posted: 75000, pending: 500, projected: 75500 },
  },
  accounts: [{
    id: "0198b7ae-5e93-72d9-ab00-32b0861a3f37",
    name: "Checking",
    type: "bank",
    currency: "TRY",
    native_balance_minor: { posted: 125000, pending: -5000, projected: 120000 },
    base_balance_minor: { posted: 125000, pending: -5000, projected: 120000 },
  }],
  categories: [{
    id: "0198b7ae-5e93-72da-b7aa-cd015d4bb77a",
    name: "Food",
    kind: "expense",
    direct_base_minor: { posted: 75000, pending: 500, projected: 75500 },
    rolled_up_base_minor: { posted: 75000, pending: 500, projected: 75500 },
  }],
};

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("FinancialDashboard", () => {
  it("keeps authoritative and pending totals explicit and exposes drill-down links", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) =>
      Promise.resolve(String(input).includes("/budgets?")
        ? jsonResponse(monthlyBudget())
        : jsonResponse(projection)),
    ));
    renderDashboard();

    const balance = (await screen.findByText("Balance")).closest("article");
    expect(balance).not.toBeNull();
    expect(balance).toHaveTextContent(/1,250\.00/);
    expect(within(balance!).getByText("Pending delta")).toBeInTheDocument();
    expect(balance).toHaveTextContent(/-.*50\.00/);
    expect(screen.getByText("Checking")).toBeInTheDocument();
    expect(screen.getByText("Food")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Manage accounts" })).toHaveAttribute(
      "href",
      "#accounts-heading",
    );
    expect(screen.getByRole("link", { name: "Review transactions" })).toHaveAttribute(
      "href",
      "#transactions-heading",
    );
    expect(screen.getByRole("heading", { name: "August plan" })).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Current monthly budget usage" }))
      .toHaveAttribute("value", "26");
    expect(screen.getByRole("link", { name: "Review budget" })).toHaveAttribute(
      "href",
      "#monthly-budget-heading",
    );
  });

  it("requests an explicit inclusive date range only after it is applied", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => Promise.resolve(
      String(input).includes("/budgets?")
        ? jsonResponse({}, 404)
        : jsonResponse(projection),
    ));
    vi.stubGlobal("fetch", fetchMock);
    renderDashboard();
    await screen.findByText("Checking");

    fireEvent.change(screen.getByLabelText("Projection start date"), {
      target: { value: "2026-07-01" },
    });
    fireEvent.change(screen.getByLabelText("Projection end date"), {
      target: { value: "2026-07-31" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        `/v1/workspaces/${workspace.id}/financial-projection?from_date=2026-07-01&to_date=2026-07-31`,
        expect.any(Object),
      );
    });
  });
});

function renderDashboard() {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <FinancialDashboard workspace={workspace} />
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function monthlyBudget() {
  return {
    id: "0198b7ae-5e93-72d9-ab00-32b0861a3f38",
    workspace_id: workspace.id,
    name: "August plan",
    month: "2026-08",
    timezone: workspace.timezone,
    base_currency: "TRY",
    planned_base_minor: 5000,
    used_base_minor: 1300,
    remaining_base_minor: 3700,
    items: [],
    created_at: "2026-08-01T08:00:00Z",
    updated_at: "2026-08-18T08:00:00Z",
  };
}
