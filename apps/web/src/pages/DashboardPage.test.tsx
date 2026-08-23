import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, expect, test } from "vitest";

import type { SessionResponse } from "../api/client";
import { expectNoAccessibilityViolations } from "../test/accessibility";
import { DashboardPage } from "./DashboardPage";

const personalId = "0198b7ae-5e93-72d8-99af-ff40c48ad342";
const familyId = "0198b7ae-5e93-72d8-99af-ff40c48ad343";
const session: SessionResponse = {
  user: {
    id: "0198b7ae-5e93-72d7-a256-2a0f6622c7ec",
    email: "owner@example.com",
    display_name: "Budget Owner",
  },
  workspaces: [
    {
      id: personalId,
      name: "Personal",
      base_currency: "TRY",
      timezone: "Europe/Istanbul",
      role: "owner",
    },
    {
      id: familyId,
      name: "Family",
      base_currency: "EUR",
      timezone: "Europe/Istanbul",
      role: "member",
    },
  ],
};

afterEach(() => cleanup());

test("exposes an accessible adaptive shell and preserves the destination when switching workspaces", async () => {
  const { container } = render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[`/workspaces/${personalId}/more`]}>
        <Routes>
          <Route path="/workspaces/:workspaceId/*" element={<DashboardPage session={session} />} />
        </Routes>
        <LocationProbe />
      </MemoryRouter>
    </QueryClientProvider>,
  );

  expect(screen.getByRole("complementary", { name: "Workspace navigation" })).toBeInTheDocument();
  const compactNavigation = screen.getByRole("navigation", { name: "Primary navigation" });
  expect(within(compactNavigation).getAllByRole("link")).toHaveLength(5);
  expect(within(compactNavigation).getByRole("link", { name: "More" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  expect(screen.getByRole("main")).toHaveAttribute("id", "workspace-content");
  const moreDestinations = screen.getByRole("navigation", { name: "More workspace destinations" });
  expect(within(moreDestinations).getByRole("link", { name: /Reports/ })).toHaveAttribute(
    "href",
    `/workspaces/${personalId}/reports`,
  );
  const workspaceSwitcher = screen.getByRole("navigation", { name: "Available workspaces" });
  expect(within(workspaceSwitcher).getByRole("link", { name: /Personal/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
  expect(screen.getByRole("heading", { name: "Budget Owner", level: 2 })).toBeInTheDocument();
  expect(screen.getAllByText("owner@example.com")).toHaveLength(2);
  expect(screen.getByLabelText("Invitation code")).toBeInTheDocument();
  await expectNoAccessibilityViolations(container);

  fireEvent.change(screen.getAllByLabelText("Current workspace")[0], {
    target: { value: familyId },
  });

  expect(await screen.findByTestId("current-location")).toHaveTextContent(
    `/workspaces/${familyId}/more`,
  );
  expect(screen.getByRole("heading", { name: "Family", level: 2 })).toBeInTheDocument();
});

test("marks the compact More destination as current for its grouped pages", () => {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[`/workspaces/${personalId}/people`]}>
        <Routes>
          <Route path="/workspaces/:workspaceId/*" element={<DashboardPage session={session} />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );

  const compactNavigation = screen.getByRole("navigation", { name: "Primary navigation" });
  expect(within(compactNavigation).getByRole("link", { name: "More" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});

function LocationProbe() {
  return <output data-testid="current-location">{useLocation().pathname}</output>;
}
