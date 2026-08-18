-- name: GetBudgetWorkspaceSettings :one
SELECT base_currency, timezone
FROM workspaces
WHERE id = $1;

-- name: ListBudgetCategoryRules :many
SELECT
    categories.id,
    categories.parent_id,
    categories.kind,
    categories.archived_at,
    EXISTS (
        SELECT 1
        FROM budgets
        JOIN budget_items
          ON budget_items.workspace_id = budgets.workspace_id
         AND budget_items.budget_id = budgets.id
        WHERE budgets.workspace_id = categories.workspace_id
          AND budgets.starts_on = sqlc.arg(starts_on)::DATE
          AND budget_items.category_id = categories.id
    ) AS already_budgeted
FROM categories
WHERE categories.workspace_id = sqlc.arg(workspace_id)
ORDER BY categories.id;

-- name: GetMonthlyBudget :one
SELECT
    budgets.id,
    budgets.workspace_id,
    budgets.name,
    budgets.starts_on,
    workspaces.timezone,
    workspaces.base_currency,
    budgets.created_at,
    budgets.updated_at
FROM budgets
JOIN workspaces ON workspaces.id = budgets.workspace_id
WHERE budgets.workspace_id = sqlc.arg(workspace_id)
  AND budgets.starts_on = sqlc.arg(starts_on)::DATE;

-- name: ListMonthlyBudgetItems :many
WITH RECURSIVE category_descendants AS (
    SELECT
        budget_items.id AS budget_item_id,
        budget_items.category_id AS descendant_id
    FROM budget_items
    WHERE budget_items.workspace_id = sqlc.arg(workspace_id)
      AND budget_items.budget_id = sqlc.arg(budget_id)

    UNION ALL

    SELECT
        category_descendants.budget_item_id,
        child.id AS descendant_id
    FROM category_descendants
    JOIN categories AS child
      ON child.workspace_id = sqlc.arg(workspace_id)
     AND child.parent_id = category_descendants.descendant_id
),
item_usage AS (
    SELECT
        category_descendants.budget_item_id,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE transactions.id IS NOT NULL
        ), 0)::BIGINT
            AS signed_allocation_base_minor
    FROM category_descendants
    LEFT JOIN transaction_allocations
      ON transaction_allocations.workspace_id = sqlc.arg(workspace_id)
     AND transaction_allocations.category_id = category_descendants.descendant_id
    LEFT JOIN transactions
      ON transactions.workspace_id = transaction_allocations.workspace_id
     AND transactions.id = transaction_allocations.transaction_id
     AND transactions.status = 'posted'
     AND transactions.deleted_at IS NULL
     AND transactions.transaction_date >= sqlc.arg(starts_on)::DATE
     AND transactions.transaction_date < (sqlc.arg(starts_on)::DATE + INTERVAL '1 month')
    GROUP BY category_descendants.budget_item_id
)
SELECT
    budget_items.id,
    budget_items.category_id,
    categories.name AS category_name,
    categories.icon AS category_icon,
    categories.archived_at AS category_archived_at,
    budget_items.amount_base_minor AS planned_base_minor,
    item_usage.signed_allocation_base_minor
FROM budget_items
JOIN categories
  ON categories.workspace_id = budget_items.workspace_id
 AND categories.id = budget_items.category_id
JOIN item_usage ON item_usage.budget_item_id = budget_items.id
WHERE budget_items.workspace_id = sqlc.arg(workspace_id)
  AND budget_items.budget_id = sqlc.arg(budget_id)
ORDER BY lower(categories.name), budget_items.id;

-- name: UpsertMonthlyBudget :one
INSERT INTO budgets (
    id, workspace_id, name, period, starts_on, rollover, active
) VALUES (
    sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(name), 'monthly',
    sqlc.arg(starts_on), false, true
)
ON CONFLICT (workspace_id, starts_on) DO UPDATE
SET name = EXCLUDED.name,
    updated_at = now()
RETURNING id;

-- name: UpsertMonthlyBudgetItem :one
INSERT INTO budget_items (
    id, workspace_id, budget_id, category_id, amount_base_minor
) VALUES (
    sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(budget_id),
    sqlc.arg(category_id), sqlc.arg(amount_base_minor)
)
ON CONFLICT (budget_id, category_id) DO UPDATE
SET amount_base_minor = EXCLUDED.amount_base_minor
RETURNING id;

-- name: DeleteOmittedMonthlyBudgetItems :exec
DELETE FROM budget_items
WHERE workspace_id = sqlc.arg(workspace_id)
  AND budget_id = sqlc.arg(budget_id)
  AND NOT (category_id = ANY(sqlc.arg(category_ids)::UUID[]));
