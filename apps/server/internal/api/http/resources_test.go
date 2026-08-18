package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nihatatay93/budget/internal/account"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/workspace"
)

const (
	testAccountID  = "0198b7ae-5e93-72da-b7aa-cd015d4bb779"
	testCategoryID = "0198b7ae-5e93-72da-b7aa-cd015d4bb77a"
)

type fakeAccountService struct {
	listResult []account.Account
	listErr    error
	createErr  error
	getErr     error
	getResult  account.Account
	updateErr  error
	archiveErr error
	listCall   resourceListCall
	createCall int
}

func (s *fakeAccountService) List(
	_ context.Context, workspaceID, userID string, includeArchived bool,
) ([]account.Account, error) {
	s.listCall = resourceListCall{workspaceID, userID, includeArchived}
	return s.listResult, s.listErr
}
func (s *fakeAccountService) Get(context.Context, string, string, string) (account.Account, error) {
	if s.getErr != nil {
		return account.Account{}, s.getErr
	}
	return s.getResult, nil
}
func (s *fakeAccountService) Create(
	context.Context, string, string, account.WriteInput,
) (account.Account, error) {
	s.createCall++
	return account.Account{}, s.createErr
}
func (s *fakeAccountService) Update(
	context.Context, string, string, string, account.WriteInput,
) (account.Account, error) {
	if s.updateErr != nil {
		return account.Account{}, s.updateErr
	}
	return s.getResult, nil
}
func (s *fakeAccountService) Archive(context.Context, string, string, string) error {
	return s.archiveErr
}

type fakeCategoryService struct {
	createErr  error
	createCall int
	getErr     error
	archiveErr error
}

func (*fakeCategoryService) List(context.Context, string, string, bool) ([]category.Category, error) {
	return nil, nil
}
func (s *fakeCategoryService) Get(context.Context, string, string, string) (category.Category, error) {
	if s.getErr != nil {
		return category.Category{}, s.getErr
	}
	return category.Category{ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: "expense"}, nil
}
func (s *fakeCategoryService) Create(
	context.Context, string, string, category.WriteInput,
) (category.Category, error) {
	s.createCall++
	return category.Category{}, s.createErr
}
func (*fakeCategoryService) Update(
	context.Context, string, string, string, category.WriteInput,
) (category.Category, error) {
	return category.Category{}, category.ErrNotFound
}
func (s *fakeCategoryService) Archive(context.Context, string, string, string) error {
	return s.archiveErr
}

type resourceListCall struct {
	workspaceID     string
	userID          string
	includeArchived bool
}

func TestListAccountsPassesWorkspaceScopeAndArchiveFilter(t *testing.T) {
	authentication := &fakeAuthService{principal: testAuthResult("bearer").Principal}
	accounts := &fakeAccountService{listResult: []account.Account{{
		ID: testAccountID, WorkspaceID: testWorkspaceID, Name: "Checking",
		Type: account.TypeBank, Currency: "USD", BalanceMinor: 1250,
	}}}
	router := resourceTestRouter(t, authentication, accounts, &fakeCategoryService{})

	response := performJSON(
		t, router, http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/accounts?include_archived=true", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if accounts.listCall != (resourceListCall{testWorkspaceID, testUserID, true}) {
		t.Fatalf("List() call = %#v", accounts.listCall)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	var body struct {
		Accounts []struct {
			ID           string `json:"id"`
			BalanceMinor int64  `json:"balance_minor"`
		} `json:"accounts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Accounts) != 1 || body.Accounts[0].ID != testAccountID || body.Accounts[0].BalanceMinor != 1250 {
		t.Fatalf("accounts = %#v", body.Accounts)
	}
}

func TestCreateCategoryMapsHierarchyConflict(t *testing.T) {
	authentication := &fakeAuthService{principal: testAuthResult("bearer").Principal}
	categories := &fakeCategoryService{createErr: category.ErrHierarchyConflict}
	router := resourceTestRouter(t, authentication, &fakeAccountService{}, categories)

	response := performJSON(
		t, router, http.MethodPost, "/v1/workspaces/"+testWorkspaceID+"/categories",
		`{"name":"Food","kind":"expense","parent_id":"`+testCategoryID+`"}`,
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	if categories.createCall != 1 {
		t.Fatalf("Create() calls = %d, want 1", categories.createCall)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "category_hierarchy_conflict" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

func TestCookieAuthenticationRejectsCrossSiteAccountMutation(t *testing.T) {
	authentication := &fakeAuthService{principal: testAuthResult("cookie").Principal}
	accounts := &fakeAccountService{}
	router := resourceTestRouter(t, authentication, accounts, &fakeCategoryService{})

	response := performJSON(
		t, router, http.MethodPost, "/v1/workspaces/"+testWorkspaceID+"/accounts",
		`{"name":"Cash","type":"cash","currency":"USD"}`,
		map[string]string{
			"Cookie": "budget_session=raw-token", "Origin": "https://attacker.example",
			"Sec-Fetch-Site": "cross-site",
		},
	)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if accounts.createCall != 0 {
		t.Fatal("cross-site request reached the account service")
	}
}

func resourceTestRouter(
	t *testing.T,
	authentication authService,
	accounts accountService,
	categories categoryService,
) http.Handler {
	t.Helper()
	services := testServices()
	services.Authentication = authentication
	services.Accounts = accounts
	services.Categories = categories
	return testRouter(t, services)
}

// These exercise the central domain-error mapping end to end: the handlers no longer map
// forbidden or not-found themselves, so a regression there would surface here as a 500.
func TestAccountEndpointsMapDomainErrorsCentrally(t *testing.T) {
	tests := []struct {
		name       string
		service    *fakeAccountService
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "get missing account",
			service:    &fakeAccountService{getErr: account.ErrNotFound},
			method:     http.MethodGet,
			path:       "/accounts/" + testAccountID,
			wantStatus: http.StatusNotFound, wantCode: "not_found",
		},
		{
			name:       "get outside workspace",
			service:    &fakeAccountService{getErr: workspace.ErrForbidden},
			method:     http.MethodGet,
			path:       "/accounts/" + testAccountID,
			wantStatus: http.StatusForbidden, wantCode: "forbidden",
		},
		{
			name:       "archive missing account",
			service:    &fakeAccountService{archiveErr: account.ErrNotFound},
			method:     http.MethodDelete,
			path:       "/accounts/" + testAccountID,
			wantStatus: http.StatusNotFound, wantCode: "not_found",
		},
		{
			name:       "create rejected input",
			service:    &fakeAccountService{createErr: account.ErrInvalidInput},
			method:     http.MethodPost,
			path:       "/accounts",
			body:       `{"name":"Cash","type":"cash","currency":"TRY"}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
		},
		{
			// Currency lock stays in the handler: it needs its own code and message.
			name:       "update locked currency",
			service:    &fakeAccountService{updateErr: account.ErrCurrencyLocked},
			method:     http.MethodPut,
			path:       "/accounts/" + testAccountID,
			body:       `{"name":"Cash","type":"cash","currency":"USD"}`,
			wantStatus: http.StatusConflict, wantCode: "account_currency_locked",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			services := testServices()
			services.Accounts = test.service
			response := performJSON(
				t, testRouter(t, services), test.method,
				"/v1/workspaces/"+testWorkspaceID+test.path, test.body,
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			assertErrorResponse(t, response, test.wantStatus, test.wantCode)
		})
	}
}

func TestCategoryEndpointsMapDomainErrorsCentrally(t *testing.T) {
	tests := []struct {
		name       string
		service    *fakeCategoryService
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "get missing category",
			service:    &fakeCategoryService{getErr: category.ErrNotFound},
			method:     http.MethodGet,
			path:       "/categories/" + testCategoryID,
			wantStatus: http.StatusNotFound, wantCode: "not_found",
		},
		{
			name:       "archive outside workspace",
			service:    &fakeCategoryService{archiveErr: workspace.ErrForbidden},
			method:     http.MethodDelete,
			path:       "/categories/" + testCategoryID,
			wantStatus: http.StatusForbidden, wantCode: "forbidden",
		},
		{
			// Protection stays in the handler so the message names the cause.
			name:       "archive protected category",
			service:    &fakeCategoryService{archiveErr: category.ErrProtected},
			method:     http.MethodDelete,
			path:       "/categories/" + testCategoryID,
			wantStatus: http.StatusConflict, wantCode: "system_category_protected",
		},
		{
			name:       "archive parent with children",
			service:    &fakeCategoryService{archiveErr: category.ErrHasChildren},
			method:     http.MethodDelete,
			path:       "/categories/" + testCategoryID,
			wantStatus: http.StatusConflict, wantCode: "category_has_children",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			services := testServices()
			services.Categories = test.service
			response := performJSON(
				t, testRouter(t, services), test.method,
				"/v1/workspaces/"+testWorkspaceID+test.path, "",
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			assertErrorResponse(t, response, test.wantStatus, test.wantCode)
		})
	}
}

func TestArchiveAccountReturnsNoContent(t *testing.T) {
	services := testServices()
	services.Accounts = &fakeAccountService{}
	response := performJSON(
		t, testRouter(t, services), http.MethodDelete,
		"/v1/workspaces/"+testWorkspaceID+"/accounts/"+testAccountID, "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header missing")
	}
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d: %s", response.Code, status, response.Body.String())
	}
	// Every error response carries the request ID in both the header and the body, whether it
	// was produced by a handler or by the central mapping.
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header missing")
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
	if body.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("body request ID %q does not match header", body.Error.RequestID)
	}
}
