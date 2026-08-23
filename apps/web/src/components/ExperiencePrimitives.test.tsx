import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, expect, test } from "vitest";

import { AppStatus, BrandMark } from "./ExperiencePrimitives";

afterEach(() => cleanup());

test("presents loading feedback as a polite status", () => {
  render(
    <AppStatus
      description="Preparing your financial workspace."
      eyebrow="Secure session"
      title="Opening Budget"
    />,
  );

  expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
  expect(screen.getByRole("heading", { name: "Opening Budget" })).toBeInTheDocument();
});

test("presents connection failures as actionable alerts", () => {
  render(
    <AppStatus
      action={<button type="button">Try again</button>}
      description="The server could not be reached."
      eyebrow="Connection problem"
      title="Budget is unavailable"
      tone="error"
    />,
  );

  expect(screen.getByRole("alert")).toHaveTextContent("The server could not be reached.");
  expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
});

test("keeps the decorative brand mark out of the accessibility tree", () => {
  const { container } = render(<BrandMark withName />);

  expect(within(container).getByText("Budget")).toBeInTheDocument();
  expect(container.querySelector(".brand-mark")).toHaveAttribute("aria-hidden", "true");
});
