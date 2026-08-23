import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AcceptInvitationPanel } from "./AcceptInvitationPanel";

const acceptanceToken = "P4tGZ3sYy1s0Rn7cQvT2XkLb8mHfJ9aWuE6dN5rVzQo";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("AcceptInvitationPanel", () => {
  it("submits the credential only in the request body and reports the joined workspace", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => jsonResponse({
      workspace: {
        id: "workspace-2",
        name: "Family",
        base_currency: "TRY",
        timezone: "Europe/Istanbul",
        role: "member",
      },
      member: {
        user_id: "user-1",
        email: "member@example.com",
        display_name: "Member",
        role: "member",
        joined_at: "2026-08-23T08:00:00Z",
      },
    }));
    vi.stubGlobal("fetch", fetchMock);
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <AcceptInvitationPanel />
      </QueryClientProvider>,
    );

    fireEvent.change(screen.getByLabelText("Invitation code"), { target: { value: acceptanceToken } });
    fireEvent.click(screen.getByRole("button", { name: "Join workspace" }));

    expect(await screen.findByText(/You joined/)).toHaveTextContent("Family");
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe("/v1/invitations/accept");
    expect(String(path)).not.toContain(acceptanceToken);
    expect(JSON.parse(String(init?.body))).toEqual({ token: acceptanceToken });
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
