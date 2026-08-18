import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, it, vi } from "vitest";

import type { SessionResponse } from "../../api/client";
import { expectNoAccessibilityViolations } from "../../test/accessibility";
import { AcceptInvitationPanel } from "./AcceptInvitationPanel";
import { InvitationsPanel } from "./InvitationsPanel";
import { MembersPanel } from "./MembersPanel";

type Workspace = SessionResponse["workspaces"][number];

const workspaceID = "0198b7ae-5e93-72d8-99af-ff40c48ad342";
const ownerID = "0198b7ae-5e93-72d7-a256-2a0f6622c7ec";

const workspace: Workspace = {
  id: workspaceID, name: "Atay Family", base_currency: "TRY",
  timezone: "Europe/Istanbul", role: "owner",
};

const members = [{
  user_id: ownerID, email: "owner@example.com", display_name: "Owner",
  role: "owner", joined_at: "2026-08-01T00:00:00Z",
}];

const invitations = [{
  id: "0198b7ae-5e93-72d9-ab00-32b0861a3f37",
  workspace_id: workspaceID,
  email: "invited@example.com",
  role: "member",
  invited_by: ownerID,
  inviter_display_name: "Owner",
  expires_at: "2026-09-01T00:00:00Z",
  created_at: "2026-08-18T00:00:00Z",
}];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

/*
 * These screens are the ones a shared workspace depends on, so their controls have to be
 * reachable and named. The per-member role control in particular is a repeated widget whose
 * label is visually hidden; without a check, losing that label is invisible in review.
 */
describe("collaboration accessibility", () => {
  it("renders the members panel without violations", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ members })));
    const { container } = renderWithClient(
      <MembersPanel workspace={workspace} currentUserId={ownerID} />,
    );
    await screen.findByText("owner@example.com");
    await expectNoAccessibilityViolations(container);
  });

  it("renders the invitations panel without violations", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ invitations })));
    const { container } = renderWithClient(<InvitationsPanel workspace={workspace} />);
    await screen.findByText("invited@example.com");
    await expectNoAccessibilityViolations(container);
  });

  it("renders the accept panel without violations", async () => {
    const { container } = renderWithClient(<AcceptInvitationPanel />);
    await screen.findByLabelText("Invitation code");
    await expectNoAccessibilityViolations(container);
  });
});

function renderWithClient(element: React.ReactElement) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      {element}
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
