-- name: ListCategories :many
SELECT
    id,
    workspace_id,
    parent_id,
    name,
    kind,
    system_key,
    icon,
    archived_at
FROM categories
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (sqlc.arg(include_archived)::BOOLEAN OR archived_at IS NULL)
ORDER BY kind, parent_id NULLS FIRST, lower(name), id;

-- name: GetCategory :one
SELECT
    id,
    workspace_id,
    parent_id,
    name,
    kind,
    system_key,
    icon,
    archived_at
FROM categories
WHERE workspace_id = $1
  AND id = $2;

-- name: CreateCategory :exec
INSERT INTO categories (
    id, workspace_id, parent_id, name, kind, icon
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: UpdateCategory :execrows
UPDATE categories
SET parent_id = sqlc.narg(parent_id),
    name = sqlc.arg(name),
    kind = sqlc.arg(kind),
    icon = sqlc.narg(icon),
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: ArchiveCategory :execrows
UPDATE categories
SET archived_at = COALESCE(archived_at, now()),
    updated_at = CASE WHEN archived_at IS NULL THEN now() ELSE updated_at END
WHERE workspace_id = $1
  AND id = $2;

-- name: CategoryHasChildren :one
SELECT EXISTS (
    SELECT 1
    FROM categories
    WHERE workspace_id = $1
      AND parent_id = $2
      AND archived_at IS NULL
);

-- name: CategoryHasAnyChildren :one
SELECT EXISTS (
    SELECT 1
    FROM categories
    WHERE workspace_id = $1
      AND parent_id = $2
);

-- name: CategoryHasAllocations :one
SELECT EXISTS (
    SELECT 1
    FROM transaction_allocations
    WHERE workspace_id = $1
      AND category_id = $2
);
