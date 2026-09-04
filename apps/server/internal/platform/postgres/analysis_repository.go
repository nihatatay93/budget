package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/analysis"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
	"github.com/nihatatay93/budget/internal/workspace"
)

type AnalysisRepository struct {
	pool *pgxpool.Pool
}

func NewAnalysisRepository(pool *pgxpool.Pool) *AnalysisRepository {
	return &AnalysisRepository{pool: pool}
}

// Load reads every analysis aggregate inside one repeatable-read snapshot so the totals, the
// series, and the breakdowns can never disagree with each other.
func (r *AnalysisRepository) Load(
	ctx context.Context,
	workspaceID string,
	query analysis.Query,
	now time.Time,
) (analysis.Snapshot, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return analysis.Snapshot{}, err
	}
	databaseTransaction, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("begin analysis snapshot: %w", err)
	}
	defer func() { _ = databaseTransaction.Rollback(ctx) }()

	queries := sqlc.New(databaseTransaction)
	settings, err := queries.GetReportingWorkspace(ctx, workspaceUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return analysis.Snapshot{}, workspace.ErrForbidden
	}
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("get analysis workspace: %w", err)
	}
	baseCurrency, ok := money.Parse(settings.BaseCurrency)
	if !ok {
		return analysis.Snapshot{}, analysis.ErrInvalidData
	}
	period, err := analysis.ResolvePeriod(query, settings.Timezone, baseCurrency, now)
	if err != nil {
		return analysis.Snapshot{}, err
	}
	fromDate := postgresDate(period.FromDate)
	toDate := postgresDate(period.ToDate)
	comparisonFromDate := postgresDate(period.ComparisonFromDate)
	granularity := string(period.Granularity)

	totalsRow, err := queries.GetAnalysisTotals(ctx, sqlc.GetAnalysisTotalsParams{
		WorkspaceID: workspaceUUID, ComparisonFromDate: comparisonFromDate,
		ToDate: toDate, FromDate: fromDate,
	})
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("get analysis totals: %w", err)
	}

	bucketRows, err := queries.ListAnalysisBuckets(ctx, sqlc.ListAnalysisBucketsParams{
		WorkspaceID: workspaceUUID, FromDate: fromDate, ToDate: toDate,
		Granularity: granularity,
	})
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("list analysis buckets: %w", err)
	}
	series := make([]analysis.BucketSnapshot, 0, len(bucketRows))
	for _, row := range bucketRows {
		series = append(series, analysis.BucketSnapshot{
			StartDate: row.StartDate.Time, EndDate: row.EndDate.Time,
			IncomeSignedMinor: row.IncomeSignedMinor, ExpenseSignedMinor: row.ExpenseSignedMinor,
			TransactionCount: row.TransactionCount,
		})
	}

	categoryRows, err := queries.ListAnalysisCategoryTotals(
		ctx,
		sqlc.ListAnalysisCategoryTotalsParams{
			WorkspaceID: workspaceUUID, FromDate: fromDate, ToDate: toDate,
			ComparisonFromDate: comparisonFromDate,
		},
	)
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("list analysis category totals: %w", err)
	}
	categories := make([]analysis.CategorySnapshot, 0, len(categoryRows))
	for _, row := range categoryRows {
		id, err := stringUUID(row.ID)
		if err != nil {
			return analysis.Snapshot{}, err
		}
		categories = append(categories, analysis.CategorySnapshot{
			ID: id, ParentID: uuidStringPointer(row.ParentID), Name: row.Name,
			Kind: analysis.CategoryKind(row.Kind), SystemKey: stringPointer(row.SystemKey),
			PredefinedKey: stringPointer(row.PredefinedKey), IconType: row.IconType,
			IconValue: row.IconValue, ColorKey: row.ColorKey,
			ArchivedAt:                  timePointer(row.ArchivedAt),
			DirectSignedMinor:           row.DirectSignedMinor,
			RolledSignedMinor:           row.RolledSignedMinor,
			ComparisonDirectSignedMinor: row.ComparisonDirectSignedMinor,
			ComparisonRolledSignedMinor: row.ComparisonRolledSignedMinor,
			TransactionCount:            row.TransactionCount,
			RolledTransactionCount:      row.RolledTransactionCount,
			SmallestSignedMinor:         row.SmallestSignedMinor,
			LargestSignedMinor:          row.LargestSignedMinor,
			FirstDate:                   datePointer(row.FirstDate),
			LastDate:                    datePointer(row.LastDate),
		})
	}

	categoryBucketRows, err := queries.ListAnalysisCategoryBuckets(
		ctx,
		sqlc.ListAnalysisCategoryBucketsParams{
			WorkspaceID: workspaceUUID, FromDate: fromDate, ToDate: toDate,
			Granularity: granularity,
		},
	)
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("list analysis category buckets: %w", err)
	}
	categorySeries := make([]analysis.CategoryPointSnapshot, 0, len(categoryBucketRows))
	for _, row := range categoryBucketRows {
		id, err := stringUUID(row.CategoryID)
		if err != nil {
			return analysis.Snapshot{}, err
		}
		categorySeries = append(categorySeries, analysis.CategoryPointSnapshot{
			CategoryID: id, StartDate: row.BucketAnchor.Time, SignedMinor: row.SignedMinor,
		})
	}

	weekdayRows, err := queries.ListAnalysisWeekdays(ctx, sqlc.ListAnalysisWeekdaysParams{
		WorkspaceID: workspaceUUID, FromDate: fromDate, ToDate: toDate,
	})
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("list analysis weekdays: %w", err)
	}
	weekdays := make([]analysis.WeekdaySnapshot, 0, len(weekdayRows))
	for _, row := range weekdayRows {
		weekdays = append(weekdays, analysis.WeekdaySnapshot{
			Weekday: int(row.Weekday), IncomeSignedMinor: row.IncomeSignedMinor,
			ExpenseSignedMinor: row.ExpenseSignedMinor, TransactionCount: row.TransactionCount,
		})
	}

	dayRows, err := queries.ListAnalysisDays(ctx, sqlc.ListAnalysisDaysParams{
		WorkspaceID: workspaceUUID, FromDate: fromDate, ToDate: toDate,
	})
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("list analysis days: %w", err)
	}
	days := make([]analysis.DaySnapshot, 0, len(dayRows))
	for _, row := range dayRows {
		days = append(days, analysis.DaySnapshot{
			Date: row.ActivityDate.Time, IncomeSignedMinor: row.IncomeSignedMinor,
			ExpenseSignedMinor: row.ExpenseSignedMinor, TransactionCount: row.TransactionCount,
		})
	}

	payeeRows, err := queries.ListAnalysisPayees(ctx, sqlc.ListAnalysisPayeesParams{
		WorkspaceID: workspaceUUID, FromDate: fromDate, ToDate: toDate,
		RowLimit: analysis.PayeeLimit(),
	})
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("list analysis payees: %w", err)
	}
	payees := make([]analysis.PayeeSnapshot, 0, len(payeeRows))
	for _, row := range payeeRows {
		payees = append(payees, analysis.PayeeSnapshot{
			Payee: row.Payee, ExpenseSignedMinor: row.ExpenseSignedMinor,
			IncomeSignedMinor: row.IncomeSignedMinor, TransactionCount: row.TransactionCount,
			FirstDate: row.FirstDate.Time, LastDate: row.LastDate.Time,
		})
	}

	accountRows, err := queries.ListAnalysisAccountActivity(
		ctx,
		sqlc.ListAnalysisAccountActivityParams{
			WorkspaceID: workspaceUUID, FromDate: fromDate, ToDate: toDate,
		},
	)
	if err != nil {
		return analysis.Snapshot{}, fmt.Errorf("list analysis account activity: %w", err)
	}
	accounts := make([]analysis.AccountSnapshot, 0, len(accountRows))
	for _, row := range accountRows {
		id, err := stringUUID(row.ID)
		if err != nil {
			return analysis.Snapshot{}, err
		}
		currency, ok := money.Parse(row.Currency)
		if !ok {
			return analysis.Snapshot{}, analysis.ErrInvalidData
		}
		accounts = append(accounts, analysis.AccountSnapshot{
			ID: id, Name: row.Name, Type: row.Type, Currency: currency,
			ArchivedAt: timePointer(row.ArchivedAt), OutflowSignedMinor: row.OutflowSignedMinor,
			InflowSignedMinor: row.InflowSignedMinor, TransactionCount: row.TransactionCount,
		})
	}

	if err := databaseTransaction.Commit(ctx); err != nil {
		return analysis.Snapshot{}, fmt.Errorf("commit analysis snapshot: %w", err)
	}
	return analysis.Snapshot{
		Period: period,
		Totals: analysis.TotalsSnapshot{
			IncomeSignedMinor:            totalsRow.IncomeSignedMinor,
			ExpenseSignedMinor:           totalsRow.ExpenseSignedMinor,
			ComparisonIncomeSignedMinor:  totalsRow.ComparisonIncomeSignedMinor,
			ComparisonExpenseSignedMinor: totalsRow.ComparisonExpenseSignedMinor,
			TransactionCount:             totalsRow.TransactionCount,
			SpendingTransactionCount:     totalsRow.SpendingTransactionCount,
			SmallestExpenseSignedMinor:   totalsRow.SmallestExpenseSignedMinor,
			SpendingDayCount:             totalsRow.SpendingDayCount,
		},
		Series: series, Categories: categories, CategorySeries: categorySeries,
		Weekdays: weekdays, Days: days, Payees: payees, Accounts: accounts,
	}, nil
}

// datePointer converts an optional SQL date into a domain value. An absent date means the
// aggregate found no activity, not that the data is malformed.
func datePointer(value pgtype.Date) *time.Time {
	if !value.Valid {
		return nil
	}
	converted := value.Time
	return &converted
}
