//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nihatatay93/budget/internal/account"
	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/category"
	cryptoplatform "github.com/nihatatay93/budget/internal/platform/crypto"
	reportingdomain "github.com/nihatatay93/budget/internal/reporting"
	transactiondomain "github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

func TestReportingRepositoryFinancialProjection(t *testing.T) {
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
		Email: "reporting@example.com", Password: "a sufficiently long password",
		DisplayName: "Reporting Owner", WorkspaceName: "Reporting", BaseCurrency: "TRY",
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

	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		transactiondomain.WriteInput{
			Kind: transactiondomain.KindAdjustment, Status: transactiondomain.StatusPosted,
			TransactionDate: reportingDate(2026, time.July, 31),
			Entries:         []transactiondomain.EntryInput{{AccountID: checking.ID, AmountMinor: 10000}},
		})
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 1), transactiondomain.StatusPosted,
			checking.ID, restaurants.ID, -1500,
		))
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 5), transactiondomain.StatusPosted,
			checking.ID, restaurants.ID, 200,
		))
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 10), transactiondomain.StatusPending,
			checking.ID, restaurants.ID, -300,
		))
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 18), transactiondomain.StatusPosted,
			checking.ID, salary.ID, 5000,
		))
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 18), transactiondomain.StatusPending,
			checking.ID, salary.ID, 700,
		))
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		transactiondomain.WriteInput{
			Kind: transactiondomain.KindTransfer, Status: transactiondomain.StatusPosted,
			TransactionDate: reportingDate(2026, time.August, 15),
			Entries: []transactiondomain.EntryInput{
				{AccountID: checking.ID, AmountMinor: -1000},
				{AccountID: wallet.ID, AmountMinor: 1000},
			},
		})
	deleted := createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 12), transactiondomain.StatusPosted,
			checking.ID, restaurants.ID, -400,
		))
	if err := transactionService.SoftDelete(ctx, workspaceID, ownerID, deleted.ID); err != nil {
		t.Fatalf("soft-delete reporting transaction: %v", err)
	}
	createReportingTransaction(t, ctx, transactionService, workspaceID, ownerID,
		standardReportingTransaction(
			reportingDate(2026, time.August, 19), transactiondomain.StatusPosted,
			checking.ID, salary.ID, 9000,
		))
	if err := accountService.Archive(ctx, workspaceID, ownerID, wallet.ID); err != nil {
		t.Fatalf("archive reporting account: %v", err)
	}
	if err := categoryService.Archive(ctx, workspaceID, ownerID, restaurants.ID); err != nil {
		t.Fatalf("archive reporting category: %v", err)
	}

	fromDate := reportingDate(2026, time.August, 1)
	toDate := reportingDate(2026, time.August, 18)
	reports := reportingdomain.NewService(
		NewReportingRepository(store.Pool()), access,
		func() time.Time { return reportingDate(2026, time.August, 18) },
	)
	projection, err := reports.Project(
		ctx, workspaceID, ownerID,
		reportingdomain.Query{FromDate: &fromDate, ToDate: &toDate},
	)
	if err != nil {
		t.Fatalf("project financial report: %v", err)
	}

	if projection.Summary.BalanceBaseMinor != (reportingdomain.Amounts{
		Posted: 13700, Pending: 400, Projected: 14100,
	}) {
		t.Fatalf("base balance = %#v", projection.Summary.BalanceBaseMinor)
	}
	if projection.Summary.IncomeBaseMinor != (reportingdomain.Amounts{
		Posted: 5000, Pending: 700, Projected: 5700,
	}) {
		t.Fatalf("income = %#v", projection.Summary.IncomeBaseMinor)
	}
	if projection.Summary.SpendingBaseMinor != (reportingdomain.Amounts{
		Posted: 1300, Pending: 300, Projected: 1600,
	}) {
		t.Fatalf("spending = %#v", projection.Summary.SpendingBaseMinor)
	}

	checkingProjection := reportingAccountByID(t, projection, checking.ID)
	if checkingProjection.Native != (reportingdomain.Amounts{
		Posted: 12700, Pending: 400, Projected: 13100,
	}) {
		t.Fatalf("checking balance = %#v", checkingProjection.Native)
	}
	walletProjection := reportingAccountByID(t, projection, wallet.ID)
	if walletProjection.ArchivedAt == nil || walletProjection.Native.Posted != 1000 {
		t.Fatalf("archived wallet projection = %#v", walletProjection)
	}

	foodProjection := reportingCategoryByID(t, projection, food.ID)
	if foodProjection.Direct != (reportingdomain.Amounts{}) ||
		foodProjection.RolledUp != (reportingdomain.Amounts{
			Posted: 1300, Pending: 300, Projected: 1600,
		}) {
		t.Fatalf("food rollup = %#v", foodProjection)
	}
	restaurantProjection := reportingCategoryByID(t, projection, restaurants.ID)
	if restaurantProjection.ArchivedAt == nil ||
		restaurantProjection.Direct != foodProjection.RolledUp {
		t.Fatalf("archived restaurant projection = %#v", restaurantProjection)
	}
	salaryProjection := reportingCategoryByID(t, projection, salary.ID)
	if salaryProjection.Direct != (reportingdomain.Amounts{
		Posted: 5000, Pending: 700, Projected: 5700,
	}) {
		t.Fatalf("salary activity = %#v", salaryProjection.Direct)
	}
}

func createReportingAccount(
	t *testing.T,
	ctx context.Context,
	service *account.Service,
	workspaceID, userID, name string,
	accountType account.Type,
) account.Account {
	t.Helper()
	value, err := service.Create(ctx, workspaceID, userID, account.WriteInput{
		Name: name, Type: accountType, Currency: "TRY",
	})
	if err != nil {
		t.Fatalf("create reporting account %q: %v", name, err)
	}
	return value
}

func createReportingCategory(
	t *testing.T,
	ctx context.Context,
	service *category.Service,
	workspaceID, userID, name string,
	kind category.Kind,
	parentID *string,
) category.Category {
	t.Helper()
	value, err := service.Create(ctx, workspaceID, userID, category.WriteInput{
		Name: name, Kind: kind, ParentID: parentID,
	})
	if err != nil {
		t.Fatalf("create reporting category %q: %v", name, err)
	}
	return value
}

func createReportingTransaction(
	t *testing.T,
	ctx context.Context,
	service *transactiondomain.Service,
	workspaceID, userID string,
	input transactiondomain.WriteInput,
) transactiondomain.Transaction {
	t.Helper()
	value, err := service.Create(ctx, workspaceID, userID, input)
	if err != nil {
		t.Fatalf("create reporting transaction: %v", err)
	}
	return value
}

func standardReportingTransaction(
	date time.Time,
	status transactiondomain.Status,
	accountID, categoryID string,
	amount int64,
) transactiondomain.WriteInput {
	return transactiondomain.WriteInput{
		Kind: transactiondomain.KindStandard, Status: status, TransactionDate: date,
		Entries: []transactiondomain.EntryInput{{AccountID: accountID, AmountMinor: amount}},
		Allocations: []transactiondomain.AllocationInput{{
			CategoryID: categoryID, AmountBaseMinor: amount,
		}},
	}
}

func reportingAccountByID(
	t *testing.T,
	projection reportingdomain.Projection,
	id string,
) reportingdomain.Account {
	t.Helper()
	for _, value := range projection.Accounts {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("reporting account %s not found", id)
	return reportingdomain.Account{}
}

func reportingCategoryByID(
	t *testing.T,
	projection reportingdomain.Projection,
	id string,
) reportingdomain.Category {
	t.Helper()
	for _, value := range projection.Categories {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("reporting category %s not found", id)
	return reportingdomain.Category{}
}

func reportingDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
