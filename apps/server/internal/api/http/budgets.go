package httpapi

import (
	"context"
	"errors"

	"github.com/google/uuid"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/budget"
)

func (s *server) GetMonthlyBudget(
	ctx context.Context,
	request openapi.GetMonthlyBudgetRequestObject,
) (openapi.GetMonthlyBudgetResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var requestedMonth *string
	if request.Params.Month != nil {
		value := string(*request.Params.Month)
		requestedMonth = &value
	}
	value, err := s.Budgets.Get(
		ctx, request.WorkspaceId.String(), principal.User.ID, requestedMonth,
	)
	switch {
	case errors.Is(err, budget.ErrInvalidInput):
		return openapi.GetMonthlyBudget400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	case err != nil:
		return nil, err
	}
	body, err := monthlyBudgetResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.GetMonthlyBudget200JSONResponse{
		Body: body, Headers: openapi.GetMonthlyBudget200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) ReplaceMonthlyBudget(
	ctx context.Context,
	request openapi.ReplaceMonthlyBudgetRequestObject,
) (openapi.ReplaceMonthlyBudgetResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.ReplaceMonthlyBudget400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	}
	items := make([]budget.ItemInput, 0, len(request.Body.Items))
	for _, item := range request.Body.Items {
		items = append(items, budget.ItemInput{
			CategoryID: item.CategoryId.String(), AmountBaseMinor: item.AmountBaseMinor,
		})
	}
	value, err := s.Budgets.Replace(
		ctx, request.WorkspaceId.String(), principal.User.ID, string(request.Month),
		budget.WriteInput{Name: request.Body.Name, Items: items},
	)
	switch {
	case errors.Is(err, budget.ErrInvalidInput), errors.Is(err, budget.ErrAmountOverflow):
		return openapi.ReplaceMonthlyBudget400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	case errors.Is(err, budget.ErrCategoryNotFound):
		return budgetConflict(requestID, "budget_category_not_found", "A selected category no longer exists."), nil
	case errors.Is(err, budget.ErrCategoryKind):
		return budgetConflict(requestID, "budget_category_kind", "Budgets can only target expense categories."), nil
	case errors.Is(err, budget.ErrCategoryArchived):
		return budgetConflict(requestID, "budget_category_archived", "An archived category cannot be newly budgeted."), nil
	case errors.Is(err, budget.ErrCategoryDuplicate):
		return budgetConflict(requestID, "budget_category_duplicate", "Each category can appear only once."), nil
	case errors.Is(err, budget.ErrCategoryOverlap):
		return budgetConflict(requestID, "budget_category_overlap", "A budget cannot include overlapping category branches."), nil
	case err != nil:
		return nil, err
	}
	body, err := monthlyBudgetResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.ReplaceMonthlyBudget200JSONResponse{
		Body: body, Headers: openapi.ReplaceMonthlyBudget200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func monthlyBudgetResponse(value budget.Budget) (openapi.MonthlyBudget, error) {
	id, err := uuid.Parse(value.ID)
	if err != nil {
		return openapi.MonthlyBudget{}, err
	}
	workspaceID, err := uuid.Parse(value.WorkspaceID)
	if err != nil {
		return openapi.MonthlyBudget{}, err
	}
	items := make([]openapi.MonthlyBudgetItem, 0, len(value.Items))
	for _, item := range value.Items {
		itemID, err := uuid.Parse(item.ID)
		if err != nil {
			return openapi.MonthlyBudget{}, err
		}
		categoryID, err := uuid.Parse(item.CategoryID)
		if err != nil {
			return openapi.MonthlyBudget{}, err
		}
		items = append(items, openapi.MonthlyBudgetItem{
			Id: itemID, CategoryId: categoryID, CategoryName: item.CategoryName,
			CategoryPredefinedKey: predefinedCategoryKey(item.CategoryPredefinedKey),
			CategoryIcon:          item.CategoryIcon,
			CategoryIconType:      openapi.CategoryIconType(item.CategoryIconType),
			CategoryIconValue:     item.CategoryIconValue,
			CategoryColorKey:      openapi.CategoryColorKey(item.CategoryColorKey),
			CategoryArchivedAt:    item.CategoryArchivedAt,
			PlannedBaseMinor:      item.PlannedBaseMinor, UsedBaseMinor: item.UsedBaseMinor,
			RemainingBaseMinor: item.RemainingBaseMinor,
		})
	}
	return openapi.MonthlyBudget{
		Id: id, WorkspaceId: workspaceID, Name: value.Name, Month: value.Month,
		Timezone: value.Timezone, BaseCurrency: openapi.Currency(value.BaseCurrency),
		PlannedBaseMinor: value.PlannedBaseMinor, UsedBaseMinor: value.UsedBaseMinor,
		RemainingBaseMinor: value.RemainingBaseMinor, Items: items,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}

func predefinedCategoryKey(value *string) *openapi.PredefinedCategoryKey {
	if value == nil {
		return nil
	}
	key := openapi.PredefinedCategoryKey(*value)
	return &key
}

func budgetConflict(requestID, code, message string) openapi.ReplaceMonthlyBudget409JSONResponse {
	return openapi.ReplaceMonthlyBudget409JSONResponse{
		ConflictJSONResponse: conflict(requestID, code, message),
	}
}
