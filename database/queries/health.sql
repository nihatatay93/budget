-- name: GetDatabaseTime :one
SELECT now() AS database_time;

-- name: GetAccountBalance :one
SELECT COALESCE(SUM(entry.amount_minor), 0)::BIGINT AS balance_minor
FROM transaction_entries AS entry
JOIN transactions AS transaction ON transaction.id = entry.transaction_id
WHERE entry.workspace_id = $1
  AND entry.account_id = $2
  AND transaction.status = 'posted'
  AND transaction.deleted_at IS NULL;
