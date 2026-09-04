package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/analysis"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

type fakeAnalysisService struct {
	query  analysis.Query
	result analysis.Analysis
	err    error
}

func (s *fakeAnalysisService) Analyze(
	_ context.Context,
	_, _ string,
	query analysis.Query,
) (analysis.Analysis, error) {
	s.query = query
	return s.result, s.err
}

func analysisTestRouter(t *testing.T, service analysisService) http.Handler {
	t.Helper()
	services := testServices()
	services.Analysis = service
	return testRouter(t, services)
}

func analysisFixture() analysis.Analysis {
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	archived := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	parentID := testCategoryID
	systemKey := "uncategorized_expense"
	return analysis.Analysis{
		Period: analysis.Period{
			FromDate: from, ToDate: to,
			ComparisonFromDate: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
			ComparisonToDate:   time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC),
			Granularity:        analysis.GranularityWeek,
			Timezone:           "Europe/Istanbul", BaseCurrency: money.TRY,
		},
		Totals: analysis.Totals{
			IncomeBaseMinor: 500000, SpendingBaseMinor: 320000, NetBaseMinor: 180000,
			ComparisonIncomeBaseMinor: 480000, ComparisonSpendingBaseMinor: 350000,
			ComparisonNetBaseMinor: 130000, TransactionCount: 42,
			SpendingTransactionCount: 33, LargestSpendingBaseMinor: 90000,
			SpendingDayCount: 18, DayCount: 31,
		},
		Series: []analysis.Bucket{{
			StartDate: from, EndDate: time.Date(2026, time.August, 7, 0, 0, 0, 0, time.UTC),
			IncomeBaseMinor: 500000, SpendingBaseMinor: 120000, NetBaseMinor: 380000,
			TransactionCount: 12,
		}},
		Categories: []analysis.Category{
			{
				ID: testCategoryID, Name: "Food", Kind: analysis.CategoryExpense,
				IconType: "system", IconValue: "utensils", ColorKey: "orange",
				DirectBaseMinor: 200000, RolledUpBaseMinor: 260000,
				ComparisonDirectBaseMinor: 150000, ComparisonRolledUpBaseMinor: 190000,
				TransactionCount: 20, RolledUpTransactionCount: 26, LargestBaseMinor: 45000,
				FirstDate: &from, LastDate: &to,
			},
			{
				ID: testAccountID, ParentID: &parentID, Name: "Restaurants",
				Kind: analysis.CategoryExpense, IconType: "emoji", IconValue: "🍔",
				ColorKey: "amber", DirectBaseMinor: 60000, RolledUpBaseMinor: 60000,
			},
			{
				ID: testTransactionID, Name: "Uncategorized Expense",
				Kind: analysis.CategoryExpense, SystemKey: &systemKey, IconType: "system",
				IconValue: "ellipsis", ColorKey: "slate", ArchivedAt: &archived,
			},
		},
		CategorySeries: []analysis.CategoryPoint{
			{CategoryID: testCategoryID, StartDate: from, BaseMinor: 200000},
		},
		Weekdays: []analysis.Weekday{
			{Weekday: 6, IncomeBaseMinor: 0, SpendingBaseMinor: 140000, TransactionCount: 9},
		},
		Days: []analysis.Day{
			{Date: from, IncomeBaseMinor: 500000, SpendingBaseMinor: 4000, TransactionCount: 2},
		},
		Payees: []analysis.Payee{{
			Payee: "Migros", SpendingBaseMinor: 88000, TransactionCount: 7,
			FirstDate: from, LastDate: to,
		}},
		Accounts: []analysis.Account{{
			ID: testAccountID, Name: "Checking", Type: "bank", Currency: money.TRY,
			OutflowBaseMinor: 320000, InflowBaseMinor: 500000, TransactionCount: 42,
		}},
	}
}

func TestGetSpendingAnalysisPassesQueryAndReturnsEveryBreakdown(t *testing.T) {
	service := &fakeAnalysisService{result: analysisFixture()}

	response := performJSON(
		t, analysisTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+
			"/spending-analysis?from_date=2026-08-01&to_date=2026-08-31&granularity=week", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if service.query.FromDate == nil || service.query.ToDate == nil {
		t.Fatal("query dates = nil, want the supplied inclusive window")
	}
	if got := service.query.FromDate.Format(time.DateOnly); got != "2026-08-01" {
		t.Fatalf("query from date = %s, want 2026-08-01", got)
	}
	if service.query.Granularity != analysis.GranularityWeek {
		t.Fatalf("query granularity = %q, want week", service.query.Granularity)
	}

	var body struct {
		Period struct {
			FromDate           string `json:"from_date"`
			ComparisonFromDate string `json:"comparison_from_date"`
			ComparisonToDate   string `json:"comparison_to_date"`
			Granularity        string `json:"granularity"`
			BaseCurrency       string `json:"base_currency"`
		} `json:"period"`
		Totals struct {
			SpendingBaseMinor           int64 `json:"spending_base_minor"`
			ComparisonSpendingBaseMinor int64 `json:"comparison_spending_base_minor"`
			DayCount                    int64 `json:"day_count"`
		} `json:"totals"`
		Series []struct {
			StartDate         string `json:"start_date"`
			EndDate           string `json:"end_date"`
			SpendingBaseMinor int64  `json:"spending_base_minor"`
		} `json:"series"`
		Categories []struct {
			ID                        string  `json:"id"`
			ParentID                  *string `json:"parent_id"`
			SystemKey                 *string `json:"system_key"`
			IconValue                 string  `json:"icon_value"`
			ColorKey                  string  `json:"color_key"`
			ArchivedAt                *string `json:"archived_at"`
			RolledUpBaseMinor         int64   `json:"rolled_up_base_minor"`
			ComparisonDirectBaseMinor int64   `json:"comparison_direct_base_minor"`
			FirstDate                 *string `json:"first_date"`
		} `json:"categories"`
		CategorySeries []struct {
			CategoryID string `json:"category_id"`
			StartDate  string `json:"start_date"`
			BaseMinor  int64  `json:"base_minor"`
		} `json:"category_series"`
		Weekdays []struct {
			Weekday           int   `json:"weekday"`
			SpendingBaseMinor int64 `json:"spending_base_minor"`
		} `json:"weekdays"`
		Days []struct {
			Date              string `json:"date"`
			SpendingBaseMinor int64  `json:"spending_base_minor"`
		} `json:"days"`
		Payees []struct {
			Payee             string `json:"payee"`
			SpendingBaseMinor int64  `json:"spending_base_minor"`
			LastDate          string `json:"last_date"`
		} `json:"payees"`
		Accounts []struct {
			Name             string `json:"name"`
			OutflowBaseMinor int64  `json:"outflow_base_minor"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Period.FromDate != "2026-08-01" || body.Period.ComparisonFromDate != "2026-07-01" ||
		body.Period.ComparisonToDate != "2026-07-31" {
		t.Fatalf("period = %+v, want the window and its equal-length predecessor", body.Period)
	}
	if body.Period.Granularity != "week" || body.Period.BaseCurrency != "TRY" {
		t.Fatalf("period = %+v, want week buckets in TRY", body.Period)
	}
	if body.Totals.SpendingBaseMinor != 320000 ||
		body.Totals.ComparisonSpendingBaseMinor != 350000 || body.Totals.DayCount != 31 {
		t.Fatalf("totals = %+v, want the window and comparison figures", body.Totals)
	}
	if len(body.Series) != 1 || body.Series[0].StartDate != "2026-08-01" ||
		body.Series[0].EndDate != "2026-08-07" || body.Series[0].SpendingBaseMinor != 120000 {
		t.Fatalf("series = %+v, want one bounded bucket", body.Series)
	}
	if len(body.Categories) != 3 {
		t.Fatalf("categories = %d, want 3", len(body.Categories))
	}
	if body.Categories[0].RolledUpBaseMinor != 260000 ||
		body.Categories[0].ComparisonDirectBaseMinor != 150000 ||
		body.Categories[0].FirstDate == nil || *body.Categories[0].FirstDate != "2026-08-01" {
		t.Fatalf("first category = %+v, want rolled-up, comparison, and first-date figures",
			body.Categories[0])
	}
	if body.Categories[1].ParentID == nil || *body.Categories[1].ParentID != testCategoryID {
		t.Fatalf("child category parent = %v, want the hierarchy preserved", body.Categories[1].ParentID)
	}
	if body.Categories[1].IconValue != "🍔" || body.Categories[1].ColorKey != "amber" {
		t.Fatalf("child category appearance = %+v, want the stored icon and color", body.Categories[1])
	}
	if body.Categories[2].SystemKey == nil || *body.Categories[2].SystemKey != "uncategorized_expense" ||
		body.Categories[2].ArchivedAt == nil {
		t.Fatalf("system category = %+v, want its key and archival timestamp", body.Categories[2])
	}
	if len(body.CategorySeries) != 1 || body.CategorySeries[0].CategoryID != testCategoryID ||
		body.CategorySeries[0].StartDate != "2026-08-01" {
		t.Fatalf("category series = %+v, want the sparse per-category points", body.CategorySeries)
	}
	if len(body.Weekdays) != 1 || body.Weekdays[0].Weekday != 6 ||
		body.Weekdays[0].SpendingBaseMinor != 140000 {
		t.Fatalf("weekdays = %+v, want ISO weekday spending", body.Weekdays)
	}
	if len(body.Days) != 1 || body.Days[0].Date != "2026-08-01" {
		t.Fatalf("days = %+v, want daily activity", body.Days)
	}
	if len(body.Payees) != 1 || body.Payees[0].Payee != "Migros" ||
		body.Payees[0].LastDate != "2026-08-31" {
		t.Fatalf("payees = %+v, want the ranked payee", body.Payees)
	}
	if len(body.Accounts) != 1 || body.Accounts[0].OutflowBaseMinor != 320000 {
		t.Fatalf("accounts = %+v, want account outflow", body.Accounts)
	}
}

// Omitting the dates leaves the window to the server, which is how the default trailing-year
// view is requested.
func TestGetSpendingAnalysisWithoutDatesLeavesTheWindowUnset(t *testing.T) {
	service := &fakeAnalysisService{result: analysisFixture()}

	response := performJSON(
		t, analysisTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/spending-analysis", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if service.query.FromDate != nil || service.query.ToDate != nil ||
		service.query.Granularity != "" {
		t.Fatalf("query = %+v, want an unset window and granularity", service.query)
	}
}

func TestGetSpendingAnalysisMapsDomainFailures(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid range", err: analysis.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{name: "not a member", err: workspace.ErrForbidden, wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeAnalysisService{err: test.err}
			response := performJSON(
				t, analysisTestRouter(t, service), http.MethodGet,
				"/v1/workspaces/"+testWorkspaceID+"/spending-analysis?from_date=2026-08-01", "",
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s",
					response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

// A granularity outside the contract must be refused at the binding layer rather than
// reaching the domain, where it would become part of a SQL bucket width.
func TestGetSpendingAnalysisRejectsUnknownGranularity(t *testing.T) {
	service := &fakeAnalysisService{result: analysisFixture()}

	response := performJSON(
		t, analysisTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/spending-analysis?granularity=century", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
	}
}
