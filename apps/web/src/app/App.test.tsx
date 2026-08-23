import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";

import { App } from "./App";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

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

  expect(screen.getByRole("heading", { name: "Opening Budget" })).toBeInTheDocument();
  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Budget" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Create a workspace" })).toHaveAttribute(
    "href",
    "/register",
  );
});

test("deep-links into a workspace destination and navigates without losing its workspace", async () => {
  const workspaceId = "0198b7ae-5e93-72d8-99af-ff40c48ad342";
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const path = String(input);
    if (path === "/v1/session") {
      return Promise.resolve(jsonResponse({
        user: { id: "0198b7ae-5e93-72d7-a256-2a0f6622c7ec", email: "owner@example.com", display_name: "Owner" },
        workspaces: [{
          id: workspaceId,
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
        workspace_id: workspaceId,
        name: "Checking",
        type: "bank",
        currency: "TRY",
        balance_minor: 0,
      }] }));
    }
    if (path.endsWith("/categories")) {
      return Promise.resolve(jsonResponse({ categories: [{
        id: "0198b7ae-5e93-72da-b7aa-cd015d4bb77a",
        workspace_id: workspaceId,
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
      <MemoryRouter initialEntries={[`/workspaces/${workspaceId}/accounts`]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );

  expect(await screen.findByRole("heading", { name: "Accounts", level: 1 })).toBeInTheDocument();
  expect(await screen.findByText("Checking")).toBeInTheDocument();
  const management = screen.getByRole("navigation", { name: "Manage" });
  expect(within(management).getByRole("link", { name: "Accounts" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  fireEvent.click(within(management).getByRole("link", { name: "Categories" }));
  expect(await screen.findByRole("heading", { name: "Categories", level: 1 })).toBeInTheDocument();
  expect(await screen.findByText("Uncategorized Expense")).toBeInTheDocument();
  expect(screen.getByText("Protected")).toBeInTheDocument();
});

test("marks the revoked session absent before routing to sign in", async () => {
  const workspaceId = "0198b7ae-5e93-72d8-99af-ff40c48ad342";
  let authenticated = true;
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    if (path === "/v1/auth/logout" && init?.method === "POST") {
      authenticated = false;
      return Promise.resolve(new Response(null, { status: 204 }));
    }
    if (path === "/v1/session") {
      if (!authenticated) {
        return Promise.resolve(jsonResponse({
          error: { code: "unauthorized", message: "Authentication is required.", request_id: "test" },
        }, 401));
      }
      return Promise.resolve(jsonResponse({
        user: {
          id: "0198b7ae-5e93-72d7-a256-2a0f6622c7ec",
          email: "owner@example.com",
          display_name: "Owner",
        },
        workspaces: [{
          id: workspaceId,
          name: "Personal",
          base_currency: "TRY",
          timezone: "Europe/Istanbul",
          role: "owner",
        }],
      }));
    }
    return Promise.resolve(jsonResponse({}, 404));
  });
  vi.stubGlobal("fetch", fetchMock);

  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={[`/workspaces/${workspaceId}/more`]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );

  expect(await screen.findByRole("heading", { name: "More", level: 1 })).toBeInTheDocument();
  fireEvent.click(screen.getAllByRole("button", { name: "Sign out" })[0]);

  await waitFor(() => expect(document.body).toHaveTextContent("Sign in"));
  expect(fetchMock).toHaveBeenCalledWith(
    "/v1/auth/logout",
    expect.objectContaining({ method: "POST" }),
  );
  expect(fetchMock.mock.calls.filter(([input]) => String(input) === "/v1/session")).toHaveLength(1);
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
