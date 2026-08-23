import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { SessionResponse, WorkspaceMember } from "../../api/client";
import { MembersPanel } from "./MembersPanel";

type Workspace = SessionResponse["workspaces"][number];

const workspaceID = "0198b7ae-5e93-72d8-99af-ff40c48ad342";
const ownerID = "0198b7ae-5e93-72d7-a256-2a0f6622c7ec";
const adminID = "0198b7ae-5e93-72d7-a256-2a0f6622c7ed";
const memberID = "0198b7ae-5e93-72d7-a256-2a0f6622c7ee";

function workspaceAs(role: Workspace["role"]): Workspace {
  return {
    id: workspaceID,
    name: "Atay Family",
    base_currency: "TRY",
    timezone: "Europe/Istanbul",
    role,
  };
}

const members: WorkspaceMember[] = [
  {
    user_id: ownerID, email: "owner@example.com", display_name: "Owner",
    role: "owner", joined_at: "2026-08-01T00:00:00Z",
  },
  {
    user_id: adminID, email: "admin@example.com", display_name: "Admin",
    role: "admin", joined_at: "2026-08-02T00:00:00Z",
  },
  {
    user_id: memberID, email: "member@example.com", display_name: "Member",
    role: "member", joined_at: "2026-08-03T00:00:00Z",
  },
];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("MembersPanel", () => {
  it("lets an owner change any role and remove anyone", async () => {
    renderPanel(workspaceAs("owner"), ownerID);
    expect(await screen.findByText("member@example.com")).toBeInTheDocument();

    // The owner sees a role control for every member, including themselves.
    expect(screen.getAllByRole("combobox")).toHaveLength(3);
    expect(screen.getAllByRole("button", { name: "Remove" })).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Leave" })).toBeInTheDocument();
  });

  // An admin may not touch a peer or the owner, so those rows offer no controls at all.
  it("hides controls an admin may not use", async () => {
    renderPanel(workspaceAs("admin"), adminID);
    expect(await screen.findByText("member@example.com")).toBeInTheDocument();

    // Only the member row is adjustable; owner and the admin's own row are not.
    expect(screen.getAllByRole("combobox")).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "Remove" })).toHaveLength(1);
    // An admin may still leave.
    expect(screen.getByRole("button", { name: "Leave" })).toBeInTheDocument();
  });

  // A viewer can do nothing but leave, and must not be offered a role control.
  it("offers a viewer only the option to leave", async () => {
    const viewerID = "0198b7ae-5e93-72d7-a256-2a0f6622c7ef";
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse({
          members: [
            ...members,
            {
              user_id: viewerID, email: "viewer@example.com", display_name: "Viewer",
              role: "viewer", joined_at: "2026-08-04T00:00:00Z",
            },
          ],
        }),
      ),
    );
    render(
      <QueryClientProvider client={newClient()}>
        <MembersPanel workspace={workspaceAs("viewer")} currentUserId={viewerID} />
      </QueryClientProvider>,
    );
    await screen.findByText("viewer@example.com");

    expect(screen.queryAllByRole("combobox")).toHaveLength(0);
    expect(screen.queryAllByRole("button", { name: "Remove" })).toHaveLength(0);
    expect(screen.getByRole("button", { name: "Leave" })).toBeInTheDocument();
  });

  it("surfaces a rejected change instead of failing silently", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) =>
        init?.method === "PATCH"
          ? apiError(403, "You do not have access to this workspace operation.")
          : jsonResponse({ members }),
      ),
    );
    render(
      <QueryClientProvider client={newClient()}>
        <MembersPanel workspace={workspaceAs("owner")} currentUserId={ownerID} />
      </QueryClientProvider>,
    );
    const select = (await screen.findAllByRole("combobox"))[2];
    fireEvent.change(select, { target: { value: "viewer" } });

    await waitFor(() =>
      expect(screen.getByText("You do not have access to this workspace operation."))
        .toBeInTheDocument(),
    );
  });

  it("requires explicit confirmation before removing a member", async () => {
    renderPanel(workspaceAs("owner"), ownerID);
    const member = (await screen.findByText("member@example.com")).closest("article");
    fireEvent.click(within(member!).getByRole("button", { name: "Remove" }));

    const dialog = screen.getByRole("dialog", { name: "Remove Member?" });
    expect(dialog).toHaveTextContent("immediately lose access");
    expect(within(dialog).getByRole("button", { name: "Remove member" })).toBeInTheDocument();
  });
});

function newClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function renderPanel(workspace: Workspace, currentUserId: string) {
  vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ members })));
  render(
    <QueryClientProvider client={newClient()}>
      <MembersPanel workspace={workspace} currentUserId={currentUserId} />
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
  return new Response(
    JSON.stringify({ error: { code: "forbidden", message, request_id: "test" } }),
    { status, headers: { "Content-Type": "application/json" } },
  );
}
