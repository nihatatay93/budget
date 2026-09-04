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
	"github.com/nihatatay93/budget/internal/category"
	cryptoplatform "github.com/nihatatay93/budget/internal/platform/crypto"
	transactiondomain "github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

func TestTransactionRepositoryLifecycleAndBalances(t *testing.T) {
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
		Email: "ledger@example.com", Password: "a sufficiently long password",
		DisplayName: "Ledger Owner", WorkspaceName: "Ledger", BaseCurrency: "TRY",
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
	repository := NewTransactionRepository(store.Pool())
	transactions := transactiondomain.NewService(repository, access, nil)

	checking, err := accountService.Create(ctx, workspaceID, ownerID, account.WriteInput{
		Name: "Checking", Type: account.TypeBank, Currency: "TRY",
	})
	if err != nil {
		t.Fatalf("create checking account: %v", err)
	}
	wallet, err := accountService.Create(ctx, workspaceID, ownerID, account.WriteInput{
		Name: "Wallet", Type: account.TypeCash, Currency: "TRY",
	})
	if err != nil {
		t.Fatalf("create wallet account: %v", err)
	}
	food, err := categoryService.Create(ctx, workspaceID, ownerID, category.WriteInput{
		Name: "Food", Kind: category.KindExpense,
	})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	date := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)

	expense, err := transactions.Create(ctx, workspaceID, ownerID, transactiondomain.WriteInput{
		Kind: transactiondomain.KindStandard, Status: transactiondomain.StatusPosted,
		TransactionDate: date,
		Entries:         []transactiondomain.EntryInput{{AccountID: checking.ID, AmountMinor: -1500}},
		Allocations:     []transactiondomain.AllocationInput{{CategoryID: food.ID, AmountBaseMinor: allocationAmountOf(-1500)}},
	})
	if err != nil {
		t.Fatalf("create posted expense: %v", err)
	}
	assertAccountBalance(t, ctx, accountService, workspaceID, ownerID, checking.ID, -1500)

	pending, err := transactions.Create(ctx, workspaceID, ownerID, transactiondomain.WriteInput{
		Kind: transactiondomain.KindAdjustment, Status: transactiondomain.StatusPending,
		TransactionDate: date,
		Entries:         []transactiondomain.EntryInput{{AccountID: checking.ID, AmountMinor: 2000}},
	})
	if err != nil {
		t.Fatalf("create pending adjustment: %v", err)
	}
	assertAccountBalance(t, ctx, accountService, workspaceID, ownerID, checking.ID, -1500)

	pending, err = transactions.Update(ctx, workspaceID, ownerID, pending.ID, transactiondomain.WriteInput{
		Kind: transactiondomain.KindAdjustment, Status: transactiondomain.StatusPosted,
		TransactionDate: date,
		Entries:         []transactiondomain.EntryInput{{AccountID: checking.ID, AmountMinor: 2000}},
	})
	if err != nil {
		t.Fatalf("post adjustment: %v", err)
	}
	assertAccountBalance(t, ctx, accountService, workspaceID, ownerID, checking.ID, 500)

	transfer, err := transactions.Create(ctx, workspaceID, ownerID, transactiondomain.WriteInput{
		Kind: transactiondomain.KindTransfer, Status: transactiondomain.StatusPosted,
		TransactionDate: date,
		Entries: []transactiondomain.EntryInput{
			{AccountID: checking.ID, AmountMinor: -500},
			{AccountID: wallet.ID, AmountMinor: 500},
		},
	})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	if len(transfer.Allocations) != 0 {
		t.Fatalf("transfer allocations = %#v, want none", transfer.Allocations)
	}
	assertAccountBalance(t, ctx, accountService, workspaceID, ownerID, checking.ID, 0)
	assertAccountBalance(t, ctx, accountService, workspaceID, ownerID, wallet.ID, 500)

	values, err := transactions.List(ctx, workspaceID, ownerID, transactiondomain.ListFilter{})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if len(values) != 3 {
		t.Fatalf("transactions = %d, want 3", len(values))
	}
	if err := transactions.SoftDelete(ctx, workspaceID, ownerID, pending.ID); err != nil {
		t.Fatalf("soft-delete adjustment: %v", err)
	}
	if _, err := transactions.Get(ctx, workspaceID, ownerID, pending.ID); !errors.Is(err, transactiondomain.ErrNotFound) {
		t.Fatalf("get deleted transaction error = %v, want ErrNotFound", err)
	}
	assertAccountBalance(t, ctx, accountService, workspaceID, ownerID, checking.ID, -2000)

	invalidTransfer := transactiondomain.Transaction{
		ID: uuid.NewString(), WorkspaceID: workspaceID, Kind: transactiondomain.KindTransfer,
		Status: transactiondomain.StatusPosted, TransactionDate: date,
		Source: transactiondomain.SourceManual, CreatedBy: ownerID, UpdatedBy: ownerID,
		Entries: []transactiondomain.Entry{{
			ID: uuid.NewString(), AccountID: checking.ID, AmountMinor: -100, BaseAmountMinor: -100,
		}},
	}
	if _, err := repository.Create(ctx, invalidTransfer); !errors.Is(err, transactiondomain.ErrDoesNotReconcile) {
		t.Fatalf("invalid deferred transfer error = %v, want ErrDoesNotReconcile", err)
	}

	if err := transactions.SoftDelete(ctx, workspaceID, ownerID, expense.ID); err != nil {
		t.Fatalf("soft-delete expense: %v", err)
	}
}

// allocationAmountOf states an allocation amount explicitly. A nil amount would let the
// service derive the lone allocation from the entry total, which is a different rule than
// the one under test here.
func allocationAmountOf(value int64) *int64 { return &value }

func assertAccountBalance(
	t *testing.T,
	ctx context.Context,
	service *account.Service,
	workspaceID, userID, accountID string,
	want int64,
) {
	t.Helper()
	value, err := service.Get(ctx, workspaceID, userID, accountID)
	if err != nil {
		t.Fatalf("get account %s: %v", accountID, err)
	}
	if value.BalanceMinor != want {
		t.Fatalf("account %s balance = %d, want %d", accountID, value.BalanceMinor, want)
	}
}
