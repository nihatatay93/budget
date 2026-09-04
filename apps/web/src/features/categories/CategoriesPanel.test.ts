import { describe, expect, it } from "vitest";

import type { components } from "../../api/generated/schema";
import { descendantIds } from "./CategoriesPanel";

type Category = components["schemas"]["Category"];

function category(id: string, parentId?: string): Category {
  return {
    id,
    workspace_id: "0198b7ae-5e93-72d8-99af-ff40c48ad342",
    name: id,
    kind: "expense",
    icon_type: "system",
    icon_value: "ellipsis",
    color_key: "slate",
    ...(parentId ? { parent_id: parentId } : {}),
  };
}

describe("descendantIds", () => {
  it("excludes the edited category and descendants at every depth", () => {
    const categories = [
      category("grandchild", "child"),
      category("unrelated"),
      category("child", "parent"),
      category("parent"),
    ];

    expect(descendantIds(categories, "parent")).toEqual(
      new Set(["parent", "child", "grandchild"]),
    );
  });

  it("excludes nothing while creating a category", () => {
    expect(descendantIds([category("existing")], undefined)).toEqual(new Set());
  });
});
