import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Category } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
import { CategoriesPanel } from "./CategoriesPanel";

const workspace = {
  id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
  name: "Personal",
  base_currency: "TRY" as const,
  timezone: "Europe/Istanbul",
  role: "owner" as const,
};

const categories: Category[] = [
  {
    id: "restaurants",
    workspace_id: workspace.id,
    parent_id: "food",
    name: "Restaurants",
    kind: "expense",
  },
  {
    id: "food",
    workspace_id: workspace.id,
    name: "Food",
    kind: "expense",
    icon: "🍲",
  },
  {
    id: "uncategorized-expense",
    workspace_id: workspace.id,
    name: "Uncategorized",
    kind: "expense",
    system_key: "uncategorized_expense",
  },
  {
    id: "salary",
    workspace_id: workspace.id,
    name: "Salary",
    kind: "income",
  },
  {
    id: "old-travel",
    workspace_id: workspace.id,
    name: "Old travel",
    kind: "expense",
    archived_at: "2026-07-01T10:00:00Z",
  },
];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

/**
 * Categories are presented as tiles now, and a tile reveals its own row of actions when chosen,
 * so a test reaches Edit or Archive by picking the category first.
 */
async function openCategory(name: string) {
  fireEvent.click(await screen.findByRole("button", { name }));
  return document.querySelector(".category-hierarchy-row") as HTMLElement;
}

describe("CategoriesPanel presentation", () => {
  it("separates kinds, preserves hierarchy, and identifies protected and archived rows", async () => {
    installFetch();
    const { container } = renderPanel(true);

    const expense = await screen.findByRole("region", { name: "Expense" });
    // Food heads its own section and is also the first tile inside it, because a parent is a
    // category in its own right.
    expect(within(expense).getByRole("heading", { name: "Food" })).toBeInTheDocument();
    expect(within(expense).getByRole("button", { name: "Restaurants" })).toBeInTheDocument();
    const protectedRow = await openCategory("Uncategorized Expense");
    expect(within(protectedRow).getByText("Protected")).toBeInTheDocument();
    expect(within(protectedRow).queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Income" })).toHaveTextContent("Salary");
    expect(screen.getByText("1 archived category")).toBeInTheDocument();
    expect(screen.getByText("Old travel")).toBeInTheDocument();
    expect(screen.getByText(/without constraining their sign/i)).toBeInTheDocument();
    await expectNoAccessibilityViolations(container);
  });

  it("does not offer descendants as parents while editing a hierarchy", async () => {
    installFetch();
    renderPanel(true);
    const food = await openCategory("Food");
    fireEvent.click(within(food).getByRole("button", { name: "Edit" }));

    const parent = screen.getByLabelText("Parent (optional)");
    expect(screen.getByRole("dialog", { name: "Edit Food" })).toBeInTheDocument();
    expect(within(parent).queryByRole("option", { name: "Food" })).not.toBeInTheDocument();
    expect(within(parent).queryByRole("option", { name: "Restaurants" })).not.toBeInTheDocument();
    expect(within(parent).queryByRole("option", { name: "Salary" })).not.toBeInTheDocument();
  });

  it("confirms archival and surfaces the active-child protection", async () => {
    installFetch({ archiveConflict: true });
    renderPanel(true);
    const food = await openCategory("Food");
    fireEvent.click(within(food).getByRole("button", { name: "Archive" }));

    const dialog = screen.getByRole("dialog", { name: "Archive Food?" });
    expect(dialog).toHaveTextContent("active children cannot be archived");
    fireEvent.click(within(dialog).getByRole("button", { name: "Archive category" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Category has active children.");
  });

  it("keeps protected and ordinary categories read-only for viewers", async () => {
    installFetch();
    renderPanel(false);

    // Food names its section and its own tile, so the heading is the unambiguous one.
    expect(await screen.findByRole("heading", { name: "Food" })).toBeInTheDocument();
    expect(screen.getByText(/Viewer access can review the reporting hierarchy/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add category" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Archive" })).not.toBeInTheDocument();
  });

  it("explains the expected protected baseline when no categories are returned", async () => {
    installFetch({ categoryData: [] });
    renderPanel(true);

    expect(await screen.findByText("No active categories")).toBeInTheDocument();
    expect(screen.getByText(/Protected Uncategorized categories are normally created/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create category" })).toBeInTheDocument();
  });

  it("creates a category with the selected emoji and semantic color key", async () => {
    const fetchMock = installFetch();
    renderPanel(true);
    fireEvent.click(await screen.findByRole("button", { name: "Add category" }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Side project" } });
    fireEvent.click(screen.getByRole("button", { name: "Emoji" }));
    fireEvent.click(screen.getByRole("button", { name: "Use 👩🏽‍💻 as the category icon" }));
    fireEvent.click(screen.getByRole("button", { name: "Purple color" }));
    await expectNoAccessibilityViolations(screen.getByRole("dialog"));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Add category" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/categories$/),
      expect.objectContaining({ method: "POST" }),
    ));
    const request = fetchMock.mock.calls.find(([, init]) => init?.method === "POST");
    if (!request) throw new Error("The create request was not made.");
    expect(JSON.parse(String(request[1]?.body))).toMatchObject({
      name: "Side project", kind: "expense", icon_type: "emoji", icon_value: "👩🏽‍💻", color_key: "purple",
    });
  });

  it("updates an existing category with the selected semantic appearance", async () => {
    const fetchMock = installFetch();
    renderPanel(true);
    const food = await openCategory("Food");
    fireEvent.click(within(food).getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "System icon" }));
    fireEvent.click(screen.getByRole("button", { name: "Home" }));
    fireEvent.click(screen.getByRole("button", { name: "Green color" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Save category" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/categories\/food$/),
      expect.objectContaining({ method: "PUT" }),
    ));
    const request = fetchMock.mock.calls.find(([, init]) => init?.method === "PUT");
    if (!request) throw new Error("The update request was not made.");
    expect(JSON.parse(String(request[1]?.body))).toMatchObject({
      name: "Food", kind: "expense", icon_type: "system", icon_value: "home", color_key: "green",
    });
  });
});

function installFetch({
  archiveConflict = false,
  categoryData = categories,
}: {
  archiveConflict?: boolean;
  categoryData?: Category[];
} = {}) {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    if (path.endsWith("/categories") && (!init?.method || init.method === "GET")) {
      return Promise.resolve(jsonResponse({ categories: categoryData }));
    }
    if (init?.method === "DELETE" && archiveConflict) {
      return Promise.resolve(apiError(409, "Category has active children."));
    }
    if (init?.method === "DELETE") return Promise.resolve(new Response(null, { status: 204 }));
    return Promise.resolve(jsonResponse(categories[0]));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function renderPanel(canManage: boolean) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <CategoriesPanel canManage={canManage} workspace={{ ...workspace, role: canManage ? "owner" : "viewer" }} />
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
