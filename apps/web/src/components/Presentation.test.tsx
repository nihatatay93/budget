import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, expect, test, vi } from "vitest";

import { expectNoAccessibilityViolations } from "../test/accessibility";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  ModalDialog,
  ProgressMeter,
  StatusBadge,
  ToastRegion,
} from "./Presentation";

afterEach(() => cleanup());

test("presents semantic resource and progress feedback", () => {
  render(
    <>
      <LoadingState label="Loading accounts" rows={2} />
      <EmptyState compact icon="accounts" title="No accounts yet" />
      <InlineNotice tone="danger">The change could not be saved.</InlineNotice>
      <StatusBadge tone="positive">Posted</StatusBadge>
      <ProgressMeter label="Monthly plan usage" value={72} />
    </>,
  );

  expect(screen.getByRole("status", { name: "Loading accounts" })).toBeInTheDocument();
  expect(screen.getByText("No accounts yet")).toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent("could not be saved");
  expect(screen.getByText("Posted")).toBeInTheDocument();
  expect(screen.getByRole("progressbar", { name: "Monthly plan usage" })).toHaveValue(72);
});

test("closes modal dialogs with Escape and restores focus", () => {
  const close = vi.fn();
  render(<DialogHarness onClose={close} />);
  const trigger = screen.getByRole("button", { name: "Open editor" });
  trigger.focus();
  fireEvent.click(trigger);
  const dialog = screen.getByRole("dialog", { name: "Edit account" });
  fireEvent.keyDown(dialog, { key: "Escape" });
  expect(close).toHaveBeenCalledOnce();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();
});

test("traps focus inside dialogs and requires an explicit action for protected content", () => {
  const close = vi.fn();
  render(
    <ModalDialog dismissible={false} onClose={close} open title="Save invitation code">
      <button type="button">Copy code</button>
      <button type="button">I saved the code</button>
    </ModalDialog>,
  );

  const dialog = screen.getByRole("dialog", { name: "Save invitation code" });
  const firstAction = screen.getByRole("button", { name: "Copy code" });
  const lastAction = screen.getByRole("button", { name: "I saved the code" });

  expect(dialog).toHaveFocus();
  lastAction.focus();
  fireEvent.keyDown(dialog, { key: "Tab" });
  expect(firstAction).toHaveFocus();
  fireEvent.keyDown(dialog, { key: "Tab", shiftKey: true });
  expect(lastAction).toHaveFocus();

  fireEvent.keyDown(dialog, { key: "Escape" });
  expect(close).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  expect(dialog).toBeInTheDocument();
});

function DialogHarness({ onClose }: { onClose: () => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)} type="button">Open editor</button>
      <ModalDialog
        onClose={() => {
          onClose();
          setOpen(false);
        }}
        open={open}
        title="Edit account"
      >
        <input aria-label="Account name" />
      </ModalDialog>
    </>
  );
}

test("announces and dismisses toast messages", () => {
  const dismiss = vi.fn();
  render(
    <ToastRegion
      messages={[{ id: "saved", title: "Account saved", tone: "positive" }]}
      onDismiss={dismiss}
    />,
  );

  expect(screen.getByRole("region", { name: "Notifications" })).toHaveAttribute("aria-live", "polite");
  fireEvent.click(screen.getByRole("button", { name: "Dismiss Account saved" }));
  expect(dismiss).toHaveBeenCalledWith("saved");
});

test("renders the modal and feedback primitives without automated accessibility violations", async () => {
  const { container } = render(
    <ModalDialog onClose={() => undefined} open title="Create account">
      <label>Account name<input /></label>
    </ModalDialog>,
  );

  await expectNoAccessibilityViolations(container);
});
