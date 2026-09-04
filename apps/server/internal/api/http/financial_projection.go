package httpapi

import (
	"context"
	"errors"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/reporting"
)

func (s *server) GetFinancialProjection(
	ctx context.Context,
	request openapi.GetFinancialProjectionRequestObject,
) (openapi.GetFinancialProjectionResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	query := reporting.Query{}
	if request.Params.FromDate != nil {
		query.FromDate = &request.Params.FromDate.Time
	}
	if request.Params.ToDate != nil {
		query.ToDate = &request.Params.ToDate.Time
	}
	projection, err := s.Reporting.Project(
		ctx, request.WorkspaceId.String(), principal.User.ID, query,
	)
	switch {
	case errors.Is(err, reporting.ErrInvalidInput):
		return openapi.GetFinancialProjection400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	case err != nil:
		return nil, err
	}
	body, err := financialProjectionResponse(projection)
	if err != nil {
		return nil, err
	}
	return openapi.GetFinancialProjection200JSONResponse{
		Body:    body,
		Headers: openapi.GetFinancialProjection200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func financialProjectionResponse(value reporting.Projection) (openapi.FinancialProjection, error) {
	accounts := make([]openapi.FinancialProjectionAccount, 0, len(value.Accounts))
	for _, account := range value.Accounts {
		id, err := uuid.Parse(account.ID)
		if err != nil {
			return openapi.FinancialProjection{}, err
		}
		accounts = append(accounts, openapi.FinancialProjectionAccount{
			Id: id, Name: account.Name, Type: openapi.AccountType(account.Type),
			Currency: openapi.Currency(account.Currency), ArchivedAt: account.ArchivedAt,
			NativeBalanceMinor: projectionAmounts(account.Native),
			BaseBalanceMinor:   projectionAmounts(account.Base),
		})
	}
	categories := make([]openapi.FinancialProjectionCategory, 0, len(value.Categories))
	for _, category := range value.Categories {
		id, err := uuid.Parse(category.ID)
		if err != nil {
			return openapi.FinancialProjection{}, err
		}
		var parentID *uuid.UUID
		if category.ParentID != nil {
			parsed, err := uuid.Parse(*category.ParentID)
			if err != nil {
				return openapi.FinancialProjection{}, err
			}
			parentID = &parsed
		}
		var systemKey *openapi.SystemCategoryKey
		if category.SystemKey != nil {
			converted := openapi.SystemCategoryKey(*category.SystemKey)
			systemKey = &converted
		}
		categories = append(categories, openapi.FinancialProjectionCategory{
			Id: id, ParentId: parentID, Name: category.Name,
			Kind: openapi.CategoryKind(category.Kind), SystemKey: systemKey,
			PredefinedKey: predefinedCategoryKey(category.PredefinedKey),
			Icon:          category.Icon, IconType: openapi.CategoryIconType(category.IconType),
			IconValue: category.IconValue, ColorKey: openapi.CategoryColorKey(category.ColorKey),
			ArchivedAt:        category.ArchivedAt,
			DirectBaseMinor:   projectionAmounts(category.Direct),
			RolledUpBaseMinor: projectionAmounts(category.RolledUp),
		})
	}
	return openapi.FinancialProjection{
		Period: openapi.FinancialProjectionPeriod{
			FromDate:     openapi_types.Date{Time: value.Period.FromDate},
			ToDate:       openapi_types.Date{Time: value.Period.ToDate},
			Timezone:     value.Period.Timezone,
			BaseCurrency: openapi.Currency(value.Period.BaseCurrency),
		},
		Summary: openapi.FinancialProjectionSummary{
			BalanceBaseMinor:  projectionAmounts(value.Summary.BalanceBaseMinor),
			IncomeBaseMinor:   projectionAmounts(value.Summary.IncomeBaseMinor),
			SpendingBaseMinor: projectionAmounts(value.Summary.SpendingBaseMinor),
		},
		Accounts: accounts, Categories: categories,
	}, nil
}

func projectionAmounts(value reporting.Amounts) openapi.ProjectionAmounts {
	return openapi.ProjectionAmounts{
		Posted: value.Posted, Pending: value.Pending, Projected: value.Projected,
	}
}
