-- name: LockWorkspaceForCollaboration :one
SELECT id
FROM workspaces
WHERE id = $1
FOR UPDATE;

-- name: ListActiveWorkspaceMembers :many
SELECT
    workspace_members.user_id,
    users.email,
    users.display_name,
    workspace_members.role,
    workspace_members.joined_at
FROM workspace_members
JOIN users ON users.id = workspace_members.user_id
WHERE workspace_members.workspace_id = $1
  AND workspace_members.removed_at IS NULL
ORDER BY
    CASE workspace_members.role
        WHEN 'owner' THEN 1
        WHEN 'admin' THEN 2
        WHEN 'member' THEN 3
        ELSE 4
    END,
    lower(users.display_name),
    workspace_members.user_id;

-- name: GetActiveWorkspaceMember :one
SELECT
    workspace_members.user_id,
    users.email,
    users.display_name,
    workspace_members.role,
    workspace_members.joined_at
FROM workspace_members
JOIN users ON users.id = workspace_members.user_id
WHERE workspace_members.workspace_id = $1
  AND workspace_members.user_id = $2
  AND workspace_members.removed_at IS NULL;

-- name: GetActiveWorkspaceMemberByEmail :one
SELECT workspace_members.user_id
FROM workspace_members
JOIN users ON users.id = workspace_members.user_id
WHERE workspace_members.workspace_id = $1
  AND lower(users.email) = lower($2)
  AND workspace_members.removed_at IS NULL;

-- name: CountActiveWorkspaceOwners :one
SELECT count(*)
FROM workspace_members
WHERE workspace_id = $1
  AND role = 'owner'
  AND removed_at IS NULL;

-- name: ListPendingWorkspaceInvitations :many
SELECT
    workspace_invitations.id,
    workspace_invitations.workspace_id,
    workspace_invitations.email,
    workspace_invitations.role,
    workspace_invitations.invited_by,
    users.display_name AS inviter_display_name,
    workspace_invitations.expires_at,
    workspace_invitations.created_at
FROM workspace_invitations
JOIN users ON users.id = workspace_invitations.invited_by
WHERE workspace_invitations.workspace_id = $1
  AND workspace_invitations.accepted_at IS NULL
  AND workspace_invitations.revoked_at IS NULL
  AND workspace_invitations.expires_at > $2
ORDER BY workspace_invitations.created_at DESC, workspace_invitations.id DESC;

-- name: GetPendingWorkspaceInvitation :one
SELECT
    workspace_invitations.id,
    workspace_invitations.workspace_id,
    workspace_invitations.email,
    workspace_invitations.role,
    workspace_invitations.invited_by,
    users.display_name AS inviter_display_name,
    workspace_invitations.expires_at,
    workspace_invitations.created_at
FROM workspace_invitations
JOIN users ON users.id = workspace_invitations.invited_by
WHERE workspace_invitations.workspace_id = $1
  AND workspace_invitations.id = $2
  AND workspace_invitations.accepted_at IS NULL
  AND workspace_invitations.revoked_at IS NULL
  AND workspace_invitations.expires_at > $3;

-- name: RevokeOpenWorkspaceInvitationsForEmail :exec
UPDATE workspace_invitations
SET revoked_at = $3
WHERE workspace_id = $1
  AND lower(email) = lower($2)
  AND accepted_at IS NULL
  AND revoked_at IS NULL;

-- name: CreateWorkspaceInvitation :one
INSERT INTO workspace_invitations (
    id, workspace_id, email, role, token_hash, invited_by, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, workspace_id, email, role, invited_by, expires_at, created_at;

-- name: RevokeWorkspaceInvitation :execrows
UPDATE workspace_invitations
SET revoked_at = $3
WHERE workspace_id = $1
  AND id = $2
  AND accepted_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > $3;

-- name: GetWorkspaceInvitationByTokenHash :one
SELECT
    id,
    workspace_id,
    email,
    role,
    invited_by,
    expires_at,
    accepted_at,
    accepted_by,
    revoked_at,
    created_at
FROM workspace_invitations
WHERE token_hash = $1;

-- name: GetWorkspaceInvitationByTokenHashForUpdate :one
SELECT
    id,
    workspace_id,
    email,
    role,
    invited_by,
    expires_at,
    accepted_at,
    accepted_by,
    revoked_at,
    created_at
FROM workspace_invitations
WHERE token_hash = $1
FOR UPDATE;

-- name: GetWorkspaceMembershipForUpdate :one
SELECT role, joined_at, removed_at
FROM workspace_members
WHERE workspace_id = $1
  AND user_id = $2
FOR UPDATE;

-- name: ActivateWorkspaceMembership :one
INSERT INTO workspace_members (workspace_id, user_id, role, joined_at, removed_at)
VALUES ($1, $2, $3, $4, NULL)
ON CONFLICT (workspace_id, user_id) DO UPDATE
SET role = EXCLUDED.role,
    joined_at = EXCLUDED.joined_at,
    removed_at = NULL
RETURNING role, joined_at;

-- name: AcceptWorkspaceInvitation :execrows
UPDATE workspace_invitations
SET accepted_at = $3,
    accepted_by = $4
WHERE id = $1
  AND workspace_id = $2
  AND accepted_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > $3;

-- name: UpdateWorkspaceMembershipRole :execrows
UPDATE workspace_members
SET role = $3
WHERE workspace_id = $1
  AND user_id = $2
  AND removed_at IS NULL;

-- name: RemoveWorkspaceMembership :execrows
UPDATE workspace_members
SET removed_at = $3
WHERE workspace_id = $1
  AND user_id = $2
  AND removed_at IS NULL;

-- name: GetAcceptedWorkspaceSummary :one
SELECT
    workspaces.id,
    workspaces.name,
    workspaces.base_currency,
    workspaces.timezone,
    workspace_members.role
FROM workspaces
JOIN workspace_members ON workspace_members.workspace_id = workspaces.id
WHERE workspaces.id = $1
  AND workspace_members.user_id = $2
  AND workspace_members.removed_at IS NULL;
