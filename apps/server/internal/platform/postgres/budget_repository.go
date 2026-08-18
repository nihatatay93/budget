package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/budget"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
	"github.com/nihatatay93/budget/internal/workspace"
)

type BudgetRepository struct {
	pool *pgxpool.Pool
}

func NewBudgetRepository(pool *pgxpool.Pool) *BudgetRepository {
	return &BudgetRepository{pool: pool}
}

func (r *BudgetRepository) WorkspaceSettings(
	ctx context.Context,
	workspaceID string,
) (budget.WorkspaceSettings, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return budget.WorkspaceSettings{}, err
	}
	row, err := sqlc.New(r.pool).GetBudgetWorkspaceSettings(ctx, workspaceUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return budget.WorkspaceSettings{}, workspace.ErrForbidden
	}
	if err != nil {
		return budget.WorkspaceSettings{}, fmt.Errorf("get budget workspace settings: %w", err)
	}
	currency, ok := money.Parse(row.BaseCurrency)
	if !ok {
		return budget.WorkspaceSettings{}, budget.ErrInvalidData
	}
	return budget.WorkspaceSettings{Timezone: row.Timezone, BaseCurrency: currency}, nil
}

func (r *BudgetRepository) CategoriesForMonth(
	ctx context.Context,
	workspaceID string,
	month budget.Month,
) ([]budget.CategoryRule, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlc.New(r.pool).ListBudgetCategoryRules(
		ctx,
		sqlc.ListBudgetCategoryRulesParams{
			StartsOn: postgresDate(month.StartDate()), WorkspaceID: workspaceUUID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list budget category rules: %w", err)
	}
	result := make([]budget.CategoryRule, 0, len(rows))
	for _, row := range rows {
		id, err := stringUUID(row.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, budget.CategoryRule{
			ID: id, ParentID: uuidStringPointer(row.ParentID), Kind: category.Kind(row.Kind),
			ArchivedAt: timePointer(row.ArchivedAt), AlreadyBudgeted: row.AlreadyBudgeted,
		})
	}
	return result, nil
}

func (r *BudgetRepository) Get(
	ctx context.Context,
	workspaceID string,
	month budget.Month,
) (budget.Snapshot, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return budget.Snapshot{}, err
	}
	databaseTransaction, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return budget.Snapshot{}, fmt.Errorf("begin budget snapshot: %w", err)
	}
	defer func() { _ = databaseTransaction.Rollback(ctx) }()

	snapshot, err := loadBudgetSnapshot(
		ctx, sqlc.New(databaseTransaction), workspaceUUID, month,
	)
	if err != nil {
		return budget.Snapshot{}, err
	}
	if err := databaseTransaction.Commit(ctx); err != nil {
		return budget.Snapshot{}, fmt.Errorf("commit budget snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *BudgetRepository) Replace(
	ctx context.Context,
	workspaceID string,
	month budget.Month,
	command budget.ReplaceCommand,
) (budget.Snapshot, error) {
	workspaceUUID, newBudgetUUID, err := resourceUUIDs(workspaceID, command.NewBudgetID)
	if err != nil {
		return budget.Snapshot{}, err
	}
	databaseTransaction, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return budget.Snapshot{}, fmt.Errorf("begin budget replacement: %w", err)
	}
	defer func() { _ = databaseTransaction.Rollback(ctx) }()
	queries := sqlc.New(databaseTransaction)
	startsOn := postgresDate(month.StartDate())

	budgetUUID, err := queries.UpsertMonthlyBudget(ctx, sqlc.UpsertMonthlyBudgetParams{
		ID: newBudgetUUID, WorkspaceID: workspaceUUID, Name: command.Name, StartsOn: startsOn,
	})
	if err != nil {
		return budget.Snapshot{}, mapBudgetWriteError("upsert monthly budget", err)
	}
	categoryIDs := make([]pgtype.UUID, 0, len(command.Items))
	for _, item := range command.Items {
		itemUUID, categoryUUID, err := resourceUUIDs(item.ID, item.CategoryID)
		if err != nil {
			return budget.Snapshot{}, err
		}
		if _, err := queries.UpsertMonthlyBudgetItem(
			ctx,
			sqlc.UpsertMonthlyBudgetItemParams{
				ID: itemUUID, WorkspaceID: workspaceUUID, BudgetID: budgetUUID,
				CategoryID: categoryUUID, AmountBaseMinor: item.AmountBaseMinor,
			},
		); err != nil {
			return budget.Snapshot{}, mapBudgetWriteError("upsert monthly budget item", err)
		}
		categoryIDs = append(categoryIDs, categoryUUID)
	}
	if err := queries.DeleteOmittedMonthlyBudgetItems(
		ctx,
		sqlc.DeleteOmittedMonthlyBudgetItemsParams{
			WorkspaceID: workspaceUUID, BudgetID: budgetUUID, CategoryIds: categoryIDs,
		},
	); err != nil {
		return budget.Snapshot{}, mapBudgetWriteError("delete omitted monthly budget items", err)
	}

	snapshot, err := loadBudgetSnapshot(ctx, queries, workspaceUUID, month)
	if err != nil {
		return budget.Snapshot{}, err
	}
	if err := databaseTransaction.Commit(ctx); err != nil {
		return budget.Snapshot{}, mapBudgetWriteError("commit monthly budget", err)
	}
	return snapshot, nil
}

func loadBudgetSnapshot(
	ctx context.Context,
	queries *sqlc.Queries,
	workspaceID pgtype.UUID,
	month budget.Month,
) (budget.Snapshot, error) {
	startsOn := postgresDate(month.StartDate())
	header, err := queries.GetMonthlyBudget(ctx, sqlc.GetMonthlyBudgetParams{
		WorkspaceID: workspaceID, StartsOn: startsOn,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return budget.Snapshot{}, budget.ErrNotFound
	}
	if err != nil {
		return budget.Snapshot{}, fmt.Errorf("get monthly budget: %w", err)
	}
	rows, err := queries.ListMonthlyBudgetItems(ctx, sqlc.ListMonthlyBudgetItemsParams{
		WorkspaceID: workspaceID, BudgetID: header.ID, StartsOn: startsOn,
	})
	if err != nil {
		return budget.Snapshot{}, fmt.Errorf("list monthly budget items: %w", err)
	}
	id, err := stringUUID(header.ID)
	if err != nil {
		return budget.Snapshot{}, err
	}
	workspaceIDString, err := stringUUID(header.WorkspaceID)
	if err != nil {
		return budget.Snapshot{}, err
	}
	currency, ok := money.Parse(header.BaseCurrency)
	if !ok || !header.StartsOn.Valid || !header.CreatedAt.Valid || !header.UpdatedAt.Valid {
		return budget.Snapshot{}, budget.ErrInvalidData
	}
	items := make([]budget.ItemSnapshot, 0, len(rows))
	for _, row := range rows {
		itemID, err := stringUUID(row.ID)
		if err != nil {
			return budget.Snapshot{}, err
		}
		categoryID, err := stringUUID(row.CategoryID)
		if err != nil {
			return budget.Snapshot{}, err
		}
		items = append(items, budget.ItemSnapshot{
			ID: itemID, CategoryID: categoryID, CategoryName: row.CategoryName,
			CategoryIcon:              stringPointer(row.CategoryIcon),
			CategoryArchivedAt:        timePointer(row.CategoryArchivedAt),
			PlannedBaseMinor:          row.PlannedBaseMinor,
			SignedAllocationBaseMinor: row.SignedAllocationBaseMinor,
		})
	}
	return budget.Snapshot{
		ID: id, WorkspaceID: workspaceIDString, Name: header.Name,
		StartsOn: header.StartsOn.Time, Timezone: header.Timezone, BaseCurrency: currency,
		Items: items, CreatedAt: header.CreatedAt.Time, UpdatedAt: header.UpdatedAt.Time,
	}, nil
}

func mapBudgetWriteError(action string, err error) error {
	switch {
	case constraintViolation(err, "budget_items_expense_category"):
		return budget.ErrCategoryKind
	case constraintViolation(err, "budget_items_new_active_category"):
		return budget.ErrCategoryArchived
	case constraintViolation(err, "budget_items_non_overlapping_categories"):
		return budget.ErrCategoryOverlap
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23503" {
		return budget.ErrCategoryNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}
