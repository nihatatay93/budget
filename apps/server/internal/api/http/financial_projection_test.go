package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/reporting"
	"github.com/nihatatay93/budget/internal/workspace"
)

type fakeReportingService struct {
	query      reporting.Query
	projection reporting.Projection
	err        error
}

func (s *fakeReportingService) Project(
	_ context.Context,
	_, _ string,
	query reporting.Query,
) (reporting.Projection, error) {
	s.query = query
	return s.projection, s.err
}

func reportingTestRouter(t *testing.T, reports reportingService) http.Handler {
	t.Helper()
	services := testServices()
	services.Reporting = reports
	return testRouter(t, services)
}

func TestGetFinancialProjectionPassesRangeAndReturnsProjection(t *testing.T) {
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)
	service := &fakeReportingService{projection: reporting.Projection{
		Period: reporting.Period{
			FromDate: from, ToDate: to, Timezone: "Europe/Istanbul", BaseCurrency: money.TRY,
		},
		Summary: reporting.Summary{
			BalanceBaseMinor:  reporting.Amounts{Posted: 10000, Pending: -500, Projected: 9500},
			IncomeBaseMinor:   reporting.Amounts{Posted: 5000, Pending: 0, Projected: 5000},
			SpendingBaseMinor: reporting.Amounts{Posted: 1500, Pending: 500, Projected: 2000},
		},
		Accounts: []reporting.Account{{
			ID: testAccountID, Name: "Checking", Type: "bank", Currency: money.TRY,
			Native: reporting.Amounts{Posted: 10000, Pending: -500, Projected: 9500},
			Base:   reporting.Amounts{Posted: 10000, Pending: -500, Projected: 9500},
		}},
		Categories: []reporting.Category{{
			ID: testCategoryID, Name: "Food", Kind: reporting.CategoryExpense,
			Direct:   reporting.Amounts{Posted: 1500, Pending: 500, Projected: 2000},
			RolledUp: reporting.Amounts{Posted: 1500, Pending: 500, Projected: 2000},
		}},
	}}
	response := performJSON(
		t, reportingTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+
			"/financial-projection?from_date=2026-08-01&to_date=2026-08-18", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.query.FromDate == nil || service.query.ToDate == nil ||
		service.query.FromDate.Format(time.DateOnly) != "2026-08-01" ||
		service.query.ToDate.Format(time.DateOnly) != "2026-08-18" {
		t.Fatalf("Project() query = %#v", service.query)
	}
	var body struct {
		Period struct {
			Timezone string `json:"timezone"`
		} `json:"period"`
		Summary struct {
			Spending struct {
				Projected int64 `json:"projected"`
			} `json:"spending_base_minor"`
		} `json:"summary"`
		Accounts   []json.RawMessage `json:"accounts"`
		Categories []json.RawMessage `json:"categories"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Period.Timezone != "Europe/Istanbul" || body.Summary.Spending.Projected != 2000 ||
		len(body.Accounts) != 1 || len(body.Categories) != 1 {
		t.Fatalf("projection response = %#v", body)
	}
}

func TestGetFinancialProjectionMapsDomainErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid range", err: reporting.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{name: "not a member", err: workspace.ErrForbidden, wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeReportingService{err: test.err}
			response := performJSON(
				t, reportingTestRouter(t, service), http.MethodGet,
				"/v1/workspaces/"+testWorkspaceID+"/financial-projection?from_date=2026-08-01", "",
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

// The projection carries the category tree, so parent links, protected system categories,
// icons, and archival timestamps all have to survive conversion. A dropped parent_id would
// flatten the hierarchy the dashboard groups by.
func TestFinancialProjectionResponseCarriesHierarchyAndOptionalFields(t *testing.T) {
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)
	archived := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	parentID := testCategoryID
	systemKey := "uncategorized_expense"
	icon := "🍔"
	amounts := reporting.Amounts{Posted: 1500, Pending: 500, Projected: 2000}

	service := &fakeReportingService{projection: reporting.Projection{
		Period: reporting.Period{
			FromDate: from, ToDate: to, Timezone: "Europe/Istanbul", BaseCurrency: money.TRY,
		},
		Summary: reporting.Summary{
			BalanceBaseMinor:  amounts,
			IncomeBaseMinor:   amounts,
			SpendingBaseMinor: amounts,
		},
		Accounts: []reporting.Account{
			{
				ID: testAccountID, Name: "Checking", Type: "bank", Currency: money.TRY,
				Native: amounts, Base: amounts,
			},
			{
				ID: testTransactionID, Name: "Old card", Type: "credit_card", Currency: money.USD,
				ArchivedAt: &archived, Native: amounts, Base: amounts,
			},
		},
		Categories: []reporting.Category{
			{
				ID: testCategoryID, Name: "Food", Kind: reporting.CategoryExpense,
				Icon: &icon, Direct: amounts, RolledUp: amounts,
			},
			{
				ID: testAccountID, ParentID: &parentID, Name: "Restaurants",
				Kind: reporting.CategoryExpense, Direct: amounts, RolledUp: amounts,
			},
			{
				ID: testTransactionID, Name: "Uncategorized Expense",
				Kind: reporting.CategoryExpense, SystemKey: &systemKey,
				ArchivedAt: &archived, Direct: amounts, RolledUp: amounts,
			},
		},
	}}

	response := performJSON(
		t, reportingTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+
			"/financial-projection?from_date=2026-08-01&to_date=2026-08-18", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Period struct {
			BaseCurrency string `json:"base_currency"`
			FromDate     string `json:"from_date"`
		} `json:"period"`
		Accounts []struct {
			Currency   string  `json:"currency"`
			ArchivedAt *string `json:"archived_at"`
		} `json:"accounts"`
		Categories []struct {
			Name       string  `json:"name"`
			ParentID   *string `json:"parent_id"`
			SystemKey  *string `json:"system_key"`
			Icon       *string `json:"icon"`
			ArchivedAt *string `json:"archived_at"`
		} `json:"categories"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Period.BaseCurrency != "TRY" || body.Period.FromDate != "2026-08-01" {
		t.Fatalf("period = %#v", body.Period)
	}
	if len(body.Accounts) != 2 || body.Accounts[0].ArchivedAt != nil {
		t.Fatalf("accounts = %#v", body.Accounts)
	}
	if body.Accounts[1].ArchivedAt == nil || body.Accounts[1].Currency != "USD" {
		t.Fatalf("archived account = %#v", body.Accounts[1])
	}
	if len(body.Categories) != 3 {
		t.Fatalf("categories = %#v", body.Categories)
	}
	if body.Categories[0].Icon == nil || *body.Categories[0].Icon != icon ||
		body.Categories[0].ParentID != nil {
		t.Fatalf("root category = %#v", body.Categories[0])
	}
	if body.Categories[1].ParentID == nil || *body.Categories[1].ParentID != testCategoryID {
		t.Fatalf("child category lost its parent: %#v", body.Categories[1])
	}
	if body.Categories[2].SystemKey == nil || *body.Categories[2].SystemKey != systemKey ||
		body.Categories[2].ArchivedAt == nil {
		t.Fatalf("system category = %#v", body.Categories[2])
	}
}

func TestFinancialProjectionEnforcesMembership(t *testing.T) {
	service := &fakeReportingService{err: workspace.ErrForbidden}
	response := performJSON(
		t, reportingTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+
			"/financial-projection?from_date=2026-08-01&to_date=2026-08-18", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusForbidden, "forbidden")
}
