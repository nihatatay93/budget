package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type healthyDatabase struct{}

func (healthyDatabase) Ping(context.Context) error { return nil }

type unhealthyDatabase struct{}

func (unhealthyDatabase) Ping(context.Context) error { return errors.New("database unavailable") }

func TestHealth(t *testing.T) {
	router := newTestRouter(t, healthyDatabase{})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get(requestIDHeader) == "" {
		t.Fatal("response does not contain a request ID")
	}
}

func TestRequestID(t *testing.T) {
	router := newTestRouter(t, healthyDatabase{})

	t.Run("preserves a valid caller value", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		request.Header.Set(requestIDHeader, "caller-request-123")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if got := response.Header().Get(requestIDHeader); got != "caller-request-123" {
			t.Fatalf("request ID = %q, want caller-request-123", got)
		}
	})

	t.Run("replaces an invalid caller value", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		request.Header.Set(requestIDHeader, "contains spaces")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		got := response.Header().Get(requestIDHeader)
		if got == "" || got == "contains spaces" {
			t.Fatalf("request ID was not replaced: %q", got)
		}
	})
}

func TestReadinessUnavailable(t *testing.T) {
	router := newTestRouter(t, unhealthyDatabase{})
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestUnknownAPIRouteDoesNotFallBackToSPA(t *testing.T) {
	router := newTestRouter(t, healthyDatabase{})

	request := httptest.NewRequest(http.MethodGet, "/v1/not-found", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
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
	if body.Error.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", body.Error.Code)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != response.Header().Get(requestIDHeader) {
		t.Fatalf("body request ID %q does not match response header", body.Error.RequestID)
	}
}

func newTestRouter(t *testing.T, database pinger) http.Handler {
	t.Helper()
	services := testServices()
	services.Database = database
	return testRouter(t, services)
}

// The SPA and API share an origin, so one policy covers both. The application loads nothing
// from anywhere else, which is what lets the policy stay this strict.
func TestSecurityHeadersAreSetOnEveryResponse(t *testing.T) {
	router := newTestRouter(t, healthyDatabase{})

	for _, path := range []string{"/healthz", "/v1/session", "/"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			policy := recorder.Header().Get("Content-Security-Policy")
			for _, directive := range []string{
				"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'",
				"base-uri 'self'", "form-action 'self'",
			} {
				if !strings.Contains(policy, directive) {
					t.Fatalf("policy %q is missing %q", policy, directive)
				}
			}
			if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("responses may be content-sniffed")
			}
			if recorder.Header().Get("Referrer-Policy") != "same-origin" {
				t.Fatal("referrer policy is not set")
			}
		})
	}
}

// HSTS pins a browser to HTTPS, so sending it from a plain-HTTP development server would
// make the application unreachable there. It follows the same signal as the secure cookie.
func TestStrictTransportSecurityFollowsTheDeploymentScheme(t *testing.T) {
	services := testServices()
	options := testOptions()
	options.HSTS = false
	plain, err := NewRouter(services, options)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	options.HSTS = true
	secured, err := NewRouter(services, options)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	for name, router := range map[string]http.Handler{"plain": plain, "secured": secured} {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		header := recorder.Header().Get("Strict-Transport-Security")
		if name == "plain" && header != "" {
			t.Fatalf("HSTS was sent over plain HTTP: %q", header)
		}
		if name == "secured" && !strings.Contains(header, "max-age=") {
			t.Fatalf("HSTS missing on a TLS deployment: %q", header)
		}
	}
}
