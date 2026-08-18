import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { SessionResponse, WorkspaceInvitation } from "../../api/client";
import { InvitationsPanel } from "./InvitationsPanel";

type Workspace = SessionResponse["workspaces"][number];

const workspaceID = "0198b7ae-5e93-72d8-99af-ff40c48ad342";
const inviterID = "0198b7ae-5e93-72d7-a256-2a0f6622c7ec";
const acceptanceToken = "P4tGZ3sYy1s0Rn7cQvT2XkLb8mHfJ9aWuE6dN5rVzQo";

function workspaceAs(role: Workspace["role"]): Workspace {
  return {
    id: workspaceID, name: "Atay Family", base_currency: "TRY",
    timezone: "Europe/Istanbul", role,
  };
}

const invitations: WorkspaceInvitation[] = [{
  id: "0198b7ae-5e93-72d9-ab00-32b0861a3f37",
  workspace_id: workspaceID,
  email: "invited@example.com",
  role: "member",
  invited_by: inviterID,
  inviter_display_name: "Owner",
  expires_at: "2026-09-01T00:00:00Z",
  created_at: "2026-08-18T00:00:00Z",
}];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("InvitationsPanel", () => {
  it("offers an owner every invitable role", async () => {
    const fetchMock = stubFetch();
    renderPanel("owner");
    expect(await screen.findByText("invited@example.com")).toBeInTheDocument();

    const options = screen.getAllByRole("option").map((option) => option.textContent);
    expect(options).toEqual(["admin", "member", "viewer"]);
    expect(fetchMock).toHaveBeenCalled();
  });

  // An admin may not create a peer, so "admin" must not be selectable.
  it("withholds the admin role from an admin", async () => {
    stubFetch();
    renderPanel("admin");
    await screen.findByText("invited@example.com");

    const options = screen.getAllByRole("option").map((option) => option.textContent);
    expect(options).toEqual(["member", "viewer"]);
  });

  // Pending invitations expose non-members' email addresses. A member must see nothing and
  // the panel must not even ask, rather than relying on the server's 403.
  it("renders nothing and issues no request for a member", () => {
    const fetchMock = stubFetch();
    renderPanel("member");

    expect(screen.queryByRole("heading", { name: "Invitations" })).not.toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("discloses the one-time acceptance token after creating an invitation", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) =>
      init?.method === "POST"
        ? jsonResponse({ invitation: invitations[0], acceptance_token: acceptanceToken }, 201)
        : jsonResponse({ invitations }),
    );
    vi.stubGlobal("fetch", fetchMock);
    renderPanel("owner");
    await screen.findByText("invited@example.com");

    fireEvent.change(screen.getByLabelText("Email"), {
      target: { value: "new@example.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create invitation" }));

    await waitFor(() => expect(screen.getByText(acceptanceToken)).toBeInTheDocument());
    // The token is a credential: it must reach the page body and nothing else.
    const posted = fetchMock.mock.calls.find(([, init]) => init?.method === "POST");
    expect(posted?.[0]).toBe(`/v1/workspaces/${workspaceID}/invitations`);
    expect(String(posted?.[0])).not.toContain(acceptanceToken);
  });
});

function stubFetch() {
  const fetchMock = vi.fn(async () => jsonResponse({ invitations }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function renderPanel(role: Workspace["role"]) {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <InvitationsPanel workspace={workspaceAs(role)} />
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
