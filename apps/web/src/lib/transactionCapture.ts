import type { Account, Category, Transaction, TransactionWriteRequest } from "../api/client";
import type { Currency } from "./currency";
import { majorAmountInput, parseMajorAmount } from "./currency";
import { t } from "./i18n";

export { categoryBranch, rootCategory } from "./categoryTree";

/**
 * The everyday capture form collects what a person knows — what it was, how much, which
 * category, when — and derives the transaction aggregate documented in docs/domain-model.md
 * from it. The detailed editor still writes entries and allocations directly.
 *
 * Two rules make the derivation safe. The amount is typed once without a sign, so an entry and
 * its allocation can never disagree and reconciliation cannot fail from this form. And a
 * transaction the draft cannot express — a split, a balance adjustment, more than one account
 * entry, a mixed-currency transfer, or a refund allocated against an expense category — has no
 * draft at all, which is what sends the detailed editor to the screen instead of flattening it.
 */
export type CaptureType = "expense" | "income" | "transfer";

export type CaptureDraft = {
  type: CaptureType;
  /** Unsigned major-unit amount in the chosen account's currency. */
  amount: string;
  /**
   * Unsigned major-unit amount in the workspace base currency, for a foreign-currency account.
   * Optional: left empty, the server books the transaction date's rate and derives the
   * allocation from it.
   */
  baseAmount: string;
  /** Empty means the server applies the protected uncategorized category. */
  categoryId: string;
  accountId: string;
  /** Destination account for a transfer. */
  toAccountId: string;
  transactionDate: string;
  payee: string;
  description: string;
  notes: string;
  pending: boolean;
};

export type CaptureContext = {
  accounts: Account[];
  baseCurrency: Currency;
  categories?: Category[];
};

/**
 * The parts of a transaction the simple form reads. A saved `Transaction` satisfies it, and so
 * does an unsaved set of detailed-editor fields, which is what lets the editor answer whether it
 * can hand the work back to the simple form.
 */
export type CaptureSource = Pick<
  Transaction,
  "kind" | "status" | "transaction_date" | "payee" | "description" | "notes" | "entries" | "allocations"
>;

export function emptyCaptureDraft(transactionDate: string, accountId = ""): CaptureDraft {
  return {
    type: "expense",
    amount: "",
    baseAmount: "",
    categoryId: "",
    accountId,
    toAccountId: "",
    transactionDate,
    payee: "",
    description: "",
    notes: "",
    pending: false,
  };
}

/** The account of the most recent transaction, which is the one a person is most likely to use again. */
export function suggestedAccountId(transactions: Transaction[], accounts: Account[]): string {
  const usable = new Set(accounts.map((account) => account.id));
  const recent = [...transactions]
    .sort((left, right) => right.transaction_date.localeCompare(left.transaction_date))
    .flatMap((transaction) => transaction.entries.map((entry) => entry.account_id))
    .find((accountId) => usable.has(accountId));
  return recent ?? accounts[0]?.id ?? "";
}

export function captureRequest(
  draft: CaptureDraft,
  { accounts, baseCurrency }: CaptureContext,
): { request: TransactionWriteRequest } | { error: string } {
  const amount = parseMajorAmount(draft.amount);
  if (amount === null || amount === 0) {
    return { error: t("Enter an amount above zero with at most two decimals.") };
  }
  const magnitude = Math.abs(amount);
  const account = accounts.find((candidate) => candidate.id === draft.accountId);
  if (!account) return { error: t("Choose an account.") };
  const shared = {
    status: draft.pending ? ("pending" as const) : ("posted" as const),
    transaction_date: draft.transactionDate,
    ...(draft.payee.trim() ? { payee: draft.payee.trim() } : {}),
    ...(draft.description.trim() ? { description: draft.description.trim() } : {}),
    ...(draft.notes.trim() ? { notes: draft.notes.trim() } : {}),
  };

  if (draft.type === "transfer") {
    const destination = accounts.find((candidate) => candidate.id === draft.toAccountId);
    if (!destination || destination.id === account.id) {
      return { error: t("Choose two different accounts for a transfer.") };
    }
    if (destination.currency !== account.currency) {
      return { error: t("A transfer between two currencies needs the detailed editor.") };
    }
    return {
      request: {
        kind: "transfer",
        ...shared,
        entries: [
          { account_id: account.id, amount_minor: -magnitude },
          { account_id: destination.id, amount_minor: magnitude },
        ],
        allocations: [],
      },
    };
  }

  const direction = draft.type === "expense" ? -1 : 1;
  const signed = direction * magnitude;
  // A foreign account may state its base-currency value, and must when the deployment has no
  // rate feed. Left empty, both the entry and its allocation are booked from the transaction
  // date's rate on the server, which is the only place that rate is known.
  let baseMinor: number | undefined;
  if (account.currency !== baseCurrency && draft.baseAmount.trim()) {
    const base = parseMajorAmount(draft.baseAmount);
    if (base === null || base === 0) {
      return {
        error: t("Enter the {currency} value of this amount as well.", { currency: baseCurrency }),
      };
    }
    baseMinor = direction * Math.abs(base);
  } else if (account.currency === baseCurrency) {
    baseMinor = signed;
  }
  return {
    request: {
      kind: "standard",
      ...shared,
      entries: [{
        account_id: account.id,
        amount_minor: signed,
        ...(baseMinor === undefined || baseMinor === signed ? {} : { base_amount_minor: baseMinor }),
      }],
      allocations: draft.categoryId
        ? [{
          category_id: draft.categoryId,
          ...(baseMinor === undefined ? {} : { amount_base_minor: baseMinor }),
        }]
        : [],
    },
  };
}

/** The draft that reproduces this transaction, or undefined when only the detailed editor can. */
export function captureDraft(
  transaction: CaptureSource,
  { accounts, baseCurrency, categories = [] }: CaptureContext,
): CaptureDraft | undefined {
  if (transaction.kind === "adjustment") return undefined;
  const shared = {
    transactionDate: transaction.transaction_date,
    payee: transaction.payee ?? "",
    description: transaction.description ?? "",
    notes: transaction.notes ?? "",
    pending: transaction.status === "pending",
  };

  if (transaction.kind === "transfer") {
    if (transaction.entries.length !== 2 || transaction.allocations.length > 0) return undefined;
    const source = transaction.entries.find((entry) => entry.amount_minor < 0);
    const destination = transaction.entries.find((entry) => entry.amount_minor > 0);
    if (!source || !destination || source.amount_minor !== -destination.amount_minor) return undefined;
    const from = accounts.find((account) => account.id === source.account_id);
    const to = accounts.find((account) => account.id === destination.account_id);
    if (!from || !to || from.currency !== to.currency) return undefined;
    return {
      ...shared,
      type: "transfer",
      amount: majorAmountInput(Math.abs(source.amount_minor)),
      baseAmount: "",
      categoryId: "",
      accountId: from.id,
      toAccountId: to.id,
    };
  }

  if (transaction.entries.length !== 1 || transaction.allocations.length > 1) return undefined;
  const [entry] = transaction.entries;
  if (entry.amount_minor === 0 || Math.sign(entry.amount_minor) !== Math.sign(entry.base_amount_minor)) {
    return undefined;
  }
  const account = accounts.find((candidate) => candidate.id === entry.account_id);
  if (!account) return undefined;
  const type: CaptureType = entry.base_amount_minor < 0 ? "expense" : "income";

  let categoryId = "";
  const [allocation] = transaction.allocations;
  if (allocation) {
    if (allocation.amount_base_minor !== entry.base_amount_minor) return undefined;
    const category = categories.find((candidate) => candidate.id === allocation.category_id);
    // A refund allocated to an expense category, or a reversal against an income one, is a
    // valid ledger entry the simple form has no way to offer. It belongs in the detailed editor.
    if (!category || category.kind !== type) return undefined;
    categoryId = category.system_key ? "" : category.id;
  }
  return {
    ...shared,
    type,
    amount: majorAmountInput(Math.abs(entry.amount_minor)),
    baseAmount: account.currency === baseCurrency ? "" : majorAmountInput(Math.abs(entry.base_amount_minor)),
    categoryId,
    accountId: account.id,
    toAccountId: "",
  };
}
