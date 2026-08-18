-- +goose Up

-- Give reconciliation failures a stable constraint name so repository adapters can map a
-- deferred commit failure to the same domain error returned by pre-persistence validation.
-- NUMERIC totals also avoid an intermediate BIGINT overflow hiding the actual invariant.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_transaction_reconciliation(target_transaction_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    transaction_kind TEXT;
    entry_total NUMERIC;
    entry_count BIGINT;
    allocation_total NUMERIC;
    allocation_count BIGINT;
BEGIN
    SELECT kind
    INTO transaction_kind
    FROM transactions
    WHERE id = target_transaction_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT COALESCE(SUM(base_amount_minor), 0), COUNT(*)
    INTO entry_total, entry_count
    FROM transaction_entries
    WHERE transaction_id = target_transaction_id;

    SELECT COALESCE(SUM(amount_base_minor), 0), COUNT(*)
    INTO allocation_total, allocation_count
    FROM transaction_allocations
    WHERE transaction_id = target_transaction_id;

    IF entry_count = 0 THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            CONSTRAINT = 'transactions_reconciliation',
            MESSAGE = 'transaction must contain at least one entry';
    ELSIF transaction_kind = 'transfer' THEN
        IF entry_count < 2 OR allocation_count <> 0 OR entry_total <> 0 THEN
            RAISE EXCEPTION USING
                ERRCODE = '23514',
                CONSTRAINT = 'transactions_reconciliation',
                MESSAGE = 'transfer transaction does not reconcile';
        END IF;
    ELSIF transaction_kind = 'standard' THEN
        IF allocation_count = 0 OR allocation_total <> entry_total THEN
            RAISE EXCEPTION USING
                ERRCODE = '23514',
                CONSTRAINT = 'transactions_reconciliation',
                MESSAGE = 'standard transaction does not reconcile';
        END IF;
    ELSIF transaction_kind = 'adjustment' THEN
        IF allocation_count > 0 AND allocation_total <> entry_total THEN
            RAISE EXCEPTION USING
                ERRCODE = '23514',
                CONSTRAINT = 'transactions_reconciliation',
                MESSAGE = 'adjustment transaction does not reconcile';
        END IF;
    END IF;

    RETURN;
END;
$$;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_transaction_reconciliation(target_transaction_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    transaction_kind TEXT;
    entry_total BIGINT;
    allocation_total BIGINT;
    allocation_count BIGINT;
BEGIN
    SELECT kind INTO transaction_kind FROM transactions WHERE id = target_transaction_id;
    IF NOT FOUND THEN RETURN; END IF;
    SELECT COALESCE(SUM(base_amount_minor), 0) INTO entry_total
    FROM transaction_entries WHERE transaction_id = target_transaction_id;
    SELECT COALESCE(SUM(amount_base_minor), 0), COUNT(*)
    INTO allocation_total, allocation_count
    FROM transaction_allocations WHERE transaction_id = target_transaction_id;
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
END;
$$;
-- +goose StatementEnd
