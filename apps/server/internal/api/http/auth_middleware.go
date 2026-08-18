package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/auth"
)

type principalContextKey struct{}

type authService interface {
	Register(context.Context, auth.RegisterInput) (auth.AuthResult, error)
	Login(context.Context, auth.LoginInput) (auth.AuthResult, error)
	Authenticate(context.Context, string, auth.Transport) (auth.Principal, error)
	Session(context.Context, auth.Principal) (auth.AuthResult, error)
	Logout(context.Context, auth.Principal) error
}

func authenticationMiddleware(service authService, publicOrigin string) openapi.StrictMiddlewareFunc {
	return func(next openapi.StrictHandlerFunc, operationID string) openapi.StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			if operationID == "Register" || operationID == "Login" {
				if requestedTransport(request) == auth.TransportCookie && !validRequestOrigin(r, publicOrigin) {
					writeError(w, r, http.StatusForbidden, "csrf_rejected", "The request origin is not allowed.")
					return nil, nil
				}
				return next(ctx, w, r, request)
			}
			if operationID == "GetHealth" || operationID == "GetReadiness" {
				return next(ctx, w, r, request)
			}
			if service == nil {
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error.")
				return nil, nil
			}
			token, transport, ok := requestCredential(r)
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication is required or the credentials are invalid.")
				return nil, nil
			}
			principal, err := service.Authenticate(ctx, token, transport)
			if err != nil {
				if errors.Is(err, auth.ErrUnauthorized) {
					writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication is required or the credentials are invalid.")
					return nil, nil
				}
				return nil, err
			}
			if transport == auth.TransportCookie && unsafeMethod(r.Method) && !validRequestOrigin(r, publicOrigin) {
				writeError(w, r, http.StatusForbidden, "csrf_rejected", "The request origin is not allowed.")
				return nil, nil
			}
			ctx = context.WithValue(ctx, principalContextKey{}, principal)
			return next(ctx, w, r, request)
		}
	}
}

func unsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions && method != http.MethodTrace
}

func principalFromContext(ctx context.Context) (auth.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(auth.Principal)
	return principal, ok
}

func requestedTransport(request any) auth.Transport {
	switch typed := request.(type) {
	case openapi.RegisterRequestObject:
		if typed.Body != nil {
			return auth.Transport(typed.Body.Transport)
		}
	case openapi.LoginRequestObject:
		if typed.Body != nil {
			return auth.Transport(typed.Body.Transport)
		}
	}
	return ""
}

func requestCredential(r *http.Request) (string, auth.Transport, bool) {
	var bearerToken string
	authorization := r.Header.Values("Authorization")
	if len(authorization) > 1 {
		return "", "", false
	}
	if len(authorization) == 1 {
		const prefix = "Bearer "
		if !strings.HasPrefix(authorization[0], prefix) {
			return "", "", false
		}
		bearerToken = strings.TrimPrefix(authorization[0], prefix)
		if bearerToken == "" || strings.ContainsAny(bearerToken, " \t\r\n") {
			return "", "", false
		}
	}

	var cookieToken string
	for _, cookie := range r.Cookies() {
		if cookie.Name != sessionCookieName {
			continue
		}
		if cookieToken != "" || cookie.Value == "" {
			return "", "", false
		}
		cookieToken = cookie.Value
	}
	if bearerToken != "" && cookieToken != "" {
		return "", "", false
	}
	if bearerToken != "" {
		return bearerToken, auth.TransportBearer, true
	}
	if cookieToken != "" {
		return cookieToken, auth.TransportCookie, true
	}
	return "", "", false
}

func validRequestOrigin(r *http.Request, publicOrigin string) bool {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
		return false
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return origin == publicOrigin
	}
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return false
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return parsed.Scheme+"://"+parsed.Host == publicOrigin
}

func noStoreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func limitAPIBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") && unsafeMethod(r.Method) {
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		}
		next.ServeHTTP(w, r)
	})
}
