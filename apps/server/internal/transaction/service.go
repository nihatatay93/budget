package transaction

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
	ErrInvalidInput           = errors.New("invalid transaction input")
	ErrNotFound               = errors.New("transaction not found")
	ErrReferenceInvalid       = errors.New("transaction references an unavailable resource")
	ErrBookingRateUnavailable = errors.New("historical booking rate is unavailable")
)

type Status string

const (
	StatusPending Status = "pending"
	StatusPosted  Status = "posted"
)

func (s Status) Valid() bool { return s == StatusPending || s == StatusPosted }

type Source string

const (
	SourceManual    Source = "manual"
	SourceRecurring Source = "recurring"
	SourceImport    Source = "import"
	SourceAPI       Source = "api"
)

type Entry struct {
	ID              string
	AccountID       string
	AmountMinor     int64
	BaseAmountMinor int64
}

type Allocation struct {
	ID              string
	CategoryID      string
	AmountBaseMinor int64
}

type Transaction struct {
	ID              string
	WorkspaceID     string
	Kind            Kind
	Status          Status
	TransactionDate time.Time
	Payee           *string
	Description     *string
	Notes           *string
	Source          Source
	CreatedBy       string
	UpdatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Entries         []Entry
	Allocations     []Allocation
}

type EntryInput struct {
	AccountID       string
	AmountMinor     int64
	BaseAmountMinor *int64
}

type AllocationInput struct {
	CategoryID string
	// AmountBaseMinor may be nil on a lone allocation, which then takes the transaction's
	// total entry base amount. See the allocation reconciliation rules in
	// docs/domain-model.md.
	AmountBaseMinor *int64
}

type WriteInput struct {
	Kind            Kind
	Status          Status
	TransactionDate time.Time
	Payee           *string
	Description     *string
	Notes           *string
	Entries         []EntryInput
	Allocations     []AllocationInput
}

type ListFilter struct {
	From  *time.Time
	To    *time.Time
	Limit int
}

type Repository interface {
	List(context.Context, string, ListFilter) ([]Transaction, error)
	Get(context.Context, string, string) (Transaction, error)
	Create(context.Context, Transaction) (Transaction, error)
	Update(context.Context, Transaction) (Transaction, error)
	SoftDelete(context.Context, string, string, string) error
	WorkspaceBaseCurrency(context.Context, string) (money.Currency, error)
	AccountCurrency(context.Context, string, string) (money.Currency, error)
	CategoryExists(context.Context, string, string) (bool, error)
	SystemCategoryID(context.Context, string, string) (string, error)
}

type BookingRateResolver interface {
	BaseAmount(
		context.Context, time.Time, money.Currency, money.Currency, int64,
	) (int64, error)
}

type Service struct {
	repository Repository
	access     *workspace.Authorizer
	booking    BookingRateResolver
}

func NewService(
	repository Repository,
	access *workspace.Authorizer,
	booking BookingRateResolver,
) *Service {
	return &Service{repository: repository, access: access, booking: booking}
}

func (s *Service) List(
	ctx context.Context,
	workspaceID, userID string,
	filter ListFilter,
) ([]Transaction, error) {
	if !validUUID(workspaceID) || !validUUID(userID) || !validFilter(filter) {
		return nil, ErrInvalidInput
	}
	if filter.Limit == 0 {
		filter.Limit = 100
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, workspaceID, filter)
}

func (s *Service) Get(
	ctx context.Context,
	workspaceID, userID, transactionID string,
) (Transaction, error) {
	if !validUUID(workspaceID) || !validUUID(userID) || !validUUID(transactionID) {
		return Transaction{}, ErrInvalidInput
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return Transaction{}, err
	}
	return s.repository.Get(ctx, workspaceID, transactionID)
}

func (s *Service) Create(
	ctx context.Context,
	workspaceID, userID string,
	input WriteInput,
) (Transaction, error) {
	if !validUUID(workspaceID) || !validUUID(userID) {
		return Transaction{}, ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return Transaction{}, err
	}
	prepared, err := s.prepare(ctx, workspaceID, userID, input)
	if err != nil {
		return Transaction{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Transaction{}, fmt.Errorf("create transaction ID: %w", err)
	}
	prepared.ID = id.String()
	prepared.Source = SourceManual
	prepared.CreatedBy = userID
	prepared.UpdatedBy = userID
	return s.repository.Create(ctx, prepared)
}

func (s *Service) Update(
	ctx context.Context,
	workspaceID, userID, transactionID string,
	input WriteInput,
) (Transaction, error) {
	if !validUUID(workspaceID) || !validUUID(userID) || !validUUID(transactionID) {
		return Transaction{}, ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return Transaction{}, err
	}
	current, err := s.repository.Get(ctx, workspaceID, transactionID)
	if err != nil {
		return Transaction{}, err
	}
	prepared, err := s.prepare(ctx, workspaceID, userID, input)
	if err != nil {
		return Transaction{}, err
	}
	prepared.ID = transactionID
	prepared.Source = current.Source
	prepared.CreatedBy = current.CreatedBy
	prepared.UpdatedBy = userID
	return s.repository.Update(ctx, prepared)
}

func (s *Service) SoftDelete(
	ctx context.Context,
	workspaceID, userID, transactionID string,
) error {
	if !validUUID(workspaceID) || !validUUID(userID) || !validUUID(transactionID) {
		return ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repository.SoftDelete(ctx, workspaceID, transactionID, userID)
}

func (s *Service) prepare(
	ctx context.Context,
	workspaceID, userID string,
	input WriteInput,
) (Transaction, error) {
	if !input.Kind.Valid() || !input.Status.Valid() || input.TransactionDate.IsZero() ||
		len(input.Entries) == 0 || len(input.Entries) > 50 || len(input.Allocations) > 100 {
		return Transaction{}, ErrInvalidInput
	}
	payee, ok := normalizeOptional(input.Payee, 200)
	if !ok {
		return Transaction{}, ErrInvalidInput
	}
	description, ok := normalizeOptional(input.Description, 500)
	if !ok {
		return Transaction{}, ErrInvalidInput
	}
	notes, ok := normalizeOptional(input.Notes, 4000)
	if !ok {
		return Transaction{}, ErrInvalidInput
	}
	baseCurrency, err := s.repository.WorkspaceBaseCurrency(ctx, workspaceID)
	if err != nil {
		return Transaction{}, err
	}
	result := Transaction{
		WorkspaceID: workspaceID, Kind: input.Kind, Status: input.Status,
		TransactionDate: input.TransactionDate, Payee: payee, Description: description, Notes: notes,
		CreatedBy: userID, UpdatedBy: userID,
	}
	entryAmounts := make([]int64, 0, len(input.Entries))
	for _, candidate := range input.Entries {
		if !validUUID(candidate.AccountID) || candidate.AmountMinor == 0 {
			return Transaction{}, ErrInvalidInput
		}
		currency, err := s.repository.AccountCurrency(ctx, workspaceID, candidate.AccountID)
		if err != nil {
			return Transaction{}, err
		}
		baseAmount, err := s.bookBaseAmount(
			ctx, input.TransactionDate, currency, baseCurrency,
			candidate.AmountMinor, candidate.BaseAmountMinor,
		)
		if err != nil {
			return Transaction{}, err
		}
		id, err := uuid.NewV7()
		if err != nil {
			return Transaction{}, fmt.Errorf("create transaction entry ID: %w", err)
		}
		result.Entries = append(result.Entries, Entry{
			ID: id.String(), AccountID: candidate.AccountID,
			AmountMinor: candidate.AmountMinor, BaseAmountMinor: baseAmount,
		})
		entryAmounts = append(entryAmounts, baseAmount)
	}
	if input.Kind == KindTransfer && len(result.Entries) < 2 {
		return Transaction{}, ErrInvalidInput
	}
	for _, candidate := range input.Allocations {
		if !validUUID(candidate.CategoryID) {
			return Transaction{}, ErrInvalidInput
		}
		amount, err := s.allocationAmount(candidate.AmountBaseMinor, entryAmounts, len(input.Allocations))
		if err != nil {
			return Transaction{}, err
		}
		exists, err := s.repository.CategoryExists(ctx, workspaceID, candidate.CategoryID)
		if err != nil {
			return Transaction{}, err
		}
		if !exists {
			return Transaction{}, ErrReferenceInvalid
		}
		allocation, err := newAllocation(candidate.CategoryID, amount)
		if err != nil {
			return Transaction{}, err
		}
		result.Allocations = append(result.Allocations, allocation)
	}
	if input.Kind == KindStandard && len(result.Allocations) == 0 {
		total, ok := sum(entryAmounts)
		if !ok || total == 0 {
			return Transaction{}, ErrDoesNotReconcile
		}
		key := "uncategorized_income"
		if total < 0 {
			key = "uncategorized_expense"
		}
		categoryID, err := s.repository.SystemCategoryID(ctx, workspaceID, key)
		if err != nil {
			return Transaction{}, err
		}
		allocation, err := newAllocation(categoryID, total)
		if err != nil {
			return Transaction{}, err
		}
		result.Allocations = append(result.Allocations, allocation)
	}
	allocationAmounts := make([]int64, 0, len(result.Allocations))
	for _, allocation := range result.Allocations {
		allocationAmounts = append(allocationAmounts, allocation.AmountBaseMinor)
	}
	if err := ValidateReconciliation(input.Kind, entryAmounts, allocationAmounts); err != nil {
		return Transaction{}, err
	}
	return result, nil
}

// allocationAmount resolves a stated amount, or derives the only allocation's amount from the
// entries. Choosing a category must not oblige a client to restate a figure the server has just
// computed — for a foreign-currency account, the value booked at the transaction date's rate,
// which the client has no way to know. A split states every amount, because dividing a
// transaction between categories is the client's decision and cannot be derived.
func (s *Service) allocationAmount(explicit *int64, entryAmounts []int64, allocations int) (int64, error) {
	if explicit != nil {
		if *explicit == 0 {
			return 0, ErrInvalidInput
		}
		return *explicit, nil
	}
	if allocations != 1 {
		return 0, ErrInvalidInput
	}
	total, ok := sum(entryAmounts)
	if !ok || total == 0 {
		return 0, ErrDoesNotReconcile
	}
	return total, nil
}

func (s *Service) bookBaseAmount(
	ctx context.Context,
	date time.Time,
	accountCurrency, baseCurrency money.Currency,
	amount int64,
	explicit *int64,
) (int64, error) {
	if accountCurrency == baseCurrency {
		if explicit != nil && *explicit != amount {
			return 0, ErrInvalidInput
		}
		return amount, nil
	}
	if explicit != nil {
		if (*explicit < 0 && amount > 0) || (*explicit > 0 && amount < 0) {
			return 0, ErrInvalidInput
		}
		return *explicit, nil
	}
	if s.booking == nil {
		return 0, ErrBookingRateUnavailable
	}
	value, err := s.booking.BaseAmount(ctx, date, accountCurrency, baseCurrency, amount)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrBookingRateUnavailable, err)
	}
	return value, nil
}

func newAllocation(categoryID string, amount int64) (Allocation, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Allocation{}, fmt.Errorf("create transaction allocation ID: %w", err)
	}
	return Allocation{ID: id.String(), CategoryID: categoryID, AmountBaseMinor: amount}, nil
}

func (k Kind) Valid() bool {
	return k == KindStandard || k == KindTransfer || k == KindAdjustment
}

func validFilter(filter ListFilter) bool {
	if filter.Limit < 0 || filter.Limit > 200 {
		return false
	}
	return filter.From == nil || filter.To == nil || !filter.From.After(*filter.To)
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func normalizeOptional(value *string, limit int) (*string, bool) {
	if value == nil {
		return nil, true
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || utf8.RuneCountInString(normalized) > limit {
		return nil, false
	}
	return &normalized, true
}
