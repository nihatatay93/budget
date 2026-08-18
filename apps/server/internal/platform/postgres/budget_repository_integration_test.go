//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nihatatay93/budget/internal/account"
	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/budget"
	"github.com/nihatatay93/budget/internal/category"
	cryptoplatform "github.com/nihatatay93/budget/internal/platform/crypto"
	transactiondomain "github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

func TestBudgetRepositoryMonthlyReplacementAndPostedUsage(t *testing.T) {
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
		Email: "budgets@example.com", Password: "a sufficiently long password",
		DisplayName: "Budget Owner", WorkspaceName: "Budgets", BaseCurrency: "TRY",
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
	budgetService := budget.NewService(
		NewBudgetRepository(store.Pool()), access,
		func() time.Time { return budgetDate(2026, time.August, 18) },
	)

	checking := createBudgetAccount(t, ctx, accountService, workspaceID, ownerID)
	food := createBudgetCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Food", category.KindExpense, nil,
	)
	restaurants := createBudgetCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Restaurants", category.KindExpense,
		&food.ID,
	)
	utilities := createBudgetCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Utilities", category.KindExpense, nil,
	)
	travel := createBudgetCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Travel", category.KindExpense, nil,
	)
	oldExpense := createBudgetCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Old expense", category.KindExpense, nil,
	)
	income := createBudgetCategory(
		t, ctx, categoryService, workspaceID, ownerID, "Salary", category.KindIncome, nil,
	)
	if err := categoryService.Archive(ctx, workspaceID, ownerID, oldExpense.ID); err != nil {
		t.Fatalf("archive unused expense category: %v", err)
	}

	createBudgetTransaction(
		t, ctx, transactionService, workspaceID, ownerID, checking.ID, restaurants.ID,
		budgetDate(2026, time.August, 1), transactiondomain.StatusPosted, -1500,
	)
	createBudgetTransaction(
		t, ctx, transactionService, workspaceID, ownerID, checking.ID, restaurants.ID,
		budgetDate(2026, time.August, 5), transactiondomain.StatusPosted, 200,
	)
	createBudgetTransaction(
		t, ctx, transactionService, workspaceID, ownerID, checking.ID, restaurants.ID,
		budgetDate(2026, time.August, 10), transactiondomain.StatusPending, -300,
	)
	deleted := createBudgetTransaction(
		t, ctx, transactionService, workspaceID, ownerID, checking.ID, restaurants.ID,
		budgetDate(2026, time.August, 12), transactiondomain.StatusPosted, -400,
	)
	if err := transactionService.SoftDelete(ctx, workspaceID, ownerID, deleted.ID); err != nil {
		t.Fatalf("soft-delete budget transaction: %v", err)
	}
	createBudgetTransaction(
		t, ctx, transactionService, workspaceID, ownerID, checking.ID, restaurants.ID,
		budgetDate(2026, time.July, 31), transactiondomain.StatusPosted, -700,
	)

	created, err := budgetService.Replace(ctx, workspaceID, ownerID, "2026-08", budget.WriteInput{
		Name: "August plan",
		Items: []budget.ItemInput{
			{CategoryID: food.ID, AmountBaseMinor: 5000},
			{CategoryID: utilities.ID, AmountBaseMinor: 2000},
			{CategoryID: travel.ID, AmountBaseMinor: 1000},
		},
	})
	if err != nil {
		t.Fatalf("create monthly budget: %v", err)
	}
	if created.PlannedBaseMinor != 8000 || created.UsedBaseMinor != 1300 ||
		created.RemainingBaseMinor != 6700 {
		t.Fatalf("created budget totals = %#v", created)
	}
	foodItem := budgetItemByCategory(t, created, food.ID)
	if foodItem.UsedBaseMinor != 1300 || foodItem.RemainingBaseMinor != 3700 {
		t.Fatalf("food subtree usage = %#v", foodItem)
	}
	originalBudgetID := created.ID
	originalFoodItemID := foodItem.ID
	originalUtilitiesItemID := budgetItemByCategory(t, created, utilities.ID).ID

	if err := categoryService.Archive(ctx, workspaceID, ownerID, travel.ID); err != nil {
		t.Fatalf("archive retained budget category: %v", err)
	}
	replaced, err := budgetService.Replace(ctx, workspaceID, ownerID, "2026-08", budget.WriteInput{
		Name: "Revised August plan",
		Items: []budget.ItemInput{
			{CategoryID: food.ID, AmountBaseMinor: 5500},
			{CategoryID: utilities.ID, AmountBaseMinor: 2500},
			{CategoryID: travel.ID, AmountBaseMinor: 1250},
		},
	})
	if err != nil {
		t.Fatalf("replace budget retaining archived category: %v", err)
	}
	if replaced.ID != originalBudgetID || budgetItemByCategory(t, replaced, food.ID).ID != originalFoodItemID ||
		budgetItemByCategory(t, replaced, utilities.ID).ID != originalUtilitiesItemID {
		t.Fatalf("replacement did not preserve aggregate IDs: %#v", replaced)
	}
	if budgetItemByCategory(t, replaced, travel.ID).CategoryArchivedAt == nil {
		t.Fatalf("archived category state missing from item: %#v", replaced)
	}

	replaced, err = budgetService.Replace(ctx, workspaceID, ownerID, "2026-08", budget.WriteInput{
		Name: "Final August plan",
		Items: []budget.ItemInput{
			{CategoryID: food.ID, AmountBaseMinor: 5500},
			{CategoryID: utilities.ID, AmountBaseMinor: 2500},
		},
	})
	if err != nil {
		t.Fatalf("replace budget omitting an item: %v", err)
	}
	if len(replaced.Items) != 2 {
		t.Fatalf("omitted budget item was not deleted: %#v", replaced.Items)
	}
	loaded, err := budgetService.Get(ctx, workspaceID, ownerID, nil)
	if err != nil {
		t.Fatalf("get current workspace month: %v", err)
	}
	if loaded.ID != originalBudgetID || loaded.Month != "2026-08" || loaded.UsedBaseMinor != 1300 {
		t.Fatalf("loaded budget = %#v", loaded)
	}

	_, err = budgetService.Replace(ctx, workspaceID, ownerID, "2026-08", budget.WriteInput{
		Name: "Invalid archived", Items: []budget.ItemInput{
			{CategoryID: oldExpense.ID, AmountBaseMinor: 1000},
		},
	})
	if !errors.Is(err, budget.ErrCategoryArchived) {
		t.Fatalf("new archived category error = %v", err)
	}
	_, err = budgetService.Replace(ctx, workspaceID, ownerID, "2026-08", budget.WriteInput{
		Name: "Invalid kind", Items: []budget.ItemInput{
			{CategoryID: income.ID, AmountBaseMinor: 1000},
		},
	})
	if !errors.Is(err, budget.ErrCategoryKind) {
		t.Fatalf("income category error = %v", err)
	}
	_, err = budgetService.Replace(ctx, workspaceID, ownerID, "2026-08", budget.WriteInput{
		Name: "Invalid overlap", Items: []budget.ItemInput{
			{CategoryID: food.ID, AmountBaseMinor: 1000},
			{CategoryID: restaurants.ID, AmountBaseMinor: 1000},
		},
	})
	if !errors.Is(err, budget.ErrCategoryOverlap) {
		t.Fatalf("overlapping category error = %v", err)
	}

	budgetRepository := NewBudgetRepository(store.Pool())
	_, err = budgetRepository.Replace(ctx, workspaceID, budget.Month{Year: 2026, Month: time.August}, budget.ReplaceCommand{
		NewBudgetID: uuid.NewString(), Name: "Database kind guard",
		Items: []budget.ReplaceItem{
			{ID: uuid.NewString(), CategoryID: income.ID, AmountBaseMinor: 1000},
		},
	})
	if !errors.Is(err, budget.ErrCategoryKind) {
		t.Fatalf("database income-category guard error = %v", err)
	}
	_, err = budgetRepository.Replace(ctx, workspaceID, budget.Month{Year: 2026, Month: time.August}, budget.ReplaceCommand{
		NewBudgetID: uuid.NewString(), Name: "Database archive guard",
		Items: []budget.ReplaceItem{
			{ID: uuid.NewString(), CategoryID: oldExpense.ID, AmountBaseMinor: 1000},
		},
	})
	if !errors.Is(err, budget.ErrCategoryArchived) {
		t.Fatalf("database archived-category guard error = %v", err)
	}
	_, err = budgetRepository.Replace(ctx, workspaceID, budget.Month{Year: 2026, Month: time.August}, budget.ReplaceCommand{
		NewBudgetID: uuid.NewString(), Name: "Database overlap guard",
		Items: []budget.ReplaceItem{
			{ID: uuid.NewString(), CategoryID: food.ID, AmountBaseMinor: 1000},
			{ID: uuid.NewString(), CategoryID: restaurants.ID, AmountBaseMinor: 1000},
		},
	})
	if !errors.Is(err, budget.ErrCategoryOverlap) {
		t.Fatalf("database branch-overlap guard error = %v", err)
	}

	_, err = categoryService.Update(ctx, workspaceID, ownerID, utilities.ID, category.WriteInput{
		Name: utilities.Name, Kind: utilities.Kind, ParentID: &food.ID,
	})
	if !errors.Is(err, category.ErrHierarchyConflict) {
		t.Fatalf("budget-breaking category reparent error = %v", err)
	}
}

func createBudgetAccount(
	t *testing.T,
	ctx context.Context,
	service *account.Service,
	workspaceID, userID string,
) account.Account {
	t.Helper()
	value, err := service.Create(ctx, workspaceID, userID, account.WriteInput{
		Name: "Checking", Type: account.TypeBank, Currency: "TRY",
	})
	if err != nil {
		t.Fatalf("create budget account: %v", err)
	}
	return value
}

func createBudgetCategory(
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
		t.Fatalf("create budget category %q: %v", name, err)
	}
	return value
}

func createBudgetTransaction(
	t *testing.T,
	ctx context.Context,
	service *transactiondomain.Service,
	workspaceID, userID, accountID, categoryID string,
	date time.Time,
	status transactiondomain.Status,
	amount int64,
) transactiondomain.Transaction {
	t.Helper()
	value, err := service.Create(ctx, workspaceID, userID, transactiondomain.WriteInput{
		Kind: transactiondomain.KindStandard, Status: status, TransactionDate: date,
		Entries: []transactiondomain.EntryInput{
			{AccountID: accountID, AmountMinor: amount, BaseAmountMinor: &amount},
		},
		Allocations: []transactiondomain.AllocationInput{
			{CategoryID: categoryID, AmountBaseMinor: amount},
		},
	})
	if err != nil {
		t.Fatalf("create budget transaction: %v", err)
	}
	return value
}

func budgetItemByCategory(
	t *testing.T,
	value budget.Budget,
	categoryID string,
) budget.Item {
	t.Helper()
	for _, item := range value.Items {
		if item.CategoryID == categoryID {
			return item
		}
	}
	t.Fatalf("budget item for category %s not found", categoryID)
	return budget.Item{}
}

func budgetDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}
