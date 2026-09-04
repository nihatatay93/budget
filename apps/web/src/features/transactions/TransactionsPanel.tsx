import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import {
  APIError,
  type Account,
  type SessionResponse,
  type Transaction as LedgerTransaction,
  type TransactionWriteRequest,
  accountsQueryKey,
  categoriesQueryKey,
  createTransaction,
  deleteTransaction,
  financialProjectionQueryPrefix,
  listAccounts,
  listCategories,
  listTransactions,
  monthlyBudgetQueryPrefix,
  spendingAnalysisQueryPrefix,
  transactionsQueryKey,
  updateTransaction,
} from "../../api/client";
import { AppIcon } from "../../components/ExperiencePrimitives";
import { CategoryAppearance, CategoryLabel } from "../../components/CategoryAppearance";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  ModalDialog,
  MoneyAmount,
  StatusBadge,
  type ToastMessage,
  ToastRegion,
} from "../../components/Presentation";
import { majorAmountInput, parseMajorAmount } from "../../lib/currency";
import { categoryName, t } from "../../lib/i18n";
import {
  type CaptureDraft,
  captureDraft,
  captureRequest,
  emptyCaptureDraft,
  suggestedAccountId,
} from "../../lib/transactionCapture";
import { TransactionCaptureForm } from "./TransactionCaptureForm";
import { frequentCategoryIds } from "../categories/CategoryTiles";

type Workspace = SessionResponse["workspaces"][number];
type EntryDraft = { key: string; accountId: string; amount: string; baseAmount: string };
type AllocationDraft = { key: string; categoryId: string; amount: string };
type DetailedFields = {
  kind: TransactionWriteRequest["kind"];
  status: TransactionWriteRequest["status"];
  transactionDate: string;
  payee: string;
  description: string;
  notes: string;
  entries: EntryDraft[];
  allocations: AllocationDraft[];
};
type KindFilter = "all" | LedgerTransaction["kind"];
type StatusFilter = "all" | LedgerTransaction["status"];

let nextDraftKey = 0;
let nextToastKey = 0;

function draftKey() {
  nextDraftKey += 1;
  return `transaction-draft-${nextDraftKey}`;
}

function toastKey() {
  nextToastKey += 1;
  return `transaction-toast-${nextToastKey}`;
}

function today() {
  const date = new Date();
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function emptyEntry(accountId = ""): EntryDraft {
  return { key: draftKey(), accountId, amount: "", baseAmount: "" };
}

function emptyAllocation(categoryId = ""): AllocationDraft {
  return { key: draftKey(), categoryId, amount: "" };
}

/** The detailed fields as one comparable value, ignoring the keys that only identify a row. */
function detailedSignature(fields: DetailedFields) {
  return JSON.stringify({
    ...fields,
    entries: fields.entries.map(({ key, ...entry }) => entry),
    allocations: fields.allocations.map(({ key, ...allocation }) => allocation),
  });
}

export function TransactionsPanel({ workspace, canManage }: { workspace: Workspace; canManage: boolean }) {
  const queryClient = useQueryClient();
  const transactions = useQuery({
    queryKey: transactionsQueryKey(workspace.id),
    queryFn: () => listTransactions(workspace.id),
  });
  const accounts = useQuery({
    queryKey: accountsQueryKey(workspace.id),
    queryFn: () => listAccounts(workspace.id),
  });
  const categories = useQuery({
    queryKey: categoriesQueryKey(workspace.id),
    queryFn: () => listCategories(workspace.id),
  });
  const [search, setSearch] = useState("");
  const [kindFilter, setKindFilter] = useState<KindFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<LedgerTransaction>();
  const [deleteTarget, setDeleteTarget] = useState<LedgerTransaction>();
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const [kind, setKind] = useState<TransactionWriteRequest["kind"]>("standard");
  const [status, setStatus] = useState<TransactionWriteRequest["status"]>("posted");
  const [transactionDate, setTransactionDate] = useState(today);
  const [payee, setPayee] = useState("");
  const [description, setDescription] = useState("");
  const [notes, setNotes] = useState("");
  const [entries, setEntries] = useState<EntryDraft[]>([emptyEntry()]);
  const [allocations, setAllocations] = useState<AllocationDraft[]>([]);
  const [validation, setValidation] = useState("");
  const [editorMode, setEditorMode] = useState<"simple" | "detailed">("simple");
  const [draft, setDraft] = useState<CaptureDraft>(() => emptyCaptureDraft(today()));
  const [detailedOrigin, setDetailedOrigin] = useState<string>();

  const accountNames = useMemo(
    () => new Map((accounts.data ?? []).map((account) => [account.id, account.name])),
    [accounts.data],
  );
  const categoryNames = useMemo(
    () => new Map((categories.data ?? []).map((category) => [category.id, categoryName(category)])),
    [categories.data],
  );
  const categoryById = useMemo(
    () => new Map((categories.data ?? []).map((category) => [category.id, category])),
    [categories.data],
  );
  // The picker offers what this workspace actually uses, newest activity first.
  const frequentCategories = useMemo(
    () => frequentCategoryIds(
      [...(transactions.data ?? [])]
        .sort((left, right) => right.transaction_date.localeCompare(left.transaction_date))
        .slice(0, 80)
        .map((transaction) => transaction.allocations),
    ),
    [transactions.data],
  );
  const captureContext = useMemo(
    () => ({
      accounts: accounts.data ?? [],
      baseCurrency: workspace.base_currency,
      categories: categories.data ?? [],
    }),
    [accounts.data, categories.data, workspace.base_currency],
  );
  const filteredTransactions = useMemo(() => {
    const term = search.trim().toLocaleLowerCase();
    return (transactions.data ?? []).filter((value) => {
      if (kindFilter !== "all" && value.kind !== kindFilter) return false;
      if (statusFilter !== "all" && value.status !== statusFilter) return false;
      if (!term) return true;
      const searchable = [
        value.payee,
        value.description,
        value.notes,
        value.transaction_date,
        transactionKindLabel(value.kind),
        value.status,
        ...value.entries.map((entry) => accountNames.get(entry.account_id)),
        ...value.allocations.map((allocation) => categoryNames.get(allocation.category_id)),
      ].filter(Boolean).join(" ").toLocaleLowerCase();
      return searchable.includes(term);
    });
  }, [accountNames, categoryNames, kindFilter, search, statusFilter, transactions.data]);

  const save = useMutation({
    mutationFn: (input: TransactionWriteRequest) =>
      editing
        ? updateTransaction(workspace.id, editing.id, input)
        : createTransaction(workspace.id, input),
    onSuccess: async () => {
      const title = editing ? t("Transaction updated") : t("Transaction added");
      closeEditor();
      setToasts((current) => [...current, { id: toastKey(), title, tone: "positive" }]);
      await invalidateFinancialViews();
    },
  });
  const remove = useMutation({
    mutationFn: (transactionId: string) => deleteTransaction(workspace.id, transactionId),
    onSuccess: async () => {
      setDeleteTarget(undefined);
      setToasts((current) => [...current, {
        id: toastKey(),
        title: t("Transaction deleted"),
        description: t("Its entries no longer affect balances."),
        tone: "positive",
      }]);
      await invalidateFinancialViews();
    },
  });

  async function invalidateFinancialViews() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: transactionsQueryKey(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: accountsQueryKey(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: spendingAnalysisQueryPrefix(workspace.id) }),
    ]);
  }

  function resetDraft() {
    setEditing(undefined);
    setKind("standard");
    setStatus("posted");
    setTransactionDate(today());
    setPayee("");
    setDescription("");
    setNotes("");
    setEntries([emptyEntry(accounts.data?.[0]?.id)]);
    setAllocations([]);
    setValidation("");
    setEditorMode("simple");
    setDetailedOrigin(undefined);
    setDraft(emptyCaptureDraft(
      today(),
      suggestedAccountId(transactions.data ?? [], accounts.data ?? []),
    ));
    save.reset();
  }

  function startCreate() {
    resetDraft();
    setEditorOpen(true);
  }

  function closeEditor() {
    setEditorOpen(false);
    resetDraft();
  }

  function edit(value: LedgerTransaction) {
    setEditing(value);
    setKind(value.kind);
    setStatus(value.status);
    setTransactionDate(value.transaction_date);
    setPayee(value.payee ?? "");
    setDescription(value.description ?? "");
    setNotes(value.notes ?? "");
    setEntries(value.entries.map((entry) => ({
      key: draftKey(),
      accountId: entry.account_id,
      amount: majorAmountInput(entry.amount_minor),
      baseAmount: accountCurrency(accounts.data ?? [], entry.account_id) === workspace.base_currency
        ? ""
        : majorAmountInput(entry.base_amount_minor),
    })));
    setAllocations(value.allocations.map((allocation) => ({
      key: draftKey(),
      categoryId: allocation.category_id,
      amount: majorAmountInput(allocation.amount_base_minor),
    })));
    setValidation("");
    // The simple form only opens on a transaction it can reproduce exactly; everything else —
    // a split, an adjustment, several account entries — goes straight to the detailed editor
    // rather than being flattened into a shape it never had.
    const captured = captureDraft(value, captureContext);
    setDraft(captured ?? emptyCaptureDraft(value.transaction_date, value.entries[0]?.account_id));
    setDetailedOrigin(undefined);
    setEditorMode(captured ? "simple" : "detailed");
    save.reset();
    setEditorOpen(true);
  }

  function detailedFieldsFrom(request: TransactionWriteRequest): DetailedFields {
    return {
      kind: request.kind,
      status: request.status,
      transactionDate: request.transaction_date,
      payee: request.payee ?? "",
      description: request.description ?? "",
      notes: request.notes ?? "",
      entries: request.entries.map((entry) => ({
        key: draftKey(),
        accountId: entry.account_id,
        amount: majorAmountInput(entry.amount_minor),
        baseAmount: entry.base_amount_minor === undefined ? "" : majorAmountInput(entry.base_amount_minor),
      })),
      allocations: (request.allocations ?? []).map((allocation) => ({
        key: draftKey(),
        categoryId: allocation.category_id,
        amount: allocation.amount_base_minor === undefined
          ? ""
          : majorAmountInput(allocation.amount_base_minor),
      })),
    };
  }

  function applyDetailedFields(fields: DetailedFields) {
    setKind(fields.kind);
    setStatus(fields.status);
    setTransactionDate(fields.transactionDate);
    setPayee(fields.payee);
    setDescription(fields.description);
    setNotes(fields.notes);
    setEntries(fields.entries);
    setAllocations(fields.allocations);
  }

  function openDetailedEditor() {
    const result = captureRequest(draft, captureContext);
    const fields = "request" in result ? detailedFieldsFrom(result.request) : {
      // An incomplete draft still carries the answers already given into the wider form.
      kind: draft.type === "transfer" ? ("transfer" as const) : ("standard" as const),
      status: draft.pending ? ("pending" as const) : ("posted" as const),
      transactionDate: draft.transactionDate,
      payee: draft.payee,
      description: draft.description,
      notes: draft.notes,
      entries: [emptyEntry(draft.accountId), ...(draft.type === "transfer" ? [emptyEntry(draft.toAccountId)] : [])],
      allocations: draft.categoryId ? [emptyAllocation(draft.categoryId)] : [],
    };
    applyDetailedFields(fields);
    // Remembering what the detailed editor opened with is what lets an untouched visit go
    // straight back. An empty draft has no transaction to rebuild from, so without this the
    // return trip would be refused for work nobody has done yet.
    setDetailedOrigin(detailedSignature(fields));
    setValidation("");
    setEditorMode("detailed");
  }

  /** The detailed fields as a transaction, so the simple form can say whether it can show them. */
  function draftFromDetailed(): CaptureDraft | undefined {
    const built: LedgerTransaction["entries"] = [];
    for (const entry of entries) {
      const amount = parseMajorAmount(entry.amount);
      const currency = accountCurrency(accounts.data ?? [], entry.accountId);
      if (amount === null || amount === 0 || !currency) return undefined;
      const stated = currency === workspace.base_currency ? amount : parseMajorAmount(entry.baseAmount);
      // An amount left to the transaction date's rate is unknown here, and the simple form
      // carries it as an empty field either way. Standing the entry amount in its place keeps
      // the shape checks below — sign, count, reconciliation — reading what they would read if
      // the rate were known; the draft still takes its text from the fields themselves.
      const base = entry.baseAmount.trim() || currency === workspace.base_currency ? stated : amount;
      if (base === null || base === 0) return undefined;
      built.push({ account_id: entry.accountId, amount_minor: amount, base_amount_minor: base });
    }
    const builtAllocations: LedgerTransaction["allocations"] = [];
    for (const allocation of allocations) {
      const derived = allocations.length === 1 && !allocation.amount.trim();
      const amount = derived
        ? built.reduce((total, entry) => total + entry.base_amount_minor, 0)
        : parseMajorAmount(allocation.amount);
      if (!allocation.categoryId || amount === null) return undefined;
      builtAllocations.push({ category_id: allocation.categoryId, amount_base_minor: amount });
    }
    const candidate = captureDraft({
      kind,
      status,
      transaction_date: transactionDate,
      ...(payee.trim() ? { payee: payee.trim() } : {}),
      ...(description.trim() ? { description: description.trim() } : {}),
      ...(notes.trim() ? { notes: notes.trim() } : {}),
      entries: built,
      allocations: builtAllocations,
    }, captureContext);
    return candidate && { ...candidate, baseAmount: entries[0]?.baseAmount.trim() ?? "" };
  }

  function openSimpleEditor() {
    const untouched = detailedOrigin !== undefined && detailedOrigin === detailedSignature({
      kind, status, transactionDate, payee, description, notes, entries, allocations,
    });
    if (untouched) {
      setValidation("");
      setEditorMode("simple");
      return;
    }
    const candidate = draftFromDetailed();
    if (!candidate) {
      setValidation(t("This transaction needs the detailed editor. The simple form cannot show splits, several account entries, or a balance adjustment."));
      return;
    }
    setDraft(candidate);
    setValidation("");
    setEditorMode("simple");
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    setValidation("");
    if (editorMode === "simple") {
      const result = captureRequest(draft, captureContext);
      if ("error" in result) {
        setValidation(result.error);
        return;
      }
      save.mutate(result.request);
      return;
    }
    if (entries.length === 0 || (kind === "transfer" && entries.length < 2)) {
      setValidation(kind === "transfer" ? t("A transfer needs at least two entries.") : t("Add an entry."));
      return;
    }
    const parsedEntries: TransactionWriteRequest["entries"] = [];
    for (const entry of entries) {
      const amount = parseMajorAmount(entry.amount);
      const baseAmount = entry.baseAmount.trim() ? parseMajorAmount(entry.baseAmount) : undefined;
      if (!entry.accountId || amount === null || amount === 0
          || (entry.baseAmount.trim() && baseAmount === null)
          || (baseAmount !== undefined && baseAmount !== null && baseAmount !== 0
            && Math.sign(baseAmount) !== Math.sign(amount))) {
        setValidation(
          t("Every entry needs an account and a non-zero amount with at most two decimals; a non-zero manual base amount must use the same sign."),
        );
        return;
      }
      const parsedEntry: TransactionWriteRequest["entries"][number] = {
        account_id: entry.accountId,
        amount_minor: amount,
      };
      if (baseAmount !== undefined && baseAmount !== null) parsedEntry.base_amount_minor = baseAmount;
      parsedEntries.push(parsedEntry);
    }
    const parsedAllocations: NonNullable<TransactionWriteRequest["allocations"]> = [];
    for (const allocation of allocations) {
      // A single allocation may leave its amount to the server, which takes the entry total —
      // the same allowance the entry's own base amount already has. A split cannot: dividing a
      // transaction between categories is a decision only this form can make.
      const derived = allocations.length === 1 && !allocation.amount.trim();
      const amount = derived ? undefined : parseMajorAmount(allocation.amount);
      if (!allocation.categoryId || (!derived && (amount === null || amount === 0))) {
        setValidation(allocations.length === 1
          ? t("Every allocation needs a category, and a non-zero base-currency amount unless it is left to the transaction date's rate.")
          : t("Every allocation needs a category and a non-zero base-currency amount."));
        return;
      }
      parsedAllocations.push({
        category_id: allocation.categoryId,
        ...(amount === undefined || amount === null ? {} : { amount_base_minor: amount }),
      });
    }
    save.mutate({
      kind,
      status,
      transaction_date: transactionDate,
      ...(payee.trim() ? { payee: payee.trim() } : {}),
      ...(description.trim() ? { description: description.trim() } : {}),
      ...(notes.trim() ? { notes: notes.trim() } : {}),
      entries: parsedEntries,
      allocations: kind === "transfer" ? [] : parsedAllocations,
    });
  }

  function clearFilters() {
    setSearch("");
    setKindFilter("all");
    setStatusFilter("all");
  }

  const activeFilterCount = (kindFilter === "all" ? 0 : 1) + (statusFilter === "all" ? 0 : 1);
  const dependenciesUnavailable = accounts.isError || categories.isError;

  return (
    <section className="transactions-workspace" aria-labelledby="transaction-register-heading">
      <div className="transaction-toolbar">
        <label className="transaction-search">
          <span className="visually-hidden">{t("Search transactions")}</span>
          <AppIcon name="transactions" />
          <input
            placeholder={t("Search payee, account, category…")}
            type="search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </label>
        <details className="transaction-filter">
          <summary>
            <AppIcon name="filter" />
            {activeFilterCount > 0 ? t("Filters ({count})", { count: activeFilterCount }) : t("Filters")}
          </summary>
          <div className="transaction-filter-panel">
            <label>
              {t("Transaction kind")}
              <select value={kindFilter} onChange={(event) => setKindFilter(event.target.value as KindFilter)}>
                <option value="all">{t("All types")}</option>
                <option value="standard">{t("Expense & income")}</option>
                <option value="transfer">{t("Transfers")}</option>
                <option value="adjustment">{t("Adjustments")}</option>
              </select>
            </label>
            <label>
              {t("Transaction status")}
              <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as StatusFilter)}>
                <option value="all">{t("All statuses")}</option>
                <option value="posted">{t("Posted")}</option>
                <option value="pending">{t("Pending")}</option>
              </select>
            </label>
            {activeFilterCount > 0 ? (
              <button className="text-button" onClick={clearFilters} type="button">{t("Clear filters")}</button>
            ) : null}
          </div>
        </details>
        {canManage ? (
          <button disabled={accounts.data?.length === 0} onClick={startCreate} type="button">
            {t("Add transaction")}
          </button>
        ) : null}
      </div>

      <div className="transaction-register-heading">
        <div>
          <p className="eyebrow">{t("Ledger register")}</p>
          <h2 id="transaction-register-heading">{t("Activity")}</h2>
        </div>
        <span>{t("{shown} of {total}", { shown: filteredTransactions.length, total: transactions.data?.length ?? 0 })}</span>
      </div>

      {canManage && accounts.data?.length === 0 && !accounts.isPending ? (
        <InlineNotice
          action={<Link to={`/workspaces/${workspace.id}/accounts`}>{t("Add account")}</Link>}
          title={t("An account is required")}
          tone="warning"
        >{t("Create an account before recording a transaction.")}</InlineNotice>
      ) : null}
      {!canManage ? (
        <InlineNotice title={t("Read-only access")}>{t("You can review and filter activity, but only workspace managers can change it.")}</InlineNotice>
      ) : null}
      {transactions.isPending ? <LoadingState label={t("Loading transactions")} rows={5} /> : null}
      {transactions.error ? <InlineNotice tone="danger">{transactions.error.message}</InlineNotice> : null}
      {!transactions.isPending && !transactions.error && transactions.data?.length === 0 ? (
        <EmptyState
          action={canManage && accounts.data?.length ? <button onClick={startCreate} type="button">{t("Add transaction")}</button> : undefined}
          description={t("Expenses, income, transfers, and adjustments will appear here.")}
          icon="transactions"
          title={t("No transactions yet")}
        />
      ) : null}
      {Boolean(transactions.data?.length) && filteredTransactions.length === 0 ? (
        <EmptyState
          action={<button className="secondary-button" onClick={clearFilters} type="button">{t("Clear filters")}</button>}
          description={t("Try a different search term, type, or status.")}
          icon="transactions"
          title={t("No matching transactions")}
        />
      ) : null}

      <div className="transaction-register">
        {transactionDayGroups(filteredTransactions).map((group) => (
          <section className="transaction-day" key={group.date}>
            <h3 className="transaction-day-heading">
              <span>{formatTransactionDate(group.date)}</span>
              {group.total === 0 ? null : (
                <MoneyAmount amount={group.total} currency={workspace.base_currency} signed />
              )}
            </h3>
            {group.transactions.map((value) => (
              <TransactionRegisterRow
                accountNames={accountNames}
                canManage={canManage}
                categoryNames={categoryNames}
                categoryById={categoryById}
                key={value.id}
                onDelete={() => setDeleteTarget(value)}
                onEdit={() => edit(value)}
                showAccount={(accounts.data?.length ?? 0) > 1}
                transaction={value}
                workspace={workspace}
              />
            ))}
          </section>
        ))}
      </div>
      {remove.error ? <InlineNotice tone="danger">{mutationMessage(remove.error)}</InlineNotice> : null}

      <ModalDialog
        description={editorMode === "simple"
          ? t("Enter what you spent or received. Budget records the account and category effects for you.")
          : t("Entries affect account balances; allocations affect spending, income, and budget reporting.")}
        footer={(
          <>
            <button className="secondary-button" onClick={closeEditor} type="button">{t("Cancel")}</button>
            <button
              disabled={save.isPending || accounts.data?.length === 0}
              form="transaction-editor-form"
              type="submit"
            >
              {save.isPending ? t("Saving…") : editing ? t("Save transaction") : t("Add transaction")}
            </button>
          </>
        )}
        onClose={closeEditor}
        open={editorOpen}
        placement="drawer"
        title={editing ? t("Edit transaction") : t("Add transaction")}
      >
        <form className="transaction-editor-form" id="transaction-editor-form" onSubmit={submit}>
          {dependenciesUnavailable ? (
            <InlineNotice tone="danger">{t("Accounts or categories could not be loaded for this editor.")}</InlineNotice>
          ) : null}
          {editorMode === "simple" ? (
            <TransactionCaptureForm
              accounts={accounts.data ?? []}
              categories={categories.data ?? []}
              draft={draft}
              frequentCategories={frequentCategories}
              onChange={(patch) => setDraft((current) => ({ ...current, ...patch }))}
              onDetailed={openDetailedEditor}
              workspace={workspace}
            />
          ) : (
            <>
            <div className="form-columns transaction-basics">
              <label>
                {t("Kind")}
                <select
                  value={kind}
                  onChange={(event) => {
                    const next = event.target.value as typeof kind;
                    setKind(next);
                    if (next === "transfer") {
                      setAllocations([]);
                      setEntries((current) => current.length >= 2
                        ? current
                        : [...current, emptyEntry(accounts.data?.[1]?.id ?? accounts.data?.[0]?.id)]);
                    }
                  }}
                >
                  <option value="standard">{t("Expense or income")}</option>
                  <option value="transfer">{t("Transfer")}</option>
                  <option value="adjustment">{t("Balance adjustment")}</option>
                </select>
              </label>
              <label>
                {t("Status")}
                <select value={status} onChange={(event) => setStatus(event.target.value as typeof status)}>
                  <option value="posted">{t("Posted")}</option>
                  <option value="pending">{t("Pending")}</option>
                </select>
              </label>
              <label>
                {t("Date")}
                <input required type="date" value={transactionDate} onChange={(event) => setTransactionDate(event.target.value)} />
              </label>
              <label>
                {t("Payee (optional)")}
                <input maxLength={200} value={payee} onChange={(event) => setPayee(event.target.value)} />
              </label>
            </div>
            <label>
              {t("Description (optional)")}
              <input maxLength={500} value={description} onChange={(event) => setDescription(event.target.value)} />
            </label>

            <fieldset className="transaction-lines">
              <legend>{t("Account entries")}</legend>
              <p>{t("Use negative amounts for money leaving an account and positive amounts for money entering it.")}</p>
              {entries.map((entry, index) => {
                const currency = accountCurrency(accounts.data ?? [], entry.accountId);
                return (
                  <div className="transaction-line" key={entry.key}>
                    <label>
                      {t("Account")}
                      <select
                        required
                        value={entry.accountId}
                        onChange={(event) => setEntries((current) => current.map((item) =>
                          item.key === entry.key ? { ...item, accountId: event.target.value, baseAmount: "" } : item,
                        ))}
                      >
                        <option value="">{t("Choose an account")}</option>
                        {accounts.data?.map((account) => (
                          <option key={account.id} value={account.id}>{account.name} · {account.currency}</option>
                        ))}
                      </select>
                    </label>
                    <label>
                      {t("Amount")} {currency ? `(${currency})` : ""}
                      <input
                        inputMode="decimal"
                        placeholder="-12.50"
                        required
                        value={entry.amount}
                        onChange={(event) => setEntries((current) => current.map((item) =>
                          item.key === entry.key ? { ...item, amount: event.target.value } : item,
                        ))}
                      />
                    </label>
                    {currency && currency !== workspace.base_currency ? (
                      <label>
                        {t("Base amount ({currency}, optional)", { currency: workspace.base_currency })}
                        <input
                          inputMode="decimal"
                          placeholder={t("Auto historical rate")}
                          value={entry.baseAmount}
                          onChange={(event) => setEntries((current) => current.map((item) =>
                            item.key === entry.key ? { ...item, baseAmount: event.target.value } : item,
                          ))}
                        />
                      </label>
                    ) : null}
                    {entries.length > 1 ? (
                      <button className="text-button danger" onClick={() => setEntries((current) => current.filter((item) => item.key !== entry.key))} type="button">
                        {t("Remove entry {number}", { number: index + 1 })}
                      </button>
                    ) : null}
                  </div>
                );
              })}
              <button className="secondary-button line-button" onClick={() => setEntries((current) => [...current, emptyEntry(accounts.data?.[0]?.id)])} type="button">
                {t("Add account entry")}
              </button>
            </fieldset>

            {kind !== "transfer" ? (
              <fieldset className="transaction-lines">
                <legend>{t("Category allocations")}</legend>
                <p>{t("Amounts are in {currency}. Leave the whole section empty on a standard transaction to use Uncategorized automatically, or leave a single allocation's amount empty to book it at the transaction date's rate.", { currency: workspace.base_currency })}</p>
                {allocations.map((allocation, index) => (
                  <div className="transaction-line" key={allocation.key}>
                    <label>
                      {t("Category")}
                      <select
                        required
                        value={allocation.categoryId}
                        onChange={(event) => setAllocations((current) => current.map((item) =>
                          item.key === allocation.key ? { ...item, categoryId: event.target.value } : item,
                        ))}
                      >
                        <option value="">{t("Choose a category")}</option>
                        {(["expense", "income"] as const).map((categoryKind) => (
                          <optgroup key={categoryKind} label={t(`transactions.${categoryKind}Categories`)}>
                            {categories.data?.filter((category) => category.kind === categoryKind).map((category) => (
                              <option key={category.id} value={category.id}>{categoryName(category)}</option>
                            ))}
                          </optgroup>
                        ))}
                      </select>
                      {categoryById.get(allocation.categoryId) ? <CategoryLabel {...categoryAppearance(categoryById.get(allocation.categoryId)!)} /> : null}
                    </label>
                    <label>
                      {t("Base amount ({currency})", { currency: workspace.base_currency })}
                      <input
                        inputMode="decimal"
                        placeholder={allocations.length === 1 ? t("Rate for that date") : "-12.50"}
                        required={allocations.length > 1}
                        value={allocation.amount}
                        onChange={(event) => setAllocations((current) => current.map((item) =>
                          item.key === allocation.key ? { ...item, amount: event.target.value } : item,
                        ))}
                      />
                    </label>
                    <button className="text-button danger" onClick={() => setAllocations((current) => current.filter((item) => item.key !== allocation.key))} type="button">
                      {t("Remove allocation {number}", { number: index + 1 })}
                    </button>
                  </div>
                ))}
                <button className="secondary-button line-button" onClick={() => setAllocations((current) => [...current, emptyAllocation(categories.data?.[0]?.id)])} type="button">
                  {t("Add category allocation")}
                </button>
              </fieldset>
            ) : null}
            <label>
              {t("Notes (optional)")}
              <input maxLength={4000} value={notes} onChange={(event) => setNotes(event.target.value)} />
            </label>
              <button className="text-button" onClick={openSimpleEditor} type="button">
                {t("Back to the simple form")}
              </button>
            </>
          )}
          {validation ? <InlineNotice tone="danger">{validation}</InlineNotice> : null}
          {save.error ? <InlineNotice tone="danger">{mutationMessage(save.error)}</InlineNotice> : null}
        </form>
      </ModalDialog>

      <ModalDialog
        description={t("This is a soft deletion. The transaction remains recoverable in storage, but stops affecting balances and reports.")}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setDeleteTarget(undefined)} type="button">{t("Cancel")}</button>
            <button
              className="danger-button"
              disabled={remove.isPending}
              onClick={() => deleteTarget && remove.mutate(deleteTarget.id)}
              type="button"
            >{remove.isPending ? t("Deleting…") : t("Delete transaction")}</button>
          </>
        )}
        onClose={() => setDeleteTarget(undefined)}
        open={Boolean(deleteTarget)}
        title={t("Delete this transaction?")}
      >
        <p>{t("{transaction} will stop affecting account balances, budgets, and reports.", { transaction: deleteTarget ? transactionTitle(deleteTarget) : t("Transaction") })}</p>
      </ModalDialog>

      <ToastRegion
        messages={toasts}
        onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))}
      />
    </section>
  );
}

function TransactionRegisterRow({
  accountNames,
  canManage,
  categoryNames,
  categoryById,
  onDelete,
  onEdit,
  showAccount,
  transaction,
  workspace,
}: {
  accountNames: Map<string, string>;
  canManage: boolean;
  categoryNames: Map<string, string>;
  categoryById: Map<string, import("../../api/client").Category>;
  onDelete: () => void;
  onEdit: () => void;
  showAccount: boolean;
  transaction: LedgerTransaction;
  workspace: Workspace;
}) {
  const total = transactionTotal(transaction);
  const categories = transaction.allocations
    .map((allocation) => categoryById.get(allocation.category_id))
    .filter((category) => category !== undefined);
  const direction = transactionDirection(transaction, total);
  return (
    <article className="transaction-register-row">
      {categories[0] ? (
        <CategoryAppearance
          colorKey={categories[0].color_key}
          iconType={categories[0].icon_type}
          iconValue={categories[0].icon_value ?? categories[0].icon}
        />
      ) : (
        <span aria-hidden="true" className={`transaction-direction transaction-direction-${direction}`}>
          <AppIcon name={transaction.kind === "transfer" ? "accounts" : "transactions"} />
        </span>
      )}
      <div className="transaction-register-copy">
        <div>
          <strong>{transactionTitle(transaction, categoryNames)}</strong>
          <span className="transaction-register-badges">
            {transaction.status === "pending" ? <StatusBadge tone="warning">{t("Pending")}</StatusBadge> : null}
            {transaction.kind !== "standard" ? <StatusBadge>{transactionKindLabel(transaction.kind)}</StatusBadge> : null}
          </span>
        </div>
        <small>{transactionSupportingLine(transaction, { accountNames, categoryNames, showAccount })}</small>
      </div>
      <div className="transaction-register-amount">
        {transaction.kind === "transfer" ? <span>{t("Transfer")}</span> : total === null ? <span>{t("Amount unavailable")}</span> : (
          <MoneyAmount amount={total} currency={workspace.base_currency} signed />
        )}
      </div>
      {canManage ? (
        <div className="transaction-register-actions">
          <button className="text-button" onClick={onEdit} type="button">{t("Edit")}</button>
          <button className="text-button danger" onClick={onDelete} type="button">{t("Delete")}</button>
        </div>
      ) : null}
    </article>
  );
}

function accountCurrency(accounts: Account[], accountId: string) {
  return accounts.find((account) => account.id === accountId)?.currency;
}

function categoryAppearance(category: import("../../api/client").Category) {
  return { colorKey: category.color_key, iconType: category.icon_type, iconValue: category.icon_value ?? category.icon, name: categoryName(category) };
}

function transactionTitle(transaction: LedgerTransaction, categoryNames?: Map<string, string>) {
  return transaction.payee
    ?? transaction.description
    ?? (categoryNames && transaction.allocations[0]
      ? categoryNames.get(transaction.allocations[0].category_id)
      : undefined)
    ?? transactionKindLabel(transaction.kind);
}

/**
 * One supporting line rather than three. The date already heads the day's group, so the line
 * carries the category — or the account when the category is already the title. This mirrors
 * the iOS register so a row reads the same on either client.
 */
function transactionSupportingLine(
  transaction: LedgerTransaction,
  { accountNames, categoryNames, showAccount }: {
    accountNames: Map<string, string>;
    categoryNames: Map<string, string>;
    showAccount: boolean;
  },
) {
  const parts: string[] = [];
  const named = transaction.payee ?? transaction.description;
  const categoryTitle = transaction.allocations[0]
    ? categoryNames.get(transaction.allocations[0].category_id) ?? t("Unavailable category")
    : undefined;
  const accounts = transaction.entries
    .map((entry) => accountNames.get(entry.account_id) ?? t("Unavailable account"))
    .join(" → ");
  if (named && categoryTitle) {
    parts.push(categoryTitle);
    if (showAccount && accounts) parts.push(accounts);
  } else if (accounts) {
    parts.push(accounts);
  }
  if (parts.length === 0) parts.push(transactionKindLabel(transaction.kind));
  if (transaction.allocations.length > 1) parts.push(`+${transaction.allocations.length - 1}`);
  return parts.join(" · ");
}

/**
 * The register in the order the ledger returned it, cut into days. A day's reading excludes
 * transfers for the same reason every other total does: moving your own money is not spending.
 */
function transactionDayGroups(transactions: LedgerTransaction[]) {
  const groups: { date: string; total: number; transactions: LedgerTransaction[] }[] = [];
  const byDate = new Map<string, { date: string; total: number; transactions: LedgerTransaction[] }>();
  for (const transaction of transactions) {
    let group = byDate.get(transaction.transaction_date);
    if (!group) {
      group = { date: transaction.transaction_date, total: 0, transactions: [] };
      byDate.set(transaction.transaction_date, group);
      groups.push(group);
    }
    group.transactions.push(transaction);
    if (transaction.kind === "transfer") continue;
    const next = group.total + (transactionTotal(transaction) ?? 0);
    if (Number.isSafeInteger(next)) group.total = next;
  }
  return groups;
}

function transactionKindLabel(kind: LedgerTransaction["kind"]) {
  if (kind === "standard") return t("Expense or income");
  if (kind === "transfer") return t("Transfer");
  return t("Balance adjustment");
}

function transactionTotal(transaction: LedgerTransaction): number | null {
  let total = 0;
  for (const entry of transaction.entries) {
    const next = total + entry.base_amount_minor;
    if (!Number.isSafeInteger(next)) return null;
    total = next;
  }
  return total;
}

function transactionDirection(transaction: LedgerTransaction, total: number | null) {
  if (transaction.kind === "transfer") return "transfer";
  if (total === null || total === 0) return "neutral";
  return total > 0 ? "income" : "expense";
}

function formatTransactionDate(value: string) {
  const [year, month, day] = value.split("-").map(Number);
  if (!year || !month || !day) return value;
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric" })
    .format(new Date(Date.UTC(year, month - 1, day)));
}

function mutationMessage(error: Error) {
  return error instanceof APIError ? error.message : t("The change could not be saved.");
}
