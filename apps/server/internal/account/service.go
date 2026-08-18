package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

var (
	ErrInvalidInput   = errors.New("invalid account input")
	ErrNotFound       = errors.New("account not found")
	ErrCurrencyLocked = errors.New("account currency is locked by financial history")
)

type Type string

const (
	TypeBank       Type = "bank"
	TypeCash       Type = "cash"
	TypeCreditCard Type = "credit_card"
	TypeSavings    Type = "savings"
	TypeInvestment Type = "investment"
	TypeOther      Type = "other"
)

func (t Type) Valid() bool {
	switch t {
	case TypeBank, TypeCash, TypeCreditCard, TypeSavings, TypeInvestment, TypeOther:
		return true
	default:
		return false
	}
}

type Account struct {
	ID              string
	WorkspaceID     string
	Name            string
	Type            Type
	Currency        string
	InstitutionName *string
	ArchivedAt      *time.Time
	BalanceMinor    int64
}

type WriteInput struct {
	Name            string
	Type            Type
	Currency        string
	InstitutionName *string
}

type Repository interface {
	List(context.Context, string, bool) ([]Account, error)
	Get(context.Context, string, string) (Account, error)
	Create(context.Context, Account) (Account, error)
	Update(context.Context, string, string, WriteInput) (Account, error)
	Archive(context.Context, string, string) error
}

type Service struct {
	repository Repository
	access     *workspace.Authorizer
}

func NewService(repository Repository, access *workspace.Authorizer) *Service {
	return &Service{repository: repository, access: access}
}

func (s *Service) List(
	ctx context.Context,
	workspaceID, userID string,
	includeArchived bool,
) ([]Account, error) {
	if !validID(workspaceID) || !validID(userID) {
		return nil, ErrInvalidInput
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, workspaceID, includeArchived)
}

func (s *Service) Get(ctx context.Context, workspaceID, userID, accountID string) (Account, error) {
	if !validID(workspaceID) || !validID(userID) || !validID(accountID) {
		return Account{}, ErrInvalidInput
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return Account{}, err
	}
	return s.repository.Get(ctx, workspaceID, accountID)
}

func (s *Service) Create(
	ctx context.Context,
	workspaceID, userID string,
	input WriteInput,
) (Account, error) {
	input, err := normalizeWriteInput(input)
	if err != nil || !validID(workspaceID) || !validID(userID) {
		return Account{}, ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return Account{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Account{}, fmt.Errorf("create account ID: %w", err)
	}
	return s.repository.Create(ctx, Account{
		ID: id.String(), WorkspaceID: workspaceID, Name: input.Name, Type: input.Type,
		Currency: input.Currency, InstitutionName: input.InstitutionName,
	})
}

func (s *Service) Update(
	ctx context.Context,
	workspaceID, userID, accountID string,
	input WriteInput,
) (Account, error) {
	input, err := normalizeWriteInput(input)
	if err != nil || !validID(workspaceID) || !validID(userID) || !validID(accountID) {
		return Account{}, ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return Account{}, err
	}
	return s.repository.Update(ctx, workspaceID, accountID, input)
}

func (s *Service) Archive(ctx context.Context, workspaceID, userID, accountID string) error {
	if !validID(workspaceID) || !validID(userID) || !validID(accountID) {
		return ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repository.Archive(ctx, workspaceID, accountID)
}

func normalizeWriteInput(input WriteInput) (WriteInput, error) {
	currency, ok := money.Parse(input.Currency)
	if !ok {
		return WriteInput{}, ErrInvalidInput
	}
	input.Currency = currency.String()
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 || !input.Type.Valid() {
		return WriteInput{}, ErrInvalidInput
	}
	if input.InstitutionName != nil {
		value := strings.TrimSpace(*input.InstitutionName)
		if value == "" || utf8.RuneCountInString(value) > 100 {
			return WriteInput{}, ErrInvalidInput
		}
		input.InstitutionName = &value
	}
	return input, nil
}

func validID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
