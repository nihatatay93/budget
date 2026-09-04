import type { Category } from "../api/client";

/**
 * Category hierarchy shared by everything that presents categories: the Categories destination,
 * the capture form's picker, and the transaction draft that has to name a branch by its root.
 *
 * The seeded defaults arrive in groups — a group is an ordinary category its members hang under
 * — but nothing here assumes that. A workspace that flattens or rearranges its own categories
 * gets sections describing whatever it actually built.
 */
export type CategorySection = {
  root: Category;
  /** The root itself at depth 0, then its descendants, depth-first. */
  members: { category: Category; depth: number }[];
};

/** The top of a category's branch, which is what a picker offers first. */
export function rootCategory(categories: Category[], categoryId: string): Category | undefined {
  const byId = new Map(categories.map((category) => [category.id, category]));
  let current = byId.get(categoryId);
  // Ancestry is server-validated as acyclic; the visit set keeps a corrupt response from hanging.
  const visited = new Set<string>();
  while (current?.parent_id && !visited.has(current.id)) {
    visited.add(current.id);
    const parent = byId.get(current.parent_id);
    if (!parent) break;
    current = parent;
  }
  return current;
}

/**
 * Every category under a root, depth-first, so a branch reads as a branch. Depth accompanies
 * each row because the domain model allows deeper nesting than the two levels a picker shows
 * as "category" and "subcategory".
 */
export function categoryBranch(
  categories: Category[],
  rootId: string,
): { category: Category; depth: number }[] {
  const rows: { category: Category; depth: number }[] = [];
  const visited = new Set<string>();
  function append(parentId: string, depth: number) {
    for (const category of categories.filter((candidate) => candidate.parent_id === parentId)) {
      if (visited.has(category.id)) continue;
      visited.add(category.id);
      rows.push({ category, depth });
      append(category.id, depth + 1);
    }
  }
  append(rootId, 0);
  return rows;
}

/**
 * One section per root, each holding the root and its descendants. The root is a member of its
 * own section because a group is spendable: a workspace can allocate to Entertainment itself
 * rather than to one of the things inside it.
 */
export function categorySections(
  categories: Category[],
  compare: (left: Category, right: Category) => number,
): CategorySection[] {
  const present = new Set(categories.map((category) => category.id));
  // A category whose parent was filtered out of this set reads as a root here, so an archived
  // or wrong-kind parent never hides its children.
  const roots = categories
    .filter((category) => !category.parent_id || !present.has(category.parent_id))
    .sort(compare);
  return roots.map((root) => ({
    root,
    members: [
      { category: root, depth: 0 },
      ...categoryBranch(categories, root.id).map((row) => ({ ...row, depth: row.depth + 1 })),
    ],
  }));
}
