import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Account, ExchangeRate } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
import { AccountsPanel } from "./AccountsPanel";

const workspace = {
  id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
  name: "Personal",
  base_currency: "TRY" as const,
  timezone: "Europe/Istanbul",
  role: "owner" as const,
};

const accounts: Account[] = [
  {
    id: "account-1",
    workspace_id: workspace.id,
    name: "Everyday",
    type: "bank",
    currency: "TRY",
    institution_name: "Budget Bank",
    balance_minor: 100000,
  },
  {
    id: "account-2",
    workspace_id: workspace.id,
    name: "Travel card",
    type: "credit_card",
    currency: "EUR",
    balance_minor: -14820,
  },
  {
    id: "account-3",
    workspace_id: workspace.id,
    name: "Old cash",
    type: "cash",
    currency: "TRY",
    balance_minor: 5000,
    archived_at: "2026-07-01T12:00:00Z",
  },
];

const rates: ExchangeRate[] = [{
  base_currency: "TRY",
  quote_currency: "EUR",
  rate: "0.020000",
  rate_date: "2026-08-22",
}];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("AccountsPanel", () => {
  it("separates active and archived accounts and explains multi-currency totals", async () => {
    installFetch();
    const { container } = renderPanel(true);

    expect(await screen.findByText("Everyday")).toBeInTheDocument();
    expect(screen.getByText("Travel card")).toBeInTheDocument();
    expect(screen.getByText(/another currency.*not included/i)).toBeInTheDocument();
    expect(screen.getByText("1 archived account")).toBeInTheDocument();
    expect(screen.getByText("Old cash")).toBeInTheDocument();
    expect(screen.getByLabelText("Show in")).toHaveValue("TRY");
    await expectNoAccessibilityViolations(container);
  });

  it("surfaces the account currency lock from the server in the editor", async () => {
    installFetch({ currencyConflict: true });
    renderPanel(true);
    const everyday = (await screen.findByText("Everyday")).closest("article");
    fireEvent.click(within(everyday!).getByRole("button", { name: "Edit" }));

    expect(screen.getByRole("dialog", { name: "Edit Everyday" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Currency"), { target: { value: "USD" } });
    fireEvent.click(screen.getByRole("button", { name: "Save account" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Account currency cannot change after history exists.",
    );
  });

  it("confirms archival and preserves a clear historical-data warning", async () => {
    const fetchMock = installFetch();
    renderPanel(true);
    const everyday = (await screen.findByText("Everyday")).closest("article");
    fireEvent.click(within(everyday!).getByRole("button", { name: "Archive" }));

    const dialog = screen.getByRole("dialog", { name: "Archive Everyday?" });
    expect(dialog).toHaveTextContent("preserving every historical entry and report");
    fireEvent.click(within(dialog).getByRole("button", { name: "Archive account" }));

    expect(await screen.findByText("Account archived")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      `/v1/workspaces/${workspace.id}/accounts/account-1`,
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("keeps balances and archived accounts read-only for viewers", async () => {
    installFetch();
    renderPanel(false);

    expect(await screen.findByText("Everyday")).toBeInTheDocument();
    expect(screen.getByText(/Viewer access can review balances/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add account" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Archive" })).not.toBeInTheDocument();
  });

  it("offers a focused first-account action when the workspace is empty", async () => {
    installFetch({ accountData: [], ratesData: [] });
    renderPanel(true);

    expect(await screen.findByText("No active accounts")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create first account" })).toBeInTheDocument();
  });
});

function installFetch({
  accountData = accounts,
  currencyConflict = false,
  ratesData = rates,
}: {
  accountData?: Account[];
  currencyConflict?: boolean;
  ratesData?: ExchangeRate[];
} = {}) {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    if (path.endsWith("/exchange-rates")) return Promise.resolve(jsonResponse({ rates: ratesData }));
    if (path.endsWith("/accounts") && (!init?.method || init.method === "GET")) {
      return Promise.resolve(jsonResponse({ accounts: accountData }));
    }
    if (init?.method === "PUT" && currencyConflict) {
      return Promise.resolve(apiError(409, "Account currency cannot change after history exists."));
    }
    if (init?.method === "DELETE") return Promise.resolve(new Response(null, { status: 204 }));
    return Promise.resolve(jsonResponse(accounts[0]));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function renderPanel(canManage: boolean) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <AccountsPanel canManage={canManage} workspace={{ ...workspace, role: canManage ? "owner" : "viewer" }} />
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function apiError(status: number, message: string) {
  return jsonResponse({ error: { code: "test_error", message, request_id: "test" } }, status);
}
