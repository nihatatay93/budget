package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/nihatatay93/budget/internal/money"
)

const (
	TransportCookie Transport = "cookie"
	TransportBearer Transport = "bearer"
)

var (
	ErrInvalidInput       = errors.New("invalid authentication input")
	ErrEmailTaken         = errors.New("email address is already registered")
	ErrInvalidCredentials = errors.New("invalid email address or password")
	ErrUnauthorized       = errors.New("unauthorized")
)

type Transport string

func (t Transport) Valid() bool {
	return t == TransportCookie || t == TransportBearer
}

type User struct {
	ID          string
	Email       string
	DisplayName string
}

type Workspace struct {
	ID           string
	Name         string
	BaseCurrency string
	Timezone     string
	Role         string
}

type Principal struct {
	SessionID string
	User      User
	Transport Transport
}

type RegisterInput struct {
	Email         string
	Password      string
	DisplayName   string
	WorkspaceName string
	BaseCurrency  string
	Timezone      string
	Transport     Transport
}

type LoginInput struct {
	Email     string
	Password  string
	Transport Transport
}

type AuthResult struct {
	Token      string
	Principal  Principal
	Workspaces []Workspace
}

type Registration struct {
	UserID            string
	Email             string
	PasswordHash      string
	DisplayName       string
	WorkspaceID       string
	WorkspaceName     string
	BaseCurrency      string
	Timezone          string
	ExpenseCategoryID string
	IncomeCategoryID  string
	Session           Session
}

type StoredUser struct {
	User
	PasswordHash string
}

type Session struct {
	ID        string
	UserID    string
	TokenHash []byte
	Transport Transport
	ExpiresAt time.Time
}

type Repository interface {
	Register(context.Context, Registration) error
	UserByEmail(context.Context, string) (StoredUser, error)
	CreateSession(context.Context, Session) error
	SessionByTokenHash(context.Context, []byte) (Principal, error)
	DeleteSession(context.Context, string, string) error
	ListWorkspaces(context.Context, string) ([]Workspace, error)
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) (bool, error)
}

type Service struct {
	repository Repository
	passwords  PasswordHasher
	sessionTTL time.Duration
	now        func() time.Time
	dummyHash  string
}

func NewService(repository Repository, passwords PasswordHasher, sessionTTL time.Duration) (*Service, error) {
	dummyHash, err := passwords.Hash("not-a-real-user-password")
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	return &Service{
		repository: repository,
		passwords:  passwords,
		sessionTTL: sessionTTL,
		now:        time.Now,
		dummyHash:  dummyHash,
	}, nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	normalized, err := normalizeRegistration(input)
	if err != nil {
		return AuthResult{}, err
	}
	passwordHash, err := s.passwords.Hash(normalized.Password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}
	token, tokenHash, err := newToken()
	if err != nil {
		return AuthResult{}, fmt.Errorf("create session token: %w", err)
	}

	ids := make([]string, 5)
	for index := range ids {
		id, err := newID()
		if err != nil {
			return AuthResult{}, err
		}
		ids[index] = id
	}
	userID, workspaceID := ids[0], ids[1]
	session := Session{
		ID:        ids[2],
		UserID:    userID,
		TokenHash: tokenHash,
		Transport: normalized.Transport,
		ExpiresAt: s.now().Add(s.sessionTTL),
	}
	registration := Registration{
		UserID:            userID,
		Email:             normalized.Email,
		PasswordHash:      passwordHash,
		DisplayName:       normalized.DisplayName,
		WorkspaceID:       workspaceID,
		WorkspaceName:     normalized.WorkspaceName,
		BaseCurrency:      normalized.BaseCurrency,
		Timezone:          normalized.Timezone,
		ExpenseCategoryID: ids[3],
		IncomeCategoryID:  ids[4],
		Session:           session,
	}
	if err := s.repository.Register(ctx, registration); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token: token,
		Principal: Principal{
			SessionID: session.ID,
			User:      User{ID: userID, Email: normalized.Email, DisplayName: normalized.DisplayName},
			Transport: normalized.Transport,
		},
		Workspaces: []Workspace{{
			ID: workspaceID, Name: normalized.WorkspaceName, BaseCurrency: normalized.BaseCurrency,
			Timezone: normalized.Timezone, Role: "owner",
		}},
	}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil || !input.Transport.Valid() || !validPassword(input.Password) {
		return AuthResult{}, ErrInvalidCredentials
	}
	user, err := s.repository.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			_, _ = s.passwords.Verify(input.Password, s.dummyHash)
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}
	valid, err := s.passwords.Verify(input.Password, user.PasswordHash)
	if err != nil {
		return AuthResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !valid {
		return AuthResult{}, ErrInvalidCredentials
	}
	token, tokenHash, err := newToken()
	if err != nil {
		return AuthResult{}, fmt.Errorf("create session token: %w", err)
	}
	sessionID, err := newID()
	if err != nil {
		return AuthResult{}, err
	}
	session := Session{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: tokenHash,
		Transport: input.Transport,
		ExpiresAt: s.now().Add(s.sessionTTL),
	}
	workspaces, err := s.repository.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.repository.CreateSession(ctx, session); err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		Token:      token,
		Principal:  Principal{SessionID: session.ID, User: user.User, Transport: input.Transport},
		Workspaces: workspaces,
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string, transport Transport) (Principal, error) {
	if token == "" || !transport.Valid() {
		return Principal{}, ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(token))
	principal, err := s.repository.SessionByTokenHash(ctx, hash[:])
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return Principal{}, ErrUnauthorized
		}
		return Principal{}, err
	}
	if principal.Transport != transport {
		return Principal{}, ErrUnauthorized
	}
	return principal, nil
}

func (s *Service) Session(ctx context.Context, principal Principal) (AuthResult, error) {
	workspaces, err := s.repository.ListWorkspaces(ctx, principal.User.ID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Principal: principal, Workspaces: workspaces}, nil
}

func (s *Service) Logout(ctx context.Context, principal Principal) error {
	return s.repository.DeleteSession(ctx, principal.SessionID, principal.User.ID)
}

func normalizeRegistration(input RegisterInput) (RegisterInput, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil || !input.Transport.Valid() || !validPassword(input.Password) {
		return RegisterInput{}, ErrInvalidInput
	}
	displayName := strings.TrimSpace(input.DisplayName)
	workspaceName := strings.TrimSpace(input.WorkspaceName)
	currency, supportedCurrency := money.Parse(input.BaseCurrency)
	timezone := strings.TrimSpace(input.Timezone)
	if displayName == "" || utf8.RuneCountInString(displayName) > 100 ||
		workspaceName == "" || utf8.RuneCountInString(workspaceName) > 100 ||
		utf8.RuneCountInString(timezone) > 100 || !supportedCurrency {
		return RegisterInput{}, ErrInvalidInput
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return RegisterInput{}, ErrInvalidInput
	}
	input.Email = email
	input.DisplayName = displayName
	input.WorkspaceName = workspaceName
	input.BaseCurrency = currency.String()
	input.Timezone = timezone
	return input, nil
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized || len(normalized) > 254 {
		return "", ErrInvalidInput
	}
	return normalized, nil
}

func validPassword(password string) bool {
	length := utf8.RuneCountInString(password)
	return utf8.ValidString(password) && length >= 15 && length <= 128
}

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("create UUIDv7: %w", err)
	}
	return id.String(), nil
}
