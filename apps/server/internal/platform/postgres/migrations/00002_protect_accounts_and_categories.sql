-- +goose Up

-- The guards below supersede the first-migration triggers. Drop the originals so a single
-- guard owns each rule: the old category trigger returns NEW on DELETE, which is NULL for a
-- delete and therefore silently cancels every category deletion, and the old currency
-- trigger raises without a constraint name that the repository can map to a domain error.
DROP TRIGGER IF EXISTS categories_protect_system_category ON categories;
DROP FUNCTION IF EXISTS protect_system_category();
DROP TRIGGER IF EXISTS accounts_protect_currency ON accounts;
DROP FUNCTION IF EXISTS protect_account_currency();

-- +goose StatementBegin
CREATE FUNCTION protect_account_currency_with_history()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.currency IS DISTINCT FROM OLD.currency
       AND EXISTS (
           SELECT 1
           FROM transaction_entries
           WHERE workspace_id = OLD.workspace_id
             AND account_id = OLD.id
       ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'accounts_currency_with_history',
            MESSAGE = 'account currency cannot change after financial history exists';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER accounts_currency_with_history
BEFORE UPDATE OF currency ON accounts
FOR EACH ROW
EXECUTE FUNCTION protect_account_currency_with_history();

-- +goose StatementBegin
CREATE FUNCTION protect_category_structure()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    parent_kind TEXT;
    parent_archived_at TIMESTAMPTZ;
BEGIN
    IF TG_OP = 'DELETE' THEN
        -- Protect system categories only while their workspace still exists. A workspace
        -- delete cascades into categories after the workspace row is gone, so an
        -- unconditional guard here would make workspaces impossible to delete.
        IF OLD.system_key IS NOT NULL AND EXISTS (
            SELECT 1 FROM workspaces WHERE id = OLD.workspace_id
        ) THEN
            RAISE EXCEPTION USING
                ERRCODE = '23514',
                CONSTRAINT = 'categories_system_protected',
                MESSAGE = 'system categories cannot be deleted';
        END IF;
        RETURN OLD;
    END IF;

    IF TG_OP = 'UPDATE'
       AND OLD.system_key IS NOT NULL
       AND (
           NEW.workspace_id IS DISTINCT FROM OLD.workspace_id OR
           NEW.parent_id IS DISTINCT FROM OLD.parent_id OR
           NEW.name IS DISTINCT FROM OLD.name OR
           NEW.kind IS DISTINCT FROM OLD.kind OR
           NEW.system_key IS DISTINCT FROM OLD.system_key OR
           NEW.archived_at IS DISTINCT FROM OLD.archived_at
       ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'categories_system_protected',
            MESSAGE = 'system categories are protected';
    END IF;

    IF NEW.parent_id IS NOT NULL THEN
        SELECT kind, archived_at
        INTO parent_kind, parent_archived_at
        FROM categories
        WHERE id = NEW.parent_id
          AND workspace_id = NEW.workspace_id;

        IF FOUND AND (parent_kind IS DISTINCT FROM NEW.kind OR parent_archived_at IS NOT NULL) THEN
            RAISE EXCEPTION USING
                ERRCODE = '23514',
                CONSTRAINT = 'categories_parent_compatible',
                MESSAGE = 'category parent must be active and have the same kind';
        END IF;

        IF NEW.id = NEW.parent_id OR EXISTS (
            WITH RECURSIVE ancestors AS (
                SELECT id, parent_id
                FROM categories
                WHERE id = NEW.parent_id
                  AND workspace_id = NEW.workspace_id
                UNION ALL
                SELECT parent.id, parent.parent_id
                FROM categories parent
                JOIN ancestors child ON child.parent_id = parent.id
                WHERE parent.workspace_id = NEW.workspace_id
            )
            SELECT 1 FROM ancestors WHERE id = NEW.id
        ) THEN
            RAISE EXCEPTION USING
                ERRCODE = '23514',
                CONSTRAINT = 'categories_hierarchy_acyclic',
                MESSAGE = 'category hierarchy cannot contain a cycle';
        END IF;
    END IF;

    IF TG_OP = 'UPDATE' AND NEW.kind IS DISTINCT FROM OLD.kind AND (
        EXISTS (
            SELECT 1 FROM categories
            WHERE workspace_id = OLD.workspace_id
              AND parent_id = OLD.id
        ) OR EXISTS (
            SELECT 1 FROM transaction_allocations
            WHERE workspace_id = OLD.workspace_id
              AND category_id = OLD.id
        )
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'categories_kind_with_relationships',
            MESSAGE = 'category kind cannot change while relationships exist';
    END IF;

    IF TG_OP = 'UPDATE'
       AND OLD.archived_at IS NULL
       AND NEW.archived_at IS NOT NULL
       AND EXISTS (
           SELECT 1 FROM categories
           WHERE workspace_id = OLD.workspace_id
             AND parent_id = OLD.id
             AND archived_at IS NULL
       ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'categories_active_children',
            MESSAGE = 'category with active children cannot be archived';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER categories_protect_structure_update
BEFORE INSERT OR UPDATE ON categories
FOR EACH ROW
EXECUTE FUNCTION protect_category_structure();

CREATE TRIGGER categories_protect_structure_delete
BEFORE DELETE ON categories
FOR EACH ROW
EXECUTE FUNCTION protect_category_structure();

-- +goose Down

DROP TRIGGER IF EXISTS categories_protect_structure_delete ON categories;
DROP TRIGGER IF EXISTS categories_protect_structure_update ON categories;
DROP FUNCTION IF EXISTS protect_category_structure();
DROP TRIGGER IF EXISTS accounts_currency_with_history ON accounts;
DROP FUNCTION IF EXISTS protect_account_currency_with_history();

-- Restore the first-migration guards this migration replaced.
-- +goose StatementBegin
CREATE FUNCTION protect_account_currency()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.currency <> OLD.currency AND EXISTS (
        SELECT 1 FROM transaction_entries WHERE account_id = OLD.id
    ) THEN
        RAISE EXCEPTION 'account currency cannot change after financial history exists';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER accounts_protect_currency
BEFORE UPDATE OF currency ON accounts
FOR EACH ROW
EXECUTE FUNCTION protect_account_currency();

-- +goose StatementBegin
CREATE FUNCTION protect_system_category()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' AND OLD.system_key IS NOT NULL THEN
        RAISE EXCEPTION 'protected system categories cannot be deleted';
    END IF;

    IF OLD.system_key IS NOT NULL AND (
        NEW.system_key IS DISTINCT FROM OLD.system_key OR
        NEW.workspace_id IS DISTINCT FROM OLD.workspace_id OR
        NEW.name IS DISTINCT FROM OLD.name OR
        NEW.kind IS DISTINCT FROM OLD.kind OR
        NEW.archived_at IS DISTINCT FROM OLD.archived_at
    ) THEN
        RAISE EXCEPTION 'protected system categories cannot be repurposed or archived';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER categories_protect_system_category
BEFORE UPDATE OR DELETE ON categories
FOR EACH ROW
EXECUTE FUNCTION protect_system_category();
