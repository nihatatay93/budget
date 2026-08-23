import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { FinancialProjection } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
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
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse(projection))));
    const { container } = renderDashboard();

    const balance = (await screen.findByText("Balance")).closest("article");
    expect(balance).not.toBeNull();
    expect(balance).toHaveTextContent(/1,250\.00/);
    expect(within(balance!).getByText("Pending delta")).toBeInTheDocument();
    expect(balance).toHaveTextContent(/-.*50\.00/);
    expect(screen.getByText("Checking")).toBeInTheDocument();
    expect(screen.getByText("Food")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Manage accounts" })).toHaveAttribute(
      "href",
      `/workspaces/${workspace.id}/accounts`,
    );
    expect(screen.getByRole("link", { name: "Review transactions" })).toHaveAttribute(
      "href",
      `/workspaces/${workspace.id}/transactions`,
    );
    expect(screen.getByRole("img", { name: /Income:.*2,000\.00/ })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /Spending:.*750\.00/ })).toBeInTheDocument();
    expect(screen.getByText("Net cash flow")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Review monthly plan" })).toHaveAttribute(
      "href",
      `/workspaces/${workspace.id}/budget`,
    );
    await expectNoAccessibilityViolations(container);
  });

  it("requests an explicit inclusive date range only after it is applied", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => Promise.resolve(
      jsonResponse(projection),
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
    fireEvent.click(screen.getByRole("button", { name: "Apply range" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        `/v1/workspaces/${workspace.id}/financial-projection?from_date=2026-07-01&to_date=2026-07-31`,
        expect.any(Object),
      );
    });
  });

  it("rejects an inverted date range before making another request", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(jsonResponse(projection)));
    vi.stubGlobal("fetch", fetchMock);
    renderDashboard();
    await screen.findByText("Checking");

    fireEvent.change(screen.getByLabelText("Projection start date"), {
      target: { value: "2026-08-20" },
    });
    fireEvent.change(screen.getByLabelText("Projection end date"), {
      target: { value: "2026-08-01" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply range" }));

    expect(screen.getByRole("alert")).toHaveTextContent("on or before the end date");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

function renderDashboard() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <FinancialDashboard workspace={workspace} />
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
