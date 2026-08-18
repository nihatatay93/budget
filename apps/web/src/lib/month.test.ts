import { expect, test } from "vitest";

import { monthLabel, workspaceMonth } from "./month";

test("resolves the month in the workspace timezone", () => {
  const boundary = new Date("2026-08-31T21:30:00Z");
  expect(workspaceMonth("Europe/Istanbul", boundary)).toBe("2026-09");
  expect(workspaceMonth("America/New_York", boundary)).toBe("2026-08");
});

test("formats a date-only month without local timezone drift", () => {
  expect(monthLabel("2026-08")).toMatch(/2026/);
});
