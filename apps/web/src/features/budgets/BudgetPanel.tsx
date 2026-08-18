import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useMemo, useState } from "react";

import {
  APIError,
  type Category,
  type MonthlyBudget,
  type MonthlyBudgetWriteRequest,
  type SessionResponse,
  categoriesQueryKey,
  getMonthlyBudget,
  listCategories,
  monthlyBudgetQueryKey,
  replaceMonthlyBudget,
} from "../../api/client";
import { formatMoney, majorAmountInput, parseMajorAmount } from "../../lib/currency";
import { monthLabel, workspaceMonth } from "../../lib/month";

type Workspace = SessionResponse["workspaces"][number];
type ItemDraft = { key: string; categoryId: string; amount: string };

let nextBudgetDraftKey = 0;
function budgetDraftKey() {
  nextBudgetDraftKey += 1;
  return `budget-draft-${nextBudgetDraftKey}`;
}

export function BudgetPanel({ workspace, canManage }: { workspace: Workspace; canManage: boolean }) {
  const queryClient = useQueryClient();
  const [month, setMonth] = useState(() => workspaceMonth(workspace.timezone));
  const budgetQuery = useQuery({
    queryKey: monthlyBudgetQueryKey(workspace.id, month),
    queryFn: () => getMonthlyBudget(workspace.id, month),
    retry: false,
  });
  const categoriesQuery = useQuery({
    queryKey: categoriesQueryKey(workspace.id),
    queryFn: () => listCategories(workspace.id),
  });
  const [name, setName] = useState("");
  const [items, setItems] = useState<ItemDraft[]>([]);
  const [validation, setValidation] = useState("");

  const noBudget = budgetQuery.error instanceof APIError && budgetQuery.error.status === 404;
  useEffect(() => {
    if (budgetQuery.data) {
      setName(budgetQuery.data.name);
      setItems(budgetQuery.data.items.map((item) => ({
        key: budgetDraftKey(), categoryId: item.category_id,
        amount: majorAmountInput(item.planned_base_minor),
      })));
      setValidation("");
    } else if (noBudget) {
      setName(`${monthLabel(month)} plan`);
      setItems([]);
      setValidation("");
    }
  }, [budgetQuery.data, month, noBudget]);

  const save = useMutation({
    mutationFn: (input: MonthlyBudgetWriteRequest) =>
      replaceMonthlyBudget(workspace.id, month, input),
    onSuccess: (value) => {
      queryClient.setQueryData(monthlyBudgetQueryKey(workspace.id, month), value);
    },
  });

  const activeExpenseCategories = useMemo(
    () => (categoriesQuery.data ?? []).filter(
      (category) => category.kind === "expense" && !category.archived_at,
    ),
    [categoriesQuery.data],
  );

  function addItem() {
    const selected = new Set(items.map((item) => item.categoryId));
    const category = activeExpenseCategories.find((candidate) => !selected.has(candidate.id));
    setItems((current) => [
      ...current,
      { key: budgetDraftKey(), categoryId: category?.id ?? "", amount: "" },
    ]);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    setValidation("");
    const normalizedName = name.trim();
    if (!normalizedName || items.length === 0) {
      setValidation("Give the plan a name and add at least one expense category.");
      return;
    }
    const selected = new Set<string>();
    const writes: MonthlyBudgetWriteRequest["items"] = [];
    let total = 0;
    for (const item of items) {
      const amount = parseMajorAmount(item.amount);
      if (!item.categoryId || amount === null || amount <= 0) {
        setValidation("Every budget item needs an expense category and a positive amount.");
        return;
      }
      if (selected.has(item.categoryId)) {
        setValidation("Each category can appear only once in a monthly plan.");
        return;
      }
      selected.add(item.categoryId);
      total += amount;
      if (!Number.isSafeInteger(total)) {
        setValidation("The planned total is too large.");
        return;
      }
      writes.push({ category_id: item.categoryId, amount_base_minor: amount });
    }
    if (hasBranchOverlap(selected, categoriesQuery.data ?? [])) {
      setValidation("Choose a category or its subcategories, not both.");
      return;
    }
    save.mutate({ name: normalizedName, items: writes });
  }

  return (
    <section className="budget-panel" aria-labelledby="monthly-budget-heading">
      <div className="budget-heading">
        <div>
          <p className="eyebrow">Posted allocation plan</p>
          <h2 id="monthly-budget-heading">Monthly budget</h2>
          <p>Refunds reduce usage; pending transactions stay outside the authoritative total.</p>
        </div>
        <label className="budget-month-picker">
          Month
          <input
            aria-label="Budget month"
            type="month"
            value={month}
            onChange={(event) => {
              if (event.target.value) {
                save.reset();
                setValidation("");
                setMonth(event.target.value);
              }
            }}
          />
        </label>
      </div>

      {budgetQuery.isPending ? <p className="resource-state">Loading monthly budget…</p> : null}
      {budgetQuery.isError && !noBudget ? (
        <div className="projection-error" role="alert">
          <p>{budgetQuery.error.message}</p>
          <button className="secondary-button" type="button" onClick={() => void budgetQuery.refetch()}>
            Try again
          </button>
        </div>
      ) : null}
      {noBudget ? (
        <p className="budget-empty">No plan exists for {monthLabel(month)} yet.</p>
      ) : null}
      {budgetQuery.data ? <BudgetUsage value={budgetQuery.data} /> : null}

      {!budgetQuery.isPending && (budgetQuery.data || noBudget) && canManage ? (
        <form className="budget-form" onSubmit={submit}>
          <div className="budget-form-heading">
            <h3>{budgetQuery.data ? "Edit complete plan" : "Create this plan"}</h3>
            <span>Amounts use {workspace.base_currency}.</span>
          </div>
          <label>
            Plan name
            <input required maxLength={100} value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <div className="budget-drafts">
            {items.map((item, index) => (
              <div className="budget-draft" key={item.key}>
                <label>
                  Expense category
                  <select
                    aria-label={`Budget category ${index + 1}`}
                    value={item.categoryId}
                    onChange={(event) => setItems((current) => current.map((candidate) =>
                      candidate.key === item.key
                        ? { ...candidate, categoryId: event.target.value }
                        : candidate,
                    ))}
                  >
                    <option value="">Choose category</option>
                    {budgetCategoryOptions(item, activeExpenseCategories, budgetQuery.data).map((category) => (
                      <option key={category.id} value={category.id}>{category.label}</option>
                    ))}
                  </select>
                </label>
                <label>
                  Planned amount
                  <input
                    aria-label={`Budget amount ${index + 1}`}
                    inputMode="decimal"
                    placeholder="0.00"
                    value={item.amount}
                    onChange={(event) => setItems((current) => current.map((candidate) =>
                      candidate.key === item.key ? { ...candidate, amount: event.target.value } : candidate,
                    ))}
                  />
                </label>
                <button
                  className="text-button danger"
                  type="button"
                  onClick={() => setItems((current) => current.filter((candidate) => candidate.key !== item.key))}
                >
                  Remove
                </button>
              </div>
            ))}
          </div>
          <button className="secondary-button budget-add" type="button" onClick={addItem}>
            Add category
          </button>
          {validation ? <p className="form-error" role="alert">{validation}</p> : null}
          {save.error ? (
            <p className="form-error" role="alert">
              {save.error instanceof APIError ? save.error.message : "The monthly plan could not be saved."}
            </p>
          ) : null}
          <div className="form-actions">
            <button disabled={save.isPending} type="submit">
              {save.isPending ? "Saving…" : "Save monthly plan"}
            </button>
          </div>
        </form>
      ) : null}
      {!budgetQuery.isPending && (budgetQuery.data || noBudget) && !canManage ? (
        <p className="permission-note">Viewer access can review budget usage but cannot change the plan.</p>
      ) : null}
    </section>
  );
}

function BudgetUsage({ value }: { value: MonthlyBudget }) {
  return (
    <div className="budget-usage">
      <div className="budget-plan-title">
        <strong>{value.name}</strong>
        <span>{monthLabel(value.month)} · {value.timezone}</span>
      </div>
      <div className="budget-total-grid">
        <BudgetTotal label="Planned" value={value.planned_base_minor} currency={value.base_currency} />
        <BudgetTotal label="Used" value={value.used_base_minor} currency={value.base_currency} />
        <BudgetTotal label="Remaining" value={value.remaining_base_minor} currency={value.base_currency} />
      </div>
      <div className="budget-item-list">
        {value.items.map((item) => {
          const progress = Math.max(0, Math.min(100, item.used_base_minor / item.planned_base_minor * 100));
          return (
            <article className="budget-usage-item" key={item.id}>
              <div className="budget-usage-copy">
                <div>
                  <strong>{item.category_icon ? `${item.category_icon} ` : ""}{item.category_name}</strong>
                  <small>{item.category_archived_at ? "Archived category · " : ""}Includes subcategories</small>
                </div>
                <div>
                  <strong>{formatMoney(item.used_base_minor, value.base_currency)}</strong>
                  <small>of {formatMoney(item.planned_base_minor, value.base_currency)}</small>
                </div>
              </div>
              <progress
                aria-label={`${item.category_name} budget usage`}
                max={100}
                value={progress}
              />
              <small className={item.remaining_base_minor < 0 ? "budget-over" : ""}>
                {formatMoney(item.remaining_base_minor, value.base_currency)} remaining
              </small>
            </article>
          );
        })}
      </div>
    </div>
  );
}

function BudgetTotal({ label, value, currency }: {
  label: string;
  value: number;
  currency: MonthlyBudget["base_currency"];
}) {
  return <article><span>{label}</span><strong>{formatMoney(value, currency)}</strong></article>;
}

function budgetCategoryOptions(
  draft: ItemDraft,
  active: Category[],
  saved: MonthlyBudget | undefined,
) {
  const options = active.map((category) => ({
    id: category.id, label: `${category.icon ? `${category.icon} ` : ""}${category.name}`,
  }));
  const retained = saved?.items.find((item) => item.category_id === draft.categoryId);
  if (retained && !options.some((option) => option.id === retained.category_id)) {
    options.push({
      id: retained.category_id,
      label: `${retained.category_icon ? `${retained.category_icon} ` : ""}${retained.category_name} (archived)`,
    });
  }
  return options.sort((left, right) => left.label.localeCompare(right.label));
}

function hasBranchOverlap(selected: Set<string>, categories: Category[]): boolean {
  const byID = new Map(categories.map((category) => [category.id, category]));
  for (const categoryID of selected) {
    const visited = new Set([categoryID]);
    let parentID = byID.get(categoryID)?.parent_id;
    while (parentID && !visited.has(parentID)) {
      if (selected.has(parentID)) return true;
      visited.add(parentID);
      parentID = byID.get(parentID)?.parent_id;
    }
  }
  return false;
}
