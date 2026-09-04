/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

/*
 * Analysis reads the ledger but owns no mutation of its own, so it can only be refreshed by
 * whichever feature changed the data underneath it. That makes it exactly the cache a new
 * feature forgets: everything still compiles, every test still passes, and the screen quietly
 * shows figures from before the edit.
 *
 * It shipped that way — `spendingAnalysisQueryPrefix` was exported and never called. This
 * asserts the relationship the panels have to honour, rather than any particular panel.
 */

const panels = import.meta.glob<string>(
  [
    "../transactions/TransactionsPanel.tsx",
    "../accounts/AccountsPanel.tsx",
    "../categories/CategoriesPanel.tsx",
  ],
  { query: "?raw", import: "default", eager: true },
);

describe("analysis cache invalidation", () => {
  it("covers every panel that mutates ledger data", () => {
    // A glob that silently matched nothing would make the assertion below vacuous.
    expect(Object.keys(panels)).toHaveLength(3);

    const missing = Object.entries(panels)
      .filter(([, source]) => !source.includes("spendingAnalysisQueryPrefix"))
      .map(([path]) => path);
    expect(
      missing,
      `these panels change data the analysis reads but never invalidate it:\n${missing.join("\n")}`,
    ).toEqual([]);
  });

  it("invalidates analysis wherever it already invalidates the projection", () => {
    // The projection and the analysis derive from the same ledger, so any edit that makes one
    // stale makes the other stale. Counting them together catches a panel that gains a second
    // mutation later and updates only the older cache.
    const uneven = Object.entries(panels)
      .map(([path, source]) => ({
        path,
        projection: source.match(/financialProjectionQueryPrefix\(/g)?.length ?? 0,
        analysis: source.match(/spendingAnalysisQueryPrefix\(/g)?.length ?? 0,
      }))
      // The import line counts once for each symbol, so both sides carry the same offset.
      .filter((entry) => entry.projection !== entry.analysis);
    expect(
      uneven,
      `projection and analysis invalidation have drifted apart:\n${JSON.stringify(uneven, null, 2)}`,
    ).toEqual([]);
  });
});
