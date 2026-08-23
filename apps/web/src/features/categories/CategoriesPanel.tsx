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
  listCategories,
  monthlyBudgetQueryPrefix,
  updateCategory,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";
import { AppIcon } from "../../components/ExperiencePrimitives";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  ModalDialog,
  StatusBadge,
  ToastRegion,
  type ToastMessage,
} from "../../components/Presentation";

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
  const [icon, setIcon] = useState("");
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
        title: editing ? "Category updated" : "Category created",
        tone: "positive",
      }]);
      reset();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: categoriesQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
      ]);
    },
  });
  const archive = useMutation({
    mutationFn: (categoryId: string) => archiveCategory(workspace.id, categoryId),
    onSuccess: async () => {
      setToasts([{
        id: `category-archive-${archiving?.id ?? "complete"}`,
        title: "Category archived",
        description: "Historical allocations remain available to reports.",
        tone: "positive",
      }]);
      setArchiving(undefined);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: categoriesQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
      ]);
    },
  });

  function reset() {
    setEditing(undefined);
    setName("");
    setKind("expense");
    setParentId("");
    setIcon("");
    setEditorOpen(false);
    save.reset();
  }

  function edit(category: Category) {
    setEditing(category);
    setName(category.name);
    setKind(category.kind);
    setParentId(category.parent_id ?? "");
    setIcon(category.icon ?? "");
    setEditorOpen(true);
  }

  function create() {
    reset();
    setEditorOpen(true);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    save.mutate({
      name,
      kind,
      ...(parentId ? { parent_id: parentId } : {}),
      ...(icon.trim() ? { icon: icon.trim() } : {}),
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
          <p className="eyebrow">How activity is reported</p>
          <h2 id="categories-heading">Categories</h2>
          <p>Expense and income categories organize allocations without constraining their sign.</p>
        </div>
        {canManage ? <button onClick={create} type="button">Add category</button> : null}
      </div>
      {query.isPending ? <LoadingState label="Loading categories" rows={5} /> : null}
      {query.isError ? (
        <InlineNotice
          action={<button className="secondary-button" onClick={() => void query.refetch()} type="button">Try again</button>}
          title="Categories could not be loaded"
          tone="danger"
        >
          <p>{query.error.message}</p>
        </InlineNotice>
      ) : null}
      {!query.isPending && !query.isError && activeRows.length === 0 ? (
        <EmptyState
          action={canManage ? <button onClick={create} type="button">Create category</button> : undefined}
          description="Protected Uncategorized categories are normally created with the workspace."
          icon="categories"
          title="No active categories"
        />
      ) : null}
      {activeRows.length > 0 ? (
        <div className="category-kind-grid">
          <CategoryGroup canManage={canManage} kind="expense" onArchive={confirmArchive} onEdit={edit} rows={activeRows} />
          <CategoryGroup canManage={canManage} kind="income" onArchive={confirmArchive} onEdit={edit} rows={activeRows} />
        </div>
      ) : null}
      {archivedRows.length > 0 ? (
        <details className="archived-resource-group">
          <summary>{archivedRows.length} archived categor{archivedRows.length === 1 ? "y" : "ies"}</summary>
          <div className="category-archive-list">
            {archivedRows.map(({ category, depth }) => (
              <CategoryRow canManage={false} category={category} depth={depth} key={category.id} onArchive={confirmArchive} onEdit={edit} />
            ))}
          </div>
        </details>
      ) : null}
      {!canManage && !query.isPending ? (
        <InlineNotice title="Read-only categories"><p>Viewer access can review the reporting hierarchy but cannot change it.</p></InlineNotice>
      ) : null}
      {canManage ? (
        <ModalDialog
          description="Categories group allocations for reports and monthly plans. Parent and child categories must share a kind."
          footer={(
            <>
              <button className="secondary-button" onClick={reset} type="button">Cancel</button>
              <button disabled={save.isPending} form="category-editor" type="submit">
                {save.isPending ? "Saving…" : editing ? "Save category" : "Add category"}
              </button>
            </>
          )}
          onClose={reset}
          open={editorOpen}
          placement="drawer"
          title={editing ? `Edit ${editing.name}` : "Add category"}
        >
          <form className="resource-form resource-editor-form" id="category-editor" onSubmit={submit}>
          <label>
            Name
            <input required maxLength={100} value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <div className="form-columns">
            <label>
              Kind
              <select
                value={kind}
                onChange={(event) => {
                  setKind(event.target.value as typeof kind);
                  setParentId("");
                }}
              >
                <option value="expense">Expense</option>
                <option value="income">Income</option>
              </select>
            </label>
            <label>
              Parent (optional)
              <select value={parentId} onChange={(event) => setParentId(event.target.value)}>
                <option value="">Top level</option>
                {parents?.map((category) => <option key={category.id} value={category.id}>{category.name}</option>)}
              </select>
            </label>
          </div>
          <label>
            Icon (optional)
            <input maxLength={64} value={icon} onChange={(event) => setIcon(event.target.value)} />
          </label>
          <MutationError mutation={save} />
          </form>
        </ModalDialog>
      ) : null}
      <ModalDialog
        description="Archiving removes this category from new allocations while preserving historical reporting."
        footer={(
          <>
            <button className="secondary-button" onClick={() => setArchiving(undefined)} type="button">Cancel</button>
            <button
              className="danger-button"
              disabled={archive.isPending}
              onClick={() => archiving && archive.mutate(archiving.id)}
              type="button"
            >
              {archive.isPending ? "Archiving…" : "Archive category"}
            </button>
          </>
        )}
        onClose={() => setArchiving(undefined)}
        open={Boolean(archiving)}
        title={`Archive ${archiving?.name ?? "category"}?`}
      >
        <InlineNotice title="Check child categories first" tone="warning">
          <p>Categories with active children cannot be archived until their active descendants are moved or archived.</p>
        </InlineNotice>
        <MutationError mutation={archive} />
      </ModalDialog>
      <ToastRegion messages={toasts} onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))} />
    </section>
  );
}

function CategoryGroup({ canManage, kind, onArchive, onEdit, rows }: {
  canManage: boolean;
  kind: Category["kind"];
  onArchive: (category: Category) => void;
  onEdit: (category: Category) => void;
  rows: { category: Category; depth: number }[];
}) {
  const matching = rows.filter(({ category }) => category.kind === kind);
  return (
    <section className="category-kind-card" aria-labelledby={`${kind}-categories-heading`}>
      <div className="category-kind-heading">
        <span aria-hidden="true" className="resource-icon"><AppIcon name="categories" size={18} /></span>
        <div>
          <h3 id={`${kind}-categories-heading`}>{kind === "expense" ? "Expense" : "Income"}</h3>
          <span>{matching.length} categor{matching.length === 1 ? "y" : "ies"}</span>
        </div>
      </div>
      {matching.length === 0 ? <EmptyState compact title={`No ${kind} categories`} /> : null}
      <div className="category-hierarchy">
        {matching.map(({ category, depth }) => (
          <CategoryRow
            canManage={canManage}
            category={category}
            depth={depth}
            key={category.id}
            onArchive={onArchive}
            onEdit={onEdit}
          />
        ))}
      </div>
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
          {category.icon ? <span aria-hidden="true">{category.icon} </span> : null}
          <span>{category.name}</span>
        </strong>
        <small>{depth > 0 ? "Subcategory" : "Top level"}</small>
      </div>
      {category.system_key ? <StatusBadge tone="neutral">Protected</StatusBadge> : null}
      {canManage && !category.system_key ? (
        <div className="category-row-actions">
          <button className="text-button" onClick={() => onEdit(category)} type="button">Edit</button>
          <button className="text-button danger" onClick={() => onArchive(category)} type="button">Archive</button>
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
