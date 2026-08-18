-- name: ListAccounts :many
SELECT
    accounts.id,
    accounts.workspace_id,
    accounts.name,
    accounts.type,
    accounts.currency,
    accounts.institution_name,
    accounts.archived_at,
    COALESCE(SUM(
        CASE
            WHEN transactions.status = 'posted' AND transactions.deleted_at IS NULL
                THEN transaction_entries.amount_minor
            ELSE 0
        END
    ), 0)::BIGINT AS balance_minor
FROM accounts
LEFT JOIN transaction_entries
    ON transaction_entries.workspace_id = accounts.workspace_id
   AND transaction_entries.account_id = accounts.id
LEFT JOIN transactions
    ON transactions.workspace_id = transaction_entries.workspace_id
   AND transactions.id = transaction_entries.transaction_id
WHERE accounts.workspace_id = sqlc.arg(workspace_id)
  AND (sqlc.arg(include_archived)::BOOLEAN OR accounts.archived_at IS NULL)
GROUP BY accounts.id
ORDER BY accounts.archived_at NULLS FIRST, lower(accounts.name), accounts.id;

-- name: GetAccount :one
SELECT
    accounts.id,
    accounts.workspace_id,
    accounts.name,
    accounts.type,
    accounts.currency,
    accounts.institution_name,
    accounts.archived_at,
    COALESCE(SUM(
        CASE
            WHEN transactions.status = 'posted' AND transactions.deleted_at IS NULL
                THEN transaction_entries.amount_minor
            ELSE 0
        END
    ), 0)::BIGINT AS balance_minor
FROM accounts
LEFT JOIN transaction_entries
    ON transaction_entries.workspace_id = accounts.workspace_id
   AND transaction_entries.account_id = accounts.id
LEFT JOIN transactions
    ON transactions.workspace_id = transaction_entries.workspace_id
   AND transactions.id = transaction_entries.transaction_id
WHERE accounts.workspace_id = $1
  AND accounts.id = $2
GROUP BY accounts.id;

-- name: CreateAccount :exec
INSERT INTO accounts (
    id, workspace_id, name, type, currency, institution_name
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: UpdateAccount :execrows
UPDATE accounts
SET name = sqlc.arg(name),
    type = sqlc.arg(type),
    currency = sqlc.arg(currency),
    institution_name = sqlc.narg(institution_name),
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: ArchiveAccount :execrows
UPDATE accounts
SET archived_at = COALESCE(archived_at, now()),
    updated_at = CASE WHEN archived_at IS NULL THEN now() ELSE updated_at END
WHERE workspace_id = $1
  AND id = $2;
