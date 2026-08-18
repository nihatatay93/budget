-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, display_name)
VALUES ($1, $2, $3, $4)
RETURNING id, email, display_name;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, display_name
FROM users
WHERE lower(email) = lower($1);

-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, base_currency, timezone, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, base_currency, timezone;

-- name: AddWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3);

-- name: CreateSystemCategory :exec
INSERT INTO categories (id, workspace_id, name, kind, system_key)
VALUES ($1, $2, $3, $4, $5);

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, transport, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSessionByTokenHash :one
SELECT
    sessions.id AS session_id,
    sessions.user_id,
    sessions.transport,
    users.email,
    users.display_name
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = $1
  AND sessions.expires_at > now();

-- name: DeleteSession :execrows
DELETE FROM sessions
WHERE id = $1
  AND user_id = $2;

-- name: ListWorkspacesByUser :many
SELECT
    workspaces.id,
    workspaces.name,
    workspaces.base_currency,
    workspaces.timezone,
    workspace_members.role
FROM workspace_members
JOIN workspaces ON workspaces.id = workspace_members.workspace_id
WHERE workspace_members.user_id = $1
  AND workspace_members.removed_at IS NULL
ORDER BY workspace_members.joined_at, workspaces.id;
