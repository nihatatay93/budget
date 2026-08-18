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
import { ResourceState } from "../../components/ResourceState";

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
  const save = useMutation({
    mutationFn: (input: CategoryWriteRequest) =>
      editing
        ? updateCategory(workspace.id, editing.id, input)
        : createCategory(workspace.id, input),
    onSuccess: async () => {
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
    onSuccess: () => Promise.all([
      queryClient.invalidateQueries({ queryKey: categoriesQueryKey(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
    ]),
  });

  function reset() {
    setEditing(undefined);
    setName("");
    setKind("expense");
    setParentId("");
    setIcon("");
  }

  function edit(category: Category) {
    setEditing(category);
    setName(category.name);
    setKind(category.kind);
    setParentId(category.parent_id ?? "");
    setIcon(category.icon ?? "");
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

  // A category cannot be parented under itself or any of its own descendants.
  const excluded = descendantIds(query.data ?? [], editing?.id);
  const parents = query.data?.filter(
    (category) => category.kind === kind && !excluded.has(category.id) && !category.archived_at,
  );
  const categoryRows = categoryTree(query.data ?? []);

  return (
    <section className="setup-panel" aria-labelledby="categories-heading">
      <div className="section-heading">
        <div>
          <p className="eyebrow">How activity is reported</p>
          <h2 id="categories-heading">Categories</h2>
        </div>
        <span>{query.data?.length ?? 0} active</span>
      </div>
      <ResourceState query={query} empty="Protected Uncategorized categories are created automatically." />
      <div className="resource-list">
        {categoryRows.map(({ category, depth }) => (
          <article className="resource-row" key={category.id}>
            <div style={{ paddingInlineStart: `${depth * 1.1}rem` }}>
              <strong>{category.icon ? `${category.icon} ` : ""}{category.name}</strong>
              <small>{category.kind}{category.system_key ? " · protected" : ""}</small>
            </div>
            {canManage && !category.system_key ? (
              <div className="resource-actions">
                <button className="text-button" type="button" onClick={() => edit(category)}>Edit</button>
                <button
                  className="text-button danger"
                  type="button"
                  onClick={() => {
                    if (window.confirm(`Archive ${category.name}?`)) archive.mutate(category.id);
                  }}
                >
                  Archive
                </button>
              </div>
            ) : null}
          </article>
        ))}
      </div>
      <MutationError mutation={archive} />
      {canManage ? (
        <form className="resource-form" onSubmit={submit}>
          <h3>{editing ? `Edit ${editing.name}` : "Add category"}</h3>
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
          <div className="form-actions">
            <button disabled={save.isPending} type="submit">
              {editing ? "Save category" : "Add category"}
            </button>
            {editing ? <button className="secondary-button" type="button" onClick={reset}>Cancel</button> : null}
          </div>
        </form>
      ) : null}
    </section>
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
