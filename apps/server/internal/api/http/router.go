package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/webui"
)

type pinger interface {
	Ping(context.Context) error
}

// Services are the router's collaborators. Every field except ExchangeRates is required, and
// a missing one fails router construction rather than surfacing as a 500 per request.
type Services struct {
	Database       pinger
	Authentication authService
	Accounts       accountService
	Categories     categoryService
	Transactions   transactionService
	Budgets        budgetService
	Reporting      reportingService
	Analysis       analysisService
	Collaboration  collaborationService

	// ExchangeRates is optional. A nil value means the operator did not enable rate
	// fetching, which the rates endpoint reports as unavailable rather than as a fault.
	ExchangeRates exchangeRateService
}

// Options are the router's non-dependency settings.
type Options struct {
	PublicOrigin string
	CookieSecure bool
	Logger       *slog.Logger
	// TrustedProxies are the networks whose forwarded-for headers may be believed. Empty
	// means the immediate peer address is used, which is correct for a direct deployment.
	TrustedProxies []*net.IPNet
	// AuthRateLimit throttles credential endpoints, in attempts per minute per client. Zero
	// disables throttling.
	AuthRateLimit int
	AuthRateBurst int
	// HSTS is sent only when the deployment terminates TLS.
	HSTS bool
	// SessionTTL is how long an issued session lasts. The session cookie is given the same
	// lifetime so a browser stops presenting a credential the server has already expired.
	SessionTTL time.Duration
}

// NewRouter builds the HTTP handler. It reports every missing required service at once so a
// misconfigured deployment fails at startup with a complete list.
func NewRouter(services Services, options Options) (http.Handler, error) {
	if options.Logger == nil {
		return nil, errors.New("router options require a logger")
	}
	if err := services.validate(); err != nil {
		return nil, err
	}
	// An interface holding a nil pointer is not equal to nil, so a disabled optional service
	// would otherwise pass a nil check and then panic on first use.
	if isNil(services.ExchangeRates) {
		services.ExchangeRates = nil
	}

	spa, err := webui.NewHandler()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	strictServer := openapi.NewStrictHandlerWithOptions(
		&server{
			Services:     services,
			cookieSecure: options.CookieSecure,
			sessionTTL:   options.SessionTTL,
		},
		[]openapi.StrictMiddlewareFunc{
			authenticationMiddleware(services.Authentication, options.PublicOrigin),
		},
		openapi.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "The request is invalid.")
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				// Shared domain failures answer consistently here, so a handler that returns
				// one unchanged cannot give it the wrong status.
				if status, code, message, handled := domainErrorResponse(err); handled {
					writeError(w, r, status, code, message)
					return
				}
				options.Logger.Error(
					"write API response", "error", err, "request_id", requestID(r.Context()),
				)
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error.")
			},
		},
	)
	openapi.HandlerWithOptions(strictServer, openapi.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "The request is invalid.")
		},
	})
	mux.HandleFunc("GET /v1/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found", "The requested resource was not found.")
	})
	mux.Handle("GET /", spa)

	var limiter *rateLimiter
	if options.AuthRateLimit > 0 {
		burst := options.AuthRateBurst
		if burst <= 0 {
			burst = options.AuthRateLimit
		}
		limiter = newRateLimiter(options.AuthRateLimit, burst, nil)
	}

	// Ordering matters: a request gets an identifier first so every later layer can log it,
	// panics are contained next, and throttling runs before the body is read so a flood costs
	// as little as possible.
	handler := securityHeadersMiddleware(options.HSTS,
		noStoreMiddleware(limitAPIBodyMiddleware(mux)))
	handler = rateLimitMiddleware(limiter, options.TrustedProxies, handler)
	return requestIDMiddleware(recoverMiddleware(options.Logger, handler)), nil
}

func (s Services) validate() error {
	required := []struct {
		name  string
		value any
	}{
		{"Database", s.Database},
		{"Authentication", s.Authentication},
		{"Accounts", s.Accounts},
		{"Categories", s.Categories},
		{"Transactions", s.Transactions},
		{"Budgets", s.Budgets},
		{"Reporting", s.Reporting},
		{"Analysis", s.Analysis},
		{"Collaboration", s.Collaboration},
	}
	missing := make([]string, 0, len(required))
	for _, service := range required {
		if isNil(service.value) {
			missing = append(missing, service.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("router requires services: %s", strings.Join(missing, ", "))
	}
	return nil
}

// isNil reports whether value is nil, including an interface carrying a nil pointer. The
// plain == nil comparison misses that second case, which is how a disabled dependency turns
// into a nil-pointer panic at request time instead of an error at startup.
func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
