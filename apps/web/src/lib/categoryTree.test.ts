import { describe, expect, it } from "vitest";

import type { Category } from "../api/client";
import { categoryBranch, categorySections, rootCategory } from "./categoryTree";

const byName = (left: Category, right: Category) => left.name.localeCompare(right.name);

const categories: Category[] = [
  category("food"),
  category("groceries", "food"),
  category("markets", "groceries"),
  category("transport"),
];

describe("categorySections", () => {
  it("gives each root a section that opens with the root itself", () => {
    const sections = categorySections(categories, byName);

    expect(sections.map((section) => section.root.id)).toEqual(["food", "transport"]);
    expect(sections[0].members).toEqual([
      { category: categories[0], depth: 0 },
      { category: categories[1], depth: 1 },
      { category: categories[2], depth: 2 },
    ]);
  });

  it("treats a category whose parent was filtered out as a root of its own", () => {
    // An expense-only or unarchived-only view must not hide children behind a missing parent.
    const withoutFood = categories.filter((value) => value.id !== "food");

    expect(categorySections(withoutFood, byName).map((section) => section.root.id))
      .toEqual(["groceries", "transport"]);
  });

  it("finds the root of a branch and lists that branch depth-first", () => {
    expect(rootCategory(categories, "markets")?.id).toBe("food");
    expect(rootCategory(categories, "food")?.id).toBe("food");
    expect(rootCategory(categories, "missing")).toBeUndefined();
    expect(categoryBranch(categories, "food")).toEqual([
      { category: categories[1], depth: 0 },
      { category: categories[2], depth: 1 },
    ]);
  });
});

function category(id: string, parentId?: string): Category {
  return {
    id,
    workspace_id: "workspace",
    name: id,
    kind: "expense",
    ...(parentId ? { parent_id: parentId } : {}),
  };
}
