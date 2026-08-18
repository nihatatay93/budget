-- name: ListTransactions :many
SELECT
    id, workspace_id, kind, status, transaction_date, payee, description, notes,
    source, created_by, updated_by, created_at, updated_at
FROM transactions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NULL
  AND (sqlc.narg(from_date)::DATE IS NULL OR transaction_date >= sqlc.narg(from_date))
  AND (sqlc.narg(to_date)::DATE IS NULL OR transaction_date <= sqlc.narg(to_date))
ORDER BY transaction_date DESC, created_at DESC, id DESC
LIMIT sqlc.arg(result_limit);

-- name: GetTransaction :one
SELECT
    id, workspace_id, kind, status, transaction_date, payee, description, notes,
    source, created_by, updated_by, created_at, updated_at
FROM transactions
WHERE workspace_id = $1
  AND id = $2
  AND deleted_at IS NULL;

-- name: ListTransactionEntries :many
SELECT transaction_id, id, account_id, amount_minor, base_amount_minor
FROM transaction_entries
WHERE workspace_id = sqlc.arg(workspace_id)
  AND transaction_id = ANY(sqlc.arg(transaction_ids)::UUID[])
ORDER BY transaction_id, created_at, id;

-- name: ListTransactionAllocations :many
SELECT transaction_id, id, category_id, amount_base_minor
FROM transaction_allocations
WHERE workspace_id = sqlc.arg(workspace_id)
  AND transaction_id = ANY(sqlc.arg(transaction_ids)::UUID[])
ORDER BY transaction_id, created_at, id;

-- name: CreateTransaction :exec
INSERT INTO transactions (
    id, workspace_id, kind, status, transaction_date, payee, description, notes,
    source, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10
);

-- name: UpdateTransaction :execrows
UPDATE transactions
SET kind = sqlc.arg(kind),
    status = sqlc.arg(status),
    transaction_date = sqlc.arg(transaction_date),
    payee = sqlc.narg(payee),
    description = sqlc.narg(description),
    notes = sqlc.narg(notes),
    updated_by = sqlc.arg(updated_by),
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: SoftDeleteTransaction :execrows
UPDATE transactions
SET deleted_at = now(),
    updated_by = sqlc.arg(updated_by),
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: DeleteTransactionEntries :exec
DELETE FROM transaction_entries
WHERE workspace_id = $1 AND transaction_id = $2;

-- name: DeleteTransactionAllocations :exec
DELETE FROM transaction_allocations
WHERE workspace_id = $1 AND transaction_id = $2;

-- name: CreateTransactionEntry :exec
INSERT INTO transaction_entries (
    id, workspace_id, transaction_id, account_id, amount_minor, base_amount_minor
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: CreateTransactionAllocation :exec
INSERT INTO transaction_allocations (
    id, workspace_id, transaction_id, category_id, amount_base_minor
) VALUES ($1, $2, $3, $4, $5);

-- name: GetTransactionAccountCurrency :one
SELECT currency
FROM accounts
WHERE workspace_id = $1 AND id = $2;

-- name: TransactionCategoryExists :one
SELECT EXISTS (
    SELECT 1 FROM categories WHERE workspace_id = $1 AND id = $2
);

-- name: GetSystemCategoryID :one
SELECT id
FROM categories
WHERE workspace_id = $1 AND system_key = $2;
