-- +goose Up

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    transport TEXT NOT NULL CHECK (transport IN ('cookie', 'bearer')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL,
    base_currency TEXT NOT NULL CHECK (base_currency ~ '^[A-Z]{3}$'),
    timezone TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE workspace_invitations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    token_hash BYTEA NOT NULL UNIQUE,
    invited_by UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, invited_by)
        REFERENCES workspace_members(workspace_id, user_id)
);

CREATE INDEX workspace_invitations_workspace_id_idx
    ON workspace_invitations (workspace_id);
CREATE INDEX workspace_invitations_email_idx
    ON workspace_invitations (lower(email));

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN (
        'bank', 'cash', 'credit_card', 'savings', 'investment', 'other'
    )),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    institution_name TEXT,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id)
);

CREATE INDEX accounts_workspace_id_idx ON accounts (workspace_id);

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    parent_id UUID,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('expense', 'income')),
    system_key TEXT CHECK (
        system_key IS NULL OR
        system_key IN ('uncategorized_expense', 'uncategorized_income')
    ),
    icon TEXT,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id),
    FOREIGN KEY (parent_id, workspace_id)
        REFERENCES categories(id, workspace_id)
);

CREATE UNIQUE INDEX categories_system_key_unique
    ON categories (workspace_id, system_key)
    WHERE system_key IS NOT NULL;
CREATE INDEX categories_workspace_id_idx ON categories (workspace_id);

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('standard', 'transfer', 'adjustment')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'posted')),
    transaction_date DATE NOT NULL,
    payee TEXT,
    description TEXT,
    notes TEXT,
    source TEXT NOT NULL CHECK (source IN ('manual', 'recurring', 'import', 'api')),
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (id, workspace_id),
    FOREIGN KEY (workspace_id, created_by)
        REFERENCES workspace_members(workspace_id, user_id),
    FOREIGN KEY (workspace_id, updated_by)
        REFERENCES workspace_members(workspace_id, user_id)
);

CREATE INDEX transactions_workspace_date_idx
    ON transactions (workspace_id, transaction_date DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE transaction_entries (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    transaction_id UUID NOT NULL,
    account_id UUID NOT NULL,
    amount_minor BIGINT NOT NULL,
    base_amount_minor BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (transaction_id, workspace_id)
        REFERENCES transactions(id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (account_id, workspace_id)
        REFERENCES accounts(id, workspace_id)
);

CREATE INDEX transaction_entries_transaction_id_idx
    ON transaction_entries (transaction_id);
CREATE INDEX transaction_entries_account_id_idx
    ON transaction_entries (workspace_id, account_id);

CREATE TABLE transaction_allocations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    transaction_id UUID NOT NULL,
    category_id UUID NOT NULL,
    amount_base_minor BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (transaction_id, workspace_id)
        REFERENCES transactions(id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id, workspace_id)
        REFERENCES categories(id, workspace_id)
);

CREATE INDEX transaction_allocations_transaction_id_idx
    ON transaction_allocations (transaction_id);
CREATE INDEX transaction_allocations_category_id_idx
    ON transaction_allocations (workspace_id, category_id);

CREATE TABLE budgets (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    period TEXT NOT NULL CHECK (period IN ('monthly')),
    starts_on DATE NOT NULL,
    rollover BOOLEAN NOT NULL DEFAULT false,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id)
);

CREATE INDEX budgets_workspace_id_idx ON budgets (workspace_id);

CREATE TABLE budget_items (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    budget_id UUID NOT NULL,
    category_id UUID NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    FOREIGN KEY (budget_id, workspace_id)
        REFERENCES budgets(id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id, workspace_id)
        REFERENCES categories(id, workspace_id),
    UNIQUE (budget_id, category_id)
);

-- +goose StatementBegin
CREATE FUNCTION assert_transaction_reconciliation(target_transaction_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    transaction_kind TEXT;
    entry_total BIGINT;
    allocation_total BIGINT;
    allocation_count BIGINT;
BEGIN
    SELECT kind
    INTO transaction_kind
    FROM transactions
    WHERE id = target_transaction_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT COALESCE(SUM(base_amount_minor), 0)
    INTO entry_total
    FROM transaction_entries
    WHERE transaction_id = target_transaction_id;

    SELECT COALESCE(SUM(amount_base_minor), 0), COUNT(*)
    INTO allocation_total, allocation_count
    FROM transaction_allocations
    WHERE transaction_id = target_transaction_id;

    IF transaction_kind = 'transfer' THEN
        IF allocation_count <> 0 OR entry_total <> 0 THEN
            RAISE EXCEPTION 'transfer transaction % does not reconcile', target_transaction_id;
        END IF;
    ELSIF transaction_kind = 'standard' THEN
        IF allocation_count = 0 OR allocation_total <> entry_total THEN
            RAISE EXCEPTION 'standard transaction % does not reconcile', target_transaction_id;
        END IF;
    ELSIF transaction_kind = 'adjustment' THEN
        IF allocation_count > 0 AND allocation_total <> entry_total THEN
            RAISE EXCEPTION 'adjustment transaction % does not reconcile', target_transaction_id;
        END IF;
    END IF;

    RETURN;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION check_transaction_reconciliation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_transaction_id UUID;
BEGIN
    IF TG_TABLE_NAME = 'transactions' THEN
        target_transaction_id := NEW.id;
    ELSE
        IF TG_OP = 'DELETE' THEN
            target_transaction_id := OLD.transaction_id;
        ELSE
            target_transaction_id := NEW.transaction_id;
        END IF;

        IF TG_OP = 'UPDATE'
            AND OLD.transaction_id IS DISTINCT FROM NEW.transaction_id THEN
            PERFORM assert_transaction_reconciliation(OLD.transaction_id);
        END IF;
    END IF;

    PERFORM assert_transaction_reconciliation(target_transaction_id);
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER transactions_reconcile
AFTER INSERT OR UPDATE OF kind ON transactions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION check_transaction_reconciliation();

CREATE CONSTRAINT TRIGGER transaction_entries_reconcile
AFTER INSERT OR UPDATE OR DELETE ON transaction_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION check_transaction_reconciliation();

CREATE CONSTRAINT TRIGGER transaction_allocations_reconcile
AFTER INSERT OR UPDATE OR DELETE ON transaction_allocations
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION check_transaction_reconciliation();

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

-- +goose Down

DROP TABLE budget_items;
DROP TABLE budgets;
DROP TABLE transaction_allocations;
DROP TABLE transaction_entries;
DROP TABLE transactions;
DROP TABLE categories;
DROP TABLE accounts;
DROP TABLE workspace_invitations;
DROP TABLE workspace_members;
DROP TABLE workspaces;
DROP TABLE sessions;
DROP TABLE users;
DROP FUNCTION protect_system_category();
DROP FUNCTION protect_account_currency();
DROP FUNCTION check_transaction_reconciliation();
DROP FUNCTION assert_transaction_reconciliation(UUID);
