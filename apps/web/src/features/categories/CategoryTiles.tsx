import { useCallback, useMemo, useState } from "react";

import type { Category } from "../../api/client";
import { CategoryAppearance } from "../../components/CategoryAppearance";
import { AppIcon } from "../../components/ExperiencePrimitives";
import { type CategorySection, categorySections } from "../../lib/categoryTree";
import { categoryName, t } from "../../lib/i18n";

/**
 * Categories as coloured sections of tiles rather than one long list. A section is a root
 * category and everything under it, and the root appears as the first tile in its own section
 * because a group is spendable in its own right.
 *
 * Which sections a person collapses is a per-device convenience, so it lives in localStorage
 * rather than on the workspace: hiding a section here is a shelf, not archiving, and must not
 * change what anyone else sees or what reports contain.
 */
const HIDDEN_SECTIONS_KEY = "budget.categorySections.hidden";

function readHiddenSections(workspaceId: string): Set<string> {
  try {
    const stored = globalThis.localStorage?.getItem(`${HIDDEN_SECTIONS_KEY}.${workspaceId}`);
    return new Set(stored ? (JSON.parse(stored) as string[]) : []);
  } catch {
    // A private window, cleared site data, or a browser refusing storage: show everything.
    return new Set();
  }
}

export function useHiddenSections(workspaceId: string) {
  const [hidden, setHidden] = useState(() => readHiddenSections(workspaceId));
  const toggle = useCallback((rootId: string) => {
    setHidden((current) => {
      const next = new Set(current);
      if (!next.delete(rootId)) next.add(rootId);
      try {
        globalThis.localStorage?.setItem(
          `${HIDDEN_SECTIONS_KEY}.${workspaceId}`,
          JSON.stringify([...next]),
        );
      } catch {
        // Storage is a convenience here; the toggle still works for this visit.
      }
      return next;
    });
  }, [workspaceId]);
  return { hidden, toggle };
}

/**
 * The categories used most often, as their own section at the top. Derived from recent activity
 * rather than stored, so it follows how a workspace actually spends without anything to maintain.
 */
export function frequentCategoryIds(
  allocations: { category_id: string }[][],
  limit = 8,
): string[] {
  const counts = new Map<string, number>();
  for (const transaction of allocations) {
    for (const allocation of transaction) {
      counts.set(allocation.category_id, (counts.get(allocation.category_id) ?? 0) + 1);
    }
  }
  return [...counts.entries()]
    .sort(([leftId, left], [rightId, right]) => right - left || leftId.localeCompare(rightId))
    .slice(0, limit)
    .map(([id]) => id);
}

export function CategoryTileSections({
  categories,
  frequent = [],
  onSelect,
  search,
  selectedId,
  workspaceId,
}: {
  categories: Category[];
  /** Category ids to offer first, most used first. */
  frequent?: string[];
  onSelect?: (category: Category) => void;
  search?: string;
  selectedId?: string;
  workspaceId: string;
}) {
  const { hidden, toggle } = useHiddenSections(workspaceId);
  const term = (search ?? "").trim().toLocaleLowerCase();
  const byId = useMemo(() => new Map(categories.map((category) => [category.id, category])), [categories]);

  const sections = useMemo(
    () => categorySections(categories, (left, right) => categoryName(left).localeCompare(categoryName(right))),
    [categories],
  );
  const frequentSection = useMemo(
    () => frequent.map((id) => byId.get(id)).filter((category) => category !== undefined),
    [byId, frequent],
  );

  const matches = (category: Category) =>
    !term || categoryName(category).toLocaleLowerCase().includes(term);

  // Searching is a flat question — "where is the taxi one" — so it looks through every section
  // at once and ignores which of them are collapsed.
  const visibleSections = sections
    .map((section) => ({
      ...section,
      members: term ? section.members.filter(({ category }) => matches(category)) : section.members,
    }))
    .filter((section) => section.members.length > 0);

  return (
    <div className="category-sections">
      {!term && frequentSection.length > 0 ? (
        <CategoryTileSection
          heading={t("Most used")}
          hidden={hidden.has("frequent")}
          members={frequentSection.map((category) => ({ category, depth: 0 }))}
          onSelect={onSelect}
          onToggle={() => toggle("frequent")}
          selectedId={selectedId}
        />
      ) : null}
      {visibleSections.map((section) => (
        <CategoryTileSection
          heading={categoryName(section.root)}
          hidden={!term && hidden.has(section.root.id)}
          key={section.root.id}
          members={section.members}
          onSelect={onSelect}
          onToggle={() => toggle(section.root.id)}
          selectedId={selectedId}
        />
      ))}
      {visibleSections.length === 0 ? <p className="category-sections-empty">{t("No matching categories")}</p> : null}
    </div>
  );
}

function CategoryTileSection({
  heading,
  hidden,
  members,
  onSelect,
  onToggle,
  selectedId,
}: {
  heading: string;
  hidden: boolean;
  members: { category: Category; depth: number }[];
  onSelect?: (category: Category) => void;
  onToggle: () => void;
  selectedId?: string;
}) {
  return (
    <section className="category-section">
      <div className="category-section-heading">
        <h3>{heading}</h3>
        <button
          aria-expanded={!hidden}
          aria-label={hidden ? t("Show {section}", { section: heading }) : t("Hide {section}", { section: heading })}
          className="category-section-toggle"
          onClick={onToggle}
          type="button"
        >
          <AppIcon name={hidden ? "eye-off" : "eye"} size={18} />
        </button>
      </div>
      {hidden ? null : (
        <div className="category-tiles">
          {members.map(({ category, depth }) => (
            <CategoryTile
              category={category}
              depth={depth}
              key={category.id}
              onSelect={onSelect}
              selected={category.id === selectedId}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function CategoryTile({
  category,
  depth,
  onSelect,
  selected,
}: {
  category: Category;
  depth: number;
  onSelect?: (category: Category) => void;
  selected: boolean;
}) {
  const name = categoryName(category);
  const content = (
    <>
      <CategoryAppearance
        colorKey={category.color_key}
        iconType={category.icon_type}
        iconValue={category.icon_value ?? category.icon}
        size={22}
      />
      <span>{name}</span>
    </>
  );
  if (!onSelect) {
    return <div className={tileClass(depth, selected)}>{content}</div>;
  }
  return (
    <button
      aria-pressed={selected}
      className={tileClass(depth, selected)}
      onClick={() => onSelect(category)}
      type="button"
    >
      {content}
    </button>
  );
}

function tileClass(depth: number, selected: boolean) {
  // Depth is worth one visual step and no more: a picker that indents four levels stops being
  // scannable, which is the whole point of tiles.
  return [
    "category-tile",
    depth > 0 ? "category-tile-nested" : "",
    selected ? "category-tile-selected" : "",
  ].filter(Boolean).join(" ");
}
