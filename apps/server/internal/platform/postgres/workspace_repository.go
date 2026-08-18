package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
	"github.com/nihatatay93/budget/internal/workspace"
)

type WorkspaceRepository struct {
	pool *pgxpool.Pool
}

func NewWorkspaceRepository(pool *pgxpool.Pool) *WorkspaceRepository {
	return &WorkspaceRepository{pool: pool}
}

// BaseCurrency returns the workspace reporting currency. Callers must verify membership
// before calling; this method does not authorize.
func (r *WorkspaceRepository) BaseCurrency(
	ctx context.Context,
	workspaceID string,
) (money.Currency, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return "", err
	}
	value, err := sqlc.New(r.pool).GetWorkspaceBaseCurrency(ctx, workspaceUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", workspace.ErrForbidden
	}
	if err != nil {
		return "", fmt.Errorf("get workspace base currency: %w", err)
	}
	currency, ok := money.Parse(value)
	if !ok {
		return "", fmt.Errorf("workspace %s has unsupported base currency %q", workspaceID, value)
	}
	return currency, nil
}

func (r *WorkspaceRepository) MemberRole(
	ctx context.Context,
	workspaceID, userID string,
) (workspace.Role, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return "", workspace.ErrForbidden
	}
	userUUID, err := postgresUUID(userID)
	if err != nil {
		return "", workspace.ErrForbidden
	}
	role, err := sqlc.New(r.pool).GetWorkspaceMemberRole(ctx, sqlc.GetWorkspaceMemberRoleParams{
		WorkspaceID: workspaceUUID,
		UserID:      userUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", workspace.ErrForbidden
	}
	if err != nil {
		return "", fmt.Errorf("get workspace member role: %w", err)
	}
	return workspace.Role(role), nil
}

// WorkspaceName returns the workspace's display name, used in invitation email. Callers must
// verify membership before exposing it; invitation delivery is already authorized by the act
// of creating the invitation.
func (r *WorkspaceRepository) WorkspaceName(
	ctx context.Context,
	workspaceID string,
) (string, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return "", err
	}
	name, err := sqlc.New(r.pool).GetWorkspaceName(ctx, workspaceUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", workspace.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get workspace name: %w", err)
	}
	return name, nil
}
