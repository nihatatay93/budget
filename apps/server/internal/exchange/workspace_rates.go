package exchange

import (
	"context"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

// WorkspaceRepository resolves the reporting currency a workspace converts from.
type WorkspaceRepository interface {
	BaseCurrency(ctx context.Context, workspaceID string) (money.Currency, error)
}

// WorkspaceService applies workspace membership rules to rate lookups. Rates themselves are
// public reference data, but the workspace's base currency is not, so the endpoint stays
// workspace-scoped and membership is verified like any other workspace read.
type WorkspaceService struct {
	rates      *Service
	access     *workspace.Authorizer
	workspaces WorkspaceRepository
}

func NewWorkspaceService(
	rates *Service,
	access *workspace.Authorizer,
	workspaces WorkspaceRepository,
) *WorkspaceService {
	return &WorkspaceService{rates: rates, access: access, workspaces: workspaces}
}

// Rates returns display conversions from the workspace base currency into the other
// supported currencies.
func (s *WorkspaceService) Rates(
	ctx context.Context,
	workspaceID, userID string,
) ([]Rate, error) {
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	base, err := s.workspaces.BaseCurrency(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.rates.Rates(ctx, base)
}
