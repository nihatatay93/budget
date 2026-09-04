//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/category"
	cryptoplatform "github.com/nihatatay93/budget/internal/platform/crypto"
)

func TestAuthRepositoryRegistrationLifecycle(t *testing.T) {
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

	service, err := auth.NewService(
		NewAuthRepository(store.Pool()), cryptoplatform.PasswordHasher{}, 24*time.Hour,
	)
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	result, err := service.Register(ctx, auth.RegisterInput{
		Email: "owner@example.com", Password: "a sufficiently long password",
		DisplayName: "Owner", WorkspaceName: "Personal", BaseCurrency: "TRY",
		Timezone: "Europe/Istanbul", Transport: auth.TransportBearer,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Counted from the canonical list: a magic number here goes stale every time a
	// predefined category is added, and reports the addition as a repository failure.
	wantPredefined := len(category.PredefinedCategories())
	var members, categories, predefinedCategories, sessions int
	var persistedHash []byte
	var transport string
	if err := store.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM workspace_members`).Scan(&members); err != nil {
		t.Fatalf("count workspace members: %v", err)
	}
	if err := store.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM categories WHERE system_key IS NOT NULL`).Scan(&categories); err != nil {
		t.Fatalf("count system categories: %v", err)
	}
	if err := store.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM categories WHERE predefined_key IS NOT NULL`).Scan(&predefinedCategories); err != nil {
		t.Fatalf("count predefined categories: %v", err)
	}
	if err := store.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if err := store.Pool().QueryRow(ctx, `SELECT token_hash, transport FROM sessions LIMIT 1`).Scan(
		&persistedHash, &transport,
	); err != nil {
		t.Fatalf("read sessions: %v", err)
	}
	if members != 1 || categories != 2 || predefinedCategories != wantPredefined || sessions != 1 || transport != "bearer" {
		t.Fatalf(
			"registration rows = members %d, categories %d, predefined categories %d, sessions %d, transport %q",
			members, categories, predefinedCategories, sessions, transport,
		)
	}
	categoryRepository := NewCategoryRepository(store.Pool())
	if len(result.Workspaces) != 1 {
		t.Fatalf("registered workspaces = %d, want 1", len(result.Workspaces))
	}
	if err := categoryRepository.EnsurePredefined(ctx, result.Workspaces[0].ID); err != nil {
		t.Fatalf("ensure predefined categories: %v", err)
	}
	if err := store.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM categories WHERE predefined_key IS NOT NULL`).Scan(&predefinedCategories); err != nil {
		t.Fatalf("recount predefined categories: %v", err)
	}
	if predefinedCategories != wantPredefined {
		t.Fatalf(
			"predefined categories after repeat = %d, want %d",
			predefinedCategories, wantPredefined,
		)
	}
	if string(persistedHash) == result.Token {
		t.Fatal("database persisted the raw bearer token")
	}
	principal, err := service.Authenticate(ctx, result.Token, auth.TransportBearer)
	if err != nil || principal.User.Email != "owner@example.com" {
		t.Fatalf("authenticate = %+v, %v", principal, err)
	}

	_, err = service.Register(ctx, auth.RegisterInput{
		Email: "OWNER@example.com", Password: "another sufficiently long password",
		DisplayName: "Other", WorkspaceName: "Other", BaseCurrency: "USD",
		Timezone: "UTC", Transport: auth.TransportCookie,
	})
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("duplicate registration error = %v, want ErrEmailTaken", err)
	}
	if err := store.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM workspaces`).Scan(&members); err != nil {
		t.Fatalf("count workspaces after duplicate: %v", err)
	}
	if members != 1 {
		t.Fatalf("workspace count after duplicate = %d, want atomic rollback", members)
	}
}
