import { describe, expect, it } from "vitest";

import { frequentCategoryIds } from "./CategoryTiles";

describe("frequentCategoryIds", () => {
  it("ranks by how often a category is allocated to", () => {
    const allocations = [
      [{ category_id: "food" }],
      [{ category_id: "food" }, { category_id: "rent" }],
      [{ category_id: "rent" }],
      [{ category_id: "food" }],
      [{ category_id: "taxi" }],
    ];

    expect(frequentCategoryIds(allocations)).toEqual(["food", "rent", "taxi"]);
  });

  it("breaks a tie predictably and honours the limit", () => {
    const allocations = [[{ category_id: "b" }], [{ category_id: "a" }], [{ category_id: "c" }]];

    expect(frequentCategoryIds(allocations, 2)).toEqual(["a", "b"]);
    expect(frequentCategoryIds([])).toEqual([]);
  });
});
