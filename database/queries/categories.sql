-- name: ListCategories :many
SELECT
    id,
    workspace_id,
    parent_id,
    name,
    kind,
    system_key,
    predefined_key,
    icon,
    icon_type,
    icon_value,
    color_key,
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
    predefined_key,
    icon,
    icon_type,
    icon_value,
    color_key,
    archived_at
FROM categories
WHERE workspace_id = $1
  AND id = $2;

-- name: CreateCategory :exec
INSERT INTO categories (
    id, workspace_id, parent_id, name, kind, icon, icon_type, icon_value, color_key
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: CreatePredefinedCategory :exec
-- A member resolves its group by predefined key inside its own workspace, so seeding never
-- has to carry identifiers between statements. Groups are inserted before their members.
-- The conflict clause refreshes server-owned appearance but only fills in a parent that is
-- still absent, so a workspace that has rearranged its own hierarchy keeps that arrangement.
INSERT INTO categories (
    workspace_id, parent_id, name, kind, predefined_key, icon, icon_type, icon_value, color_key
) VALUES (
    $1,
    CASE WHEN sqlc.narg(parent_key)::TEXT IS NULL THEN NULL ELSE (
        SELECT parent.id FROM categories parent
        WHERE parent.workspace_id = $1 AND parent.predefined_key = sqlc.narg(parent_key)::TEXT
    ) END,
    $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (workspace_id, predefined_key) WHERE predefined_key IS NOT NULL DO UPDATE
SET parent_id = COALESCE(categories.parent_id, EXCLUDED.parent_id),
    icon = EXCLUDED.icon,
    icon_type = EXCLUDED.icon_type,
    icon_value = EXCLUDED.icon_value,
    color_key = EXCLUDED.color_key,
    updated_at = now();

-- name: UpdateCategory :execrows
UPDATE categories
SET parent_id = sqlc.narg(parent_id),
    name = sqlc.arg(name),
    kind = sqlc.arg(kind),
    icon = sqlc.narg(icon),
    icon_type = sqlc.arg(icon_type),
    icon_value = sqlc.arg(icon_value),
    color_key = sqlc.arg(color_key),
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
