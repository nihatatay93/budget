//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nihatatay93/budget/internal/platform/postgres/migrations"
)

type fixture struct {
	userID            string
	workspaceID       string
	accountID         string
	expenseCategoryID string
	incomeCategoryID  string
}

func TestInitialSchema(t *testing.T) {
	ctx := context.Background()
	db := migratedDatabase(t, ctx)
	data := seedFixture(t, ctx, db)

	t.Run("creates twelve domain tables", func(t *testing.T) {
		var count int
		// exchange_rates is an infrastructure cache for display conversion, not part of the
		// twelve-table domain schema in docs/domain-model.md, so it is excluded here.
		if err := db.QueryRowContext(ctx, `
            SELECT COUNT(*)
            FROM information_schema.tables
            WHERE table_schema = 'public'
              AND table_name NOT IN ('goose_db_version', 'exchange_rates')
        `).Scan(&count); err != nil {
			t.Fatalf("count domain tables: %v", err)
		}
		if count != 12 {
			t.Fatalf("domain table count = %d, want 12", count)
		}
	})

	t.Run("pending standard transaction reconciles", func(t *testing.T) {
		insertStandardTransaction(t, ctx, db, data, "pending", -35000, data.expenseCategoryID, false)
	})

	t.Run("posted refund can use expense category", func(t *testing.T) {
		insertStandardTransaction(t, ctx, db, data, "posted", 10000, data.expenseCategoryID, false)
	})

	t.Run("negative reversal can use income category", func(t *testing.T) {
		insertStandardTransaction(t, ctx, db, data, "posted", -10000, data.incomeCategoryID, false)
	})

	t.Run("allocation can be inserted before entry", func(t *testing.T) {
		insertStandardTransaction(t, ctx, db, data, "posted", -5000, data.expenseCategoryID, true)
	})

	t.Run("invalid standard transaction fails at commit", func(t *testing.T) {
		insertStandardTransaction(t, ctx, db, data, "posted", -35000, "", false)
	})

	t.Run("deleting an allocation fails at commit", func(t *testing.T) {
		transactionID := insertStandardTransaction(
			t, ctx, db, data, "posted", -2500, data.expenseCategoryID, false,
		)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin transaction: %v", err)
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM transaction_allocations WHERE transaction_id = $1",
			transactionID,
		); err != nil {
			t.Fatalf("delete allocation: %v", err)
		}
		if err := tx.Commit(); err == nil {
			t.Fatal("commit invalid aggregate: expected reconciliation error")
		}
	})

	t.Run("moving a complete aggregate validates its former transaction", func(t *testing.T) {
		fromID := insertStandardTransaction(
			t, ctx, db, data, "posted", -7000, data.expenseCategoryID, false,
		)
		toID := insertStandardTransaction(
			t, ctx, db, data, "posted", -3000, data.expenseCategoryID, false,
		)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin transaction: %v", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE transaction_entries SET transaction_id = $1 WHERE transaction_id = $2",
			toID, fromID,
		); err != nil {
			t.Fatalf("move entry: %v", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE transaction_allocations SET transaction_id = $1 WHERE transaction_id = $2",
			toID, fromID,
		); err != nil {
			t.Fatalf("move allocation: %v", err)
		}
		if err := tx.Commit(); err == nil {
			t.Fatal("commit invalid former aggregate: expected reconciliation error")
		}
	})

	t.Run("ordinary categories can be deleted", func(t *testing.T) {
		var categoryID string
		if err := db.QueryRowContext(ctx, `
			INSERT INTO categories (workspace_id, name, kind)
			VALUES ($1, 'Temporary', 'expense')
			RETURNING id
		`, data.workspaceID).Scan(&categoryID); err != nil {
			t.Fatalf("insert temporary category: %v", err)
		}
		result, err := db.ExecContext(ctx, "DELETE FROM categories WHERE id = $1", categoryID)
		if err != nil {
			t.Fatalf("delete ordinary category: %v", err)
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			t.Fatalf("deleted rows = %d, error = %v, want 1", affected, err)
		}
	})

	t.Run("system category is protected but workspace cascade succeeds", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin disposable workspace: %v", err)
		}
		var workspaceID, categoryID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO workspaces (name, base_currency, timezone, created_by)
			VALUES ('Disposable', 'TRY', 'Europe/Istanbul', $1)
			RETURNING id
		`, data.userID).Scan(&workspaceID); err != nil {
			t.Fatalf("insert disposable workspace: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workspace_members (workspace_id, user_id, role)
			VALUES ($1, $2, 'owner')
		`, workspaceID, data.userID); err != nil {
			t.Fatalf("insert disposable owner: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit disposable workspace: %v", err)
		}
		if err := db.QueryRowContext(ctx, `
			INSERT INTO categories (workspace_id, name, kind, system_key)
			VALUES ($1, 'Uncategorized Expense', 'expense', 'uncategorized_expense')
			RETURNING id
		`, workspaceID).Scan(&categoryID); err != nil {
			t.Fatalf("insert system category: %v", err)
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM categories WHERE id = $1", categoryID); err == nil {
			t.Fatal("direct system-category delete succeeded")
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM workspaces WHERE id = $1", workspaceID); err != nil {
			t.Fatalf("delete workspace with system category: %v", err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM categories WHERE id = $1", categoryID).Scan(&count); err != nil {
			t.Fatalf("count cascaded category: %v", err)
		}
		if count != 0 {
			t.Fatalf("categories after workspace delete = %d, want 0", count)
		}
	})

	t.Run("new workspace requires an owner in the same transaction", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO workspaces (name, base_currency, timezone, created_by)
			VALUES ('Ownerless', 'TRY', 'Europe/Istanbul', $1)
		`, data.userID); err == nil {
			t.Fatal("ownerless workspace committed")
		}
	})

	t.Run("concurrent owner demotions cannot remove the final owner", func(t *testing.T) {
		var secondOwnerID string
		if err := db.QueryRowContext(ctx, `
			INSERT INTO users (email, password_hash, display_name)
			VALUES ('second-owner@example.com', 'not-a-real-hash', 'Second Owner')
			RETURNING id
		`).Scan(&secondOwnerID); err != nil {
			t.Fatalf("insert second owner user: %v", err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO workspace_members (workspace_id, user_id, role)
			VALUES ($1, $2, 'owner')
		`, data.workspaceID, secondOwnerID); err != nil {
			t.Fatalf("insert second owner membership: %v", err)
		}
		first, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin first owner transaction: %v", err)
		}
		defer func() { _ = first.Rollback() }()
		second, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin second owner transaction: %v", err)
		}
		defer func() { _ = second.Rollback() }()
		if _, err := first.ExecContext(ctx, `
			UPDATE workspace_members SET role = 'member'
			WHERE workspace_id = $1 AND user_id = $2
		`, data.workspaceID, data.userID); err != nil {
			t.Fatalf("demote first owner: %v", err)
		}
		if _, err := second.ExecContext(ctx, `
			UPDATE workspace_members SET role = 'member'
			WHERE workspace_id = $1 AND user_id = $2
		`, data.workspaceID, secondOwnerID); err != nil {
			t.Fatalf("demote second owner: %v", err)
		}
		if err := first.Commit(); err != nil {
			t.Fatalf("commit first owner demotion: %v", err)
		}
		if err := second.Commit(); err == nil {
			t.Fatal("concurrent final-owner demotion committed")
		}
		var activeOwners int
		if err := db.QueryRowContext(ctx, `
			SELECT count(*) FROM workspace_members
			WHERE workspace_id = $1 AND role = 'owner' AND removed_at IS NULL
		`, data.workspaceID).Scan(&activeOwners); err != nil {
			t.Fatalf("count remaining owners: %v", err)
		}
		if activeOwners != 1 {
			t.Fatalf("active owners = %d, want 1", activeOwners)
		}
	})
}

func TestSupportedCurrencyMigrationPreservesLegacyRows(t *testing.T) {
	ctx := context.Background()
	db, provider := migrationDatabase(t, ctx)
	if _, err := provider.UpTo(ctx, 2); err != nil {
		t.Fatalf("apply migrations through 00002: %v", err)
	}

	var userID, workspaceID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('legacy@example.com', 'not-a-real-hash', 'Legacy')
		RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO workspaces (name, base_currency, timezone, created_by)
		VALUES ('Legacy', 'GBP', 'Europe/London', $1)
		RETURNING id
	`, userID).Scan(&workspaceID); err != nil {
		t.Fatalf("insert legacy workspace: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO accounts (workspace_id, name, type, currency)
		VALUES ($1, 'Legacy account', 'bank', 'GBP')
	`, workspaceID); err != nil {
		t.Fatalf("insert legacy account: %v", err)
	}

	if _, err := provider.UpTo(ctx, 3); err != nil {
		t.Fatalf("apply supported-currency migration: %v", err)
	}
	var workspaceCurrency, accountCurrency string
	if err := db.QueryRowContext(ctx, "SELECT base_currency FROM workspaces WHERE id = $1", workspaceID).Scan(&workspaceCurrency); err != nil {
		t.Fatalf("read legacy workspace: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT currency FROM accounts WHERE workspace_id = $1", workspaceID).Scan(&accountCurrency); err != nil {
		t.Fatalf("read legacy account: %v", err)
	}
	if workspaceCurrency != "GBP" || accountCurrency != "GBP" {
		t.Fatalf("legacy currencies = %s/%s, want GBP/GBP", workspaceCurrency, accountCurrency)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO accounts (workspace_id, name, type, currency)
		VALUES ($1, 'Unsupported', 'bank', 'GBP')
	`, workspaceID); err == nil {
		t.Fatal("00003 allowed a new unsupported account currency")
	}
}

func TestCollaborationMigrationRepairsLegacyOwnerlessWorkspace(t *testing.T) {
	ctx := context.Background()
	db, provider := migrationDatabase(t, ctx)
	if _, err := provider.UpTo(ctx, 5); err != nil {
		t.Fatalf("apply migrations through 00005: %v", err)
	}

	var userID, workspaceID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('legacy-collaboration@example.com', 'not-a-real-hash', 'Legacy Collaboration')
		RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("insert legacy collaboration user: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO workspaces (name, base_currency, timezone, created_by)
		VALUES ('Legacy Collaboration', 'TRY', 'Europe/Istanbul', $1)
		RETURNING id
	`, userID).Scan(&workspaceID); err != nil {
		t.Fatalf("insert legacy ownerless workspace: %v", err)
	}

	if _, err := provider.UpTo(ctx, 6); err != nil {
		t.Fatalf("apply collaboration migration: %v", err)
	}
	var role string
	var removed bool
	if err := db.QueryRowContext(ctx, `
		SELECT role, removed_at IS NOT NULL
		FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, userID).Scan(&role, &removed); err != nil {
		t.Fatalf("read repaired owner membership: %v", err)
	}
	if role != "owner" || removed {
		t.Fatalf("repaired membership = role %q, removed %v", role, removed)
	}
}

func migratedDatabase(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	db, provider := migrationDatabase(t, ctx)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("apply embedded migrations: %v", err)
	}
	return db
}

func migrationDatabase(t *testing.T, ctx context.Context) (*sql.DB, *goose.Provider) {
	t.Helper()

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

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatalf("create Goose provider: %v", err)
	}
	return db, provider
}

func seedFixture(t *testing.T, ctx context.Context, db *sql.DB) fixture {
	t.Helper()

	var data fixture
	if err := db.QueryRowContext(ctx, `
        INSERT INTO users (email, password_hash, display_name)
        VALUES ('owner@example.com', 'not-a-real-hash', 'Owner')
        RETURNING id
    `).Scan(&data.userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin workspace fixture: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `
        INSERT INTO workspaces (name, base_currency, timezone, created_by)
        VALUES ('Personal', 'TRY', 'Europe/Istanbul', $1)
        RETURNING id
    `, data.userID).Scan(&data.workspaceID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO workspace_members (workspace_id, user_id, role)
        VALUES ($1, $2, 'owner')
    `, data.workspaceID, data.userID); err != nil {
		t.Fatalf("insert workspace member: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit workspace fixture: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
        INSERT INTO accounts (workspace_id, name, type, currency)
        VALUES ($1, 'Checking', 'bank', 'TRY')
        RETURNING id
    `, data.workspaceID).Scan(&data.accountID); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
        INSERT INTO categories (workspace_id, name, kind)
        VALUES ($1, 'Restaurants', 'expense')
        RETURNING id
	`, data.workspaceID).Scan(&data.expenseCategoryID); err != nil {
		t.Fatalf("insert expense category: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
        INSERT INTO categories (workspace_id, name, kind)
        VALUES ($1, 'Salary', 'income')
        RETURNING id
    `, data.workspaceID).Scan(&data.incomeCategoryID); err != nil {
		t.Fatalf("insert income category: %v", err)
	}

	return data
}

func insertStandardTransaction(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	data fixture,
	status string,
	amount int64,
	categoryID string,
	allocationFirst bool,
) string {
	t.Helper()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	var transactionID string
	if err := tx.QueryRowContext(ctx, `
        INSERT INTO transactions (
            workspace_id, kind, status, transaction_date, source, created_by, updated_by
        )
        VALUES ($1, 'standard', $2, CURRENT_DATE, 'manual', $3, $3)
        RETURNING id
    `, data.workspaceID, status, data.userID).Scan(&transactionID); err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
	insertEntry := func() {
		if _, err := tx.ExecContext(ctx, `
        INSERT INTO transaction_entries (
            workspace_id, transaction_id, account_id, amount_minor, base_amount_minor
        )
        VALUES ($1, $2, $3, $4, $4)
	    `, data.workspaceID, transactionID, data.accountID, amount); err != nil {
			t.Fatalf("insert entry: %v", err)
		}
	}
	insertAllocation := func() {
		if categoryID == "" {
			return
		}
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO transaction_allocations (
                workspace_id, transaction_id, category_id, amount_base_minor
            )
            VALUES ($1, $2, $3, $4)
	        `, data.workspaceID, transactionID, categoryID, amount); err != nil {
			t.Fatalf("insert allocation: %v", err)
		}
	}

	if allocationFirst {
		insertAllocation()
		insertEntry()
	} else {
		insertEntry()
		insertAllocation()
	}

	err = tx.Commit()
	if categoryID != "" && err != nil {
		t.Fatalf("commit valid aggregate: %v", err)
	}
	if categoryID == "" && err == nil {
		t.Fatal("commit invalid aggregate: expected reconciliation error")
	}
	return transactionID
}

// Existing tests cover one migration at a time. This upgrades a database that was populated
// at the initial schema all the way to head, which is what a self-hoster actually does: the
// risk is not a migration failing in isolation but a later one invalidating data an earlier
// one accepted.
func TestUpgradeFromInitialSchemaPreservesFinancialData(t *testing.T) {
	ctx := context.Background()
	db, provider := migrationDatabase(t, ctx)
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("apply the initial schema: %v", err)
	}

	var userID, workspaceID, accountID, categoryID, transactionID string
	seed := func(query string, args []any, into ...*string) {
		t.Helper()
		targets := make([]any, 0, len(into))
		for _, target := range into {
			targets = append(targets, target)
		}
		if len(targets) == 0 {
			if _, err := db.ExecContext(ctx, query, args...); err != nil {
				t.Fatalf("seed: %v\n%s", err, query)
			}
			return
		}
		if err := db.QueryRowContext(ctx, query, args...).Scan(targets...); err != nil {
			t.Fatalf("seed: %v\n%s", err, query)
		}
	}

	seed(`INSERT INTO users (email, password_hash, display_name)
	      VALUES ('legacy@example.com', 'not-a-real-hash', 'Legacy') RETURNING id`,
		nil, &userID)
	seed(`INSERT INTO workspaces (name, base_currency, timezone, created_by)
	      VALUES ('Legacy', 'TRY', 'Europe/Istanbul', $1) RETURNING id`,
		[]any{userID}, &workspaceID)
	seed(`INSERT INTO workspace_members (workspace_id, user_id, role)
	      VALUES ($1, $2, 'owner')`, []any{workspaceID, userID})
	seed(`INSERT INTO accounts (workspace_id, name, type, currency)
	      VALUES ($1, 'Checking', 'bank', 'TRY') RETURNING id`,
		[]any{workspaceID}, &accountID)
	seed(`INSERT INTO categories (workspace_id, name, kind, system_key)
	      VALUES ($1, 'Uncategorized Expense', 'expense', 'uncategorized_expense') RETURNING id`,
		[]any{workspaceID}, &categoryID)
	// The whole aggregate is one statement: the reconciliation trigger is deferred to commit,
	// so a transaction row committed on its own would fail with no entries to balance it.
	seed(`WITH new_transaction AS (
	          INSERT INTO transactions (
	              workspace_id, kind, status, transaction_date, payee, source,
	              created_by, updated_by
	          ) VALUES ($1, 'standard', 'posted', DATE '2026-08-18', 'Migros', 'manual', $2, $2)
	          RETURNING id
	      ), new_entry AS (
	          INSERT INTO transaction_entries (
	              workspace_id, transaction_id, account_id, amount_minor, base_amount_minor
	          )
	          SELECT $1, new_transaction.id, $3, -150000, -150000 FROM new_transaction
	      )
	      INSERT INTO transaction_allocations (
	          workspace_id, transaction_id, category_id, amount_base_minor
	      )
	      SELECT $1, new_transaction.id, $4, -150000 FROM new_transaction
	      RETURNING transaction_id`,
		[]any{workspaceID, userID, accountID, categoryID}, &transactionID)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("upgrade to head: %v", err)
	}

	// The ledger must survive byte for byte: an upgrade that silently rewrote a stored amount
	// would change a user's financial history.
	var entryAmount, allocationAmount int64
	if err := db.QueryRowContext(ctx,
		"SELECT amount_minor FROM transaction_entries WHERE transaction_id = $1", transactionID,
	).Scan(&entryAmount); err != nil {
		t.Fatalf("read entry after upgrade: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		"SELECT amount_base_minor FROM transaction_allocations WHERE transaction_id = $1",
		transactionID,
	).Scan(&allocationAmount); err != nil {
		t.Fatalf("read allocation after upgrade: %v", err)
	}
	if entryAmount != -150000 || allocationAmount != -150000 {
		t.Fatalf("amounts after upgrade = %d/%d, want -150000/-150000",
			entryAmount, allocationAmount)
	}

	// The derived balance still agrees with the preserved entry.
	var balance int64
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(transaction_entries.amount_minor), 0)
		FROM transaction_entries
		JOIN transactions ON transactions.id = transaction_entries.transaction_id
		WHERE transaction_entries.account_id = $1
		  AND transactions.status = 'posted'
		  AND transactions.deleted_at IS NULL
	`, accountID).Scan(&balance); err != nil {
		t.Fatalf("derive balance after upgrade: %v", err)
	}
	if balance != -150000 {
		t.Fatalf("derived balance after upgrade = %d, want -150000", balance)
	}

	// Columns added by later migrations exist with usable defaults on the legacy row.
	var removedAt *time.Time
	if err := db.QueryRowContext(ctx,
		"SELECT removed_at FROM workspace_members WHERE workspace_id = $1 AND user_id = $2",
		workspaceID, userID,
	).Scan(&removedAt); err != nil {
		t.Fatalf("read migrated membership: %v", err)
	}
	if removedAt != nil {
		t.Fatalf("upgrade marked an active member as removed at %v", removedAt)
	}

	// Invariants introduced after the data was written now apply to it.
	if _, err := db.ExecContext(ctx,
		"UPDATE accounts SET currency = 'GBP' WHERE id = $1", accountID,
	); err == nil {
		t.Fatal("an unsupported currency was accepted after the upgrade")
	}
	if _, err := db.ExecContext(ctx,
		"DELETE FROM categories WHERE id = $1", categoryID,
	); err == nil {
		t.Fatal("a protected system category was deleted after the upgrade")
	}
}
