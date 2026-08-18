-- name: GetWorkspaceBaseCurrency :one
SELECT base_currency
FROM workspaces
WHERE id = $1;

-- name: GetWorkspaceMemberRole :one
SELECT role
FROM workspace_members
WHERE workspace_id = $1
  AND user_id = $2
  AND removed_at IS NULL;

-- name: GetWorkspaceName :one
SELECT name
FROM workspaces
WHERE id = $1;
