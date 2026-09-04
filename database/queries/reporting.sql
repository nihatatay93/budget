-- name: GetReportingWorkspace :one
SELECT base_currency, timezone
FROM workspaces
WHERE id = $1;

-- name: ListReportingAccountBalances :many
SELECT
    accounts.id,
    accounts.name,
    accounts.type,
    accounts.currency,
    accounts.archived_at,
    COALESCE(SUM(transaction_entries.amount_minor) FILTER (
        WHERE transactions.status = 'posted'
          AND transactions.deleted_at IS NULL
          AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
    ), 0)::BIGINT AS posted_native_minor,
    COALESCE(SUM(transaction_entries.amount_minor) FILTER (
        WHERE transactions.status = 'pending'
          AND transactions.deleted_at IS NULL
          AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
    ), 0)::BIGINT AS pending_native_minor,
    COALESCE(SUM(transaction_entries.base_amount_minor) FILTER (
        WHERE transactions.status = 'posted'
          AND transactions.deleted_at IS NULL
          AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
    ), 0)::BIGINT AS posted_base_minor,
    COALESCE(SUM(transaction_entries.base_amount_minor) FILTER (
        WHERE transactions.status = 'pending'
          AND transactions.deleted_at IS NULL
          AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
    ), 0)::BIGINT AS pending_base_minor
FROM accounts
LEFT JOIN transaction_entries
    ON transaction_entries.workspace_id = accounts.workspace_id
   AND transaction_entries.account_id = accounts.id
LEFT JOIN transactions
    ON transactions.workspace_id = transaction_entries.workspace_id
   AND transactions.id = transaction_entries.transaction_id
WHERE accounts.workspace_id = sqlc.arg(workspace_id)
GROUP BY accounts.id
ORDER BY accounts.archived_at NULLS FIRST, lower(accounts.name), accounts.id;

-- name: ListReportingCategoryActivity :many
WITH RECURSIVE category_ancestors AS (
    SELECT categories.id AS descendant_id, categories.id AS ancestor_id
    FROM categories
    WHERE categories.workspace_id = sqlc.arg(workspace_id)

    UNION ALL

    SELECT category_ancestors.descendant_id, parent.id AS ancestor_id
    FROM category_ancestors
    JOIN categories AS child
      ON child.workspace_id = sqlc.arg(workspace_id)
     AND child.id = category_ancestors.ancestor_id
    JOIN categories AS parent
      ON parent.workspace_id = child.workspace_id
     AND parent.id = child.parent_id
),
direct_activity AS (
    SELECT
        categories.id,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE transactions.status = 'posted'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
              AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
        ), 0)::BIGINT AS direct_posted_signed_minor,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE transactions.status = 'pending'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
              AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
        ), 0)::BIGINT AS direct_pending_signed_minor
    FROM categories
    LEFT JOIN transaction_allocations
      ON transaction_allocations.workspace_id = categories.workspace_id
     AND transaction_allocations.category_id = categories.id
    LEFT JOIN transactions
      ON transactions.workspace_id = transaction_allocations.workspace_id
     AND transactions.id = transaction_allocations.transaction_id
    WHERE categories.workspace_id = sqlc.arg(workspace_id)
    GROUP BY categories.id
),
rolled_activity AS (
    SELECT
        category_ancestors.ancestor_id AS id,
        SUM(direct_activity.direct_posted_signed_minor)::BIGINT
            AS rolled_posted_signed_minor,
        SUM(direct_activity.direct_pending_signed_minor)::BIGINT
            AS rolled_pending_signed_minor
    FROM category_ancestors
    JOIN direct_activity
      ON direct_activity.id = category_ancestors.descendant_id
    GROUP BY category_ancestors.ancestor_id
)
SELECT
    categories.id,
    categories.parent_id,
    categories.name,
    categories.kind,
    categories.system_key,
    categories.predefined_key,
    categories.icon,
    categories.icon_type,
    categories.icon_value,
    categories.color_key,
    categories.archived_at,
    direct_activity.direct_posted_signed_minor,
    direct_activity.direct_pending_signed_minor,
    rolled_activity.rolled_posted_signed_minor,
    rolled_activity.rolled_pending_signed_minor
FROM categories
JOIN direct_activity ON direct_activity.id = categories.id
JOIN rolled_activity ON rolled_activity.id = categories.id
WHERE categories.workspace_id = sqlc.arg(workspace_id)
ORDER BY categories.kind, categories.parent_id NULLS FIRST, lower(categories.name), categories.id;
