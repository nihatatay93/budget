package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/nihatatay93/budget/internal/analysis"
	openapi "github.com/nihatatay93/budget/internal/api/openapi"
)

func (s *server) GetSpendingAnalysis(
	ctx context.Context,
	request openapi.GetSpendingAnalysisRequestObject,
) (openapi.GetSpendingAnalysisResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	query := analysis.Query{}
	if request.Params.FromDate != nil {
		query.FromDate = &request.Params.FromDate.Time
	}
	if request.Params.ToDate != nil {
		query.ToDate = &request.Params.ToDate.Time
	}
	if request.Params.Granularity != nil {
		// Refused here as well as in the domain: the value becomes a SQL bucket width, so an
		// unrecognized one is a client error that should never travel further inward.
		query.Granularity = analysis.Granularity(*request.Params.Granularity)
		if !query.Granularity.Valid() {
			return openapi.GetSpendingAnalysis400JSONResponse{
				BadRequestJSONResponse: badRequest(requestID),
			}, nil
		}
	}
	result, err := s.Analysis.Analyze(
		ctx, request.WorkspaceId.String(), principal.User.ID, query,
	)
	switch {
	case errors.Is(err, analysis.ErrInvalidInput):
		return openapi.GetSpendingAnalysis400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	case err != nil:
		return nil, err
	}
	body, err := spendingAnalysisResponse(result)
	if err != nil {
		return nil, err
	}
	return openapi.GetSpendingAnalysis200JSONResponse{
		Body:    body,
		Headers: openapi.GetSpendingAnalysis200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func spendingAnalysisResponse(value analysis.Analysis) (openapi.SpendingAnalysis, error) {
	series := make([]openapi.SpendingAnalysisBucket, 0, len(value.Series))
	for _, bucket := range value.Series {
		series = append(series, openapi.SpendingAnalysisBucket{
			StartDate:         openapi_types.Date{Time: bucket.StartDate},
			EndDate:           openapi_types.Date{Time: bucket.EndDate},
			IncomeBaseMinor:   bucket.IncomeBaseMinor,
			SpendingBaseMinor: bucket.SpendingBaseMinor,
			NetBaseMinor:      bucket.NetBaseMinor,
			TransactionCount:  bucket.TransactionCount,
		})
	}
	categories := make([]openapi.SpendingAnalysisCategory, 0, len(value.Categories))
	for _, category := range value.Categories {
		id, err := uuid.Parse(category.ID)
		if err != nil {
			return openapi.SpendingAnalysis{}, err
		}
		var parentID *uuid.UUID
		if category.ParentID != nil {
			parsed, err := uuid.Parse(*category.ParentID)
			if err != nil {
				return openapi.SpendingAnalysis{}, err
			}
			parentID = &parsed
		}
		var systemKey *openapi.SystemCategoryKey
		if category.SystemKey != nil {
			converted := openapi.SystemCategoryKey(*category.SystemKey)
			systemKey = &converted
		}
		categories = append(categories, openapi.SpendingAnalysisCategory{
			Id: id, ParentId: parentID, Name: category.Name,
			Kind:      openapi.CategoryKind(category.Kind),
			SystemKey: systemKey, PredefinedKey: predefinedCategoryKey(category.PredefinedKey),
			IconType:  openapi.CategoryIconType(category.IconType),
			IconValue: category.IconValue,
			ColorKey:  openapi.CategoryColorKey(category.ColorKey),

			ArchivedAt:                  category.ArchivedAt,
			DirectBaseMinor:             category.DirectBaseMinor,
			RolledUpBaseMinor:           category.RolledUpBaseMinor,
			ComparisonDirectBaseMinor:   category.ComparisonDirectBaseMinor,
			ComparisonRolledUpBaseMinor: category.ComparisonRolledUpBaseMinor,
			TransactionCount:            category.TransactionCount,
			RolledUpTransactionCount:    category.RolledUpTransactionCount,
			LargestBaseMinor:            category.LargestBaseMinor,
			FirstDate:                   optionalDate(category.FirstDate),
			LastDate:                    optionalDate(category.LastDate),
		})
	}
	categorySeries := make([]openapi.SpendingAnalysisCategoryPoint, 0, len(value.CategorySeries))
	for _, point := range value.CategorySeries {
		id, err := uuid.Parse(point.CategoryID)
		if err != nil {
			return openapi.SpendingAnalysis{}, err
		}
		categorySeries = append(categorySeries, openapi.SpendingAnalysisCategoryPoint{
			CategoryId: id,
			StartDate:  openapi_types.Date{Time: point.StartDate},
			BaseMinor:  point.BaseMinor,
		})
	}
	weekdays := make([]openapi.SpendingAnalysisWeekday, 0, len(value.Weekdays))
	for _, weekday := range value.Weekdays {
		weekdays = append(weekdays, openapi.SpendingAnalysisWeekday{
			Weekday:           weekday.Weekday,
			IncomeBaseMinor:   weekday.IncomeBaseMinor,
			SpendingBaseMinor: weekday.SpendingBaseMinor,
			TransactionCount:  weekday.TransactionCount,
		})
	}
	days := make([]openapi.SpendingAnalysisDay, 0, len(value.Days))
	for _, day := range value.Days {
		days = append(days, openapi.SpendingAnalysisDay{
			Date:              openapi_types.Date{Time: day.Date},
			IncomeBaseMinor:   day.IncomeBaseMinor,
			SpendingBaseMinor: day.SpendingBaseMinor,
			TransactionCount:  day.TransactionCount,
		})
	}
	payees := make([]openapi.SpendingAnalysisPayee, 0, len(value.Payees))
	for _, payee := range value.Payees {
		payees = append(payees, openapi.SpendingAnalysisPayee{
			Payee:             payee.Payee,
			SpendingBaseMinor: payee.SpendingBaseMinor,
			IncomeBaseMinor:   payee.IncomeBaseMinor,
			TransactionCount:  payee.TransactionCount,
			FirstDate:         openapi_types.Date{Time: payee.FirstDate},
			LastDate:          openapi_types.Date{Time: payee.LastDate},
		})
	}
	accounts := make([]openapi.SpendingAnalysisAccount, 0, len(value.Accounts))
	for _, account := range value.Accounts {
		id, err := uuid.Parse(account.ID)
		if err != nil {
			return openapi.SpendingAnalysis{}, err
		}
		accounts = append(accounts, openapi.SpendingAnalysisAccount{
			Id: id, Name: account.Name, Type: openapi.AccountType(account.Type),
			Currency: openapi.Currency(account.Currency), ArchivedAt: account.ArchivedAt,
			OutflowBaseMinor: account.OutflowBaseMinor,
			InflowBaseMinor:  account.InflowBaseMinor,
			TransactionCount: account.TransactionCount,
		})
	}
	return openapi.SpendingAnalysis{
		Period: openapi.SpendingAnalysisPeriod{
			FromDate:           openapi_types.Date{Time: value.Period.FromDate},
			ToDate:             openapi_types.Date{Time: value.Period.ToDate},
			ComparisonFromDate: openapi_types.Date{Time: value.Period.ComparisonFromDate},
			ComparisonToDate:   openapi_types.Date{Time: value.Period.ComparisonToDate},
			Granularity:        openapi.AnalysisGranularity(value.Period.Granularity),
			Timezone:           value.Period.Timezone,
			BaseCurrency:       openapi.Currency(value.Period.BaseCurrency),
		},
		Totals: openapi.SpendingAnalysisTotals{
			IncomeBaseMinor:             value.Totals.IncomeBaseMinor,
			SpendingBaseMinor:           value.Totals.SpendingBaseMinor,
			NetBaseMinor:                value.Totals.NetBaseMinor,
			ComparisonIncomeBaseMinor:   value.Totals.ComparisonIncomeBaseMinor,
			ComparisonSpendingBaseMinor: value.Totals.ComparisonSpendingBaseMinor,
			ComparisonNetBaseMinor:      value.Totals.ComparisonNetBaseMinor,
			TransactionCount:            value.Totals.TransactionCount,
			SpendingTransactionCount:    value.Totals.SpendingTransactionCount,
			LargestSpendingBaseMinor:    value.Totals.LargestSpendingBaseMinor,
			SpendingDayCount:            value.Totals.SpendingDayCount,
			DayCount:                    value.Totals.DayCount,
		},
		Series: series, Categories: categories, CategorySeries: categorySeries,
		Weekdays: weekdays, Days: days, Payees: payees, Accounts: accounts,
	}, nil
}

func optionalDate(value *time.Time) *openapi_types.Date {
	if value == nil {
		return nil
	}
	return &openapi_types.Date{Time: *value}
}
