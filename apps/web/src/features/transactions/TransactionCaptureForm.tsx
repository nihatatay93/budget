import { useMemo, useState } from "react";

import type { Account, Category, SessionResponse } from "../../api/client";
import { CategoryLabel } from "../../components/CategoryAppearance";
import { InlineNotice, ModalDialog } from "../../components/Presentation";
import { categoryName, t } from "../../lib/i18n";
import type { CaptureDraft, CaptureType } from "../../lib/transactionCapture";
import { CategoryTileSections } from "../categories/CategoryTiles";

type Workspace = SessionResponse["workspaces"][number];

const CAPTURE_TYPES: CaptureType[] = ["expense", "income", "transfer"];

/**
 * The everyday form: what it was, how much, which category, when. Signs, entries, allocations,
 * and reconciliation are derived in lib/transactionCapture rather than asked for here.
 */
export function TransactionCaptureForm({
  accounts,
  categories,
  draft,
  frequentCategories,
  onChange,
  onDetailed,
  workspace,
}: {
  accounts: Account[];
  categories: Category[];
  draft: CaptureDraft;
  frequentCategories?: string[];
  onChange: (patch: Partial<CaptureDraft>) => void;
  onDetailed: () => void;
  workspace: Workspace;
}) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const [categorySearch, setCategorySearch] = useState("");
  const account = accounts.find((candidate) => candidate.id === draft.accountId);
  const currency = account?.currency ?? workspace.base_currency;
  const foreignAccount = Boolean(account) && currency !== workspace.base_currency;
  const categoryKind = draft.type === "income" ? "income" : "expense";

  // An archived category stays selectable while it is the one this transaction already uses;
  // it just cannot be chosen for anything new.
  const selectable = useMemo(
    () => categories.filter((category) =>
      category.kind === categoryKind
      && !category.system_key
      && (!category.archived_at || category.id === draft.categoryId)),
    [categories, categoryKind, draft.categoryId],
  );
  const selectedCategory = categories.find((candidate) => candidate.id === draft.categoryId);

  return (
    <div className="transaction-capture">
      <fieldset className="capture-type">
        <legend>{t("Type")}</legend>
        {CAPTURE_TYPES.map((value) => (
          <label key={value}>
            <input
              checked={draft.type === value}
              name="capture-type"
              onChange={() => onChange({
                type: value,
                // A category belongs to one kind, and a transfer has none at all.
                ...(value === draft.type ? {} : { categoryId: "" }),
                ...(value === "transfer" ? {} : { toAccountId: "" }),
              })}
              type="radio"
              value={value}
            />
            <span>{captureTypeLabel(value)}</span>
          </label>
        ))}
      </fieldset>

      <div className="capture-amount-row">
        <label className="capture-amount">
          {t("Amount")}
          <span className="capture-amount-field">
            <span aria-hidden="true">{currency}</span>
            <input
              autoComplete="off"
              inputMode="decimal"
              onChange={(event) => onChange({ amount: event.target.value })}
              placeholder="0.00"
              required
              value={draft.amount}
            />
          </span>
        </label>
        {foreignAccount ? (
          <label>
            {t("Value in {currency} (optional)", { currency: workspace.base_currency })}
            <input
              inputMode="decimal"
              onChange={(event) => onChange({ baseAmount: event.target.value })}
              placeholder={t("Rate for that date")}
              value={draft.baseAmount}
            />
          </label>
        ) : null}
      </div>

      {draft.type === "transfer" ? (
        <div className="capture-columns">
          <label>
            {t("From account")}
            <select onChange={(event) => onChange({ accountId: event.target.value })} required value={draft.accountId}>
              <option value="">{t("Choose an account")}</option>
              {accounts.map((value) => (
                <option key={value.id} value={value.id}>{value.name} · {value.currency}</option>
              ))}
            </select>
          </label>
          <label>
            {t("To account")}
            <select onChange={(event) => onChange({ toAccountId: event.target.value })} required value={draft.toAccountId}>
              <option value="">{t("Choose an account")}</option>
              {accounts.filter((value) => value.id !== draft.accountId).map((value) => (
                <option key={value.id} value={value.id}>{value.name} · {value.currency}</option>
              ))}
            </select>
          </label>
        </div>
      ) : (
        <div className="capture-category">
          <span id="capture-category-label">{t("Category")}</span>
          <button
            aria-labelledby="capture-category-label capture-category-value"
            className="capture-category-button"
            onClick={() => { setCategorySearch(""); setPickerOpen(true); }}
            type="button"
          >
            <span id="capture-category-value">
              {selectedCategory ? (
                <CategoryLabel
                  colorKey={selectedCategory.color_key}
                  iconType={selectedCategory.icon_type}
                  iconValue={selectedCategory.icon_value ?? selectedCategory.icon}
                  name={categoryName(selectedCategory)}
                />
              ) : t("Uncategorized")}
            </span>
            <span aria-hidden="true">{t("Change")}</span>
          </button>
        </div>
      )}

      <div className="capture-columns">
        <label>
          {t("Date")}
          <input
            onChange={(event) => onChange({ transactionDate: event.target.value })}
            required
            type="date"
            value={draft.transactionDate}
          />
        </label>
        {draft.type !== "transfer" && accounts.length > 1 ? (
          <label>
            {draft.type === "expense" ? t("Paid from") : t("Received into")}
            <select onChange={(event) => onChange({ accountId: event.target.value, baseAmount: "" })} required value={draft.accountId}>
              <option value="">{t("Choose an account")}</option>
              {accounts.map((value) => (
                <option key={value.id} value={value.id}>{value.name} · {value.currency}</option>
              ))}
            </select>
          </label>
        ) : null}
      </div>
      <div className="capture-date-shortcuts">
        <button className="text-button" onClick={() => onChange({ transactionDate: dayOffset(0) })} type="button">
          {t("Today")}
        </button>
        <button className="text-button" onClick={() => onChange({ transactionDate: dayOffset(-1) })} type="button">
          {t("Yesterday")}
        </button>
      </div>

      {draft.type === "transfer" ? null : (
        <label>
          {draft.type === "expense" ? t("Paid to") : t("Received from")}
          <input
            maxLength={200}
            onChange={(event) => onChange({ payee: event.target.value })}
            placeholder={t("For example, Netflix")}
            value={draft.payee}
          />
        </label>
      )}

      <details className="capture-more">
        <summary>{t("More options")}</summary>
        <label className="capture-checkbox">
          <input
            checked={draft.pending}
            onChange={(event) => onChange({ pending: event.target.checked })}
            type="checkbox"
          />
          <span>{t("Still pending, not cleared yet")}</span>
        </label>
        <label>
          {t("Notes (optional)")}
          <input maxLength={4000} onChange={(event) => onChange({ notes: event.target.value })} value={draft.notes} />
        </label>
        <InlineNotice title={t("Splits, several accounts, and adjustments")}>
          {t("The detailed editor writes account entries and category allocations directly.")}
        </InlineNotice>
        <button className="secondary-button" onClick={onDetailed} type="button">
          {t("Use the detailed editor")}
        </button>
      </details>

      <ModalDialog
        description={t("Sections follow your category hierarchy. Choosing a section itself is a valid answer.")}
        footer={<button className="secondary-button" onClick={() => setPickerOpen(false)} type="button">{t("Cancel")}</button>}
        onClose={() => setPickerOpen(false)}
        open={pickerOpen}
        placement="drawer"
        title={t("Choose a category")}
      >
        <label className="category-picker-search">
          <span className="visually-hidden">{t("Search categories")}</span>
          <input
            onChange={(event) => setCategorySearch(event.target.value)}
            placeholder={t("Search categories")}
            type="search"
            value={categorySearch}
          />
        </label>
        <button
          className="category-uncategorized"
          onClick={() => { onChange({ categoryId: "" }); setPickerOpen(false); }}
          type="button"
        >{t("Uncategorized")}</button>
        <CategoryTileSections
          categories={selectable}
          frequent={frequentCategories}
          onSelect={(category) => { onChange({ categoryId: category.id }); setPickerOpen(false); }}
          search={categorySearch}
          selectedId={draft.categoryId}
          workspaceId={workspace.id}
        />
      </ModalDialog>
    </div>
  );
}

function captureTypeLabel(type: CaptureType) {
  if (type === "expense") return t("Expense");
  if (type === "income") return t("Income");
  return t("Transfer");
}

/** A transaction date is a calendar date in the person's own day, not a UTC instant. */
function dayOffset(days: number) {
  const date = new Date();
  date.setDate(date.getDate() + days);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}
