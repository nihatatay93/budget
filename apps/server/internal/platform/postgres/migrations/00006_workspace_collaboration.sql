-- +goose Up

ALTER TABLE workspace_members
    ADD COLUMN removed_at TIMESTAMPTZ;

-- Registration has always created an owner atomically. Repair any manually-created legacy
-- workspace that does not satisfy that application invariant before enforcing it globally.
INSERT INTO workspace_members (workspace_id, user_id, role)
SELECT workspaces.id, workspaces.created_by, 'owner'
FROM workspaces
WHERE NOT EXISTS (
    SELECT 1
    FROM workspace_members
    WHERE workspace_members.workspace_id = workspaces.id
      AND workspace_members.role = 'owner'
      AND workspace_members.removed_at IS NULL
)
ON CONFLICT (workspace_id, user_id) DO UPDATE
SET role = 'owner',
    joined_at = now(),
    removed_at = NULL;

ALTER TABLE workspace_invitations
    ADD COLUMN accepted_by UUID,
    ADD COLUMN revoked_at TIMESTAMPTZ;

UPDATE workspace_invitations
SET email = lower(btrim(email));

-- Owner invitations were never exposed by the application. Preserve any accepted historical
-- row, but revoke an unconsumed manually-created owner invitation before narrowing the policy.
UPDATE workspace_invitations
SET revoked_at = now()
WHERE role = 'owner'
  AND accepted_at IS NULL
  AND revoked_at IS NULL;

-- Keep the newest unconsumed row before adding the one-open-invitation uniqueness boundary.
WITH ranked AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY workspace_id, lower(email)
            ORDER BY created_at DESC, id DESC
        ) AS position
    FROM workspace_invitations
    WHERE accepted_at IS NULL
      AND revoked_at IS NULL
)
UPDATE workspace_invitations AS invitation
SET revoked_at = now()
FROM ranked
WHERE invitation.id = ranked.id
  AND ranked.position > 1;

ALTER TABLE workspace_invitations
    DROP CONSTRAINT workspace_invitations_role_check,
    ADD CONSTRAINT workspace_invitations_role_policy CHECK (
        role IN ('admin', 'member', 'viewer')
        OR (role = 'owner' AND accepted_at IS NOT NULL)
    ),
    ADD CONSTRAINT workspace_invitations_acceptance_actor CHECK (
        accepted_by IS NULL OR accepted_at IS NOT NULL
    ),
    ADD CONSTRAINT workspace_invitations_terminal_state CHECK (
        accepted_at IS NULL OR revoked_at IS NULL
    ),
    ADD CONSTRAINT workspace_invitations_accepted_by_member
        FOREIGN KEY (workspace_id, accepted_by)
        REFERENCES workspace_members(workspace_id, user_id);

CREATE UNIQUE INDEX workspace_invitations_one_open_email
    ON workspace_invitations (workspace_id, lower(email))
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE INDEX workspace_members_active_workspace_idx
    ON workspace_members (workspace_id, joined_at, user_id)
    WHERE removed_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION enforce_workspace_active_owner()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_workspace_id UUID;
BEGIN
    IF TG_TABLE_NAME = 'workspaces' THEN
        target_workspace_id := NEW.id;
    ELSE
        target_workspace_id := CASE
            WHEN TG_OP = 'DELETE' THEN OLD.workspace_id
            ELSE NEW.workspace_id
        END;
    END IF;

    -- Workspace deletion intentionally cascades all memberships.
    IF NOT EXISTS (SELECT 1 FROM workspaces WHERE id = target_workspace_id) THEN
        RETURN NULL;
    END IF;

    -- Serialize owner-affecting writes for this workspace. The deferred check then observes
    -- earlier commits and prevents two owners from concurrently removing the final owner.
    PERFORM 1
    FROM workspaces
    WHERE id = target_workspace_id
    FOR UPDATE;

    IF NOT EXISTS (
        SELECT 1
        FROM workspace_members
        WHERE workspace_id = target_workspace_id
          AND role = 'owner'
          AND removed_at IS NULL
    ) THEN
        RAISE EXCEPTION 'workspace % must retain an active owner', target_workspace_id
            USING ERRCODE = '23514',
                  CONSTRAINT = 'workspace_members_active_owner_required';
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER workspace_members_require_active_owner
AFTER INSERT OR UPDATE OR DELETE ON workspace_members
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION enforce_workspace_active_owner();

CREATE CONSTRAINT TRIGGER workspaces_require_active_owner
AFTER INSERT OR UPDATE ON workspaces
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION enforce_workspace_active_owner();

-- +goose Down

DROP TRIGGER workspaces_require_active_owner ON workspaces;
DROP TRIGGER workspace_members_require_active_owner ON workspace_members;
DROP FUNCTION enforce_workspace_active_owner();

DROP INDEX workspace_members_active_workspace_idx;
DROP INDEX workspace_invitations_one_open_email;

ALTER TABLE workspace_invitations
    DROP CONSTRAINT workspace_invitations_accepted_by_member,
    DROP CONSTRAINT workspace_invitations_terminal_state,
    DROP CONSTRAINT workspace_invitations_acceptance_actor,
    DROP CONSTRAINT workspace_invitations_role_policy,
    ADD CONSTRAINT workspace_invitations_role_check
        CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    DROP COLUMN revoked_at,
    DROP COLUMN accepted_by;

ALTER TABLE workspace_members
    DROP COLUMN removed_at;
