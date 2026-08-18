package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
	transactiondomain "github.com/nihatatay93/budget/internal/transaction"
)

type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

func (r *TransactionRepository) List(
	ctx context.Context,
	workspaceID string,
	filter transactiondomain.ListFilter,
) ([]transactiondomain.Transaction, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlc.New(r.pool).ListTransactions(ctx, sqlc.ListTransactionsParams{
		WorkspaceID: workspaceUUID,
		FromDate:    postgresDatePointer(filter.From),
		ToDate:      postgresDatePointer(filter.To),
		ResultLimit: int32(filter.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	values := make([]transactiondomain.Transaction, 0, len(rows))
	for _, row := range rows {
		value, err := transactionFromDatabase(
			row.ID, row.WorkspaceID, row.Kind, row.Status, row.TransactionDate,
			row.Payee, row.Description, row.Notes, row.Source,
			row.CreatedBy, row.UpdatedBy, row.CreatedAt, row.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := r.loadRelations(ctx, workspaceUUID, values); err != nil {
		return nil, err
	}
	return values, nil
}

func (r *TransactionRepository) Get(
	ctx context.Context,
	workspaceID, transactionID string,
) (transactiondomain.Transaction, error) {
	workspaceUUID, transactionUUID, err := resourceUUIDs(workspaceID, transactionID)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	row, err := sqlc.New(r.pool).GetTransaction(ctx, sqlc.GetTransactionParams{
		WorkspaceID: workspaceUUID, ID: transactionUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return transactiondomain.Transaction{}, transactiondomain.ErrNotFound
	}
	if err != nil {
		return transactiondomain.Transaction{}, fmt.Errorf("get transaction: %w", err)
	}
	value, err := transactionFromDatabase(
		row.ID, row.WorkspaceID, row.Kind, row.Status, row.TransactionDate,
		row.Payee, row.Description, row.Notes, row.Source,
		row.CreatedBy, row.UpdatedBy, row.CreatedAt, row.UpdatedAt,
	)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	values := []transactiondomain.Transaction{value}
	if err := r.loadRelations(ctx, workspaceUUID, values); err != nil {
		return transactiondomain.Transaction{}, err
	}
	return values[0], nil
}

func (r *TransactionRepository) Create(
	ctx context.Context,
	value transactiondomain.Transaction,
) (transactiondomain.Transaction, error) {
	workspaceUUID, transactionUUID, err := resourceUUIDs(value.WorkspaceID, value.ID)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	actorUUID, err := postgresUUID(value.CreatedBy)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	databaseTransaction, err := r.pool.Begin(ctx)
	if err != nil {
		return transactiondomain.Transaction{}, fmt.Errorf("begin transaction aggregate create: %w", err)
	}
	defer func() { _ = databaseTransaction.Rollback(ctx) }()
	queries := sqlc.New(databaseTransaction)
	if err := queries.CreateTransaction(ctx, sqlc.CreateTransactionParams{
		ID: transactionUUID, WorkspaceID: workspaceUUID, Kind: string(value.Kind),
		Status: string(value.Status), TransactionDate: postgresDate(value.TransactionDate),
		Payee: postgresText(value.Payee), Description: postgresText(value.Description),
		Notes: postgresText(value.Notes), Source: string(value.Source), CreatedBy: actorUUID,
	}); err != nil {
		return transactiondomain.Transaction{}, mapTransactionWriteError("create transaction", err)
	}
	if err := writeTransactionRelations(ctx, queries, workspaceUUID, transactionUUID, value); err != nil {
		return transactiondomain.Transaction{}, err
	}
	if err := databaseTransaction.Commit(ctx); err != nil {
		return transactiondomain.Transaction{}, mapTransactionWriteError("commit transaction", err)
	}
	return r.Get(ctx, value.WorkspaceID, value.ID)
}

func (r *TransactionRepository) Update(
	ctx context.Context,
	value transactiondomain.Transaction,
) (transactiondomain.Transaction, error) {
	workspaceUUID, transactionUUID, err := resourceUUIDs(value.WorkspaceID, value.ID)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	actorUUID, err := postgresUUID(value.UpdatedBy)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	databaseTransaction, err := r.pool.Begin(ctx)
	if err != nil {
		return transactiondomain.Transaction{}, fmt.Errorf("begin transaction aggregate update: %w", err)
	}
	defer func() { _ = databaseTransaction.Rollback(ctx) }()
	queries := sqlc.New(databaseTransaction)
	rows, err := queries.UpdateTransaction(ctx, sqlc.UpdateTransactionParams{
		Kind: string(value.Kind), Status: string(value.Status),
		TransactionDate: postgresDate(value.TransactionDate),
		Payee:           postgresText(value.Payee), Description: postgresText(value.Description),
		Notes: postgresText(value.Notes), UpdatedBy: actorUUID,
		WorkspaceID: workspaceUUID, ID: transactionUUID,
	})
	if err != nil {
		return transactiondomain.Transaction{}, mapTransactionWriteError("update transaction", err)
	}
	if rows == 0 {
		return transactiondomain.Transaction{}, transactiondomain.ErrNotFound
	}
	if err := queries.DeleteTransactionAllocations(ctx, sqlc.DeleteTransactionAllocationsParams{
		WorkspaceID: workspaceUUID, TransactionID: transactionUUID,
	}); err != nil {
		return transactiondomain.Transaction{}, fmt.Errorf("replace transaction allocations: %w", err)
	}
	if err := queries.DeleteTransactionEntries(ctx, sqlc.DeleteTransactionEntriesParams{
		WorkspaceID: workspaceUUID, TransactionID: transactionUUID,
	}); err != nil {
		return transactiondomain.Transaction{}, fmt.Errorf("replace transaction entries: %w", err)
	}
	if err := writeTransactionRelations(ctx, queries, workspaceUUID, transactionUUID, value); err != nil {
		return transactiondomain.Transaction{}, err
	}
	if err := databaseTransaction.Commit(ctx); err != nil {
		return transactiondomain.Transaction{}, mapTransactionWriteError("commit transaction", err)
	}
	return r.Get(ctx, value.WorkspaceID, value.ID)
}

func (r *TransactionRepository) SoftDelete(
	ctx context.Context,
	workspaceID, transactionID, userID string,
) error {
	workspaceUUID, transactionUUID, err := resourceUUIDs(workspaceID, transactionID)
	if err != nil {
		return err
	}
	userUUID, err := postgresUUID(userID)
	if err != nil {
		return err
	}
	rows, err := sqlc.New(r.pool).SoftDeleteTransaction(ctx, sqlc.SoftDeleteTransactionParams{
		UpdatedBy: userUUID, WorkspaceID: workspaceUUID, ID: transactionUUID,
	})
	if err != nil {
		return fmt.Errorf("soft-delete transaction: %w", err)
	}
	if rows == 0 {
		return transactiondomain.ErrNotFound
	}
	return nil
}

func (r *TransactionRepository) WorkspaceBaseCurrency(
	ctx context.Context,
	workspaceID string,
) (money.Currency, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return "", err
	}
	value, err := sqlc.New(r.pool).GetWorkspaceBaseCurrency(ctx, workspaceUUID)
	if err != nil {
		return "", fmt.Errorf("get transaction workspace currency: %w", err)
	}
	currency, ok := money.Parse(value)
	if !ok {
		return "", transactiondomain.ErrReferenceInvalid
	}
	return currency, nil
}

func (r *TransactionRepository) AccountCurrency(
	ctx context.Context,
	workspaceID, accountID string,
) (money.Currency, error) {
	workspaceUUID, accountUUID, err := resourceUUIDs(workspaceID, accountID)
	if err != nil {
		return "", err
	}
	value, err := sqlc.New(r.pool).GetTransactionAccountCurrency(
		ctx, sqlc.GetTransactionAccountCurrencyParams{WorkspaceID: workspaceUUID, ID: accountUUID},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", transactiondomain.ErrReferenceInvalid
	}
	if err != nil {
		return "", fmt.Errorf("get transaction account currency: %w", err)
	}
	currency, ok := money.Parse(value)
	if !ok {
		return "", transactiondomain.ErrReferenceInvalid
	}
	return currency, nil
}

func (r *TransactionRepository) CategoryExists(
	ctx context.Context,
	workspaceID, categoryID string,
) (bool, error) {
	workspaceUUID, categoryUUID, err := resourceUUIDs(workspaceID, categoryID)
	if err != nil {
		return false, err
	}
	return sqlc.New(r.pool).TransactionCategoryExists(
		ctx, sqlc.TransactionCategoryExistsParams{WorkspaceID: workspaceUUID, ID: categoryUUID},
	)
}

func (r *TransactionRepository) SystemCategoryID(
	ctx context.Context,
	workspaceID, key string,
) (string, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return "", err
	}
	id, err := sqlc.New(r.pool).GetSystemCategoryID(ctx, sqlc.GetSystemCategoryIDParams{
		WorkspaceID: workspaceUUID, SystemKey: pgtype.Text{String: key, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", transactiondomain.ErrReferenceInvalid
	}
	if err != nil {
		return "", fmt.Errorf("get system category: %w", err)
	}
	return stringUUID(id)
}

func (r *TransactionRepository) loadRelations(
	ctx context.Context,
	workspaceID pgtype.UUID,
	values []transactiondomain.Transaction,
) error {
	if len(values) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, 0, len(values))
	indices := make(map[string]int, len(values))
	for index := range values {
		id, err := postgresUUID(values[index].ID)
		if err != nil {
			return err
		}
		ids = append(ids, id)
		indices[values[index].ID] = index
	}
	queries := sqlc.New(r.pool)
	entries, err := queries.ListTransactionEntries(ctx, sqlc.ListTransactionEntriesParams{
		WorkspaceID: workspaceID, TransactionIds: ids,
	})
	if err != nil {
		return fmt.Errorf("list transaction entries: %w", err)
	}
	for _, row := range entries {
		transactionID, err := stringUUID(row.TransactionID)
		if err != nil {
			return err
		}
		id, err := stringUUID(row.ID)
		if err != nil {
			return err
		}
		accountID, err := stringUUID(row.AccountID)
		if err != nil {
			return err
		}
		index, ok := indices[transactionID]
		if !ok {
			return errors.New("transaction entry returned for unknown aggregate")
		}
		values[index].Entries = append(values[index].Entries, transactiondomain.Entry{
			ID: id, AccountID: accountID, AmountMinor: row.AmountMinor,
			BaseAmountMinor: row.BaseAmountMinor,
		})
	}
	allocations, err := queries.ListTransactionAllocations(ctx, sqlc.ListTransactionAllocationsParams{
		WorkspaceID: workspaceID, TransactionIds: ids,
	})
	if err != nil {
		return fmt.Errorf("list transaction allocations: %w", err)
	}
	for _, row := range allocations {
		transactionID, err := stringUUID(row.TransactionID)
		if err != nil {
			return err
		}
		id, err := stringUUID(row.ID)
		if err != nil {
			return err
		}
		categoryID, err := stringUUID(row.CategoryID)
		if err != nil {
			return err
		}
		index, ok := indices[transactionID]
		if !ok {
			return errors.New("transaction allocation returned for unknown aggregate")
		}
		values[index].Allocations = append(values[index].Allocations, transactiondomain.Allocation{
			ID: id, CategoryID: categoryID, AmountBaseMinor: row.AmountBaseMinor,
		})
	}
	return nil
}

func transactionFromDatabase(
	id, workspaceID pgtype.UUID,
	kind, status string,
	transactionDate pgtype.Date,
	payee, description, notes pgtype.Text,
	source string,
	createdBy, updatedBy pgtype.UUID,
	createdAt, updatedAt pgtype.Timestamptz,
) (transactiondomain.Transaction, error) {
	idString, err := stringUUID(id)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	workspaceString, err := stringUUID(workspaceID)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	createdByString, err := stringUUID(createdBy)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	updatedByString, err := stringUUID(updatedBy)
	if err != nil {
		return transactiondomain.Transaction{}, err
	}
	if !transactionDate.Valid || !createdAt.Valid || !updatedAt.Valid {
		return transactiondomain.Transaction{}, errors.New("transaction has invalid database timestamps")
	}
	return transactiondomain.Transaction{
		ID: idString, WorkspaceID: workspaceString,
		Kind: transactiondomain.Kind(kind), Status: transactiondomain.Status(status),
		TransactionDate: transactionDate.Time,
		Payee:           stringPointer(payee), Description: stringPointer(description), Notes: stringPointer(notes),
		Source: transactiondomain.Source(source), CreatedBy: createdByString, UpdatedBy: updatedByString,
		CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
		Entries: []transactiondomain.Entry{}, Allocations: []transactiondomain.Allocation{},
	}, nil
}

func writeTransactionRelations(
	ctx context.Context,
	queries *sqlc.Queries,
	workspaceID, transactionID pgtype.UUID,
	value transactiondomain.Transaction,
) error {
	for _, entry := range value.Entries {
		entryID, accountID, err := resourceUUIDs(entry.ID, entry.AccountID)
		if err != nil {
			return err
		}
		if err := queries.CreateTransactionEntry(ctx, sqlc.CreateTransactionEntryParams{
			ID: entryID, WorkspaceID: workspaceID, TransactionID: transactionID,
			AccountID: accountID, AmountMinor: entry.AmountMinor,
			BaseAmountMinor: entry.BaseAmountMinor,
		}); err != nil {
			return mapTransactionWriteError("create transaction entry", err)
		}
	}
	for _, allocation := range value.Allocations {
		allocationID, categoryID, err := resourceUUIDs(allocation.ID, allocation.CategoryID)
		if err != nil {
			return err
		}
		if err := queries.CreateTransactionAllocation(ctx, sqlc.CreateTransactionAllocationParams{
			ID: allocationID, WorkspaceID: workspaceID, TransactionID: transactionID,
			CategoryID: categoryID, AmountBaseMinor: allocation.AmountBaseMinor,
		}); err != nil {
			return mapTransactionWriteError("create transaction allocation", err)
		}
	}
	return nil
}

func mapTransactionWriteError(action string, err error) error {
	if constraintViolation(err, "transactions_reconciliation") {
		return transactiondomain.ErrDoesNotReconcile
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23503" {
		return transactiondomain.ErrReferenceInvalid
	}
	return fmt.Errorf("%s: %w", action, err)
}

func postgresDate(value time.Time) pgtype.Date {
	return pgtype.Date{Time: value, Valid: true}
}

func postgresDatePointer(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return postgresDate(*value)
}
