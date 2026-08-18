import axe from "axe-core";
import { expect } from "vitest";

/**
 * Runs axe against a rendered container and fails with the specific violations.
 *
 * Colour-contrast rules are disabled: jsdom does not lay out or paint, so axe cannot measure
 * contrast and would report false results. Contrast stays a manual review concern.
 */
export async function expectNoAccessibilityViolations(container: HTMLElement): Promise<void> {
  const results = await axe.run(container, {
    rules: { "color-contrast": { enabled: false } },
    resultTypes: ["violations"],
  });

  const violations = results.violations.map((violation) => {
    const targets = violation.nodes.map((node) => node.target.join(" ")).join(", ");
    return `${violation.id} (${violation.impact}): ${violation.help} [${targets}]`;
  });

  expect(violations, violations.join("\n")).toEqual([]);
}
