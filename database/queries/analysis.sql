-- Spending analysis reads only posted, non-deleted activity. Transfers carry no allocations,
-- so allocation-derived aggregates exclude them by construction. Amounts keep their ledger
-- sign here; the analytics domain converts them to reporting orientation.

-- name: GetAnalysisTotals :one
WITH transaction_totals AS (
    SELECT
        transactions.id,
        transactions.transaction_date,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE categories.kind = 'income'
        ), 0)::BIGINT AS income_signed_minor,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE categories.kind = 'expense'
        ), 0)::BIGINT AS expense_signed_minor
    FROM transaction_allocations
    JOIN transactions
        ON transactions.workspace_id = transaction_allocations.workspace_id
       AND transactions.id = transaction_allocations.transaction_id
    JOIN categories
        ON categories.workspace_id = transaction_allocations.workspace_id
       AND categories.id = transaction_allocations.category_id
    WHERE transaction_allocations.workspace_id = sqlc.arg(workspace_id)
      AND transactions.status = 'posted'
      AND transactions.deleted_at IS NULL
      AND transactions.transaction_date >= sqlc.arg(comparison_from_date)::DATE
      AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
    GROUP BY transactions.id, transactions.transaction_date
),
windowed AS (
    SELECT
        transaction_totals.*,
        transaction_totals.transaction_date >= sqlc.arg(from_date)::DATE AS is_current
    FROM transaction_totals
)
SELECT
    COALESCE(SUM(income_signed_minor) FILTER (WHERE is_current), 0)::BIGINT
        AS income_signed_minor,
    COALESCE(SUM(expense_signed_minor) FILTER (WHERE is_current), 0)::BIGINT
        AS expense_signed_minor,
    COALESCE(SUM(income_signed_minor) FILTER (WHERE NOT is_current), 0)::BIGINT
        AS comparison_income_signed_minor,
    COALESCE(SUM(expense_signed_minor) FILTER (WHERE NOT is_current), 0)::BIGINT
        AS comparison_expense_signed_minor,
    COUNT(*) FILTER (WHERE is_current)::BIGINT AS transaction_count,
    COUNT(*) FILTER (WHERE is_current AND expense_signed_minor <> 0)::BIGINT
        AS spending_transaction_count,
    COALESCE(MIN(expense_signed_minor) FILTER (WHERE is_current), 0)::BIGINT
        AS smallest_expense_signed_minor,
    COUNT(DISTINCT transaction_date) FILTER (WHERE is_current AND expense_signed_minor <> 0)::BIGINT
        AS spending_day_count
FROM windowed;

-- name: ListAnalysisBuckets :many
WITH bucket_starts AS (
    SELECT generate_series(
        date_trunc(sqlc.arg(granularity)::TEXT, sqlc.arg(from_date)::DATE::TIMESTAMP),
        date_trunc(sqlc.arg(granularity)::TEXT, sqlc.arg(to_date)::DATE::TIMESTAMP),
        ('1 ' || sqlc.arg(granularity)::TEXT)::INTERVAL
    )::DATE AS bucket_anchor
),
activity AS (
    SELECT
        date_trunc(
            sqlc.arg(granularity)::TEXT, transactions.transaction_date::TIMESTAMP
        )::DATE AS bucket_anchor,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE categories.kind = 'income'
        ), 0)::BIGINT AS income_signed_minor,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE categories.kind = 'expense'
        ), 0)::BIGINT AS expense_signed_minor,
        COUNT(DISTINCT transactions.id)::BIGINT AS transaction_count
    FROM transaction_allocations
    JOIN transactions
        ON transactions.workspace_id = transaction_allocations.workspace_id
       AND transactions.id = transaction_allocations.transaction_id
    JOIN categories
        ON categories.workspace_id = transaction_allocations.workspace_id
       AND categories.id = transaction_allocations.category_id
    WHERE transaction_allocations.workspace_id = sqlc.arg(workspace_id)
      AND transactions.status = 'posted'
      AND transactions.deleted_at IS NULL
      AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
      AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
    GROUP BY 1
)
SELECT
    GREATEST(bucket_starts.bucket_anchor, sqlc.arg(from_date)::DATE)::DATE AS start_date,
    LEAST(
        (
            bucket_starts.bucket_anchor
            + ('1 ' || sqlc.arg(granularity)::TEXT)::INTERVAL
            - INTERVAL '1 day'
        )::DATE,
        sqlc.arg(to_date)::DATE
    )::DATE AS end_date,
    COALESCE(activity.income_signed_minor, 0)::BIGINT AS income_signed_minor,
    COALESCE(activity.expense_signed_minor, 0)::BIGINT AS expense_signed_minor,
    COALESCE(activity.transaction_count, 0)::BIGINT AS transaction_count
FROM bucket_starts
LEFT JOIN activity ON activity.bucket_anchor = bucket_starts.bucket_anchor
ORDER BY bucket_starts.bucket_anchor;

-- name: ListAnalysisCategoryTotals :many
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
        ), 0)::BIGINT AS direct_signed_minor,
        COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
            WHERE transactions.status = 'posted'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(comparison_from_date)::DATE
              AND transactions.transaction_date < sqlc.arg(from_date)::DATE
        ), 0)::BIGINT AS comparison_direct_signed_minor,
        COUNT(DISTINCT transactions.id) FILTER (
            WHERE transactions.status = 'posted'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
              AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
        )::BIGINT AS transaction_count,
        COALESCE(MIN(transaction_allocations.amount_base_minor) FILTER (
            WHERE transactions.status = 'posted'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
              AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
        ), 0)::BIGINT AS smallest_signed_minor,
        COALESCE(MAX(transaction_allocations.amount_base_minor) FILTER (
            WHERE transactions.status = 'posted'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
              AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
        ), 0)::BIGINT AS largest_signed_minor,
        MIN(transactions.transaction_date) FILTER (
            WHERE transactions.status = 'posted'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
              AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
        )::DATE AS first_date,
        MAX(transactions.transaction_date) FILTER (
            WHERE transactions.status = 'posted'
              AND transactions.deleted_at IS NULL
              AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
              AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
        )::DATE AS last_date
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
        SUM(direct_activity.direct_signed_minor)::BIGINT AS rolled_signed_minor,
        SUM(direct_activity.comparison_direct_signed_minor)::BIGINT
            AS comparison_rolled_signed_minor,
        SUM(direct_activity.transaction_count)::BIGINT AS rolled_transaction_count
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
    direct_activity.direct_signed_minor,
    direct_activity.comparison_direct_signed_minor,
    direct_activity.transaction_count,
    direct_activity.smallest_signed_minor,
    direct_activity.largest_signed_minor,
    direct_activity.first_date,
    direct_activity.last_date,
    rolled_activity.rolled_signed_minor,
    rolled_activity.comparison_rolled_signed_minor,
    rolled_activity.rolled_transaction_count
FROM categories
JOIN direct_activity ON direct_activity.id = categories.id
JOIN rolled_activity ON rolled_activity.id = categories.id
WHERE categories.workspace_id = sqlc.arg(workspace_id)
ORDER BY categories.kind, categories.parent_id NULLS FIRST, lower(categories.name), categories.id;

-- name: ListAnalysisCategoryBuckets :many
SELECT
    transaction_allocations.category_id,
    date_trunc(
        sqlc.arg(granularity)::TEXT, transactions.transaction_date::TIMESTAMP
    )::DATE AS bucket_anchor,
    SUM(transaction_allocations.amount_base_minor)::BIGINT AS signed_minor
FROM transaction_allocations
JOIN transactions
    ON transactions.workspace_id = transaction_allocations.workspace_id
   AND transactions.id = transaction_allocations.transaction_id
WHERE transaction_allocations.workspace_id = sqlc.arg(workspace_id)
  AND transactions.status = 'posted'
  AND transactions.deleted_at IS NULL
  AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
  AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
GROUP BY 1, 2
HAVING SUM(transaction_allocations.amount_base_minor) <> 0
ORDER BY 1, 2;

-- name: ListAnalysisWeekdays :many
SELECT
    EXTRACT(ISODOW FROM transactions.transaction_date)::SMALLINT AS weekday,
    COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
        WHERE categories.kind = 'income'
    ), 0)::BIGINT AS income_signed_minor,
    COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
        WHERE categories.kind = 'expense'
    ), 0)::BIGINT AS expense_signed_minor,
    COUNT(DISTINCT transactions.id)::BIGINT AS transaction_count
FROM transaction_allocations
JOIN transactions
    ON transactions.workspace_id = transaction_allocations.workspace_id
   AND transactions.id = transaction_allocations.transaction_id
JOIN categories
    ON categories.workspace_id = transaction_allocations.workspace_id
   AND categories.id = transaction_allocations.category_id
WHERE transaction_allocations.workspace_id = sqlc.arg(workspace_id)
  AND transactions.status = 'posted'
  AND transactions.deleted_at IS NULL
  AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
  AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
GROUP BY 1
ORDER BY 1;

-- name: ListAnalysisDays :many
SELECT
    transactions.transaction_date AS activity_date,
    COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
        WHERE categories.kind = 'income'
    ), 0)::BIGINT AS income_signed_minor,
    COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
        WHERE categories.kind = 'expense'
    ), 0)::BIGINT AS expense_signed_minor,
    COUNT(DISTINCT transactions.id)::BIGINT AS transaction_count
FROM transaction_allocations
JOIN transactions
    ON transactions.workspace_id = transaction_allocations.workspace_id
   AND transactions.id = transaction_allocations.transaction_id
JOIN categories
    ON categories.workspace_id = transaction_allocations.workspace_id
   AND categories.id = transaction_allocations.category_id
WHERE transaction_allocations.workspace_id = sqlc.arg(workspace_id)
  AND transactions.status = 'posted'
  AND transactions.deleted_at IS NULL
  AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
  AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
GROUP BY 1
ORDER BY 1;

-- name: ListAnalysisPayees :many
SELECT
    btrim(transactions.payee) AS payee,
    COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
        WHERE categories.kind = 'expense'
    ), 0)::BIGINT AS expense_signed_minor,
    COALESCE(SUM(transaction_allocations.amount_base_minor) FILTER (
        WHERE categories.kind = 'income'
    ), 0)::BIGINT AS income_signed_minor,
    COUNT(DISTINCT transactions.id)::BIGINT AS transaction_count,
    MIN(transactions.transaction_date)::DATE AS first_date,
    MAX(transactions.transaction_date)::DATE AS last_date
FROM transaction_allocations
JOIN transactions
    ON transactions.workspace_id = transaction_allocations.workspace_id
   AND transactions.id = transaction_allocations.transaction_id
JOIN categories
    ON categories.workspace_id = transaction_allocations.workspace_id
   AND categories.id = transaction_allocations.category_id
WHERE transaction_allocations.workspace_id = sqlc.arg(workspace_id)
  AND transactions.status = 'posted'
  AND transactions.deleted_at IS NULL
  AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
  AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
  AND btrim(COALESCE(transactions.payee, '')) <> ''
GROUP BY 1
ORDER BY 2, 4 DESC
LIMIT sqlc.arg(row_limit);

-- name: ListAnalysisAccountActivity :many
-- Entry-derived so it answers which account money moved through. Transfers are excluded so
-- moving money between owned accounts never reads as spending.
SELECT
    accounts.id,
    accounts.name,
    accounts.type,
    accounts.currency,
    accounts.archived_at,
    COALESCE(SUM(transaction_entries.base_amount_minor) FILTER (
        WHERE transaction_entries.base_amount_minor < 0
    ), 0)::BIGINT AS outflow_signed_minor,
    COALESCE(SUM(transaction_entries.base_amount_minor) FILTER (
        WHERE transaction_entries.base_amount_minor > 0
    ), 0)::BIGINT AS inflow_signed_minor,
    COUNT(DISTINCT transactions.id)::BIGINT AS transaction_count
FROM accounts
JOIN transaction_entries
    ON transaction_entries.workspace_id = accounts.workspace_id
   AND transaction_entries.account_id = accounts.id
JOIN transactions
    ON transactions.workspace_id = transaction_entries.workspace_id
   AND transactions.id = transaction_entries.transaction_id
WHERE accounts.workspace_id = sqlc.arg(workspace_id)
  AND transactions.status = 'posted'
  AND transactions.deleted_at IS NULL
  AND transactions.kind <> 'transfer'
  AND transactions.transaction_date >= sqlc.arg(from_date)::DATE
  AND transactions.transaction_date <= sqlc.arg(to_date)::DATE
GROUP BY accounts.id
HAVING COUNT(DISTINCT transactions.id) > 0
ORDER BY lower(accounts.name), accounts.id;
