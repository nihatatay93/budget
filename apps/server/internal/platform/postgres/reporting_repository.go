package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
	"github.com/nihatatay93/budget/internal/reporting"
	"github.com/nihatatay93/budget/internal/workspace"
)

type ReportingRepository struct {
	pool *pgxpool.Pool
}

func NewReportingRepository(pool *pgxpool.Pool) *ReportingRepository {
	return &ReportingRepository{pool: pool}
}

func (r *ReportingRepository) Load(
	ctx context.Context,
	workspaceID string,
	query reporting.Query,
	now time.Time,
) (reporting.Snapshot, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return reporting.Snapshot{}, err
	}
	databaseTransaction, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return reporting.Snapshot{}, fmt.Errorf("begin reporting snapshot: %w", err)
	}
	defer func() { _ = databaseTransaction.Rollback(ctx) }()

	queries := sqlc.New(databaseTransaction)
	settings, err := queries.GetReportingWorkspace(ctx, workspaceUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return reporting.Snapshot{}, workspace.ErrForbidden
	}
	if err != nil {
		return reporting.Snapshot{}, fmt.Errorf("get reporting workspace: %w", err)
	}
	baseCurrency, ok := money.Parse(settings.BaseCurrency)
	if !ok {
		return reporting.Snapshot{}, reporting.ErrInvalidData
	}
	period, err := reporting.ResolvePeriod(query, settings.Timezone, baseCurrency, now)
	if err != nil {
		return reporting.Snapshot{}, err
	}

	accountRows, err := queries.ListReportingAccountBalances(
		ctx,
		sqlc.ListReportingAccountBalancesParams{
			ToDate: postgresDate(period.ToDate), WorkspaceID: workspaceUUID,
		},
	)
	if err != nil {
		return reporting.Snapshot{}, fmt.Errorf("list reporting account balances: %w", err)
	}
	accounts := make([]reporting.AccountSnapshot, 0, len(accountRows))
	for _, row := range accountRows {
		id, err := stringUUID(row.ID)
		if err != nil {
			return reporting.Snapshot{}, err
		}
		currency, ok := money.Parse(row.Currency)
		if !ok {
			return reporting.Snapshot{}, reporting.ErrInvalidData
		}
		accounts = append(accounts, reporting.AccountSnapshot{
			ID: id, Name: row.Name, Type: row.Type, Currency: currency,
			ArchivedAt:         timePointer(row.ArchivedAt),
			PostedNativeMinor:  row.PostedNativeMinor,
			PendingNativeMinor: row.PendingNativeMinor,
			PostedBaseMinor:    row.PostedBaseMinor,
			PendingBaseMinor:   row.PendingBaseMinor,
		})
	}

	categoryRows, err := queries.ListReportingCategoryActivity(
		ctx,
		sqlc.ListReportingCategoryActivityParams{
			WorkspaceID: workspaceUUID,
			FromDate:    postgresDate(period.FromDate),
			ToDate:      postgresDate(period.ToDate),
		},
	)
	if err != nil {
		return reporting.Snapshot{}, fmt.Errorf("list reporting category activity: %w", err)
	}
	categories := make([]reporting.CategorySnapshot, 0, len(categoryRows))
	for _, row := range categoryRows {
		id, err := stringUUID(row.ID)
		if err != nil {
			return reporting.Snapshot{}, err
		}
		categories = append(categories, reporting.CategorySnapshot{
			ID: id, ParentID: uuidStringPointer(row.ParentID), Name: row.Name,
			Kind: reporting.CategoryKind(row.Kind), SystemKey: stringPointer(row.SystemKey),
			PredefinedKey: stringPointer(row.PredefinedKey),
			Icon:          stringPointer(row.Icon), IconType: row.IconType, IconValue: row.IconValue,
			ColorKey: row.ColorKey, ArchivedAt: timePointer(row.ArchivedAt),
			DirectPostedSignedMinor:  row.DirectPostedSignedMinor,
			DirectPendingSignedMinor: row.DirectPendingSignedMinor,
			RolledPostedSignedMinor:  row.RolledPostedSignedMinor,
			RolledPendingSignedMinor: row.RolledPendingSignedMinor,
		})
	}

	if err := databaseTransaction.Commit(ctx); err != nil {
		return reporting.Snapshot{}, fmt.Errorf("commit reporting snapshot: %w", err)
	}
	return reporting.Snapshot{
		Period: period, Accounts: accounts, Categories: categories,
	}, nil
}
