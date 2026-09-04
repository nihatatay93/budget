import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Category, MonthlyBudget } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
import { BudgetPanel } from "./BudgetPanel";

const workspace = {
  id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
  name: "Personal",
  base_currency: "TRY" as const,
  timezone: "Europe/Istanbul",
  role: "owner" as const,
};
const foodID = "0198b7ae-5e93-72da-b7aa-cd015d4bb77a";
const restaurantID = "0198b7ae-5e93-72da-b7aa-cd015d4bb77b";

const categories: Category[] = [{
  id: foodID,
  workspace_id: workspace.id,
  name: "Food",
  kind: "expense",
}];

const monthlyBudget: MonthlyBudget = {
  id: "0198b7ae-5e93-72d9-ab00-32b0861a3f37",
  workspace_id: workspace.id,
  name: "August plan",
  month: "2026-08",
  timezone: "Europe/Istanbul",
  base_currency: "TRY",
  planned_base_minor: 5000,
  used_base_minor: 1300,
  remaining_base_minor: 3700,
  items: [{
    id: "0198b7ae-5e93-72d9-ab00-32b0861a3f38",
    category_id: foodID,
    category_name: "Food",
    planned_base_minor: 5000,
    used_base_minor: 1300,
    remaining_base_minor: 3700,
  }],
  created_at: "2026-08-01T08:00:00Z",
  updated_at: "2026-08-18T08:00:00Z",
};

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-18T09:00:00Z"));
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("BudgetPanel", () => {
  it("shows posted usage separately from the editable plan", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const path = String(input);
      if (path.includes("/budgets?")) return Promise.resolve(jsonResponse(monthlyBudget));
      if (path.endsWith("/categories")) return Promise.resolve(jsonResponse({ categories }));
      return Promise.resolve(apiError(404, "Not found"));
    });
    vi.stubGlobal("fetch", fetchMock);
    const { container } = renderPanel(true);

    expect(await screen.findByText("August plan", { selector: ".budget-plan-title strong" }))
      .toBeInTheDocument();
    expect(document.querySelector(".budget-usage-copy .category-label")).toHaveTextContent("Food");
    expect(screen.getByText(/37\.00.*remaining/)).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Food budget usage" })).toHaveAttribute(
      "value",
      "26",
    );
    fireEvent.click(screen.getByRole("button", { name: "Edit plan" }));
    expect(screen.getByRole("dialog", { name: /Edit .* plan/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Edit complete plan" })).toBeInTheDocument();
    await expectNoAccessibilityViolations(container);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(`/v1/workspaces/${workspace.id}/budgets\\?month=\\d{4}-\\d{2}`),
      expect.any(Object),
    );
  });

  it("creates an empty month by sending one complete replacement", async () => {
    let replacement: { path: string; body: unknown } | undefined;
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      if (path.includes("/budgets?") && !init?.method) {
        return Promise.resolve(apiError(404, "No monthly budget exists."));
      }
      if (path.endsWith("/categories")) return Promise.resolve(jsonResponse({ categories }));
      if (path.includes("/budgets/") && init?.method === "PUT") {
        replacement = { path, body: JSON.parse(String(init.body)) };
        const month = path.slice(path.lastIndexOf("/") + 1);
        return Promise.resolve(jsonResponse({ ...monthlyBudget, month }));
      }
      return Promise.resolve(apiError(404, "Not found"));
    });
    vi.stubGlobal("fetch", fetchMock);
    renderPanel(true);

    expect(await screen.findByText(/No plan exists for/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Create plan" }));
    fireEvent.change(screen.getByLabelText("Plan name"), { target: { value: "Household plan" } });
    fireEvent.click(screen.getByRole("button", { name: "Add category" }));
    fireEvent.change(screen.getByLabelText("Budget amount 1"), { target: { value: "50.00" } });
    fireEvent.click(screen.getByRole("button", { name: "Save monthly plan" }));

    await waitFor(() => expect(replacement).toBeDefined());
    expect(replacement?.path).toMatch(/\/budgets\/\d{4}-\d{2}$/);
    expect(replacement?.body).toEqual({
      name: "Household plan",
      items: [{ category_id: foodID, amount_base_minor: 5000 }],
    });
  });

  it("blocks overlapping category branches before calling the API", async () => {
    const branchCategories: Category[] = [
      ...categories,
      {
        id: restaurantID,
        workspace_id: workspace.id,
        parent_id: foodID,
        name: "Restaurants",
        kind: "expense",
      },
    ];
    let putCalls = 0;
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      if (path.includes("/budgets?") && !init?.method) {
        return Promise.resolve(apiError(404, "No monthly budget exists."));
      }
      if (path.endsWith("/categories")) {
        return Promise.resolve(jsonResponse({ categories: branchCategories }));
      }
      if (init?.method === "PUT") putCalls += 1;
      return Promise.resolve(apiError(500, "Unexpected request"));
    }));
    renderPanel(true);

    await screen.findByText(/No plan exists for/);
    fireEvent.click(screen.getByRole("button", { name: "Create plan" }));
    fireEvent.click(screen.getByRole("button", { name: "Add category" }));
    fireEvent.click(screen.getByRole("button", { name: "Add category" }));
    fireEvent.change(screen.getByLabelText("Budget amount 1"), { target: { value: "50" } });
    fireEvent.change(screen.getByLabelText("Budget amount 2"), { target: { value: "25" } });
    fireEvent.click(screen.getByRole("button", { name: "Save monthly plan" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Choose a category or its subcategories, not both.",
    );
    expect(putCalls).toBe(0);
  });

  it("keeps an empty month read-only for viewers", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const path = String(input);
      if (path.includes("/budgets?")) return Promise.resolve(apiError(404, "No budget"));
      if (path.endsWith("/categories")) return Promise.resolve(jsonResponse({ categories }));
      return Promise.resolve(apiError(404, "Not found"));
    }));
    renderPanel(false);

    expect(await screen.findByText(/No plan exists for/)).toBeInTheDocument();
    expect(screen.getByText(/Viewer access can review budget usage/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add category" })).not.toBeInTheDocument();
  });

  it("shows a recoverable error when a month cannot be loaded", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const path = String(input);
      if (path.includes("/budgets?")) return Promise.resolve(apiError(500, "Budget service is unavailable."));
      if (path.endsWith("/categories")) return Promise.resolve(jsonResponse({ categories }));
      return Promise.resolve(apiError(404, "Not found"));
    }));
    renderPanel(true);

    expect(await screen.findByRole("alert")).toHaveTextContent("Budget service is unavailable.");
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save monthly plan" })).not.toBeInTheDocument();
  });

  it("navigates months and distinguishes overspending from net refunds", async () => {
    const refundCategory: Category = {
      id: restaurantID,
      workspace_id: workspace.id,
      name: "Returns",
      kind: "expense",
    };
    const edgeBudget: MonthlyBudget = {
      ...monthlyBudget,
      planned_base_minor: 10000,
      used_base_minor: 10500,
      remaining_base_minor: -500,
      items: [
        { ...monthlyBudget.items[0], used_base_minor: 11000, remaining_base_minor: -6000 },
        {
          id: "refund-item",
          category_id: refundCategory.id,
          category_name: refundCategory.name,
          planned_base_minor: 5000,
          used_base_minor: -500,
          remaining_base_minor: 5500,
        },
      ],
    };
    const requested: string[] = [];
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const path = String(input);
      requested.push(path);
      if (path.endsWith("/categories")) {
        return Promise.resolve(jsonResponse({ categories: [...categories, refundCategory] }));
      }
      return Promise.resolve(jsonResponse(edgeBudget));
    }));
    renderPanel(true);

    expect(await screen.findByText("Over budget")).toBeInTheDocument();
    expect(screen.getByText("Net refund")).toBeInTheDocument();
    expect(screen.getByText("Over plan")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Previous month" }));
    await waitFor(() => expect(requested.some((path) => path.includes("month=2026-07"))).toBe(true));
  });
});

function renderPanel(canManage: boolean) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <BudgetPanel workspace={workspace} canManage={canManage} />
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function apiError(status: number, message: string) {
  return new Response(JSON.stringify({
    error: { code: "test_error", message, request_id: "test" },
  }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
