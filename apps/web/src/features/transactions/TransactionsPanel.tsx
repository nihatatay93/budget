import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  APIError,
  type Account,
  type SessionResponse,
  type Transaction as LedgerTransaction,
  type TransactionWriteRequest,
  accountsQueryKey,
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
import { formatMoney, majorAmountInput, parseMajorAmount } from "../../lib/currency";

type Workspace = SessionResponse["workspaces"][number];
type EntryDraft = { key: string; accountId: string; amount: string; baseAmount: string };
type AllocationDraft = { key: string; categoryId: string; amount: string };

let nextDraftKey = 0;
function draftKey() {
  nextDraftKey += 1;
  return `transaction-draft-${nextDraftKey}`;
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
    queryKey: ["workspaces", workspace.id, "categories"],
    queryFn: () => listCategories(workspace.id),
  });
  const [editing, setEditing] = useState<LedgerTransaction>();
  const [kind, setKind] = useState<TransactionWriteRequest["kind"]>("standard");
  const [status, setStatus] = useState<TransactionWriteRequest["status"]>("posted");
  const [transactionDate, setTransactionDate] = useState(today);
  const [payee, setPayee] = useState("");
  const [description, setDescription] = useState("");
  const [notes, setNotes] = useState("");
  const [entries, setEntries] = useState<EntryDraft[]>([emptyEntry()]);
  const [allocations, setAllocations] = useState<AllocationDraft[]>([]);
  const [validation, setValidation] = useState("");

  const save = useMutation({
    mutationFn: (input: TransactionWriteRequest) =>
      editing
        ? updateTransaction(workspace.id, editing.id, input)
        : createTransaction(workspace.id, input),
    onSuccess: async () => {
      reset();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: transactionsQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: accountsQueryKey(workspace.id) }),
        queryClient.invalidateQueries({
          queryKey: financialProjectionQueryPrefix(workspace.id),
        }),
        queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
      ]);
    },
  });
  const remove = useMutation({
    mutationFn: (transactionId: string) => deleteTransaction(workspace.id, transactionId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: transactionsQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: accountsQueryKey(workspace.id) }),
        queryClient.invalidateQueries({
          queryKey: financialProjectionQueryPrefix(workspace.id),
        }),
        queryClient.invalidateQueries({ queryKey: monthlyBudgetQueryPrefix(workspace.id) }),
      ]);
    },
  });

  function reset() {
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
      key: draftKey(), accountId: entry.account_id,
      amount: majorAmountInput(entry.amount_minor),
      baseAmount: accountCurrency(accounts.data ?? [], entry.account_id) === workspace.base_currency
        ? ""
        : majorAmountInput(entry.base_amount_minor),
    })));
    setAllocations(value.allocations.map((allocation) => ({
      key: draftKey(), categoryId: allocation.category_id,
      amount: majorAmountInput(allocation.amount_base_minor),
    })));
    setValidation("");
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
      if (!entry.accountId || amount === null || amount === 0 ||
          (entry.baseAmount.trim() && baseAmount === null) ||
          (baseAmount !== undefined && baseAmount !== null && baseAmount !== 0 && Math.sign(baseAmount) !== Math.sign(amount))) {
        setValidation("Every entry needs an account and a non-zero amount with at most two decimals; a non-zero manual base amount must use the same sign.");
        return;
      }
      const parsedEntry: TransactionWriteRequest["entries"][number] = {
        account_id: entry.accountId,
        amount_minor: amount,
      };
      if (baseAmount !== undefined && baseAmount !== null) {
        parsedEntry.base_amount_minor = baseAmount;
      }
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

  const accountNames = new Map((accounts.data ?? []).map((account) => [account.id, account.name]));
  const categoryNames = new Map((categories.data ?? []).map((category) => [category.id, category.name]));

  return (
    <section className="setup-panel transactions-panel" aria-labelledby="transactions-heading">
      <div className="section-heading">
        <div>
          <p className="eyebrow">The ledger</p>
          <h2 id="transactions-heading">Transactions</h2>
        </div>
        <span>{transactions.data?.length ?? 0} recent</span>
      </div>
      {transactions.isPending ? <p className="resource-state">Loading…</p> : null}
      {transactions.error ? <p className="form-error">{transactions.error.message}</p> : null}
      {transactions.data?.length === 0 ? <p className="resource-state">Record your first transaction.</p> : null}
      <div className="transaction-list">
        {transactions.data?.map((value) => {
          const total = value.entries.reduce((sum, entry) => sum + entry.base_amount_minor, 0);
          return (
            <article className="transaction-row" key={value.id}>
              <div>
                <strong>{value.payee ?? value.description ?? transactionKindLabel(value.kind)}</strong>
                <small>
                  {value.transaction_date} · {transactionKindLabel(value.kind)} · {value.status}
                </small>
                <small>
                  {value.entries.map((entry) => accountNames.get(entry.account_id) ?? "Unavailable account").join(" → ")}
                  {value.allocations.length > 0
                    ? ` · ${value.allocations.map((allocation) => categoryNames.get(allocation.category_id) ?? "Unavailable category").join(", ")}`
                    : ""}
                </small>
              </div>
              <div className="resource-actions">
                <span>{value.kind === "transfer" ? "Transfer" : formatMoney(total, workspace.base_currency)}</span>
                {canManage ? <button className="text-button" type="button" onClick={() => edit(value)}>Edit</button> : null}
                {canManage ? (
                  <button
                    className="text-button danger"
                    type="button"
                    onClick={() => {
                      if (window.confirm("Delete this transaction? Its entries will stop affecting balances.")) {
                        remove.mutate(value.id);
                      }
                    }}
                  >
                    Delete
                  </button>
                ) : null}
              </div>
            </article>
          );
        })}
      </div>
      {remove.error ? <p className="form-error">{mutationMessage(remove.error)}</p> : null}
      {canManage ? (
        <form className="resource-form transaction-form" onSubmit={submit}>
          <h3>{editing ? "Edit transaction" : "Add transaction"}</h3>
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
                      required inputMode="decimal" placeholder="-12.50" value={entry.amount}
                      onChange={(event) => setEntries((current) => current.map((item) =>
                        item.key === entry.key ? { ...item, amount: event.target.value } : item,
                      ))}
                    />
                  </label>
                  {currency && currency !== workspace.base_currency ? (
                    <label>
                      Base amount ({workspace.base_currency}, optional)
                      <input
                        inputMode="decimal" placeholder="Auto historical rate" value={entry.baseAmount}
                        onChange={(event) => setEntries((current) => current.map((item) =>
                          item.key === entry.key ? { ...item, baseAmount: event.target.value } : item,
                        ))}
                      />
                    </label>
                  ) : null}
                  {entries.length > 1 ? (
                    <button className="text-button danger" type="button" onClick={() => setEntries((current) => current.filter((item) => item.key !== entry.key))}>
                      Remove entry {index + 1}
                    </button>
                  ) : null}
                </div>
              );
            })}
            <button className="secondary-button line-button" type="button" onClick={() => setEntries((current) => [...current, emptyEntry(accounts.data?.[0]?.id)])}>
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
                      required value={allocation.categoryId}
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
                      required inputMode="decimal" placeholder="-12.50" value={allocation.amount}
                      onChange={(event) => setAllocations((current) => current.map((item) =>
                        item.key === allocation.key ? { ...item, amount: event.target.value } : item,
                      ))}
                    />
                  </label>
                  <button className="text-button danger" type="button" onClick={() => setAllocations((current) => current.filter((item) => item.key !== allocation.key))}>
                    Remove allocation {index + 1}
                  </button>
                </div>
              ))}
              <button className="secondary-button line-button" type="button" onClick={() => setAllocations((current) => [...current, emptyAllocation(categories.data?.[0]?.id)])}>
                Add category allocation
              </button>
            </fieldset>
          ) : null}
          <label>
            Notes (optional)
            <input maxLength={4000} value={notes} onChange={(event) => setNotes(event.target.value)} />
          </label>
          {validation ? <p className="form-error">{validation}</p> : null}
          {save.error ? <p className="form-error">{mutationMessage(save.error)}</p> : null}
          <div className="form-actions">
            <button disabled={save.isPending || accounts.data?.length === 0} type="submit">
              {editing ? "Save transaction" : "Add transaction"}
            </button>
            {editing ? <button className="secondary-button" type="button" onClick={reset}>Cancel</button> : null}
          </div>
        </form>
      ) : <p className="permission-note">Viewer access is read-only.</p>}
    </section>
  );
}

function accountCurrency(accounts: Account[], accountId: string) {
  return accounts.find((account) => account.id === accountId)?.currency;
}

function transactionKindLabel(kind: LedgerTransaction["kind"]) {
  if (kind === "standard") return "Expense or income";
  if (kind === "transfer") return "Transfer";
  return "Balance adjustment";
}

function mutationMessage(error: Error) {
  return error instanceof APIError ? error.message : "The change could not be saved.";
}
