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
  {
    id: "category-3", workspace_id: workspace.id, parent_id: "category-1", name: "Groceries",
    kind: "expense" as const,
  },
];

const transactions: Transaction[] = [
  transaction({
    id: "expense", payee: "Market", date: "2026-08-22", amount: -18500,
    description: "Weekly groceries",
    allocations: [{ category_id: "category-1", amount_base_minor: -18500 }],
  }),
  transaction({
    id: "income", payee: "Employer", date: "2026-08-23", amount: 2200000,
    allocations: [{ category_id: "category-2", amount_base_minor: 2200000 }],
  }),
  transaction({
    id: "expense-2", payee: "Bakery", date: "2026-08-22", amount: -1500,
    allocations: [{ category_id: "category-3", amount_base_minor: -1500 }],
  }),
  transaction({ id: "pending", payee: "Coffee", date: "2026-08-21", amount: -8500, status: "pending" }),
  transaction({ id: "adjustment", payee: "Opening balance", date: "2026-08-01", amount: 500000, kind: "adjustment" }),
  transaction({ id: "transfer", payee: "Savings move", date: "2026-08-20", amount: 0, kind: "transfer" }),
];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

/**
 * The capture form chooses a category through a sectioned picker rather than two selects. A
 * category can appear twice on purpose — once under Most used and once in its own section — so
 * this takes the first tile, which is the one nearest the top of the sheet.
 */
function chooseCategory(name: string) {
  fireEvent.click(screen.getByRole("button", { name: /^Category/ }));
  const picker = screen.getByRole("dialog", { name: "Choose a category" });
  fireEvent.click(within(picker).getAllByRole("button", { name })[0]);
}

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

  it("heads each day once and leaves transfers out of the day's reading", async () => {
    installFetch();
    renderPanel(true);

    const heading = await screen.findByRole("heading", { name: /Aug 22, 2026/ });
    const day = heading.closest("section");
    expect(within(day!).getByText("Market")).toBeInTheDocument();
    expect(within(day!).getByText("Bakery")).toBeInTheDocument();
    expect(heading.querySelector(".money-amount")).not.toBeNull();

    // The transfer's day moves money without spending it, so its reading stays silent.
    const transferDay = screen.getByRole("heading", { name: /Aug 20, 2026/ });
    expect(transferDay.querySelector(".money-amount")).toBeNull();
  });

  it("carries one supporting line per row instead of a repeated date", async () => {
    installFetch();
    renderPanel(true);

    const market = (await screen.findByText("Market")).closest("article");
    // Payee is the title, so the line carries the category, then the account.
    expect(within(market!).getByText("Food · Everyday")).toBeInTheDocument();
    expect(within(market!).queryByText(/Aug 22/)).not.toBeInTheDocument();
    expect(within(market!).queryByText("Posted")).not.toBeInTheDocument();

    // Nothing is allocated here, so the account carries the line on its own.
    const coffee = screen.getByText("Coffee").closest("article");
    expect(within(coffee!).getByText("Everyday")).toBeInTheDocument();
    expect(within(coffee!).getByText("Pending")).toBeInTheDocument();
  });

  it("collapses the kind and status filters into one control", async () => {
    installFetch();
    renderPanel(true);

    expect(await screen.findByText("Filters")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Transaction status"), { target: { value: "pending" } });
    expect(screen.getByText("Filters (1)")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Transaction kind"), { target: { value: "standard" } });
    expect(screen.getByText("Filters (2)")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(screen.getByText("Filters")).toBeInTheDocument();
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

  it("opens an everyday expense in the simple form without signs or allocations", async () => {
    installFetch();
    renderPanel(true);
    const market = (await screen.findByText("Market")).closest("article");
    expect(market).not.toBeNull();

    fireEvent.click(within(market!).getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("dialog", { name: "Edit transaction" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Expense" })).toBeChecked();
    expect(screen.getByLabelText(/Amount/)).toHaveValue("185.00");
    expect(screen.getByRole("button", { name: /^Category/ })).toHaveTextContent("Food");
    expect(screen.queryByRole("group", { name: "Account entries" })).not.toBeInTheDocument();
  });

  it("derives the entry and the allocation from one unsigned amount", async () => {
    const fetchMock = installFetch();
    const { container } = renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);

    fireEvent.change(screen.getByLabelText(/Amount/), { target: { value: "45" } });
    chooseCategory("Groceries");
    fireEvent.change(screen.getByLabelText("Date"), { target: { value: "2026-08-26" } });
    fireEvent.change(screen.getByLabelText("Paid to"), { target: { value: "Market" } });
    await expectNoAccessibilityViolations(container);
    fireEvent.submit(container.querySelector("#transaction-editor-form")!);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      `/v1/workspaces/${workspace.id}/transactions`,
      expect.objectContaining({ method: "POST" }),
    ));
    const posted = fetchMock.mock.calls.find(([, init]) => init?.method === "POST");
    expect(JSON.parse(String(posted?.[1]?.body))).toEqual({
      kind: "standard",
      status: "posted",
      transaction_date: "2026-08-26",
      payee: "Market",
      entries: [{ account_id: "account-1", amount_minor: -4500 }],
      allocations: [{ category_id: "category-3", amount_base_minor: -4500 }],
    });
  });

  it("keeps one name per transaction and drops it entirely for a transfer", async () => {
    installFetch();
    renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);

    // The name of what you bought is the payee. A second free-text description belongs to the
    // detailed editor, not to recording a subscription.
    expect(screen.getByLabelText("Paid to")).toBeInTheDocument();
    expect(screen.queryByLabelText("Description (optional)")).not.toBeInTheDocument();
    // Sections come from the hierarchy, and a parent is offered as a tile in its own section.
    fireEvent.click(screen.getByRole("button", { name: /^Category/ }));
    const picker = screen.getByRole("dialog", { name: "Choose a category" });
    expect(within(picker).getByRole("heading", { name: "Food" })).toBeInTheDocument();
    expect(within(picker).getAllByRole("button", { name: "Food" }).length).toBeGreaterThan(0);
    expect(within(picker).getAllByRole("button", { name: "Groceries" }).length).toBeGreaterThan(0);
    expect(within(picker).getByRole("heading", { name: "Most used" })).toBeInTheDocument();
    fireEvent.click(within(picker).getByRole("button", { name: "Cancel" }));

    fireEvent.click(screen.getByRole("radio", { name: "Income" }));
    expect(screen.getByLabelText("Received from")).toBeInTheDocument();

    // Moving your own money has no one to pay.
    fireEvent.click(screen.getByRole("radio", { name: "Transfer" }));
    expect(screen.queryByLabelText("Paid to")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Received from")).not.toBeInTheDocument();
  });

  it("preserves a description written in the detailed editor while editing simply", async () => {
    const fetchMock = installFetch();
    const { container } = renderPanel(true);
    const market = (await screen.findByText("Market")).closest("article");

    fireEvent.click(within(market!).getByRole("button", { name: "Edit" }));
    expect(screen.queryByLabelText("Description (optional)")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Amount/), { target: { value: "190" } });
    fireEvent.submit(container.querySelector("#transaction-editor-form")!);

    await waitFor(() => expect(fetchMock.mock.calls.some(([, init]) => init?.method === "PUT")).toBe(true));
    const saved = fetchMock.mock.calls.find(([, init]) => init?.method === "PUT");
    expect(JSON.parse(String(saved?.[1]?.body)).description).toBe("Weekly groceries");
  });

  it("offers two accounts for a transfer and no category at all", async () => {
    installFetch();
    renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    fireEvent.click(screen.getByRole("radio", { name: "Transfer" }));

    expect(screen.getByLabelText("From account")).toBeInTheDocument();
    expect(screen.getByLabelText("To account")).toBeInTheDocument();
    expect(screen.queryByLabelText("Category")).not.toBeInTheDocument();
  });

  it("sends a transaction the simple form cannot express to the detailed editor", async () => {
    installFetch();
    renderPanel(true);
    const adjustment = (await screen.findByText("Opening balance")).closest("article");

    fireEvent.click(within(adjustment!).getByRole("button", { name: "Edit" }));
    expect(screen.getByLabelText("Kind")).toHaveValue("adjustment");
    expect(screen.getByLabelText(/Amount \(TRY\)/)).toHaveValue("5000.00");
    expect(screen.queryByRole("radio", { name: "Expense" })).not.toBeInTheDocument();
  });

  it("returns from an untouched detailed editor with the draft intact", async () => {
    installFetch();
    renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    fireEvent.change(screen.getByLabelText(/Amount/), { target: { value: "45" } });
    fireEvent.click(screen.getByRole("button", { name: "Use the detailed editor" }));
    fireEvent.click(screen.getByRole("button", { name: "Back to the simple form" }));

    expect(screen.getByLabelText(/Amount/)).toHaveValue("45");
    expect(screen.queryByText(/needs the detailed editor/)).not.toBeInTheDocument();
  });

  it("returns from the detailed editor even when nothing has been entered yet", async () => {
    installFetch();
    renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    fireEvent.click(screen.getByRole("button", { name: "Use the detailed editor" }));
    fireEvent.click(screen.getByRole("button", { name: "Back to the simple form" }));

    expect(screen.getByRole("radio", { name: "Expense" })).toBeChecked();
    expect(screen.queryByText(/needs the detailed editor/)).not.toBeInTheDocument();
  });

  it("lets the detailed editor leave a lone allocation to the transaction date's rate", async () => {
    const fetchMock = installFetch();
    const { container } = renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    fireEvent.change(screen.getByLabelText(/Amount/), { target: { value: "45" } });
    fireEvent.change(screen.getByLabelText("Date"), { target: { value: "2026-08-26" } });
    fireEvent.click(screen.getByRole("button", { name: "Use the detailed editor" }));
    fireEvent.click(screen.getByRole("button", { name: "Add category allocation" }));
    fireEvent.submit(container.querySelector("#transaction-editor-form")!);

    await waitFor(() => expect(fetchMock.mock.calls.some(([, init]) => init?.method === "POST")).toBe(true));
    const posted = fetchMock.mock.calls.find(([, init]) => init?.method === "POST");
    expect(JSON.parse(String(posted?.[1]?.body)).allocations).toEqual([{ category_id: "category-1" }]);
  });

  it("keeps a split in the detailed editor and says why", async () => {
    installFetch();
    renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    fireEvent.change(screen.getByLabelText(/Amount/), { target: { value: "45" } });
    chooseCategory("Food");
    fireEvent.click(screen.getByRole("button", { name: "Use the detailed editor" }));
    // A second allocation makes this a split, which only the detailed editor can divide.
    fireEvent.click(screen.getByRole("button", { name: "Add category allocation" }));
    fireEvent.click(screen.getByRole("button", { name: "Back to the simple form" }));

    expect(screen.getByText(/needs the detailed editor/)).toBeInTheDocument();
    expect(screen.getByLabelText("Kind")).toBeInTheDocument();
  });

  it("carries a simple draft into the detailed editor, and detailed edits back", async () => {
    installFetch();
    renderPanel(true);

    const add = await screen.findByRole("button", { name: "Add transaction" });
    await waitFor(() => expect(add).toBeEnabled());
    fireEvent.click(add);
    fireEvent.change(screen.getByLabelText(/Amount/), { target: { value: "45" } });
    chooseCategory("Food");
    fireEvent.click(screen.getByRole("button", { name: "Use the detailed editor" }));

    expect(screen.getByLabelText(/Amount \(TRY\)/)).toHaveValue("-45.00");
    expect(screen.getByLabelText(/Base amount \(TRY\)/)).toHaveValue("-45.00");

    fireEvent.change(screen.getByLabelText(/Amount \(TRY\)/), { target: { value: "-60.00" } });
    fireEvent.change(screen.getByLabelText(/Base amount \(TRY\)/), { target: { value: "-60.00" } });
    fireEvent.click(screen.getByRole("button", { name: "Back to the simple form" }));

    expect(screen.getByLabelText(/Amount/)).toHaveValue("60.00");
    expect(screen.getByRole("button", { name: /^Category/ })).toHaveTextContent("Food");
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
  description,
  id,
  kind = "standard",
  payee,
  status = "posted",
}: {
  allocations?: Transaction["allocations"];
  amount: number;
  date: string;
  description?: string;
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
    ...(description ? { description } : {}),
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
