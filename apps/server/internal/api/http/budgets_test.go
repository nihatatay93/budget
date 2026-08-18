package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/budget"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

const testBudgetID = "0198c307-75e7-7899-a5c0-4839cc802be1"

type fakeBudgetService struct {
	value          budget.Budget
	err            error
	workspaceID    string
	userID         string
	requestedMonth *string
	month          string
	input          budget.WriteInput
	replaceCalls   int
}

func (s *fakeBudgetService) Get(
	_ context.Context,
	workspaceID, userID string,
	month *string,
) (budget.Budget, error) {
	s.workspaceID = workspaceID
	s.userID = userID
	s.requestedMonth = month
	return s.value, s.err
}

func (s *fakeBudgetService) Replace(
	_ context.Context,
	workspaceID, userID, month string,
	input budget.WriteInput,
) (budget.Budget, error) {
	s.workspaceID = workspaceID
	s.userID = userID
	s.month = month
	s.input = input
	s.replaceCalls++
	return s.value, s.err
}

func budgetTestRouter(t *testing.T, service budgetService) http.Handler {
	t.Helper()
	services := testServices()
	services.Budgets = service
	return testRouter(t, services)
}

func TestGetMonthlyBudgetPassesMonthAndReturnsUsage(t *testing.T) {
	service := &fakeBudgetService{value: validHTTPBudget()}
	response := performJSON(
		t, budgetTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/budgets?month=2026-08", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.workspaceID != testWorkspaceID || service.userID != testUserID ||
		service.requestedMonth == nil || *service.requestedMonth != "2026-08" {
		t.Fatalf("Get() arguments = workspace %q user %q month %#v", service.workspaceID, service.userID, service.requestedMonth)
	}
	var body struct {
		Month              string `json:"month"`
		PlannedBaseMinor   int64  `json:"planned_base_minor"`
		UsedBaseMinor      int64  `json:"used_base_minor"`
		RemainingBaseMinor int64  `json:"remaining_base_minor"`
		Items              []struct {
			CategoryName string `json:"category_name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Month != "2026-08" || body.PlannedBaseMinor != 5000 ||
		body.UsedBaseMinor != 1300 || body.RemainingBaseMinor != 3700 ||
		len(body.Items) != 1 || body.Items[0].CategoryName != "Food" {
		t.Fatalf("budget response = %#v", body)
	}
}

func TestGetMonthlyBudgetMapsDomainErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid month", err: budget.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{name: "not a member", err: workspace.ErrForbidden, wantStatus: http.StatusForbidden},
		{name: "not found", err: budget.ErrNotFound, wantStatus: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performJSON(
				t, budgetTestRouter(t, &fakeBudgetService{err: test.err}), http.MethodGet,
				"/v1/workspaces/"+testWorkspaceID+"/budgets?month=2026-08", "",
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestReplaceMonthlyBudgetPassesCompletePlan(t *testing.T) {
	service := &fakeBudgetService{value: validHTTPBudget()}
	response := performJSON(
		t, budgetTestRouter(t, service), http.MethodPut,
		"/v1/workspaces/"+testWorkspaceID+"/budgets/2026-08",
		`{"name":"August plan","items":[{"category_id":"`+testCategoryID+`","amount_base_minor":5000}]}`,
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.replaceCalls != 1 || service.workspaceID != testWorkspaceID ||
		service.userID != testUserID || service.month != "2026-08" ||
		service.input.Name != "August plan" || len(service.input.Items) != 1 ||
		service.input.Items[0].CategoryID != testCategoryID ||
		service.input.Items[0].AmountBaseMinor != 5000 {
		t.Fatalf("Replace() arguments = %#v", service)
	}
}

func TestReplaceMonthlyBudgetMapsDomainErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid", err: budget.ErrInvalidInput, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "overflow", err: budget.ErrAmountOverflow, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "viewer", err: workspace.ErrForbidden, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "missing category", err: budget.ErrCategoryNotFound, wantStatus: http.StatusConflict, wantCode: "budget_category_not_found"},
		{name: "income category", err: budget.ErrCategoryKind, wantStatus: http.StatusConflict, wantCode: "budget_category_kind"},
		{name: "archived category", err: budget.ErrCategoryArchived, wantStatus: http.StatusConflict, wantCode: "budget_category_archived"},
		{name: "duplicate category", err: budget.ErrCategoryDuplicate, wantStatus: http.StatusConflict, wantCode: "budget_category_duplicate"},
		{name: "overlap", err: budget.ErrCategoryOverlap, wantStatus: http.StatusConflict, wantCode: "budget_category_overlap"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performJSON(
				t, budgetTestRouter(t, &fakeBudgetService{err: test.err}), http.MethodPut,
				"/v1/workspaces/"+testWorkspaceID+"/budgets/2026-08",
				`{"name":"August","items":[{"category_id":"`+testCategoryID+`","amount_base_minor":1}]}`,
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Error.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", body.Error.Code, test.wantCode)
			}
		})
	}
}

func TestReplaceMonthlyBudgetRejectsCrossSiteCookieRequest(t *testing.T) {
	service := &fakeBudgetService{value: validHTTPBudget()}
	services := testServices()
	services.Authentication = &fakeAuthService{principal: testAuthResult("cookie").Principal}
	services.Budgets = service
	router := testRouter(t, services)
	response := performJSON(
		t, router, http.MethodPut,
		"/v1/workspaces/"+testWorkspaceID+"/budgets/2026-08",
		`{"name":"August","items":[{"category_id":"`+testCategoryID+`","amount_base_minor":1}]}`,
		map[string]string{
			"Cookie": "budget_session=raw-token", "Origin": "https://attacker.example",
			"Sec-Fetch-Site": "cross-site",
		},
	)
	if response.Code != http.StatusForbidden || service.replaceCalls != 0 {
		t.Fatalf("cross-site response = %d, calls = %d", response.Code, service.replaceCalls)
	}
}

func validHTTPBudget() budget.Budget {
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	return budget.Budget{
		ID: testBudgetID, WorkspaceID: testWorkspaceID, Name: "August plan", Month: "2026-08",
		Timezone: "Europe/Istanbul", BaseCurrency: money.TRY,
		PlannedBaseMinor: 5000, UsedBaseMinor: 1300, RemainingBaseMinor: 3700,
		Items: []budget.Item{{
			ID: testBudgetID, CategoryID: testCategoryID, CategoryName: "Food",
			PlannedBaseMinor: 5000, UsedBaseMinor: 1300, RemainingBaseMinor: 3700,
		}},
		CreatedAt: now, UpdatedAt: now,
	}
}

// Omitting the month asks the service for the workspace's current month; the parameter must
// arrive as nil rather than an empty string, which would be a different request.
func TestGetMonthlyBudgetWithoutMonthRequestsTheCurrentPeriod(t *testing.T) {
	service := &fakeBudgetService{value: validHTTPBudget()}
	response := performJSON(
		t, budgetTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/budgets", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if service.requestedMonth != nil {
		t.Fatalf("requested month = %q, want nil", *service.requestedMonth)
	}
}

// An archived category keeps appearing in the budget it was already part of, so its icon and
// archival timestamp must survive conversion for the client to mark it.
func TestMonthlyBudgetResponseCarriesOptionalCategoryFields(t *testing.T) {
	archived := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	icon := "🍔"
	value := validHTTPBudget()
	value.Items = append(value.Items, budget.Item{
		ID: testAccountID, CategoryID: testCategoryID, CategoryName: "Restaurants",
		CategoryIcon: &icon, CategoryArchivedAt: &archived,
		PlannedBaseMinor: 2000, UsedBaseMinor: 500, RemainingBaseMinor: 1500,
	})

	response := performJSON(
		t, budgetTestRouter(t, &fakeBudgetService{value: value}), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/budgets?month=2026-08", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		BaseCurrency string `json:"base_currency"`
		Items        []struct {
			CategoryName       string  `json:"category_name"`
			CategoryIcon       *string `json:"category_icon"`
			CategoryArchivedAt *string `json:"category_archived_at"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.BaseCurrency != "TRY" {
		t.Fatalf("base_currency = %q", body.BaseCurrency)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items = %#v", body.Items)
	}
	// The first item has neither optional field; the second has both.
	if body.Items[0].CategoryIcon != nil || body.Items[0].CategoryArchivedAt != nil {
		t.Fatalf("plain item = %#v", body.Items[0])
	}
	if body.Items[1].CategoryIcon == nil || *body.Items[1].CategoryIcon != icon {
		t.Fatalf("icon = %#v", body.Items[1].CategoryIcon)
	}
	if body.Items[1].CategoryArchivedAt == nil {
		t.Fatal("archived timestamp dropped")
	}
}

func TestReplaceMonthlyBudgetRejectsMissingBody(t *testing.T) {
	response := performJSON(
		t, budgetTestRouter(t, &fakeBudgetService{value: validHTTPBudget()}), http.MethodPut,
		"/v1/workspaces/"+testWorkspaceID+"/budgets/2026-08", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusBadRequest, "invalid_request")
}

func TestBudgetEndpointsEnforceMembership(t *testing.T) {
	service := &fakeBudgetService{err: workspace.ErrForbidden}
	router := budgetTestRouter(t, service)

	response := performJSON(
		t, router, http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/budgets?month=2026-08", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusForbidden, "forbidden")

	response = performJSON(
		t, router, http.MethodPut,
		"/v1/workspaces/"+testWorkspaceID+"/budgets/2026-08",
		`{"name":"August","items":[]}`,
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusForbidden, "forbidden")
}

// A budget item whose stored category id will not parse means a corrupt row; the response
// must fail rather than omit the item and report a total that no longer adds up.
func TestMonthlyBudgetResponseRejectsCorruptStoredIdentifiers(t *testing.T) {
	corrupt := validHTTPBudget()
	corrupt.Items[0].CategoryID = "not-a-uuid"

	response := performJSON(
		t, budgetTestRouter(t, &fakeBudgetService{value: corrupt}), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/budgets?month=2026-08", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusInternalServerError, "internal_error")
}
