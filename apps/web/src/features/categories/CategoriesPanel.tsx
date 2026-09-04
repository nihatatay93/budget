import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  type Category,
  type CategoryWriteRequest,
  type SessionResponse,
  archiveCategory,
  categoriesQueryKey,
  createCategory,
  financialProjectionQueryPrefix,
  spendingAnalysisQueryPrefix,
  listCategories,
  monthlyBudgetQueryPrefix,
  updateCategory,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";
import { AppIcon } from "../../components/ExperiencePrimitives";
import { CategoryTileSections } from "./CategoryTiles";
import {
  CategoryAppearance,
  CategoryLabel,
  categoryColorKeys,
  categoryColorStyle,
  categoryIconLabel,
  categorySystemIcons,
  isSingleGrapheme,
  type CategoryColorKey,
  type CategoryIconType,
} from "../../components/CategoryAppearance";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  ModalDialog,
  StatusBadge,
  ToastRegion,
  type ToastMessage,
} from "../../components/Presentation";
import { categoryName, t } from "../../lib/i18n";

type Workspace = SessionResponse["workspaces"][number];

export function CategoriesPanel({ workspace, canManage }: { workspace: Workspace; canManage: boolean }) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: categoriesQueryKey(workspace.id),
    queryFn: () => listCategories(workspace.id),
  });
  const [editing, setEditing] = useState<Category>();
  const [name, setName] = useState("");
  const [kind, setKind] = useState<CategoryWriteRequest["kind"]>("expense");
  const [parentId, setParentId] = useState("");
  const [iconType, setIconType] = useState<CategoryIconType>("system");
  const [iconValue, setIconValue] = useState("ellipsis");
  const [colorKey, setColorKey] = useState<CategoryColorKey>("slate");
  const [iconSearch, setIconSearch] = useState("");
  const [editorOpen, setEditorOpen] = useState(false);
  const [archiving, setArchiving] = useState<Category>();
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const save = useMutation({
    mutationFn: (input: CategoryWriteRequest) =>
      editing
        ? updateCategory(workspace.id, editing.id, input)
        : createCategory(workspace.id, input),
    onSuccess: async (category) => {
      setToasts([{
        id: `category-${category.id}-saved`,
        title: editing ? t("Category updated") : t("Category created"),
        tone: "positive",
      }]);
      reset();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: categoriesQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: spendingAnalysisQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
      ]);
    },
  });
  const archive = useMutation({
    mutationFn: (categoryId: string) => archiveCategory(workspace.id, categoryId),
    onSuccess: async () => {
      setToasts([{
        id: `category-archive-${archiving?.id ?? "complete"}`,
        title: t("Category archived"),
        description: t("Historical allocations remain available to reports."),
        tone: "positive",
      }]);
      setArchiving(undefined);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: categoriesQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: spendingAnalysisQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
      ]);
    },
  });

  function reset() {
    setEditing(undefined);
    setName("");
    setKind("expense");
    setParentId("");
    setIconType("system");
    setIconValue("ellipsis");
    setColorKey("slate");
    setIconSearch("");
    setEditorOpen(false);
    save.reset();
  }

  function edit(category: Category) {
    setEditing(category);
    setName(category.name);
    setKind(category.kind);
    setParentId(category.parent_id ?? "");
    setIconType(category.icon_type ?? (category.icon ? "emoji" : "system"));
    setIconValue(category.icon_value ?? category.icon ?? "ellipsis");
    setColorKey((category.color_key ?? "slate") as CategoryColorKey);
    setEditorOpen(true);
  }

  function create() {
    reset();
    setEditorOpen(true);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    if (iconType === "emoji" && !isSingleGrapheme(iconValue)) {
      return;
    }
    save.mutate({
      name,
      kind,
      ...(parentId ? { parent_id: parentId } : {}),
      icon_type: iconType,
      icon_value: iconValue.trim(),
      color_key: colorKey,
    });
  }

  function confirmArchive(category: Category) {
    archive.reset();
    setArchiving(category);
  }

  // A category cannot be parented under itself or any of its own descendants.
  const excluded = descendantIds(query.data ?? [], editing?.id);
  const parents = query.data?.filter(
    (category) => category.kind === kind && !excluded.has(category.id) && !category.archived_at,
  );
  const categoryRows = categoryTree(query.data ?? []);
  const activeRows = categoryRows.filter(({ category }) => !category.archived_at);
  const archivedRows = categoryRows.filter(({ category }) => category.archived_at);

  return (
    <section className="categories-workspace" aria-labelledby="categories-heading">
      <div className="resource-destination-heading">
        <div>
          <p className="eyebrow">{t("How activity is reported")}</p>
          <h2 id="categories-heading">{t("Categories")}</h2>
          <p>{t("Expense and income categories organize allocations without constraining their sign.")}</p>
        </div>
        {canManage ? <button onClick={create} type="button">{t("Add category")}</button> : null}
      </div>
      {query.isPending ? <LoadingState label={t("Loading categories")} rows={5} /> : null}
      {query.isError ? (
        <InlineNotice
          action={<button className="secondary-button" onClick={() => void query.refetch()} type="button">{t("Try again")}</button>}
          title={t("Categories could not be loaded")}
          tone="danger"
        >
          <p>{query.error.message}</p>
        </InlineNotice>
      ) : null}
      {!query.isPending && !query.isError && activeRows.length === 0 ? (
        <EmptyState
          action={canManage ? <button onClick={create} type="button">{t("Create category")}</button> : undefined}
          description={t("Protected Uncategorized categories are normally created with the workspace.")}
          icon="categories"
          title={t("No active categories")}
        />
      ) : null}
      {activeRows.length > 0 ? (
        <div className="category-kind-grid">
          <CategoryGroup canManage={canManage} kind="expense" onArchive={confirmArchive} onEdit={edit} rows={activeRows} workspaceId={workspace.id} />
          <CategoryGroup canManage={canManage} kind="income" onArchive={confirmArchive} onEdit={edit} rows={activeRows} workspaceId={workspace.id} />
        </div>
      ) : null}
      {archivedRows.length > 0 ? (
        <details className="archived-resource-group">
          <summary>{t(
            archivedRows.length === 1 ? "{count} archived category" : "{count} archived categories",
            { count: archivedRows.length },
          )}</summary>
          <div className="category-archive-list">
            {archivedRows.map(({ category, depth }) => (
              <CategoryRow canManage={false} category={category} depth={depth} key={category.id} onArchive={confirmArchive} onEdit={edit} />
            ))}
          </div>
        </details>
      ) : null}
      {!canManage && !query.isPending ? (
        <InlineNotice title={t("Read-only categories")}><p>{t("Viewer access can review the reporting hierarchy but cannot change it.")}</p></InlineNotice>
      ) : null}
      {canManage ? (
        <ModalDialog
          description={t("Categories group allocations for reports and monthly plans. Parent and child categories must share a kind.")}
          footer={(
            <>
              <button className="secondary-button" onClick={reset} type="button">{t("Cancel")}</button>
              <button disabled={save.isPending} form="category-editor" type="submit">
                {save.isPending ? t("Saving…") : editing ? t("Save category") : t("Add category")}
              </button>
            </>
          )}
          onClose={reset}
          open={editorOpen}
          placement="drawer"
          title={editing ? t("Edit {name}", { name: categoryName(editing) }) : t("Add category")}
        >
          <form className="resource-form resource-editor-form" id="category-editor" onSubmit={submit}>
          <label>
            {t("Name")}
            <input required maxLength={100} value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <div className="form-columns">
            <label>
              {t("Kind")}
              <select
                value={kind}
                onChange={(event) => {
                  setKind(event.target.value as typeof kind);
                  setParentId("");
                }}
              >
                <option value="expense">{t("category.kind.expense")}</option>
                <option value="income">{t("category.kind.income")}</option>
              </select>
            </label>
            <label>
              {t("Parent (optional)")}
              <select value={parentId} onChange={(event) => setParentId(event.target.value)}>
                <option value="">{t("Top level")}</option>
                {parents?.map((category) => <option key={category.id} value={category.id}>{categoryName(category)}</option>)}
              </select>
            </label>
          </div>
          <fieldset className="category-appearance-editor">
            <legend>{t("Category appearance")}</legend>
            <div className="category-preview" style={categoryColorStyle(colorKey)}>
              <CategoryAppearance colorKey={colorKey} iconType={iconType} iconValue={iconValue} label={t("Category preview")} size={20} />
              <strong>{name.trim() || t("Category name preview")}</strong>
            </div>
            <div className="category-appearance-tabs" aria-label={t("Icon")} role="group">
              <button aria-pressed={iconType === "system"} className="secondary-button" onClick={() => { setIconType("system"); if (!categorySystemIcons.includes(iconValue as typeof categorySystemIcons[number])) setIconValue("ellipsis"); }} type="button">{t("System icon")}</button>
              <button aria-pressed={iconType === "emoji"} className="secondary-button" onClick={() => { setIconType("emoji"); if (!isSingleGrapheme(iconValue)) setIconValue("🍀"); }} type="button">{t("Emoji")}</button>
            </div>
            {iconType === "system" ? (
              <div className="category-system-picker">
                <label className="category-icon-search">
                  <span className="visually-hidden">{t("Search icons")}</span>
                  <input onChange={(event) => setIconSearch(event.target.value)} placeholder={t("Search icons")} type="search" value={iconSearch} />
                </label>
                <div className="category-icon-grid" aria-label={t("Choose an icon")}>
                  {categorySystemIcons.filter((key) => categoryIconLabel(key).toLocaleLowerCase().includes(iconSearch.toLocaleLowerCase())).map((key) => (
                    <button aria-label={categoryIconLabel(key)} aria-pressed={iconValue === key} className="category-icon-choice" key={key} onClick={() => setIconValue(key)} style={categoryColorStyle(colorKey)} type="button">
                      <CategoryAppearance colorKey={colorKey} iconType="system" iconValue={key} size={19} />
                      <span className="visually-hidden">{categoryIconLabel(key)}</span>
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              <div className="category-emoji-picker">
                <label>
                  {t("Emoji")}
                  <input aria-describedby="emoji-help" maxLength={64} onChange={(event) => setIconValue(event.target.value)} value={iconValue} />
                </label>
                <span id="emoji-help">{t("Use one emoji, including a combined emoji from your keyboard.")}</span>
                <div className="category-emoji-palette" aria-label={t("Choose an emoji")}>
                  {["🍀", "🍲", "☕", "🛒", "🎁", "💼", "🏠", "✈️", "🎮", "💊", "🐶", "👩🏽‍💻"].map((emoji) => (
                    <button aria-label={t("Use {emoji} as the category icon", { emoji })} aria-pressed={iconValue === emoji} className="category-emoji-choice" key={emoji} onClick={() => setIconValue(emoji)} type="button">{emoji}</button>
                  ))}
                </div>
                {!isSingleGrapheme(iconValue) ? <p className="form-error" role="alert">{t("Choose one emoji only.")}</p> : null}
              </div>
            )}
            <div className="category-color-picker" aria-label={t("Choose a color")}>
              {categoryColorKeys.map((key) => (
                <button aria-label={t("{color} color", { color: t(`category.color.${key}`) })} aria-pressed={colorKey === key} className="category-color-choice" key={key} onClick={() => setColorKey(key)} style={categoryColorStyle(key)} type="button">
                  <span aria-hidden="true" />
                  <span className="visually-hidden">{t(`category.color.${key}`)}</span>
                </button>
              ))}
            </div>
          </fieldset>
          <MutationError mutation={save} />
          </form>
        </ModalDialog>
      ) : null}
      <ModalDialog
        description={t("Archiving removes this category from new allocations while preserving historical reporting.")}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setArchiving(undefined)} type="button">{t("Cancel")}</button>
            <button
              className="danger-button"
              disabled={archive.isPending}
              onClick={() => archiving && archive.mutate(archiving.id)}
              type="button"
            >
              {archive.isPending ? t("Archiving…") : t("Archive category")}
            </button>
          </>
        )}
        onClose={() => setArchiving(undefined)}
        open={Boolean(archiving)}
        title={t("Archive {name}?", { name: archiving ? categoryName(archiving) : t("category") })}
      >
        <InlineNotice title={t("Check child categories first")} tone="warning">
          <p>{t("Categories with active children cannot be archived until their active descendants are moved or archived.")}</p>
        </InlineNotice>
        <MutationError mutation={archive} />
      </ModalDialog>
      <ToastRegion messages={toasts} onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))} />
    </section>
  );
}

function CategoryGroup({ canManage, kind, onArchive, onEdit, rows, workspaceId }: {
  canManage: boolean;
  kind: Category["kind"];
  onArchive: (category: Category) => void;
  onEdit: (category: Category) => void;
  rows: { category: Category; depth: number }[];
  workspaceId: string;
}) {
  const matching = rows.filter(({ category }) => category.kind === kind);
  const [expanded, setExpanded] = useState<Category>();
  return (
    <section className="category-kind-card" aria-labelledby={`${kind}-categories-heading`}>
      <div className="category-kind-heading">
        <span aria-hidden="true" className="resource-icon"><AppIcon name="categories" size={18} /></span>
        <div>
          <h3 id={`${kind}-categories-heading`}>{t(`category.kind.${kind}`)}</h3>
          <span>{t("{count} categories", { count: matching.length })}</span>
        </div>
      </div>
      {matching.length === 0 ? <EmptyState compact title={t("No {kind} categories", { kind: t(`category.kind.${kind}`).toLocaleLowerCase() })} /> : null}
      {/* Tiles are for finding a category; the row underneath is where it is changed, so a
          selected tile reveals its own actions rather than crowding every tile with them. */}
      <CategoryTileSections
        categories={matching.map(({ category }) => category)}
        onSelect={(category) => setExpanded((current) => current?.id === category.id ? undefined : category)}
        selectedId={expanded?.id}
        workspaceId={workspaceId}
      />
      {expanded ? (
        <div className="category-hierarchy">
          <CategoryRow
            canManage={canManage}
            category={expanded}
            depth={0}
            key={expanded.id}
            onArchive={onArchive}
            onEdit={onEdit}
          />
        </div>
      ) : null}
    </section>
  );
}

function CategoryRow({ canManage, category, depth, onArchive, onEdit }: {
  canManage: boolean;
  category: Category;
  depth: number;
  onArchive: (category: Category) => void;
  onEdit: (category: Category) => void;
}) {
  return (
    <article
      className={`category-hierarchy-row${depth > 0 ? " category-hierarchy-row-nested" : ""}`}
      style={{ paddingInlineStart: `${(0.9 + depth * 1.05).toFixed(2)}rem` }}
    >
      <span aria-hidden="true" className="category-branch" />
      <div>
        <strong>
          <CategoryLabel colorKey={category.color_key} iconType={category.icon_type} iconValue={category.icon_value ?? category.icon} name={categoryName(category)} />
        </strong>
        <small>{depth > 0 ? t("Subcategory") : t("Top level")}</small>
      </div>
      {category.system_key ? <StatusBadge tone="neutral">{t("Protected")}</StatusBadge> : null}
      {category.predefined_key ? <StatusBadge tone="neutral">{t("categories.builtIn")}</StatusBadge> : null}
      {canManage && !category.system_key && !category.predefined_key ? (
        <div className="category-row-actions">
          <button className="text-button" onClick={() => onEdit(category)} type="button">{t("Edit")}</button>
          <button className="text-button danger" onClick={() => onArchive(category)} type="button">{t("Archive")}</button>
        </div>
      ) : null}
    </article>
  );
}

export function descendantIds(categories: Category[], rootId: string | undefined) {
  const excluded = new Set<string>();
  if (!rootId) return excluded;
  excluded.add(rootId);
  // Categories are unordered, so keep sweeping until no new descendant is discovered.
  for (let added = true; added; ) {
    added = false;
    for (const category of categories) {
      if (category.parent_id && excluded.has(category.parent_id) && !excluded.has(category.id)) {
        excluded.add(category.id);
        added = true;
      }
    }
  }
  return excluded;
}

function categoryTree(categories: Category[]) {
  const children = new Map<string, Category[]>();
  for (const category of categories) {
    const parentId = category.parent_id ?? "";
    children.set(parentId, [...(children.get(parentId) ?? []), category]);
  }
  for (const values of children.values()) {
    values.sort((left, right) =>
      left.kind === right.kind ? left.name.localeCompare(right.name) : left.kind.localeCompare(right.kind),
    );
  }
  const rows: { category: Category; depth: number }[] = [];
  const visited = new Set<string>();
  function append(parentId: string, depth: number) {
    for (const category of children.get(parentId) ?? []) {
      if (visited.has(category.id)) continue;
      visited.add(category.id);
      rows.push({ category, depth });
      append(category.id, depth + 1);
    }
  }
  append("", 0);
  for (const category of categories) {
    if (!visited.has(category.id)) rows.push({ category, depth: 0 });
  }
  return rows;
}
