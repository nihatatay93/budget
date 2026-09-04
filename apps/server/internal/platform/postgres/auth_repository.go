package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
)

type AuthRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

func (r *AuthRepository) Register(ctx context.Context, registration auth.Registration) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := sqlc.New(tx)
	userID, err := postgresUUID(registration.UserID)
	if err != nil {
		return err
	}
	workspaceID, err := postgresUUID(registration.WorkspaceID)
	if err != nil {
		return err
	}
	if _, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID: userID, Email: registration.Email, PasswordHash: registration.PasswordHash,
		DisplayName: registration.DisplayName,
	}); err != nil {
		if uniqueViolation(err, "users_email_unique") {
			return auth.ErrEmailTaken
		}
		return fmt.Errorf("create user: %w", err)
	}
	if _, err := queries.CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{
		ID: workspaceID, Name: registration.WorkspaceName, BaseCurrency: registration.BaseCurrency,
		Timezone: registration.Timezone, CreatedBy: userID,
	}); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	if err := queries.AddWorkspaceMember(ctx, sqlc.AddWorkspaceMemberParams{
		WorkspaceID: workspaceID, UserID: userID, Role: "owner",
	}); err != nil {
		return fmt.Errorf("add workspace owner: %w", err)
	}
	if err := r.createSystemCategories(ctx, queries, workspaceID, registration); err != nil {
		return err
	}
	if err := createPredefinedCategories(ctx, queries, workspaceID); err != nil {
		return err
	}
	if err := createSession(ctx, queries, registration.Session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit registration: %w", err)
	}
	return nil
}

func createPredefinedCategories(ctx context.Context, queries *sqlc.Queries, workspaceID pgtype.UUID) error {
	for _, value := range category.PredefinedCategories() {
		if err := queries.CreatePredefinedCategory(ctx, sqlc.CreatePredefinedCategoryParams{
			WorkspaceID: workspaceID, Name: value.Key, Kind: string(value.Kind),
			PredefinedKey: pgtype.Text{String: value.Key, Valid: true},
			ParentKey:     pgtype.Text{String: value.ParentKey, Valid: value.ParentKey != ""},
			Icon:          pgtype.Text{String: value.Appearance.IconValue, Valid: true},
			IconType:      string(value.Appearance.IconType),
			IconValue:     value.Appearance.IconValue,
			ColorKey:      value.Appearance.ColorKey,
		}); err != nil {
			return fmt.Errorf("create predefined category %s: %w", value.Key, err)
		}
	}
	return nil
}

func (r *AuthRepository) createSystemCategories(
	ctx context.Context,
	queries *sqlc.Queries,
	workspaceID pgtype.UUID,
	registration auth.Registration,
) error {
	categories := []struct {
		id, name, kind, key string
	}{
		{registration.ExpenseCategoryID, "Uncategorized Expense", "expense", "uncategorized_expense"},
		{registration.IncomeCategoryID, "Uncategorized Income", "income", "uncategorized_income"},
	}
	for _, category := range categories {
		categoryID, err := postgresUUID(category.id)
		if err != nil {
			return err
		}
		if err := queries.CreateSystemCategory(ctx, sqlc.CreateSystemCategoryParams{
			ID: categoryID, WorkspaceID: workspaceID, Name: category.name, Kind: category.kind,
			SystemKey: pgtype.Text{String: category.key, Valid: true},
		}); err != nil {
			return fmt.Errorf("create system category %s: %w", category.key, err)
		}
	}
	return nil
}

func (r *AuthRepository) UserByEmail(ctx context.Context, email string) (auth.StoredUser, error) {
	row, err := sqlc.New(r.pool).GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.StoredUser{}, auth.ErrInvalidCredentials
	}
	if err != nil {
		return auth.StoredUser{}, fmt.Errorf("get user by email: %w", err)
	}
	id, err := stringUUID(row.ID)
	if err != nil {
		return auth.StoredUser{}, err
	}
	return auth.StoredUser{
		User:         auth.User{ID: id, Email: row.Email, DisplayName: row.DisplayName},
		PasswordHash: row.PasswordHash,
	}, nil
}

func (r *AuthRepository) CreateSession(ctx context.Context, session auth.Session) error {
	return createSession(ctx, sqlc.New(r.pool), session)
}

func createSession(ctx context.Context, queries *sqlc.Queries, session auth.Session) error {
	sessionID, err := postgresUUID(session.ID)
	if err != nil {
		return err
	}
	userID, err := postgresUUID(session.UserID)
	if err != nil {
		return err
	}
	if err := queries.CreateSession(ctx, sqlc.CreateSessionParams{
		ID: sessionID, UserID: userID, TokenHash: session.TokenHash, Transport: string(session.Transport),
		ExpiresAt: pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *AuthRepository) SessionByTokenHash(ctx context.Context, tokenHash []byte) (auth.Principal, error) {
	row, err := sqlc.New(r.pool).GetSessionByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Principal{}, auth.ErrUnauthorized
	}
	if err != nil {
		return auth.Principal{}, fmt.Errorf("get session: %w", err)
	}
	sessionID, err := stringUUID(row.SessionID)
	if err != nil {
		return auth.Principal{}, err
	}
	userID, err := stringUUID(row.UserID)
	if err != nil {
		return auth.Principal{}, err
	}
	return auth.Principal{
		SessionID: sessionID,
		User:      auth.User{ID: userID, Email: row.Email, DisplayName: row.DisplayName},
		Transport: auth.Transport(row.Transport),
	}, nil
}

func (r *AuthRepository) DeleteSession(ctx context.Context, sessionID, userID string) error {
	sessionUUID, err := postgresUUID(sessionID)
	if err != nil {
		return err
	}
	userUUID, err := postgresUUID(userID)
	if err != nil {
		return err
	}
	if _, err := sqlc.New(r.pool).DeleteSession(ctx, sqlc.DeleteSessionParams{ID: sessionUUID, UserID: userUUID}); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *AuthRepository) ListWorkspaces(ctx context.Context, userID string) ([]auth.Workspace, error) {
	id, err := postgresUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlc.New(r.pool).ListWorkspacesByUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list user workspaces: %w", err)
	}
	workspaces := make([]auth.Workspace, 0, len(rows))
	for _, row := range rows {
		workspaceID, err := stringUUID(row.ID)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, auth.Workspace{
			ID: workspaceID, Name: row.Name, BaseCurrency: row.BaseCurrency,
			Timezone: row.Timezone, Role: row.Role,
		})
	}
	return workspaces, nil
}

func postgresUUID(value string) (pgtype.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}, nil
}

func stringUUID(value pgtype.UUID) (string, error) {
	if !value.Valid {
		return "", errors.New("database returned a null UUID")
	}
	return uuid.UUID(value.Bytes).String(), nil
}

func uniqueViolation(err error, constraint string) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == constraint
}
