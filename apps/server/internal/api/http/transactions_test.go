package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

const testTransactionID = "0198b7ae-5e93-72da-b7aa-cd015d4bb77b"

type fakeTransactionService struct {
	createInput transaction.WriteInput
	createErr   error
	listFilter  transaction.ListFilter
	listResult  []transaction.Transaction
	listErr     error
	getResult   transaction.Transaction
	getErr      error
	updateInput transaction.WriteInput
	updateErr   error
	deleteErr   error
	deleteCall  int
}

func (s *fakeTransactionService) List(
	_ context.Context, _, _ string, filter transaction.ListFilter,
) ([]transaction.Transaction, error) {
	s.listFilter = filter
	return s.listResult, s.listErr
}

func (s *fakeTransactionService) Get(
	context.Context, string, string, string,
) (transaction.Transaction, error) {
	if s.getErr != nil {
		return transaction.Transaction{}, s.getErr
	}
	return s.getResult, nil
}

func (s *fakeTransactionService) Create(
	_ context.Context, workspaceID, userID string, input transaction.WriteInput,
) (transaction.Transaction, error) {
	s.createInput = input
	if s.createErr != nil {
		return transaction.Transaction{}, s.createErr
	}
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	return transaction.Transaction{
		ID: testTransactionID, WorkspaceID: workspaceID, Kind: input.Kind, Status: input.Status,
		TransactionDate: input.TransactionDate, Payee: input.Payee, Source: transaction.SourceManual,
		CreatedBy: userID, UpdatedBy: userID, CreatedAt: now, UpdatedAt: now,
		Entries: []transaction.Entry{{
			ID: testTransactionID, AccountID: input.Entries[0].AccountID,
			AmountMinor: input.Entries[0].AmountMinor, BaseAmountMinor: input.Entries[0].AmountMinor,
		}}, Allocations: []transaction.Allocation{},
	}, nil
}

func (s *fakeTransactionService) Update(
	_ context.Context, _, _, _ string, input transaction.WriteInput,
) (transaction.Transaction, error) {
	s.updateInput = input
	if s.updateErr != nil {
		return transaction.Transaction{}, s.updateErr
	}
	return s.getResult, nil
}

func (s *fakeTransactionService) SoftDelete(context.Context, string, string, string) error {
	s.deleteCall++
	return s.deleteErr
}

func transactionTestRouter(t *testing.T, service transactionService) http.Handler {
	t.Helper()
	services := testServices()
	services.Transactions = service
	return testRouter(t, services)
}

func TestCreateTransactionPassesCompleteAggregate(t *testing.T) {
	service := &fakeTransactionService{}
	response := performJSON(
		t, transactionTestRouter(t, service), http.MethodPost,
		"/v1/workspaces/"+testWorkspaceID+"/transactions",
		`{"kind":"standard","status":"posted","transaction_date":"2026-08-18","payee":"Market","entries":[{"account_id":"`+testAccountID+`","amount_minor":-1250}],"allocations":[]}`,
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if service.createInput.Kind != transaction.KindStandard ||
		service.createInput.Status != transaction.StatusPosted ||
		service.createInput.TransactionDate.Format(time.DateOnly) != "2026-08-18" ||
		len(service.createInput.Entries) != 1 || service.createInput.Entries[0].AmountMinor != -1250 {
		t.Fatalf("Create() input = %#v", service.createInput)
	}
	var body struct {
		ID      string `json:"id"`
		Entries []struct {
			BaseAmountMinor int64 `json:"base_amount_minor"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != testTransactionID || len(body.Entries) != 1 || body.Entries[0].BaseAmountMinor != -1250 {
		t.Fatalf("transaction response = %#v", body)
	}
}

func TestCreateTransactionMapsDomainConflictsAndRateFailure(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"reconciliation", transaction.ErrDoesNotReconcile, http.StatusConflict, "transaction_does_not_reconcile"},
		{"reference", transaction.ErrReferenceInvalid, http.StatusConflict, "transaction_reference_invalid"},
		{"rate", transaction.ErrBookingRateUnavailable, http.StatusServiceUnavailable, "booking_rate_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeTransactionService{createErr: test.err}
			response := performJSON(
				t, transactionTestRouter(t, service), http.MethodPost,
				"/v1/workspaces/"+testWorkspaceID+"/transactions",
				`{"kind":"adjustment","status":"pending","transaction_date":"2026-08-18","entries":[{"account_id":"`+testAccountID+`","amount_minor":1}]}`,
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
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", body.Error.Code, test.wantCode)
			}
		})
	}
}

func TestListTransactionsPassesDateRangeAndLimit(t *testing.T) {
	service := &fakeTransactionService{}
	response := performJSON(
		t, transactionTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/transactions?from_date=2026-08-01&to_date=2026-08-18&limit=25", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.listFilter.Limit != 25 || service.listFilter.From == nil || service.listFilter.To == nil ||
		service.listFilter.From.Format(time.DateOnly) != "2026-08-01" ||
		service.listFilter.To.Format(time.DateOnly) != "2026-08-18" {
		t.Fatalf("List() filter = %#v", service.listFilter)
	}
}

// A ledger entry with a split across two categories, exercising every optional field and
// both child collections of the aggregate.
func splitTransaction() transaction.Transaction {
	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	payee, description, notes := "Migros", "Weekly shop", "Split between food and household"
	return transaction.Transaction{
		ID: testTransactionID, WorkspaceID: testWorkspaceID,
		Kind: transaction.KindStandard, Status: transaction.StatusPosted,
		TransactionDate: moment, Payee: &payee, Description: &description, Notes: &notes,
		Source: transaction.SourceManual, CreatedBy: testUserID, UpdatedBy: testUserID,
		CreatedAt: moment, UpdatedAt: moment,
		Entries: []transaction.Entry{{
			ID: testTransactionID, AccountID: testAccountID,
			AmountMinor: -150000, BaseAmountMinor: -150000,
		}},
		Allocations: []transaction.Allocation{
			{ID: testTransactionID, CategoryID: testCategoryID, AmountBaseMinor: -100000},
			{ID: testAccountID, CategoryID: testCategoryID, AmountBaseMinor: -50000},
		},
	}
}

func TestGetTransactionReturnsCompleteAggregate(t *testing.T) {
	service := &fakeTransactionService{getResult: splitTransaction()}
	response := performJSON(
		t, transactionTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/transactions/"+testTransactionID, "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		ID              string `json:"id"`
		Kind            string `json:"kind"`
		Status          string `json:"status"`
		TransactionDate string `json:"transaction_date"`
		Payee           string `json:"payee"`
		Notes           string `json:"notes"`
		Source          string `json:"source"`
		Entries         []struct {
			AccountID       string `json:"account_id"`
			AmountMinor     int64  `json:"amount_minor"`
			BaseAmountMinor int64  `json:"base_amount_minor"`
		} `json:"entries"`
		Allocations []struct {
			CategoryID      string `json:"category_id"`
			AmountBaseMinor int64  `json:"amount_base_minor"`
		} `json:"allocations"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != testTransactionID || body.Kind != "standard" || body.Status != "posted" {
		t.Fatalf("transaction = %#v", body)
	}
	// The reporting date must survive as a plain date, not shift with a timezone.
	if body.TransactionDate != "2026-08-18" {
		t.Fatalf("transaction_date = %q, want 2026-08-18", body.TransactionDate)
	}
	if body.Payee != "Migros" || body.Notes == "" || body.Source != "manual" {
		t.Fatalf("optional fields = %#v", body)
	}
	if len(body.Entries) != 1 || body.Entries[0].AmountMinor != -150000 {
		t.Fatalf("entries = %#v", body.Entries)
	}
	// Both halves of the split must survive conversion, and they must reconcile with the entry.
	if len(body.Allocations) != 2 {
		t.Fatalf("allocations = %#v", body.Allocations)
	}
	total := body.Allocations[0].AmountBaseMinor + body.Allocations[1].AmountBaseMinor
	if total != body.Entries[0].BaseAmountMinor {
		t.Fatalf("allocations sum to %d, entry base is %d", total, body.Entries[0].BaseAmountMinor)
	}
}

func TestTransactionEndpointsMapFailures(t *testing.T) {
	tests := []struct {
		name       string
		service    *fakeTransactionService
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:    "get missing transaction",
			service: &fakeTransactionService{getErr: transaction.ErrNotFound},
			method:  http.MethodGet, path: "/transactions/" + testTransactionID,
			wantStatus: http.StatusNotFound, wantCode: "not_found",
		},
		{
			name:    "get outside workspace",
			service: &fakeTransactionService{getErr: workspace.ErrForbidden},
			method:  http.MethodGet, path: "/transactions/" + testTransactionID,
			wantStatus: http.StatusForbidden, wantCode: "forbidden",
		},
		{
			name:    "list rejects an invalid range",
			service: &fakeTransactionService{listErr: transaction.ErrInvalidInput},
			method:  http.MethodGet, path: "/transactions",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
		},
		{
			name:    "update that does not reconcile",
			service: &fakeTransactionService{updateErr: transaction.ErrDoesNotReconcile},
			method:  http.MethodPut, path: "/transactions/" + testTransactionID,
			body:       validTransactionBody(),
			wantStatus: http.StatusConflict, wantCode: "transaction_does_not_reconcile",
		},
		{
			name:    "update referencing a missing account",
			service: &fakeTransactionService{updateErr: transaction.ErrReferenceInvalid},
			method:  http.MethodPut, path: "/transactions/" + testTransactionID,
			body:       validTransactionBody(),
			wantStatus: http.StatusConflict, wantCode: "transaction_reference_invalid",
		},
		{
			// A missing historical rate cannot be resolved by retrying the same request, so it
			// reports unavailable rather than a client error.
			name:    "update without a booking rate",
			service: &fakeTransactionService{updateErr: transaction.ErrBookingRateUnavailable},
			method:  http.MethodPut, path: "/transactions/" + testTransactionID,
			body:       validTransactionBody(),
			wantStatus: http.StatusServiceUnavailable, wantCode: "booking_rate_unavailable",
		},
		{
			name:    "delete missing transaction",
			service: &fakeTransactionService{deleteErr: transaction.ErrNotFound},
			method:  http.MethodDelete, path: "/transactions/" + testTransactionID,
			wantStatus: http.StatusNotFound, wantCode: "not_found",
		},
		{
			name:    "delete outside workspace",
			service: &fakeTransactionService{deleteErr: workspace.ErrForbidden},
			method:  http.MethodDelete, path: "/transactions/" + testTransactionID,
			wantStatus: http.StatusForbidden, wantCode: "forbidden",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performJSON(
				t, transactionTestRouter(t, test.service), test.method,
				"/v1/workspaces/"+testWorkspaceID+test.path, test.body,
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			assertErrorResponse(t, response, test.wantStatus, test.wantCode)
		})
	}
}

func TestUpdateTransactionPassesAggregateAndDeleteSoftDeletes(t *testing.T) {
	service := &fakeTransactionService{getResult: splitTransaction()}
	router := transactionTestRouter(t, service)

	response := performJSON(
		t, router, http.MethodPut,
		"/v1/workspaces/"+testWorkspaceID+"/transactions/"+testTransactionID,
		validTransactionBody(), map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", response.Code, response.Body.String())
	}
	if len(service.updateInput.Entries) != 1 || len(service.updateInput.Allocations) != 1 {
		t.Fatalf("update input = %#v", service.updateInput)
	}

	response = performJSON(
		t, router, http.MethodDelete,
		"/v1/workspaces/"+testWorkspaceID+"/transactions/"+testTransactionID, "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusNoContent || service.deleteCall != 1 {
		t.Fatalf("delete status = %d, calls = %d", response.Code, service.deleteCall)
	}
}

func TestUpdateTransactionRejectsMissingBody(t *testing.T) {
	response := performJSON(
		t, transactionTestRouter(t, &fakeTransactionService{}), http.MethodPut,
		"/v1/workspaces/"+testWorkspaceID+"/transactions/"+testTransactionID, "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusBadRequest, "invalid_request")
}

func validTransactionBody() string {
	return `{"kind":"standard","status":"posted","transaction_date":"2026-08-18",` +
		`"entries":[{"account_id":"` + testAccountID + `","amount_minor":-150000}],` +
		`"allocations":[{"category_id":"` + testCategoryID + `","amount_base_minor":-150000}]}`
}

// Create maps the same aggregate failures as update; each carries a distinct code so a
// client can tell an unbalanced split from a missing account.
func TestCreateTransactionMapsAggregateFailures(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"does not reconcile", transaction.ErrDoesNotReconcile,
			http.StatusConflict, "transaction_does_not_reconcile"},
		{"reference invalid", transaction.ErrReferenceInvalid,
			http.StatusConflict, "transaction_reference_invalid"},
		{"booking rate unavailable", transaction.ErrBookingRateUnavailable,
			http.StatusServiceUnavailable, "booking_rate_unavailable"},
		{"rejected input", transaction.ErrInvalidInput,
			http.StatusBadRequest, "invalid_request"},
		{"outside workspace", workspace.ErrForbidden,
			http.StatusForbidden, "forbidden"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeTransactionService{createErr: test.err}
			response := performJSON(
				t, transactionTestRouter(t, service), http.MethodPost,
				"/v1/workspaces/"+testWorkspaceID+"/transactions", validTransactionBody(),
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			assertErrorResponse(t, response, test.wantStatus, test.wantCode)
		})
	}
}

func TestCreateTransactionRejectsMissingBody(t *testing.T) {
	response := performJSON(
		t, transactionTestRouter(t, &fakeTransactionService{}), http.MethodPost,
		"/v1/workspaces/"+testWorkspaceID+"/transactions", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusBadRequest, "invalid_request")
}

// The date range and limit reach the service as given; a dropped bound would silently widen
// the reporting window.
func TestListTransactionsForwardsFilterAndConvertsResults(t *testing.T) {
	service := &fakeTransactionService{
		listResult: []transaction.Transaction{splitTransaction()},
	}
	response := performJSON(
		t, transactionTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+
			"/transactions?from_date=2026-08-01&to_date=2026-08-31&limit=25", "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if service.listFilter.From == nil || service.listFilter.To == nil {
		t.Fatalf("filter = %#v, want both bounds", service.listFilter)
	}
	if service.listFilter.From.Format(time.DateOnly) != "2026-08-01" ||
		service.listFilter.To.Format(time.DateOnly) != "2026-08-31" ||
		service.listFilter.Limit != 25 {
		t.Fatalf("filter = %#v", service.listFilter)
	}
	var body struct {
		Transactions []struct {
			ID          string `json:"id"`
			Allocations []struct {
				AmountBaseMinor int64 `json:"amount_base_minor"`
			} `json:"allocations"`
		} `json:"transactions"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Transactions) != 1 || len(body.Transactions[0].Allocations) != 2 {
		t.Fatalf("transactions = %#v", body.Transactions)
	}
}

// A malformed identifier is rejected while the request is bound, before any handler or
// service runs, so the service never sees one. This is what a client actually receives, and
// it is why the handlers do not map an invalid-identifier error of their own.
func TestMalformedTransactionIdentifierIsRejectedAtBinding(t *testing.T) {
	service := &fakeTransactionService{getResult: splitTransaction()}
	router := transactionTestRouter(t, service)

	for _, id := range []string{"not-a-uuid", "123", testTransactionID + "-extra"} {
		t.Run(id, func(t *testing.T) {
			response := performJSON(
				t, router, http.MethodGet,
				"/v1/workspaces/"+testWorkspaceID+"/transactions/"+id, "",
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			assertErrorResponse(t, response, http.StatusBadRequest, "invalid_request")
		})
	}
}

// Persisted identifiers are always UUIDs, so a value that will not parse means the stored
// row is corrupt. That must surface as a server error rather than a 200 carrying a
// half-built body.
func TestTransactionResponseRejectsCorruptStoredIdentifiers(t *testing.T) {
	corrupt := splitTransaction()
	corrupt.Entries[0].AccountID = "not-a-uuid"
	service := &fakeTransactionService{getResult: corrupt}

	response := performJSON(
		t, transactionTestRouter(t, service), http.MethodGet,
		"/v1/workspaces/"+testWorkspaceID+"/transactions/"+testTransactionID, "",
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	assertErrorResponse(t, response, http.StatusInternalServerError, "internal_error")
}
