import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ErrorBoundary } from "./ErrorBoundary";
import { expectNoAccessibilityViolations } from "../test/accessibility";

function Explode(): never {
  throw new Error("render exploded");
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ErrorBoundary", () => {
  it("renders its children while nothing has thrown", () => {
    render(<ErrorBoundary><p>Workspace</p></ErrorBoundary>);

    expect(screen.getByText("Workspace")).toBeInTheDocument();
  });

  it("replaces a blank page with a legible failure and a way out", async () => {
    // React logs the caught error itself; silencing it keeps the suite output readable.
    vi.spyOn(console, "error").mockImplementation(() => {});
    const { container } = render(<ErrorBoundary><Explode /></ErrorBoundary>);

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Budget could not continue")).toBeInTheDocument();
    // The reassurance matters: a thrown render says nothing about the ledger.
    expect(screen.getByText(/financial data is unaffected/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reload Budget" })).toBeInTheDocument();
    await expectNoAccessibilityViolations(container);
  });

  it("reports the failure to the console so a self-hosted deployment can diagnose it", () => {
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});

    render(<ErrorBoundary><Explode /></ErrorBoundary>);

    expect(
      logged.mock.calls.some(([message]) => message === "Unhandled interface error"),
    ).toBe(true);
  });
});
