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
	"github.com/nihatatay93/budget/internal/workspace"
)

func TestFinancialSetupRepositories(t *testing.T) {
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
		Email: "owner@example.com", Password: "a sufficiently long password",
		DisplayName: "Owner", WorkspaceName: "Personal", BaseCurrency: "TRY",
		Timezone: "Europe/Istanbul", Transport: auth.TransportBearer,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	workspaceID := registration.Workspaces[0].ID
	ownerID := registration.Principal.User.ID
	access := workspace.NewAuthorizer(NewWorkspaceRepository(store.Pool()))
	accounts := account.NewService(NewAccountRepository(store.Pool()), access)
	categories := category.NewService(NewCategoryRepository(store.Pool()), access)

	createdAccount, err := accounts.Create(ctx, workspaceID, ownerID, account.WriteInput{
		Name: "Checking", Type: account.TypeBank, Currency: "TRY",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if createdAccount.BalanceMinor != 0 {
		t.Fatalf("initial balance = %d, want 0", createdAccount.BalanceMinor)
	}

	parent, err := categories.Create(ctx, workspaceID, ownerID, category.WriteInput{
		Name: "Food", Kind: category.KindExpense,
	})
	if err != nil {
		t.Fatalf("create parent category: %v", err)
	}
	parentID := parent.ID
	child, err := categories.Create(ctx, workspaceID, ownerID, category.WriteInput{
		Name: "Dining", Kind: category.KindExpense, ParentID: &parentID,
	})
	if err != nil {
		t.Fatalf("create child category: %v", err)
	}
	childID := child.ID
	_, err = categories.Update(ctx, workspaceID, ownerID, parent.ID, category.WriteInput{
		Name: "Food", Kind: category.KindExpense, ParentID: &childID,
	})
	if !errors.Is(err, category.ErrHierarchyConflict) {
		t.Fatalf("cycle update error = %v, want ErrHierarchyConflict", err)
	}
	if err := categories.Archive(ctx, workspaceID, ownerID, parent.ID); !errors.Is(err, category.ErrHasChildren) {
		t.Fatalf("archive parent error = %v, want ErrHasChildren", err)
	}
	if err := categories.Archive(ctx, workspaceID, ownerID, child.ID); err != nil {
		t.Fatalf("archive child: %v", err)
	}
	_, err = categories.Update(ctx, workspaceID, ownerID, parent.ID, category.WriteInput{
		Name: "Food", Kind: category.KindIncome,
	})
	if !errors.Is(err, category.ErrKindLocked) {
		t.Fatalf("kind change with archived child error = %v, want ErrKindLocked", err)
	}

	allCategories, err := categories.List(ctx, workspaceID, ownerID, false)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	var systemCategory category.Category
	for _, value := range allCategories {
		if value.SystemKey != nil {
			systemCategory = value
			break
		}
	}
	if systemCategory.SystemKey == nil {
		t.Fatal("registration did not create protected system categories")
	}
	_, err = categories.Update(ctx, workspaceID, ownerID, systemCategory.ID, category.WriteInput{
		Name: "Renamed", Kind: systemCategory.Kind,
	})
	if !errors.Is(err, category.ErrProtected) {
		t.Fatalf("update system category error = %v, want ErrProtected", err)
	}
	if _, err := store.Pool().Exec(ctx, `UPDATE categories SET name = 'Bypass' WHERE id = $1`, systemCategory.ID); err == nil {
		t.Fatal("database allowed direct system-category mutation")
	}

	transactionID := uuid.New()
	entryID := uuid.New()
	transaction, err := store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin posted adjustment: %v", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO transactions (
			id, workspace_id, kind, status, transaction_date, source, created_by, updated_by
		) VALUES ($1, $2, 'adjustment', 'posted', CURRENT_DATE, 'manual', $3, $3)
	`, transactionID, workspaceID, ownerID); err != nil {
		t.Fatalf("insert adjustment transaction: %v", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO transaction_entries (
			id, workspace_id, transaction_id, account_id, amount_minor, base_amount_minor
		) VALUES ($1, $2, $3, $4, 50000, 50000)
	`, entryID, workspaceID, transactionID, createdAccount.ID); err != nil {
		t.Fatalf("insert adjustment entry: %v", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		t.Fatalf("commit posted adjustment: %v", err)
	}
	accountWithBalance, err := accounts.Get(ctx, workspaceID, ownerID, createdAccount.ID)
	if err != nil {
		t.Fatalf("get account balance: %v", err)
	}
	if accountWithBalance.BalanceMinor != 50000 {
		t.Fatalf("posted balance = %d, want 50000", accountWithBalance.BalanceMinor)
	}
	pendingTransactionID := uuid.New()
	pendingEntryID := uuid.New()
	pendingTransaction, err := store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin pending adjustment: %v", err)
	}
	if _, err := pendingTransaction.Exec(ctx, `
		INSERT INTO transactions (
			id, workspace_id, kind, status, transaction_date, source, created_by, updated_by
		) VALUES ($1, $2, 'adjustment', 'pending', CURRENT_DATE, 'manual', $3, $3)
	`, pendingTransactionID, workspaceID, ownerID); err != nil {
		t.Fatalf("insert pending adjustment: %v", err)
	}
	if _, err := pendingTransaction.Exec(ctx, `
		INSERT INTO transaction_entries (
			id, workspace_id, transaction_id, account_id, amount_minor, base_amount_minor
		) VALUES ($1, $2, $3, $4, 900000, 900000)
	`, pendingEntryID, workspaceID, pendingTransactionID, createdAccount.ID); err != nil {
		t.Fatalf("insert pending adjustment entry: %v", err)
	}
	if err := pendingTransaction.Commit(ctx); err != nil {
		t.Fatalf("commit pending adjustment: %v", err)
	}
	accountWithBalance, err = accounts.Get(ctx, workspaceID, ownerID, createdAccount.ID)
	if err != nil {
		t.Fatalf("get account balance after pending entry: %v", err)
	}
	if accountWithBalance.BalanceMinor != 50000 {
		t.Fatalf("balance including pending entry = %d, want posted-only 50000", accountWithBalance.BalanceMinor)
	}
	_, err = accounts.Update(ctx, workspaceID, ownerID, createdAccount.ID, account.WriteInput{
		Name: "Checking", Type: account.TypeBank, Currency: "USD",
	})
	if !errors.Is(err, account.ErrCurrencyLocked) {
		t.Fatalf("currency update error = %v, want ErrCurrencyLocked", err)
	}

	viewerID := uuid.New()
	if _, err := store.Pool().Exec(ctx, `
		INSERT INTO users (id, email, password_hash, display_name)
		VALUES ($1, 'viewer@example.com', 'unused', 'Viewer')
	`, viewerID); err != nil {
		t.Fatalf("insert viewer: %v", err)
	}
	if _, err := store.Pool().Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'viewer')
	`, workspaceID, viewerID); err != nil {
		t.Fatalf("add viewer: %v", err)
	}
	if _, err := accounts.List(ctx, workspaceID, viewerID.String(), false); err != nil {
		t.Fatalf("viewer list accounts: %v", err)
	}
	_, err = accounts.Create(ctx, workspaceID, viewerID.String(), account.WriteInput{
		Name: "Viewer cash", Type: account.TypeCash, Currency: "TRY",
	})
	if !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("viewer create account error = %v, want ErrForbidden", err)
	}

	if err := accounts.Archive(ctx, workspaceID, ownerID, createdAccount.ID); err != nil {
		t.Fatalf("archive account: %v", err)
	}
	activeAccounts, err := accounts.List(ctx, workspaceID, ownerID, false)
	if err != nil {
		t.Fatalf("list active accounts: %v", err)
	}
	if len(activeAccounts) != 0 {
		t.Fatalf("active accounts = %d, want 0", len(activeAccounts))
	}
	archivedAccounts, err := accounts.List(ctx, workspaceID, ownerID, true)
	if err != nil {
		t.Fatalf("list archived accounts: %v", err)
	}
	if len(archivedAccounts) != 1 || archivedAccounts[0].ArchivedAt == nil {
		t.Fatalf("archived accounts = %+v", archivedAccounts)
	}
}
