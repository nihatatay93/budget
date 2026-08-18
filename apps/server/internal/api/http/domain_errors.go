package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/nihatatay93/budget/internal/account"
	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/budget"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/reporting"
	"github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

// errUnauthenticated reports a handler reached without an authenticated caller. The
// authentication middleware rejects those requests first, so this is a guard against an
// operation being wired without authentication rather than a path users reach.
var errUnauthenticated = errors.New("request is not authenticated")

// requirePrincipal returns the authenticated caller. A handler that returns the error
// unchanged produces the correct 401 through the central mapping below.
func requirePrincipal(ctx context.Context) (auth.Principal, error) {
	principal, ok := principalFromContext(ctx)
	if !ok {
		return auth.Principal{}, errUnauthenticated
	}
	return principal, nil
}

// domainErrorResponse maps the sentinels that mean the same thing for every operation.
//
// Centralizing them means a handler cannot answer a membership failure with the wrong status
// by forgetting a case: an unmapped error reaches here and is answered consistently.
// Operation-specific conflicts stay in their handler, where the code and message are precise.
func domainErrorResponse(err error) (status int, code, message string, handled bool) {
	switch {
	case errors.Is(err, errUnauthenticated):
		return http.StatusUnauthorized, "unauthorized",
			"Authentication is required or the credentials are invalid.", true
	case errors.Is(err, workspace.ErrForbidden):
		return http.StatusForbidden, "forbidden",
			"You do not have access to this workspace operation.", true
	case errors.Is(err, account.ErrNotFound),
		errors.Is(err, category.ErrNotFound),
		errors.Is(err, transaction.ErrNotFound),
		errors.Is(err, budget.ErrNotFound),
		errors.Is(err, workspace.ErrNotFound):
		return http.StatusNotFound, "not_found", "The requested resource was not found.", true
	case errors.Is(err, account.ErrInvalidInput),
		errors.Is(err, category.ErrInvalidInput),
		errors.Is(err, transaction.ErrInvalidInput),
		errors.Is(err, budget.ErrInvalidInput),
		errors.Is(err, reporting.ErrInvalidInput),
		errors.Is(err, workspace.ErrInvalidInput):
		return http.StatusBadRequest, "invalid_request", "The request is invalid.", true
	default:
		return 0, "", "", false
	}
}
