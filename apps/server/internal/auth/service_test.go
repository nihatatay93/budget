package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	registration Registration
	user         StoredUser
	session      Session
	principal    Principal
	workspaces   []Workspace
	userError    error
}

func (r *fakeRepository) Register(_ context.Context, registration Registration) error {
	r.registration = registration
	return nil
}
func (r *fakeRepository) UserByEmail(context.Context, string) (StoredUser, error) {
	return r.user, r.userError
}
func (r *fakeRepository) CreateSession(_ context.Context, session Session) error {
	r.session = session
	return nil
}
func (r *fakeRepository) SessionByTokenHash(context.Context, []byte) (Principal, error) {
	return r.principal, nil
}
func (r *fakeRepository) DeleteSession(context.Context, string, string) error { return nil }
func (r *fakeRepository) ListWorkspaces(context.Context, string) ([]Workspace, error) {
	return r.workspaces, nil
}

type fakeHasher struct {
	verifyCalls int
}

func (*fakeHasher) Hash(password string) (string, error) { return "hash:" + password, nil }
func (h *fakeHasher) Verify(password, encoded string) (bool, error) {
	h.verifyCalls++
	return encoded == "hash:"+password, nil
}

func TestRegisterCreatesCompleteInitialWorkspace(t *testing.T) {
	repository := &fakeRepository{}
	hasher := &fakeHasher{}
	service, err := NewService(repository, hasher, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	result, err := service.Register(context.Background(), RegisterInput{
		Email: " Person@Example.com ", Password: "a sufficiently long password",
		DisplayName: " Person ", WorkspaceName: " Home ", BaseCurrency: "usd",
		Timezone: "Europe/Istanbul", Transport: TransportCookie,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if repository.registration.Email != "person@example.com" || repository.registration.BaseCurrency != "USD" {
		t.Fatalf("registration was not normalized: %+v", repository.registration)
	}
	if repository.registration.Session.ExpiresAt != now.Add(30*24*time.Hour) {
		t.Fatalf("session expiry = %v", repository.registration.Session.ExpiresAt)
	}
	if result.Token == "" || string(repository.registration.Session.TokenHash) == result.Token {
		t.Fatal("raw token was empty or persisted directly")
	}
	wantHash := sha256.Sum256([]byte(result.Token))
	if string(repository.registration.Session.TokenHash) != string(wantHash[:]) {
		t.Fatal("persisted session token hash does not match the returned token")
	}
	for name, value := range map[string]string{
		"user": repository.registration.UserID, "workspace": repository.registration.WorkspaceID,
		"expense category": repository.registration.ExpenseCategoryID,
		"income category":  repository.registration.IncomeCategoryID,
		"session":          repository.registration.Session.ID,
	} {
		id, err := uuid.Parse(value)
		if err != nil || id.Version() != 7 {
			t.Errorf("%s ID = %q, want UUIDv7", name, value)
		}
	}
	if len(result.Workspaces) != 1 || result.Workspaces[0].Role != "owner" {
		t.Fatalf("workspaces = %+v, want initial owner workspace", result.Workspaces)
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	service, err := NewService(&fakeRepository{}, &fakeHasher{}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Register(context.Background(), RegisterInput{
		Email: "person@example.com", Password: "too short", DisplayName: "Person",
		WorkspaceName: "Home", BaseCurrency: "USD", Timezone: "UTC", Transport: TransportCookie,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Register() error = %v, want ErrInvalidInput", err)
	}
}

func TestLoginUsesDummyHashForUnknownEmail(t *testing.T) {
	hasher := &fakeHasher{}
	service, err := NewService(
		&fakeRepository{userError: ErrInvalidCredentials}, hasher, time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Login(context.Background(), LoginInput{
		Email: "missing@example.com", Password: "any sufficiently long password", Transport: TransportBearer,
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if hasher.verifyCalls != 1 {
		t.Fatalf("Verify() calls = %d, want dummy verification", hasher.verifyCalls)
	}
}

func TestAuthenticateRejectsTransportMismatch(t *testing.T) {
	repository := &fakeRepository{principal: Principal{Transport: TransportCookie}}
	service, err := NewService(repository, &fakeHasher{}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Authenticate(context.Background(), "token", TransportBearer)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Authenticate() error = %v, want ErrUnauthorized", err)
	}
}
