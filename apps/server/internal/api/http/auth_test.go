package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nihatatay93/budget/internal/auth"
)

const (
	testUserID      = "0198b7ae-5e93-72d7-a256-2a0f6622c7ec"
	testWorkspaceID = "0198b7ae-5e93-72d8-99af-ff40c48ad342"
)

type fakeAuthService struct {
	registerResult auth.AuthResult
	loginResult    auth.AuthResult
	principal      auth.Principal
	authCalls      int
	logoutCalls    int
}

func (s *fakeAuthService) Register(context.Context, auth.RegisterInput) (auth.AuthResult, error) {
	return s.registerResult, nil
}
func (s *fakeAuthService) Login(context.Context, auth.LoginInput) (auth.AuthResult, error) {
	return s.loginResult, nil
}
func (s *fakeAuthService) Authenticate(context.Context, string, auth.Transport) (auth.Principal, error) {
	s.authCalls++
	return s.principal, nil
}
func (s *fakeAuthService) Session(_ context.Context, principal auth.Principal) (auth.AuthResult, error) {
	return auth.AuthResult{Principal: principal, Workspaces: testWorkspaces()}, nil
}
func (s *fakeAuthService) Logout(context.Context, auth.Principal) error {
	s.logoutCalls++
	return nil
}

func TestRegisterCookieSessionDoesNotExposeToken(t *testing.T) {
	service := &fakeAuthService{registerResult: testAuthResult(auth.TransportCookie)}
	router := authTestRouter(t, service, true)
	response := performJSON(t, router, http.MethodPost, "/v1/auth/register", registerJSON("cookie"), map[string]string{
		"Origin": "https://budget.example",
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	setCookie := response.Header().Get("Set-Cookie")
	for _, attribute := range []string{"budget_session=raw-token", "Path=/", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(setCookie, attribute) {
			t.Errorf("Set-Cookie %q does not contain %q", setCookie, attribute)
		}
	}
	if strings.Contains(response.Body.String(), "raw-token") || strings.Contains(response.Body.String(), "bearer_token") {
		t.Fatalf("cookie authentication response exposed the raw token: %s", response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
}

func TestLoginBearerSessionReturnsTokenWithoutCookie(t *testing.T) {
	service := &fakeAuthService{loginResult: testAuthResult(auth.TransportBearer)}
	router := authTestRouter(t, service, true)
	response := performJSON(t, router, http.MethodPost, "/v1/auth/login", `{
		"email":"person@example.com","password":"a sufficiently long password","transport":"bearer"
	}`, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if response.Header().Get("Set-Cookie") != "" || !strings.Contains(response.Body.String(), `"bearer_token":"raw-token"`) {
		t.Fatalf("bearer response = headers %v body %s", response.Header(), response.Body.String())
	}
}

func TestCookieAuthenticationRejectsCrossSiteLogout(t *testing.T) {
	service := &fakeAuthService{principal: testAuthResult(auth.TransportCookie).Principal}
	router := authTestRouter(t, service, true)
	response := performJSON(t, router, http.MethodPost, "/v1/auth/logout", "", map[string]string{
		"Cookie":         "budget_session=raw-token",
		"Origin":         "https://attacker.example",
		"Sec-Fetch-Site": "cross-site",
	})
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if service.logoutCalls != 0 {
		t.Fatal("cross-site request revoked the session")
	}
}

func TestBearerAuthenticationBypassesBrowserCSRFCheck(t *testing.T) {
	service := &fakeAuthService{principal: testAuthResult(auth.TransportBearer).Principal}
	router := authTestRouter(t, service, true)
	response := performJSON(t, router, http.MethodPost, "/v1/auth/logout", "", map[string]string{
		"Authorization":  "Bearer raw-token",
		"Origin":         "https://attacker.example",
		"Sec-Fetch-Site": "cross-site",
	})
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if service.logoutCalls != 1 {
		t.Fatalf("Logout() calls = %d, want 1", service.logoutCalls)
	}
}

func TestAuthenticationRejectsMixedCredentials(t *testing.T) {
	service := &fakeAuthService{}
	router := authTestRouter(t, service, true)
	response := performJSON(t, router, http.MethodGet, "/v1/session", "", map[string]string{
		"Authorization": "Bearer raw-token",
		"Cookie":        "budget_session=raw-token",
	})
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if service.authCalls != 0 {
		t.Fatal("mixed credentials reached session authentication")
	}
}

func authTestRouter(t *testing.T, service authService, cookieSecure bool) http.Handler {
	t.Helper()
	services := testServices()
	services.Authentication = service
	options := testOptions()
	options.CookieSecure = cookieSecure
	router, err := NewRouter(services, options)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return router
}

func performJSON(
	t *testing.T,
	router http.Handler,
	method, path, body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func registerJSON(transport string) string {
	payload := map[string]string{
		"email": "person@example.com", "password": "a sufficiently long password",
		"display_name": "Person", "workspace_name": "Home", "base_currency": "USD",
		"timezone": "UTC", "transport": transport,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func testAuthResult(transport auth.Transport) auth.AuthResult {
	return auth.AuthResult{
		Token: "raw-token",
		Principal: auth.Principal{
			SessionID: "0198b7ae-5e93-72d9-ab00-32b0861a3f37",
			User:      auth.User{ID: testUserID, Email: "person@example.com", DisplayName: "Person"},
			Transport: transport,
		},
		Workspaces: testWorkspaces(),
	}
}

func testWorkspaces() []auth.Workspace {
	return []auth.Workspace{{
		ID: testWorkspaceID, Name: "Home", BaseCurrency: "USD", Timezone: "UTC", Role: "owner",
	}}
}
