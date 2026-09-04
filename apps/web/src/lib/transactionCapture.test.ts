import { describe, expect, it } from "vitest";

import type { Account, Category, Transaction } from "../api/client";
import {
  captureDraft,
  captureRequest,
  categoryBranch,
  emptyCaptureDraft,
  rootCategory,
  suggestedAccountId,
} from "./transactionCapture";

const accounts: Account[] = [
  account("everyday", "TRY"),
  account("savings", "TRY"),
  account("travel", "EUR"),
];

const categories: Category[] = [
  category("food", "expense"),
  category("groceries", "expense", "food"),
  category("salary", "income"),
  { ...category("uncategorized", "expense"), system_key: "uncategorized_expense" },
];

const context = { accounts, baseCurrency: "TRY" as const, categories };

describe("captureRequest", () => {
  it("signs an expense once and mirrors it into the allocation", () => {
    const result = captureRequest(
      { ...emptyCaptureDraft("2026-08-26", "everyday"), amount: "185.00", categoryId: "groceries" },
      context,
    );

    expect(result).toEqual({
      request: {
        kind: "standard",
        status: "posted",
        transaction_date: "2026-08-26",
        entries: [{ account_id: "everyday", amount_minor: -18500 }],
        allocations: [{ category_id: "groceries", amount_base_minor: -18500 }],
      },
    });
  });

  it("reads income as positive and keeps a pending draft pending", () => {
    const result = captureRequest(
      {
        ...emptyCaptureDraft("2026-08-26", "everyday"),
        type: "income",
        amount: "22000",
        categoryId: "salary",
        payee: "  Employer  ",
        pending: true,
      },
      context,
    );

    expect(result).toEqual({
      request: {
        kind: "standard",
        status: "pending",
        transaction_date: "2026-08-26",
        payee: "Employer",
        entries: [{ account_id: "everyday", amount_minor: 2200000 }],
        allocations: [{ category_id: "salary", amount_base_minor: 2200000 }],
      },
    });
  });

  it("leaves allocations empty so the server applies its uncategorized category", () => {
    const result = captureRequest(
      { ...emptyCaptureDraft("2026-08-26", "everyday"), amount: "12.5" },
      context,
    );

    expect(result).toMatchObject({ request: { allocations: [] } });
  });

  it("leaves a foreign amount to the transaction date's rate unless it is stated", () => {
    const draft = { ...emptyCaptureDraft("2026-08-26", "travel"), amount: "40", categoryId: "food" };

    // Neither the entry nor its allocation carries a base amount, so the server books both at
    // the rate for that date — the only place that rate is known.
    expect(captureRequest(draft, context)).toEqual({
      request: {
        kind: "standard",
        status: "posted",
        transaction_date: "2026-08-26",
        entries: [{ account_id: "travel", amount_minor: -4000 }],
        allocations: [{ category_id: "food" }],
      },
    });
    expect(captureRequest({ ...draft, baseAmount: "1500" }, context)).toEqual({
      request: {
        kind: "standard",
        status: "posted",
        transaction_date: "2026-08-26",
        entries: [{ account_id: "travel", amount_minor: -4000, base_amount_minor: -150000 }],
        allocations: [{ category_id: "food", amount_base_minor: -150000 }],
      },
    });
    expect(captureRequest({ ...draft, baseAmount: "nonsense" }, context)).toHaveProperty("error");
  });

  it("balances a transfer across two accounts without allocations", () => {
    const result = captureRequest(
      {
        ...emptyCaptureDraft("2026-08-26", "everyday"),
        type: "transfer",
        amount: "1000",
        toAccountId: "savings",
      },
      context,
    );

    expect(result).toEqual({
      request: {
        kind: "transfer",
        status: "posted",
        transaction_date: "2026-08-26",
        entries: [
          { account_id: "everyday", amount_minor: -100000 },
          { account_id: "savings", amount_minor: 100000 },
        ],
        allocations: [],
      },
    });
  });

  it("rejects the drafts it cannot express", () => {
    const base = emptyCaptureDraft("2026-08-26", "everyday");

    expect(captureRequest({ ...base, amount: "0" }, context)).toHaveProperty("error");
    expect(captureRequest({ ...base, amount: "1.005" }, context)).toHaveProperty("error");
    expect(captureRequest({ ...base, amount: "10", accountId: "" }, context)).toHaveProperty("error");
    expect(captureRequest({ ...base, type: "transfer", amount: "10", toAccountId: "everyday" }, context))
      .toHaveProperty("error");
    expect(captureRequest({ ...base, type: "transfer", amount: "10", toAccountId: "travel" }, context))
      .toEqual({ error: "A transfer between two currencies needs the detailed editor." });
  });
});

describe("captureDraft", () => {
  it("round-trips a categorized expense", () => {
    const draft = captureDraft(
      transaction({
        entries: [{ account_id: "everyday", amount_minor: -18500, base_amount_minor: -18500 }],
        allocations: [{ category_id: "groceries", amount_base_minor: -18500 }],
      }),
      context,
    );

    expect(draft).toMatchObject({ type: "expense", amount: "185.00", categoryId: "groceries", accountId: "everyday" });
    expect(captureRequest(draft!, context)).toMatchObject({
      request: { entries: [{ amount_minor: -18500 }] },
    });
  });

  it("shows a protected uncategorized allocation as no category", () => {
    const draft = captureDraft(
      transaction({
        entries: [{ account_id: "everyday", amount_minor: -8500, base_amount_minor: -8500 }],
        allocations: [{ category_id: "uncategorized", amount_base_minor: -8500 }],
      }),
      context,
    );

    expect(draft).toMatchObject({ categoryId: "" });
  });

  it("keeps a balanced transfer between same-currency accounts", () => {
    const draft = captureDraft(
      transaction({
        kind: "transfer",
        entries: [
          { account_id: "everyday", amount_minor: -100000, base_amount_minor: -100000 },
          { account_id: "savings", amount_minor: 100000, base_amount_minor: 100000 },
        ],
      }),
      context,
    );

    expect(draft).toMatchObject({ type: "transfer", accountId: "everyday", toAccountId: "savings", amount: "1000.00" });
  });

  it("has no draft for anything the detailed editor owns", () => {
    const split = transaction({
      entries: [{ account_id: "everyday", amount_minor: -10000, base_amount_minor: -10000 }],
      allocations: [
        { category_id: "food", amount_base_minor: -6000 },
        { category_id: "groceries", amount_base_minor: -4000 },
      ],
    });
    const refund = transaction({
      entries: [{ account_id: "everyday", amount_minor: 5000, base_amount_minor: 5000 }],
      allocations: [{ category_id: "food", amount_base_minor: 5000 }],
    });
    const twoEntries = transaction({
      entries: [
        { account_id: "everyday", amount_minor: -6000, base_amount_minor: -6000 },
        { account_id: "savings", amount_minor: -4000, base_amount_minor: -4000 },
      ],
    });
    const mixedTransfer = transaction({
      kind: "transfer",
      entries: [
        { account_id: "everyday", amount_minor: -100000, base_amount_minor: -100000 },
        { account_id: "travel", amount_minor: 2600, base_amount_minor: 100000 },
      ],
    });

    expect(captureDraft(transaction({ kind: "adjustment" }), context)).toBeUndefined();
    expect(captureDraft(split, context)).toBeUndefined();
    expect(captureDraft(refund, context)).toBeUndefined();
    expect(captureDraft(twoEntries, context)).toBeUndefined();
    expect(captureDraft(mixedTransfer, context)).toBeUndefined();
  });
});

describe("category branches", () => {
  it("finds the root a subcategory belongs to", () => {
    expect(rootCategory(categories, "groceries")?.id).toBe("food");
    expect(rootCategory(categories, "food")?.id).toBe("food");
    expect(rootCategory(categories, "missing")).toBeUndefined();
  });

  it("lists a branch depth-first with its depth", () => {
    const nested = [...categories, category("markets", "expense", "groceries")];
    expect(categoryBranch(nested, "food")).toEqual([
      { category: nested.find((value) => value.id === "groceries"), depth: 0 },
      { category: nested.find((value) => value.id === "markets"), depth: 1 },
    ]);
  });
});

describe("suggestedAccountId", () => {
  it("offers the account of the most recent transaction", () => {
    const older = transaction({
      date: "2026-08-01",
      entries: [{ account_id: "savings", amount_minor: -100, base_amount_minor: -100 }],
    });
    const newer = transaction({
      date: "2026-08-20",
      entries: [{ account_id: "travel", amount_minor: -100, base_amount_minor: -100 }],
    });

    expect(suggestedAccountId([older, newer], accounts)).toBe("travel");
    expect(suggestedAccountId([], accounts)).toBe("everyday");
    expect(suggestedAccountId([], [])).toBe("");
  });
});

function account(id: string, currency: Account["currency"]): Account {
  return { id, workspace_id: "workspace", name: id, type: "bank", currency, balance_minor: 0 };
}

function category(id: string, kind: Category["kind"], parentId?: string): Category {
  return {
    id,
    workspace_id: "workspace",
    name: id,
    kind,
    ...(parentId ? { parent_id: parentId } : {}),
  };
}

function transaction({
  allocations = [],
  date = "2026-08-26",
  entries,
  kind = "standard",
}: {
  allocations?: Transaction["allocations"];
  date?: string;
  entries?: Transaction["entries"];
  kind?: Transaction["kind"];
}): Transaction {
  return {
    id: "transaction",
    workspace_id: "workspace",
    kind,
    status: "posted",
    transaction_date: date,
    source: "manual",
    created_by: "user",
    updated_by: "user",
    created_at: `${date}T10:00:00Z`,
    updated_at: `${date}T10:00:00Z`,
    entries: entries ?? [{ account_id: "everyday", amount_minor: -100, base_amount_minor: -100 }],
    allocations,
  };
}
