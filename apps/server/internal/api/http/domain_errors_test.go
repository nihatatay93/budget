package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/nihatatay93/budget/internal/account"
	"github.com/nihatatay93/budget/internal/budget"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/reporting"
	"github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

func TestDomainErrorResponseMapsSharedSentinels(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"unauthenticated", errUnauthenticated, http.StatusUnauthorized, "unauthorized"},
		{"forbidden", workspace.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"account missing", account.ErrNotFound, http.StatusNotFound, "not_found"},
		{"category missing", category.ErrNotFound, http.StatusNotFound, "not_found"},
		{"transaction missing", transaction.ErrNotFound, http.StatusNotFound, "not_found"},
		{"budget missing", budget.ErrNotFound, http.StatusNotFound, "not_found"},
		{"account input", account.ErrInvalidInput, http.StatusBadRequest, "invalid_request"},
		{"transaction input", transaction.ErrInvalidInput, http.StatusBadRequest, "invalid_request"},
		{"reporting input", reporting.ErrInvalidInput, http.StatusBadRequest, "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, code, _, handled := domainErrorResponse(test.err)
			if !handled || status != test.wantStatus || code != test.wantCode {
				t.Fatalf("domainErrorResponse(%v) = %d/%q/%v", test.err, status, code, handled)
			}
		})
	}
}

// Wrapping must not defeat the mapping: repositories annotate errors with context before
// they reach the handler.
func TestDomainErrorResponseSeesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("list accounts: %w", workspace.ErrForbidden)
	status, _, _, handled := domainErrorResponse(wrapped)
	if !handled || status != http.StatusForbidden {
		t.Fatalf("wrapped forbidden = %d, handled = %v", status, handled)
	}
}

// An unrecognised failure must stay a 500 so a genuine fault is never reported as a
// client error.
func TestDomainErrorResponseLeavesUnknownErrorsUnhandled(t *testing.T) {
	if _, _, _, handled := domainErrorResponse(errors.New("database exploded")); handled {
		t.Fatal("domainErrorResponse() claimed an unknown error")
	}
}

func TestRequirePrincipalReportsMissingCaller(t *testing.T) {
	if _, err := requirePrincipal(context.Background()); !errors.Is(err, errUnauthenticated) {
		t.Fatalf("requirePrincipal() error = %v, want errUnauthenticated", err)
	}
}
