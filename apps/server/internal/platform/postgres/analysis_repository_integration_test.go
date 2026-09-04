//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nihatatay93/budget/internal/account"
	analysisdomain "github.com/nihatatay93/budget/internal/analysis"
	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/category"
	cryptoplatform "github.com/nihatatay93/budget/internal/platform/crypto"
	transactiondomain "github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

// The analysis aggregates are only trustworthy if the same ledger facts that the projection
// excludes — pending activity, soft deletions, and transfers — stay out of every breakdown,
// and if the time buckets tile the window without gaps or double counting.
func TestAnalysisRepositorySpendingAnalysis(t *testing.T) {
	ctx := context.Background()
	container, err := postgrescontainer.Run(
		ctx,
		"postgres:18-alpine",
		postgrescontainer.WithDatabase("budget_test"),
		postgrescontainer.WithUsername("budget"),
		postgrescontainer.WithPassword("budget"),
		postgrescontainer.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	testcontainers.CleanupContainer(t, container)
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}
	if err := Migrate(ctx, databaseURL); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(store.Close)

	authentication, err := auth.NewService(
		NewAuthRepository(store.Pool()), cryptoplatform.PasswordHasher{}, 24*time.Hour,
	)
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	registration, err := authentication.Register(ctx, auth.RegisterInput{
		Email: "analysis@example.com", Password: "a sufficiently long password",
		DisplayName: "Analysis Owner", WorkspaceName: "Analysis", BaseCurrency: "TRY",
		Timezone: "Europe/Istanbul", Transport: auth.TransportBearer,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	workspaceID := registration.Workspaces[0].ID
	ownerID := registration.Principal.User.ID
	access := workspace.NewAuthorizer(NewWorkspaceRepository(store.Pool()))
	accountService := account.NewService(NewAccountRepository(store.Pool()), access)
	categoryService := category.NewService(NewCategoryRepository(store.Pool()), access)
	transactionService := transactiondomain.NewService(
		NewTransactionRepository(store.Pool()), access, nil,
	)

	checking := createReportingAccount(
		t, ctx, accountService, workspaceID, ownerID, "Checking", account.TypeBank,
	)
	wallet := createReportingAccount(
		t, ctx, accountService, workspaceID, ownerID, "Wallet", account.TypeCash,
	)
	food := createReportingCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Food", category.KindExpense, nil,
	)
	restaurants := createReportingCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Restaurants", category.KindExpense,
		&food.ID,
	)
	salary := createReportingCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Salary", category.KindIncome, nil,
	)

	// July belongs to the comparison window for an August analysis.
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.July, 10), transactiondomain.StatusPosted,
			checking.ID, restaurants.ID, -5000,
		))
	// 2026-08-03 is a Monday; 2026-08-08 is a Saturday.
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		payeeTransaction(
			reportingDate(2026, time.August, 3), checking.ID, restaurants.ID, -1500, "Migros",
		))
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		payeeTransaction(
			reportingDate(2026, time.August, 8), checking.ID, restaurants.ID, -2500, "Migros",
		))
	// A refund into an expense category is a positive allocation and must not read as a charge.
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 9), transactiondomain.StatusPosted,
			checking.ID, restaurants.ID, 300,
		))
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 20), transactiondomain.StatusPosted,
			checking.ID, salary.ID, 40000,
		))
	// Excluded: pending, soft-deleted, and a transfer between owned accounts.
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 11), transactiondomain.StatusPending,
			checking.ID, restaurants.ID, -9900,
		))
	deleted := createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 12), transactiondomain.StatusPosted,
			checking.ID, restaurants.ID, -8800,
		))
	if err := transactionService.SoftDelete(ctx, workspaceID, ownerID, deleted.ID); err != nil {
		t.Fatalf("soft-delete analysis transaction: %v", err)
	}
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		transactiondomain.WriteInput{
			Kind: transactiondomain.KindTransfer, Status: transactiondomain.StatusPosted,
			TransactionDate: reportingDate(2026, time.August, 15),
			Entries: []transactiondomain.EntryInput{
				{AccountID: checking.ID, AmountMinor: -7700},
				{AccountID: wallet.ID, AmountMinor: 7700},
			},
		})

	fromDate := reportingDate(2026, time.August, 1)
	toDate := reportingDate(2026, time.August, 31)
	service := analysisdomain.NewService(
		NewAnalysisRepository(store.Pool()), access,
		func() time.Time { return reportingDate(2026, time.August, 31) },
	)
	result, err := service.Analyze(ctx, workspaceID, ownerID, analysisdomain.Query{
		FromDate: &fromDate, ToDate: &toDate, Granularity: analysisdomain.GranularityWeek,
	})
	if err != nil {
		t.Fatalf("analyze spending: %v", err)
	}

	// 1500 + 2500 - 300 refund = 3700 spent; the pending, deleted, and transfer amounts are
	// large and distinctive so any leak would be obvious here.
	if result.Totals.SpendingBaseMinor != 3700 || result.Totals.IncomeBaseMinor != 40000 {
		t.Fatalf("totals = %+v, want 3700 spent and 40000 earned", result.Totals)
	}
	if result.Totals.NetBaseMinor != 36300 {
		t.Fatalf("net = %d, want 36300", result.Totals.NetBaseMinor)
	}
	if result.Totals.ComparisonSpendingBaseMinor != 5000 {
		t.Fatalf("comparison spending = %d, want July's 5000",
			result.Totals.ComparisonSpendingBaseMinor)
	}
	if result.Totals.LargestSpendingBaseMinor != 2500 {
		t.Fatalf("largest spending = %d, want 2500", result.Totals.LargestSpendingBaseMinor)
	}
	if result.Totals.SpendingDayCount != 3 || result.Totals.DayCount != 31 {
		t.Fatalf("day counts = %d spending of %d, want 3 of 31",
			result.Totals.SpendingDayCount, result.Totals.DayCount)
	}

	// Weekly buckets must tile the window: contiguous, non-overlapping, and clamped to it.
	if len(result.Series) == 0 {
		t.Fatal("series is empty, want contiguous buckets covering the window")
	}
	if !result.Series[0].StartDate.Equal(fromDate) {
		t.Fatalf("first bucket starts %s, want the window start",
			result.Series[0].StartDate.Format(time.DateOnly))
	}
	last := result.Series[len(result.Series)-1]
	if !last.EndDate.Equal(toDate) {
		t.Fatalf("last bucket ends %s, want the window end", last.EndDate.Format(time.DateOnly))
	}
	var bucketSpending int64
	for index, bucket := range result.Series {
		bucketSpending += bucket.SpendingBaseMinor
		if index == 0 {
			continue
		}
		want := result.Series[index-1].EndDate.AddDate(0, 0, 1)
		if !bucket.StartDate.Equal(want) {
			t.Fatalf("bucket %d starts %s, want %s to follow the previous bucket",
				index, bucket.StartDate.Format(time.DateOnly), want.Format(time.DateOnly))
		}
	}
	if bucketSpending != result.Totals.SpendingBaseMinor {
		t.Fatalf("bucket spending = %d, want the window total %d",
			bucketSpending, result.Totals.SpendingBaseMinor)
	}

	foodCategory := analysisCategoryByID(t, result, food.ID)
	if foodCategory.DirectBaseMinor != 0 || foodCategory.RolledUpBaseMinor != 3700 {
		t.Fatalf("food = %+v, want spending rolled up from its child", foodCategory)
	}
	if foodCategory.ComparisonRolledUpBaseMinor != 5000 {
		t.Fatalf("food comparison = %d, want July's 5000", foodCategory.ComparisonRolledUpBaseMinor)
	}
	restaurantCategory := analysisCategoryByID(t, result, restaurants.ID)
	if restaurantCategory.DirectBaseMinor != 3700 || restaurantCategory.LargestBaseMinor != 2500 {
		t.Fatalf("restaurants = %+v, want 3700 direct and a 2500 largest charge",
			restaurantCategory)
	}
	if restaurantCategory.TransactionCount != 3 {
		t.Fatalf("restaurants transactions = %d, want the two charges and the refund",
			restaurantCategory.TransactionCount)
	}
	if restaurantCategory.FirstDate == nil ||
		!restaurantCategory.FirstDate.Equal(reportingDate(2026, time.August, 3)) {
		t.Fatalf("restaurants first date = %v, want 2026-08-03", restaurantCategory.FirstDate)
	}
	salaryCategory := analysisCategoryByID(t, result, salary.ID)
	if salaryCategory.DirectBaseMinor != 40000 {
		t.Fatalf("salary = %+v, want unchanged income", salaryCategory)
	}

	var restaurantPoints int64
	for _, point := range result.CategorySeries {
		if point.CategoryID == restaurants.ID {
			restaurantPoints += point.BaseMinor
		}
	}
	if restaurantPoints != 3700 {
		t.Fatalf("restaurant series total = %d, want the category total 3700", restaurantPoints)
	}

	if spending := analysisWeekdaySpending(result, time.Monday); spending != 1500 {
		t.Fatalf("Monday spending = %d, want 1500", spending)
	}
	if spending := analysisWeekdaySpending(result, time.Saturday); spending != 2500 {
		t.Fatalf("Saturday spending = %d, want 2500", spending)
	}

	if len(result.Days) != 4 {
		t.Fatalf("days = %d, want only the four days with posted activity", len(result.Days))
	}

	if len(result.Payees) != 1 || result.Payees[0].Payee != "Migros" ||
		result.Payees[0].SpendingBaseMinor != 4000 || result.Payees[0].TransactionCount != 2 {
		t.Fatalf("payees = %+v, want Migros with both charges", result.Payees)
	}

	if len(result.Accounts) != 1 || result.Accounts[0].ID != checking.ID {
		t.Fatalf("accounts = %+v, want only the account with non-transfer activity",
			result.Accounts)
	}
	// The 7700 transfer leg left this account but is not spending.
	if result.Accounts[0].OutflowBaseMinor != 4000 ||
		result.Accounts[0].InflowBaseMinor != 40300 {
		t.Fatalf("checking activity = %+v, want transfers excluded", result.Accounts[0])
	}
}

func payeeTransaction(
	date time.Time,
	accountID, categoryID string,
	amount int64,
	payee string,
) transactiondomain.WriteInput {
	input := standardReportingTransaction(
		date, transactiondomain.StatusPosted, accountID, categoryID, amount,
	)
	input.Payee = &payee
	return input
}

func analysisCategoryByID(
	t *testing.T,
	result analysisdomain.Analysis,
	id string,
) analysisdomain.Category {
	t.Helper()
	for _, value := range result.Categories {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("analysis category %s not found", id)
	return analysisdomain.Category{}
}

func analysisWeekdaySpending(result analysisdomain.Analysis, weekday time.Weekday) int64 {
	// Go's Sunday is 0; the analysis uses ISO numbering, where Sunday is 7.
	iso := int(weekday)
	if iso == 0 {
		iso = 7
	}
	for _, value := range result.Weekdays {
		if value.Weekday == iso {
			return value.SpendingBaseMinor
		}
	}
	return 0
}
