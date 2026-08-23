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
  transactionsQueryKey,
  updateTransaction,
} from "../../api/client";
import { AppIcon } from "../../components/ExperiencePrimitives";
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

type Workspace = SessionResponse["workspaces"][number];
type EntryDraft = { key: string; accountId: string; amount: string; baseAmount: string };
type AllocationDraft = { key: string; categoryId: string; amount: string };
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

  const accountNames = useMemo(
    () => new Map((accounts.data ?? []).map((account) => [account.id, account.name])),
    [accounts.data],
  );
  const categoryNames = useMemo(
    () => new Map((categories.data ?? []).map((category) => [category.id, category.name])),
    [categories.data],
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
      const title = editing ? "Transaction updated" : "Transaction added";
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
        title: "Transaction deleted",
        description: "Its entries no longer affect balances.",
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
    save.reset();
    setEditorOpen(true);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    setValidation("");
    if (entries.length === 0 || (kind === "transfer" && entries.length < 2)) {
      setValidation(kind === "transfer" ? "A transfer needs at least two entries." : "Add an entry.");
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
          "Every entry needs an account and a non-zero amount with at most two decimals; "
          + "a non-zero manual base amount must use the same sign.",
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
      const amount = parseMajorAmount(allocation.amount);
      if (!allocation.categoryId || amount === null || amount === 0) {
        setValidation("Every allocation needs a category and a non-zero base-currency amount.");
        return;
      }
      parsedAllocations.push({ category_id: allocation.categoryId, amount_base_minor: amount });
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

  const dependenciesUnavailable = accounts.isError || categories.isError;

  return (
    <section className="transactions-workspace" aria-labelledby="transaction-register-heading">
      <div className="transaction-toolbar">
        <label className="transaction-search">
          <span className="visually-hidden">Search transactions</span>
          <AppIcon name="transactions" />
          <input
            placeholder="Search payee, account, category…"
            type="search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </label>
        <label>
          <span className="visually-hidden">Transaction kind</span>
          <select aria-label="Transaction kind" value={kindFilter} onChange={(event) => setKindFilter(event.target.value as KindFilter)}>
            <option value="all">All types</option>
            <option value="standard">Expense & income</option>
            <option value="transfer">Transfers</option>
            <option value="adjustment">Adjustments</option>
          </select>
        </label>
        <label>
          <span className="visually-hidden">Transaction status</span>
          <select aria-label="Transaction status" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as StatusFilter)}>
            <option value="all">All statuses</option>
            <option value="posted">Posted</option>
            <option value="pending">Pending</option>
          </select>
        </label>
        {canManage ? (
          <button disabled={accounts.data?.length === 0} onClick={startCreate} type="button">
            Add transaction
          </button>
        ) : null}
      </div>

      <div className="transaction-register-heading">
        <div>
          <p className="eyebrow">Ledger register</p>
          <h2 id="transaction-register-heading">Activity</h2>
        </div>
        <span>{filteredTransactions.length} of {transactions.data?.length ?? 0}</span>
      </div>

      {canManage && accounts.data?.length === 0 && !accounts.isPending ? (
        <InlineNotice
          action={<Link to={`/workspaces/${workspace.id}/accounts`}>Add account</Link>}
          title="An account is required"
          tone="warning"
        >Create an account before recording a transaction.</InlineNotice>
      ) : null}
      {!canManage ? (
        <InlineNotice title="Read-only access">You can review and filter activity, but only workspace managers can change it.</InlineNotice>
      ) : null}
      {transactions.isPending ? <LoadingState label="Loading transactions" rows={5} /> : null}
      {transactions.error ? <InlineNotice tone="danger">{transactions.error.message}</InlineNotice> : null}
      {!transactions.isPending && !transactions.error && transactions.data?.length === 0 ? (
        <EmptyState
          action={canManage && accounts.data?.length ? <button onClick={startCreate} type="button">Add transaction</button> : undefined}
          description="Expenses, income, transfers, and adjustments will appear here."
          icon="transactions"
          title="No transactions yet"
        />
      ) : null}
      {Boolean(transactions.data?.length) && filteredTransactions.length === 0 ? (
        <EmptyState
          action={<button className="secondary-button" onClick={clearFilters} type="button">Clear filters</button>}
          description="Try a different search term, type, or status."
          icon="transactions"
          title="No matching transactions"
        />
      ) : null}

      <div className="transaction-register">
        {filteredTransactions.map((value) => (
          <TransactionRegisterRow
            accountNames={accountNames}
            canManage={canManage}
            categoryNames={categoryNames}
            key={value.id}
            onDelete={() => setDeleteTarget(value)}
            onEdit={() => edit(value)}
            transaction={value}
            workspace={workspace}
          />
        ))}
      </div>
      {remove.error ? <InlineNotice tone="danger">{mutationMessage(remove.error)}</InlineNotice> : null}

      <ModalDialog
        description="Entries affect account balances; allocations affect spending, income, and budget reporting."
        footer={(
          <>
            <button className="secondary-button" onClick={closeEditor} type="button">Cancel</button>
            <button
              disabled={save.isPending || accounts.data?.length === 0}
              form="transaction-editor-form"
              type="submit"
            >
              {save.isPending ? "Saving…" : editing ? "Save transaction" : "Add transaction"}
            </button>
          </>
        )}
        onClose={closeEditor}
        open={editorOpen}
        placement="drawer"
        title={editing ? "Edit transaction" : "Add transaction"}
      >
        <form className="transaction-editor-form" id="transaction-editor-form" onSubmit={submit}>
          {dependenciesUnavailable ? (
            <InlineNotice tone="danger">Accounts or categories could not be loaded for this editor.</InlineNotice>
          ) : null}
          <div className="form-columns transaction-basics">
            <label>
              Kind
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
                <option value="standard">Expense or income</option>
                <option value="transfer">Transfer</option>
                <option value="adjustment">Balance adjustment</option>
              </select>
            </label>
            <label>
              Status
              <select value={status} onChange={(event) => setStatus(event.target.value as typeof status)}>
                <option value="posted">Posted</option>
                <option value="pending">Pending</option>
              </select>
            </label>
            <label>
              Date
              <input required type="date" value={transactionDate} onChange={(event) => setTransactionDate(event.target.value)} />
            </label>
            <label>
              Payee (optional)
              <input maxLength={200} value={payee} onChange={(event) => setPayee(event.target.value)} />
            </label>
          </div>
          <label>
            Description (optional)
            <input maxLength={500} value={description} onChange={(event) => setDescription(event.target.value)} />
          </label>

          <fieldset className="transaction-lines">
            <legend>Account entries</legend>
            <p>Use negative amounts for money leaving an account and positive amounts for money entering it.</p>
            {entries.map((entry, index) => {
              const currency = accountCurrency(accounts.data ?? [], entry.accountId);
              return (
                <div className="transaction-line" key={entry.key}>
                  <label>
                    Account
                    <select
                      required
                      value={entry.accountId}
                      onChange={(event) => setEntries((current) => current.map((item) =>
                        item.key === entry.key ? { ...item, accountId: event.target.value, baseAmount: "" } : item,
                      ))}
                    >
                      <option value="">Choose an account</option>
                      {accounts.data?.map((account) => (
                        <option key={account.id} value={account.id}>{account.name} · {account.currency}</option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Amount {currency ? `(${currency})` : ""}
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
                      Base amount ({workspace.base_currency}, optional)
                      <input
                        inputMode="decimal"
                        placeholder="Auto historical rate"
                        value={entry.baseAmount}
                        onChange={(event) => setEntries((current) => current.map((item) =>
                          item.key === entry.key ? { ...item, baseAmount: event.target.value } : item,
                        ))}
                      />
                    </label>
                  ) : null}
                  {entries.length > 1 ? (
                    <button className="text-button danger" onClick={() => setEntries((current) => current.filter((item) => item.key !== entry.key))} type="button">
                      Remove entry {index + 1}
                    </button>
                  ) : null}
                </div>
              );
            })}
            <button className="secondary-button line-button" onClick={() => setEntries((current) => [...current, emptyEntry(accounts.data?.[0]?.id)])} type="button">
              Add account entry
            </button>
          </fieldset>

          {kind !== "transfer" ? (
            <fieldset className="transaction-lines">
              <legend>Category allocations</legend>
              <p>Amounts are in {workspace.base_currency}. Leave empty on a standard transaction to use Uncategorized automatically.</p>
              {allocations.map((allocation, index) => (
                <div className="transaction-line" key={allocation.key}>
                  <label>
                    Category
                    <select
                      required
                      value={allocation.categoryId}
                      onChange={(event) => setAllocations((current) => current.map((item) =>
                        item.key === allocation.key ? { ...item, categoryId: event.target.value } : item,
                      ))}
                    >
                      <option value="">Choose a category</option>
                      {categories.data?.map((category) => (
                        <option key={category.id} value={category.id}>{category.name} · {category.kind}</option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Base amount ({workspace.base_currency})
                    <input
                      inputMode="decimal"
                      placeholder="-12.50"
                      required
                      value={allocation.amount}
                      onChange={(event) => setAllocations((current) => current.map((item) =>
                        item.key === allocation.key ? { ...item, amount: event.target.value } : item,
                      ))}
                    />
                  </label>
                  <button className="text-button danger" onClick={() => setAllocations((current) => current.filter((item) => item.key !== allocation.key))} type="button">
                    Remove allocation {index + 1}
                  </button>
                </div>
              ))}
              <button className="secondary-button line-button" onClick={() => setAllocations((current) => [...current, emptyAllocation(categories.data?.[0]?.id)])} type="button">
                Add category allocation
              </button>
            </fieldset>
          ) : null}
          <label>
            Notes (optional)
            <input maxLength={4000} value={notes} onChange={(event) => setNotes(event.target.value)} />
          </label>
          {validation ? <InlineNotice tone="danger">{validation}</InlineNotice> : null}
          {save.error ? <InlineNotice tone="danger">{mutationMessage(save.error)}</InlineNotice> : null}
        </form>
      </ModalDialog>

      <ModalDialog
        description="This is a soft deletion. The transaction remains recoverable in storage, but stops affecting balances and reports."
        footer={(
          <>
            <button className="secondary-button" onClick={() => setDeleteTarget(undefined)} type="button">Cancel</button>
            <button
              className="danger-button"
              disabled={remove.isPending}
              onClick={() => deleteTarget && remove.mutate(deleteTarget.id)}
              type="button"
            >{remove.isPending ? "Deleting…" : "Delete transaction"}</button>
          </>
        )}
        onClose={() => setDeleteTarget(undefined)}
        open={Boolean(deleteTarget)}
        title="Delete this transaction?"
      >
        <p><strong>{deleteTarget ? transactionTitle(deleteTarget) : "Transaction"}</strong> will stop affecting account balances, budgets, and reports.</p>
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
  onDelete,
  onEdit,
  transaction,
  workspace,
}: {
  accountNames: Map<string, string>;
  canManage: boolean;
  categoryNames: Map<string, string>;
  onDelete: () => void;
  onEdit: () => void;
  transaction: LedgerTransaction;
  workspace: Workspace;
}) {
  const total = transactionTotal(transaction);
  const accounts = transaction.entries
    .map((entry) => accountNames.get(entry.account_id) ?? "Unavailable account")
    .join(" → ");
  const categories = transaction.allocations
    .map((allocation) => categoryNames.get(allocation.category_id) ?? "Unavailable category")
    .join(", ");
  const direction = transactionDirection(transaction, total);
  return (
    <article className="transaction-register-row">
      <span aria-hidden="true" className={`transaction-direction transaction-direction-${direction}`}>
        <AppIcon name={transaction.kind === "transfer" ? "accounts" : "transactions"} />
      </span>
      <div className="transaction-register-copy">
        <div>
          <strong>{transactionTitle(transaction)}</strong>
          <span className="transaction-register-badges">
            <StatusBadge tone={transaction.status === "pending" ? "warning" : "positive"}>{transaction.status}</StatusBadge>
            {transaction.kind !== "standard" ? <StatusBadge>{transactionKindLabel(transaction.kind)}</StatusBadge> : null}
          </span>
        </div>
        <small>{formatTransactionDate(transaction.transaction_date)} · {accounts}</small>
        {categories ? <small>{categories}</small> : null}
      </div>
      <div className="transaction-register-amount">
        {transaction.kind === "transfer" ? <span>Transfer</span> : total === null ? <span>Amount unavailable</span> : (
          <MoneyAmount amount={total} currency={workspace.base_currency} signed />
        )}
      </div>
      {canManage ? (
        <div className="transaction-register-actions">
          <button className="text-button" onClick={onEdit} type="button">Edit</button>
          <button className="text-button danger" onClick={onDelete} type="button">Delete</button>
        </div>
      ) : null}
    </article>
  );
}

function accountCurrency(accounts: Account[], accountId: string) {
  return accounts.find((account) => account.id === accountId)?.currency;
}

function transactionTitle(transaction: LedgerTransaction) {
  return transaction.payee ?? transaction.description ?? transactionKindLabel(transaction.kind);
}

function transactionKindLabel(kind: LedgerTransaction["kind"]) {
  if (kind === "standard") return "Expense or income";
  if (kind === "transfer") return "Transfer";
  return "Balance adjustment";
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
  return error instanceof APIError ? error.message : "The change could not be saved.";
}
