-- +goose Up

ALTER TABLE budget_items
    RENAME COLUMN amount_minor TO amount_base_minor;

ALTER TABLE budget_items
    DROP CONSTRAINT budget_items_amount_minor_check,
    ADD CONSTRAINT budget_items_amount_base_minor_positive
        CHECK (amount_base_minor > 0);

ALTER TABLE budgets
    ADD CONSTRAINT budgets_name_valid
        CHECK (name = btrim(name) AND name <> '' AND char_length(name) <= 100),
    ADD CONSTRAINT budgets_month_start
        CHECK (starts_on = date_trunc('month', starts_on)::DATE),
    ADD CONSTRAINT budgets_rollover_disabled
        CHECK (rollover = false),
    ADD CONSTRAINT budgets_active_legacy_value
        CHECK (active = true),
    ADD CONSTRAINT budgets_workspace_month_unique
        UNIQUE (workspace_id, starts_on);

CREATE INDEX budget_items_workspace_budget_idx
    ON budget_items (workspace_id, budget_id);

-- Expense-only category assignment and archived-category retention are row-local rules.
-- Existing archived assignments remain editable, but a new assignment cannot be made to
-- an archived category.
-- +goose StatementBegin
CREATE FUNCTION protect_budget_item_category()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    category_kind TEXT;
    category_archived_at TIMESTAMPTZ;
BEGIN
    SELECT kind, archived_at
    INTO category_kind, category_archived_at
    FROM categories
    WHERE id = NEW.category_id
      AND workspace_id = NEW.workspace_id;

    IF FOUND AND category_kind <> 'expense' THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'budget_items_expense_category',
            MESSAGE = 'budget items must target expense categories';
    END IF;

    IF FOUND
       AND category_archived_at IS NOT NULL
       AND (
           (
               TG_OP = 'INSERT'
               AND NOT EXISTS (
                   SELECT 1
                   FROM budget_items
                   WHERE workspace_id = NEW.workspace_id
                     AND budget_id = NEW.budget_id
                     AND category_id = NEW.category_id
               )
           )
           OR (TG_OP = 'UPDATE' AND NEW.category_id IS DISTINCT FROM OLD.category_id)
       ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'budget_items_new_active_category',
            MESSAGE = 'archived categories cannot receive new budget items';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER budget_items_protect_category
BEFORE INSERT OR UPDATE OF workspace_id, category_id ON budget_items
FOR EACH ROW
EXECUTE FUNCTION protect_budget_item_category();

-- A budget may target a parent or a leaf, but never both an ancestor and one of its
-- descendants. This workspace-wide deferred assertion also protects the invariant when a
-- contributor reparents a category after a budget has been created.
-- +goose StatementBegin
CREATE FUNCTION assert_workspace_budget_category_branches(target_workspace_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        WITH RECURSIVE ancestry AS (
            SELECT id AS descendant_id, parent_id AS ancestor_id
            FROM categories
            WHERE workspace_id = target_workspace_id

            UNION ALL

            SELECT ancestry.descendant_id, parent.parent_id
            FROM ancestry
            JOIN categories AS parent
              ON parent.workspace_id = target_workspace_id
             AND parent.id = ancestry.ancestor_id
            WHERE ancestry.ancestor_id IS NOT NULL
        )
        SELECT 1
        FROM budget_items AS descendant_item
        JOIN ancestry
          ON ancestry.descendant_id = descendant_item.category_id
         AND ancestry.ancestor_id IS NOT NULL
        JOIN budget_items AS ancestor_item
          ON ancestor_item.workspace_id = descendant_item.workspace_id
         AND ancestor_item.budget_id = descendant_item.budget_id
         AND ancestor_item.category_id = ancestry.ancestor_id
        WHERE descendant_item.workspace_id = target_workspace_id
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'budget_items_non_overlapping_categories',
            MESSAGE = 'budget items cannot target overlapping category branches';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION check_budget_item_category_branches()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.workspace_id IS DISTINCT FROM NEW.workspace_id THEN
        PERFORM assert_workspace_budget_category_branches(OLD.workspace_id);
    END IF;
    PERFORM assert_workspace_budget_category_branches(
        CASE WHEN TG_OP = 'DELETE' THEN OLD.workspace_id ELSE NEW.workspace_id END
    );
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER budget_items_category_branches
AFTER INSERT OR UPDATE OR DELETE ON budget_items
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION check_budget_item_category_branches();

-- +goose StatementBegin
CREATE FUNCTION check_category_budget_branches()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.parent_id IS DISTINCT FROM OLD.parent_id THEN
        PERFORM assert_workspace_budget_category_branches(NEW.workspace_id);
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER categories_budget_branches
AFTER UPDATE ON categories
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION check_category_budget_branches();

-- +goose Down

DROP TRIGGER IF EXISTS categories_budget_branches ON categories;
DROP FUNCTION IF EXISTS check_category_budget_branches();
DROP TRIGGER IF EXISTS budget_items_category_branches ON budget_items;
DROP FUNCTION IF EXISTS check_budget_item_category_branches();
DROP FUNCTION IF EXISTS assert_workspace_budget_category_branches(UUID);
DROP TRIGGER IF EXISTS budget_items_protect_category ON budget_items;
DROP FUNCTION IF EXISTS protect_budget_item_category();

DROP INDEX IF EXISTS budget_items_workspace_budget_idx;

ALTER TABLE budgets
    DROP CONSTRAINT budgets_workspace_month_unique,
    DROP CONSTRAINT budgets_active_legacy_value,
    DROP CONSTRAINT budgets_rollover_disabled,
    DROP CONSTRAINT budgets_month_start,
    DROP CONSTRAINT budgets_name_valid;

ALTER TABLE budget_items
    DROP CONSTRAINT budget_items_amount_base_minor_positive,
    ADD CONSTRAINT budget_items_amount_minor_check CHECK (amount_base_minor >= 0);

ALTER TABLE budget_items
    RENAME COLUMN amount_base_minor TO amount_minor;
