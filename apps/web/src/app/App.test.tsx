import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";

import { App } from "./App";

afterEach(() => vi.unstubAllGlobals());

test("shows login when there is no session", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          error: { code: "unauthorized", message: "Authentication is required.", request_id: "test" },
        }),
        { status: 401, headers: { "Content-Type": "application/json" } },
      ),
    ),
  );
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={["/login"]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );

  expect(screen.getByRole("heading", { name: "Budget" })).toBeInTheDocument();
  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Create a workspace" })).toHaveAttribute(
    "href",
    "/register",
  );
});

test("loads accounts and protected categories for the selected workspace", async () => {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const path = String(input);
    if (path === "/v1/session") {
      return Promise.resolve(jsonResponse({
        user: { id: "0198b7ae-5e93-72d7-a256-2a0f6622c7ec", email: "owner@example.com", display_name: "Owner" },
        workspaces: [{
          id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
          name: "Personal",
          base_currency: "TRY",
          timezone: "Europe/Istanbul",
          role: "owner",
        }],
      }));
    }
    if (path.endsWith("/accounts")) {
      return Promise.resolve(jsonResponse({ accounts: [{
        id: "0198b7ae-5e93-72d9-ab00-32b0861a3f37",
        workspace_id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
        name: "Checking",
        type: "bank",
        currency: "TRY",
        balance_minor: 0,
      }] }));
    }
    if (path.endsWith("/categories")) {
      return Promise.resolve(jsonResponse({ categories: [{
        id: "0198b7ae-5e93-72da-b7aa-cd015d4bb77a",
        workspace_id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
        name: "Uncategorized Expense",
        kind: "expense",
        system_key: "uncategorized_expense",
      }] }));
    }
    if (path.endsWith("/financial-projection")) {
      return Promise.resolve(jsonResponse({
        period: {
          from_date: "2026-08-01",
          to_date: "2026-08-18",
          timezone: "Europe/Istanbul",
          base_currency: "TRY",
        },
        summary: {
          balance_base_minor: { posted: 0, pending: 0, projected: 0 },
          income_base_minor: { posted: 0, pending: 0, projected: 0 },
          spending_base_minor: { posted: 0, pending: 0, projected: 0 },
        },
        accounts: [],
        categories: [],
      }));
    }
    return Promise.resolve(jsonResponse({}, 404));
  });
  vi.stubGlobal("fetch", fetchMock);

  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={["/"]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );

  expect(await screen.findByRole("heading", { name: "Accounts" })).toBeInTheDocument();
  expect(await screen.findByText("Checking")).toBeInTheDocument();
  expect(await screen.findByText("Uncategorized Expense", { selector: "strong" })).toBeInTheDocument();
  expect(screen.getByText("expense · protected")).toBeInTheDocument();
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
