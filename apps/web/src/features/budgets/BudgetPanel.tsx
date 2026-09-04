import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type ReactNode, useEffect, useMemo, useState } from "react";

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
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  ModalDialog,
  MoneyAmount,
  ProgressMeter,
  StatusBadge,
  ToastRegion,
  type ToastMessage,
} from "../../components/Presentation";
import { CategoryLabel } from "../../components/CategoryAppearance";
import { formatMoney, majorAmountInput, parseMajorAmount } from "../../lib/currency";
import { categoryName, t } from "../../lib/i18n";
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
  const [editing, setEditing] = useState(false);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

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
      setName(t("{month} plan", { month: monthLabel(month) }));
      setItems([]);
      setValidation("");
    }
  }, [budgetQuery.data, month, noBudget]);

  const save = useMutation({
    mutationFn: (input: MonthlyBudgetWriteRequest) =>
      replaceMonthlyBudget(workspace.id, month, input),
    onSuccess: (value) => {
      queryClient.setQueryData(monthlyBudgetQueryKey(workspace.id, month), value);
      setEditing(false);
      setToasts([{ id: `budget-${value.updated_at}`, title: t("Monthly plan saved"), tone: "positive" }]);
    },
  });

  const activeExpenseCategories = useMemo(
    () => (categoriesQuery.data ?? []).filter(
      (category) => category.kind === "expense" && !category.archived_at,
    ),
    [categoriesQuery.data],
  );
  const categoryById = useMemo(
    () => new Map((categoriesQuery.data ?? []).map((category) => [category.id, category])),
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

  function changeMonth(value: string) {
    if (!value) return;
    save.reset();
    setValidation("");
    setEditing(false);
    setMonth(value);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    setValidation("");
    const normalizedName = name.trim();
    if (!normalizedName || items.length === 0) {
      setValidation(t("Give the plan a name and add at least one expense category."));
      return;
    }
    const selected = new Set<string>();
    const writes: MonthlyBudgetWriteRequest["items"] = [];
    let total = 0;
    for (const item of items) {
      const amount = parseMajorAmount(item.amount);
      if (!item.categoryId || amount === null || amount <= 0) {
        setValidation(t("Every budget item needs an expense category and a positive amount."));
        return;
      }
      if (selected.has(item.categoryId)) {
        setValidation(t("Each category can appear only once in a monthly plan."));
        return;
      }
      selected.add(item.categoryId);
      total += amount;
      if (!Number.isSafeInteger(total)) {
        setValidation(t("The planned total is too large."));
        return;
      }
      writes.push({ category_id: item.categoryId, amount_base_minor: amount });
    }
    if (hasBranchOverlap(selected, categoriesQuery.data ?? [])) {
      setValidation(t("Choose a category or its subcategories, not both."));
      return;
    }
    save.mutate({ name: normalizedName, items: writes });
  }

  return (
    <section className="budget-panel" aria-labelledby="monthly-budget-heading">
      <div className="budget-heading">
        <div>
          <p className="eyebrow">{t("Posted allocation plan")}</p>
          <h2 id="monthly-budget-heading">{t("Monthly budget")}</h2>
          <p>{t("Refunds reduce usage; pending transactions stay outside the authoritative total.")}</p>
        </div>
        <div className="budget-month-navigation" aria-label={t("Budget month navigation")}>
          <button
            aria-label={t("Previous month")}
            className="month-step-button"
            onClick={() => changeMonth(shiftMonth(month, -1))}
            type="button"
          >
            ‹
          </button>
          <label className="budget-month-picker">
            <span className="visually-hidden">{t("Month")}</span>
            <input
              aria-label={t("Budget month")}
              type="month"
              value={month}
              onChange={(event) => changeMonth(event.target.value)}
            />
          </label>
          <button
            aria-label={t("Next month")}
            className="month-step-button"
            onClick={() => changeMonth(shiftMonth(month, 1))}
            type="button"
          >
            ›
          </button>
        </div>
      </div>

      {budgetQuery.isPending ? <LoadingState label={t("Loading monthly budget")} rows={5} /> : null}
      {budgetQuery.isError && !noBudget ? (
        <InlineNotice
          action={<button className="secondary-button" type="button" onClick={() => void budgetQuery.refetch()}>{t("Try again")}</button>}
          title={t("This plan could not be loaded")}
          tone="danger"
        >
          <p>{budgetQuery.error.message}</p>
        </InlineNotice>
      ) : null}
      {noBudget ? (
        <EmptyState
          action={canManage ? <button onClick={() => setEditing(true)} type="button">{t("Create plan")}</button> : undefined}
          description={t("Set category targets to give every posted expense a monthly context.")}
          icon="budget"
          title={t("No plan exists for {month} yet", { month: monthLabel(month) })}
        />
      ) : null}
      {budgetQuery.data ? (
        <BudgetUsage
          action={canManage ? <button className="secondary-button" onClick={() => setEditing(true)} type="button">{t("Edit plan")}</button> : undefined}
          value={budgetQuery.data}
        />
      ) : null}

      {!budgetQuery.isPending && (budgetQuery.data || noBudget) && canManage ? (
        <ModalDialog
          description={t("Set complete category targets for this month. Saving replaces the current monthly plan.")}
          footer={(
            <>
              <button className="secondary-button" onClick={() => setEditing(false)} type="button">{t("Cancel")}</button>
              <button disabled={save.isPending} form="monthly-budget-form" type="submit">
                {save.isPending ? t("Saving…") : t("Save monthly plan")}
              </button>
            </>
          )}
          onClose={() => setEditing(false)}
          open={editing}
          placement="drawer"
          title={budgetQuery.data ? t("Edit {month} plan", { month: monthLabel(month) }) : t("Plan {month}", { month: monthLabel(month) })}
        >
          <form className="budget-form" id="monthly-budget-form" onSubmit={submit}>
          <div className="budget-form-heading">
            <h3>{budgetQuery.data ? t("Edit complete plan") : t("Create this plan")}</h3>
            <span>{t("Amounts use {currency}. Posted allocations determine usage.", { currency: workspace.base_currency })}</span>
          </div>
          <label>
            {t("Plan name")}
            <input required maxLength={100} value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <div className="budget-drafts">
            {items.map((item, index) => (
              <div className="budget-draft" key={item.key}>
                <label>
                  {t("Expense category")}
                  <select
                    aria-label={t("Budget category {number}", { number: index + 1 })}
                    value={item.categoryId}
                    onChange={(event) => setItems((current) => current.map((candidate) =>
                      candidate.key === item.key
                        ? { ...candidate, categoryId: event.target.value }
                        : candidate,
                    ))}
                  >
                    <option value="">{t("Choose category")}</option>
                    {budgetCategoryOptions(item, activeExpenseCategories, budgetQuery.data).map((category) => (
                      <option key={category.id} value={category.id}>{category.label}</option>
                    ))}
                  </select>
                  {categoryById.get(item.categoryId) ? <CategoryLabel {...categoryAppearance(categoryById.get(item.categoryId)!)} /> : null}
                </label>
                <label>
                  {t("Planned amount")}
                  <input
                    aria-label={t("Budget amount {number}", { number: index + 1 })}
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
                  {t("Remove")}
                </button>
              </div>
            ))}
          </div>
          <button className="secondary-button budget-add" type="button" onClick={addItem}>
            {t("Add category")}
          </button>
          {validation ? <p className="form-error" role="alert">{validation}</p> : null}
          {save.error ? (
            <p className="form-error" role="alert">
              {save.error instanceof APIError ? save.error.message : t("The monthly plan could not be saved.")}
            </p>
          ) : null}
          </form>
        </ModalDialog>
      ) : null}
      {!budgetQuery.isPending && (budgetQuery.data || noBudget) && !canManage ? (
        <InlineNotice title={t("Read-only budget")}>
          <p>{t("Viewer access can review budget usage but cannot change the plan.")}</p>
        </InlineNotice>
      ) : null}
      <ToastRegion messages={toasts} onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))} />
    </section>
  );
}

function BudgetUsage({ action, value }: { action?: ReactNode; value: MonthlyBudget }) {
  const totalProgress = budgetProgress(value.used_base_minor, value.planned_base_minor);
  const totalTone = value.remaining_base_minor < 0 ? "danger" : totalProgress >= 85 ? "warning" : "positive";
  return (
    <div className="budget-usage">
      <div className="budget-plan-title">
        <div>
          <strong>{value.name}</strong>
          <span>{monthLabel(value.month)} · {value.timezone}</span>
        </div>
        {action}
      </div>
      <div className="budget-total-grid">
        <BudgetTotal label={t("Planned")} value={value.planned_base_minor} currency={value.base_currency} />
        <BudgetTotal label={t("Used")} value={value.used_base_minor} currency={value.base_currency} />
        <BudgetTotal label={value.remaining_base_minor < 0 ? t("Over plan") : t("Remaining")}
          tone={value.remaining_base_minor < 0 ? "danger" : "positive"}
          value={Math.abs(value.remaining_base_minor)}
          currency={value.base_currency}
        />
      </div>
      <ProgressMeter label={t("Total monthly budget usage")} tone={totalTone} value={totalProgress} />
      <div className="budget-item-list">
        {value.items.map((item) => {
          const progress = budgetProgress(item.used_base_minor, item.planned_base_minor);
          const isRefund = item.used_base_minor < 0;
          const isOver = item.remaining_base_minor < 0;
          const tone = isOver ? "danger" : progress >= 85 ? "warning" : "positive";
          return (
            <article className="budget-usage-item" key={item.id}>
              <div className="budget-usage-copy">
                <div>
                  <strong><CategoryLabel colorKey={item.category_color_key} iconType={item.category_icon_type} iconValue={item.category_icon_value ?? item.category_icon} name={categoryName({ name: item.category_name, predefined_key: item.category_predefined_key })} /></strong>
                  <small>{item.category_archived_at ? `${t("Archived category")} · ` : ""}{t("Includes subcategories")}</small>
                </div>
                <div>
                  <strong><MoneyAmount amount={item.used_base_minor} currency={value.base_currency} signed={isRefund} /></strong>
                  <small>{t("of {amount}", { amount: formatMoney(item.planned_base_minor, value.base_currency) })}</small>
                </div>
              </div>
              <ProgressMeter
                label={t("{category} budget usage", {
                  category: categoryName({ name: item.category_name, predefined_key: item.category_predefined_key }),
                })}
                tone={tone}
                value={progress}
              />
              <div className="budget-item-status">
                <StatusBadge tone={isOver ? "danger" : isRefund ? "positive" : progress >= 85 ? "warning" : "neutral"}>
                  {isOver ? t("Over budget") : isRefund ? t("Net refund") : progress >= 85 ? t("Nearly used") : t("On track")}
                </StatusBadge>
                <small className={isOver ? "budget-over" : ""}>
                  {isOver
                    ? t("{amount} over", { amount: formatMoney(Math.abs(item.remaining_base_minor), value.base_currency) })
                    : t("{amount} remaining", { amount: formatMoney(item.remaining_base_minor, value.base_currency) })}
                </small>
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}

function BudgetTotal({ label, tone, value, currency }: {
  label: string;
  value: number;
  currency: MonthlyBudget["base_currency"];
  tone?: "danger" | "positive";
}) {
  return (
    <article className={tone ? `budget-total-${tone}` : undefined}>
      <span>{label}</span>
      <strong><MoneyAmount amount={value} currency={currency} /></strong>
    </article>
  );
}

function budgetProgress(used: number, planned: number): number {
  return planned > 0 ? Math.max(0, Math.min(100, used / planned * 100)) : 0;
}

function shiftMonth(month: string, offset: number): string {
  const [year, monthNumber] = month.split("-").map(Number);
  if (!year || !monthNumber) return month;
  const shifted = new Date(Date.UTC(year, monthNumber - 1 + offset, 1));
  return `${shifted.getUTCFullYear()}-${String(shifted.getUTCMonth() + 1).padStart(2, "0")}`;
}

function budgetCategoryOptions(
  draft: ItemDraft,
  active: Category[],
  saved: MonthlyBudget | undefined,
) {
  const options = active.map((category) => ({
    id: category.id, label: categoryName(category),
  }));
  const retained = saved?.items.find((item) => item.category_id === draft.categoryId);
  if (retained && !options.some((option) => option.id === retained.category_id)) {
    options.push({
      id: retained.category_id,
      label: `${categoryName({ name: retained.category_name, predefined_key: retained.category_predefined_key })} (${t("Archived")})`,
    });
  }
  return options.sort((left, right) => left.label.localeCompare(right.label));
}

function categoryAppearance(category: Category) {
  return { colorKey: category.color_key, iconType: category.icon_type, iconValue: category.icon_value ?? category.icon, name: categoryName(category) };
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
