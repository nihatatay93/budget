import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { SessionResponse, Transaction } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
import { TransactionsPanel } from "./TransactionsPanel";

type Workspace = SessionResponse["workspaces"][number];

const workspace: Workspace = {
  id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
  name: "Personal",
  base_currency: "TRY",
  timezone: "Europe/Istanbul",
  role: "owner",
};

const accounts = [
  {
    id: "account-1", workspace_id: workspace.id, name: "Everyday", type: "bank" as const,
    currency: "TRY" as const, balance_minor: 100000,
  },
  {
    id: "account-2", workspace_id: workspace.id, name: "Savings", type: "savings" as const,
    currency: "TRY" as const, balance_minor: 200000,
  },
];

const categories = [
  {
    id: "category-1", workspace_id: workspace.id, name: "Food", kind: "expense" as const,
  },
  {
    id: "category-2", workspace_id: workspace.id, name: "Salary", kind: "income" as const,
  },
];

const transactions: Transaction[] = [
  transaction({
    id: "expense", payee: "Market", date: "2026-08-22", amount: -18500,
    allocations: [{ category_id: "category-1", amount_base_minor: -18500 }],
  }),
  transaction({
    id: "income", payee: "Employer", date: "2026-08-23", amount: 2200000,
    allocations: [{ category_id: "category-2", amount_base_minor: 2200000 }],
  }),
  transaction({ id: "pending", payee: "Coffee", date: "2026-08-21", amount: -8500, status: "pending" }),
  transaction({ id: "adjustment", payee: "Opening balance", date: "2026-08-01", amount: 500000, kind: "adjustment" }),
  transaction({ id: "transfer", payee: "Savings move", date: "2026-08-20", amount: 0, kind: "transfer" }),
];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("TransactionsPanel", () => {
  it("renders every kind and filters the register by status, kind, and search", async () => {
    installFetch();
    const { container } = renderPanel(true);

    expect(await screen.findByText("Market")).toBeInTheDocument();
    expect(screen.getByText("Employer")).toBeInTheDocument();
    expect(screen.getByText("Savings move")).toBeInTheDocument();
    expect(screen.getByText("Opening balance")).toBeInTheDocument();
    expect(screen.getByText("Transfer", { selector: ".transaction-register-amount span" })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Transaction status"), { target: { value: "pending" } });
    expect(screen.getByText("Coffee")).toBeInTheDocument();
    expect(screen.queryByText("Market")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Transaction status"), { target: { value: "all" } });
    fireEvent.change(screen.getByLabelText("Transaction kind"), { target: { value: "transfer" } });
    expect(screen.getByText("Savings move")).toBeInTheDocument();
    expect(screen.queryByText("Employer")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Transaction kind"), { target: { value: "all" } });
    fireEvent.change(screen.getByLabelText("Search transactions"), { target: { value: "Salary" } });
    expect(screen.getByText("Employer")).toBeInTheDocument();
    expect(screen.queryByText("Coffee")).not.toBeInTheDocument();
    await expectNoAccessibilityViolations(container);
  });

  it("keeps the complete register read-only for viewers", async () => {
    installFetch();
    renderPanel(false);

    expect(await screen.findByText("Market")).toBeInTheDocument();
    expect(screen.getByText(/only workspace managers can change it/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add transaction" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
  });

  it("opens a hydrated editor and preserves transfer structure", async () => {
    installFetch();
    renderPanel(true);
    const market = (await screen.findByText("Market")).closest("article");
    expect(market).not.toBeNull();

    fireEvent.click(within(market!).getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("dialog", { name: "Edit transaction" })).toBeInTheDocument();
    expect(screen.getByLabelText(/Amount \(TRY\)/)).toHaveValue("-185.00");
    expect(screen.getByLabelText(/Base amount \(TRY\)/)).toHaveValue("-185.00");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    const add = screen.getByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    fireEvent.change(screen.getByLabelText("Kind"), { target: { value: "transfer" } });
    expect(screen.getAllByLabelText("Account")).toHaveLength(2);
    expect(screen.queryByRole("group", { name: "Category allocations" })).not.toBeInTheDocument();
  });

  it("confirms soft deletion and reports completion", async () => {
    const fetchMock = installFetch();
    renderPanel(true);
    const market = (await screen.findByText("Market")).closest("article");
    fireEvent.click(within(market!).getByRole("button", { name: "Delete" }));

    const dialog = screen.getByRole("dialog", { name: "Delete this transaction?" });
    expect(dialog).toHaveTextContent("stops affecting balances and reports");
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete transaction" }));

    expect(await screen.findByText("Transaction deleted")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      `/v1/workspaces/${workspace.id}/transactions/expense`,
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});

function installFetch() {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    if (path.endsWith("/accounts")) return Promise.resolve(jsonResponse({ accounts }));
    if (path.endsWith("/categories")) return Promise.resolve(jsonResponse({ categories }));
    if (path.endsWith("/transactions") && (!init?.method || init.method === "GET")) {
      return Promise.resolve(jsonResponse({ transactions }));
    }
    if (init?.method === "DELETE") return Promise.resolve(new Response(null, { status: 204 }));
    return Promise.resolve(jsonResponse(transactions[0]));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function renderPanel(canManage: boolean) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <TransactionsPanel canManage={canManage} workspace={{ ...workspace, role: canManage ? "owner" : "viewer" }} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function transaction({
  allocations = [],
  amount,
  date,
  id,
  kind = "standard",
  payee,
  status = "posted",
}: {
  allocations?: Transaction["allocations"];
  amount: number;
  date: string;
  id: string;
  kind?: Transaction["kind"];
  payee: string;
  status?: Transaction["status"];
}): Transaction {
  return {
    id,
    workspace_id: workspace.id,
    kind,
    status,
    transaction_date: date,
    payee,
    source: "manual",
    created_by: "user-1",
    updated_by: "user-1",
    created_at: `${date}T12:00:00Z`,
    updated_at: `${date}T12:00:00Z`,
    entries: kind === "transfer"
      ? [
          { account_id: "account-1", amount_minor: -100000, base_amount_minor: -100000 },
          { account_id: "account-2", amount_minor: 100000, base_amount_minor: 100000 },
        ]
      : [{ account_id: "account-1", amount_minor: amount, base_amount_minor: amount }],
    allocations,
  };
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
