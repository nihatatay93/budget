package httpapi

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := ""
		if values := r.Header.Values(requestIDHeader); len(values) == 1 && validRequestID.MatchString(values[0]) {
			requestID = values[0]
		}
		if requestID == "" {
			requestID = rand.Text()
		}

		r.Header.Set(requestIDHeader, requestID)
		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID)))
	})
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error(
					"panic serving request",
					"panic", recovered,
					"request_id", requestID(r.Context()),
				)
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, openapi.ErrorResponse{
		Error: openapi.APIError{
			Code:      code,
			Message:   message,
			RequestId: requestID(r.Context()),
		},
	})
}

// securityHeadersMiddleware sets response headers that constrain how a browser treats the
// application.
//
// The SPA and the API share an origin, so one policy covers both. The policy is strict
// because the application loads nothing from anywhere else: no third-party scripts, fonts, or
// images, and no framing.
func securityHeadersMiddleware(hstsEnabled bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		// connect-src stays 'self' because every request the client makes is same-origin;
		// exchange rates are fetched by the server, never the browser.
		header.Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"base-uri 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
			"img-src 'self' data:",
			"object-src 'none'",
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"connect-src 'self'",
		}, "; "))
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "same-origin")
		// The product needs none of these, and denying them shrinks what a successful
		// injection could reach.
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if hstsEnabled {
			// Only when the deployment is actually served over TLS; sending it from a plain
			// HTTP development server would pin a browser to a scheme that does not work.
			header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
