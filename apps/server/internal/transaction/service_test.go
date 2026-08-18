package transaction

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

const (
	testWorkspaceID   = "018f47da-0af1-7a2f-8c35-165c89772d5b"
	testUserID        = "018f47da-0af1-7a2f-8c35-165c89772d5c"
	testTransactionID = "018f47da-0af1-7a2f-8c35-165c89772d5d"
	testAccountID     = "018f47da-0af1-7a2f-8c35-165c89772d5e"
	testOtherAccount  = "018f47da-0af1-7a2f-8c35-165c89772d5f"
	testCategoryID    = "018f47da-0af1-7a2f-8c35-165c89772d60"
	testSystemID      = "018f47da-0af1-7a2f-8c35-165c89772d61"
)

var testDate = time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)

type transactionMembershipStub struct{ role workspace.Role }

func (s transactionMembershipStub) MemberRole(context.Context, string, string) (workspace.Role, error) {
	return s.role, nil
}

type transactionRepositoryStub struct {
	accountCurrencies map[string]money.Currency
	categoryExists    bool
	created           Transaction
	updated           Transaction
	current           Transaction
	deleted           bool
}

func (*transactionRepositoryStub) List(context.Context, string, ListFilter) ([]Transaction, error) {
	return nil, nil
}
func (s *transactionRepositoryStub) Get(context.Context, string, string) (Transaction, error) {
	if s.current.ID == "" {
		return Transaction{}, ErrNotFound
	}
	return s.current, nil
}
func (s *transactionRepositoryStub) Create(_ context.Context, value Transaction) (Transaction, error) {
	s.created = value
	return value, nil
}
func (s *transactionRepositoryStub) Update(_ context.Context, value Transaction) (Transaction, error) {
	s.updated = value
	return value, nil
}
func (s *transactionRepositoryStub) SoftDelete(context.Context, string, string, string) error {
	s.deleted = true
	return nil
}
func (*transactionRepositoryStub) WorkspaceBaseCurrency(context.Context, string) (money.Currency, error) {
	return money.TRY, nil
}
func (s *transactionRepositoryStub) AccountCurrency(_ context.Context, _, id string) (money.Currency, error) {
	value, ok := s.accountCurrencies[id]
	if !ok {
		return "", ErrReferenceInvalid
	}
	return value, nil
}
func (s *transactionRepositoryStub) CategoryExists(context.Context, string, string) (bool, error) {
	return s.categoryExists, nil
}
func (*transactionRepositoryStub) SystemCategoryID(context.Context, string, string) (string, error) {
	return testSystemID, nil
}

type bookingStub struct{ calls int }

func (s *bookingStub) BaseAmount(
	_ context.Context, _ time.Time, from, to money.Currency, amount int64,
) (int64, error) {
	s.calls++
	rate, _ := new(big.Rat).SetString("50")
	if from != money.USD || to != money.TRY {
		return 0, errors.New("unexpected pair")
	}
	return money.Convert(amount, rate)
}

func newTransactionService(repository Repository, booking BookingRateResolver) *Service {
	return NewService(
		repository,
		workspace.NewAuthorizer(transactionMembershipStub{role: workspace.RoleOwner}),
		booking,
	)
}

func TestCreateStandardBooksSameCurrencyAndAddsUncategorizedAllocation(t *testing.T) {
	repository := &transactionRepositoryStub{accountCurrencies: map[string]money.Currency{
		testAccountID: money.TRY,
	}}
	service := newTransactionService(repository, nil)
	created, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Kind: KindStandard, Status: StatusPosted, TransactionDate: testDate,
		Entries: []EntryInput{{AccountID: testAccountID, AmountMinor: -35000}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Entries) != 1 || created.Entries[0].BaseAmountMinor != -35000 {
		t.Fatalf("entries = %+v", created.Entries)
	}
	if len(created.Allocations) != 1 || created.Allocations[0].CategoryID != testSystemID ||
		created.Allocations[0].AmountBaseMinor != -35000 {
		t.Fatalf("allocations = %+v", created.Allocations)
	}
}

func TestCreateSplitAndTransferReconciliation(t *testing.T) {
	repository := &transactionRepositoryStub{
		accountCurrencies: map[string]money.Currency{
			testAccountID: money.TRY, testOtherAccount: money.TRY,
		},
		categoryExists: true,
	}
	service := newTransactionService(repository, nil)
	if _, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Kind: KindStandard, Status: StatusPending, TransactionDate: testDate,
		Entries: []EntryInput{{AccountID: testAccountID, AmountMinor: -150000}},
		Allocations: []AllocationInput{
			{CategoryID: testCategoryID, AmountBaseMinor: -100000},
			{CategoryID: testCategoryID, AmountBaseMinor: -50000},
		},
	}); err != nil {
		t.Fatalf("Create(split) error = %v", err)
	}
	if _, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Kind: KindTransfer, Status: StatusPosted, TransactionDate: testDate,
		Entries: []EntryInput{
			{AccountID: testAccountID, AmountMinor: -100000},
			{AccountID: testOtherAccount, AmountMinor: 100000},
		},
	}); err != nil {
		t.Fatalf("Create(transfer) error = %v", err)
	}
}

func TestCreateForeignEntryUsesExplicitOrHistoricalBaseAmount(t *testing.T) {
	repository := &transactionRepositoryStub{accountCurrencies: map[string]money.Currency{
		testAccountID: money.USD,
	}}
	booking := &bookingStub{}
	service := newTransactionService(repository, booking)
	created, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Kind: KindAdjustment, Status: StatusPosted, TransactionDate: testDate,
		Entries: []EntryInput{{AccountID: testAccountID, AmountMinor: 100}},
	})
	if err != nil || created.Entries[0].BaseAmountMinor != 5000 || booking.calls != 1 {
		t.Fatalf("automatic booking = %+v, calls = %d, err = %v", created.Entries, booking.calls, err)
	}
	explicit := int64(4900)
	created, err = service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Kind: KindAdjustment, Status: StatusPosted, TransactionDate: testDate,
		Entries: []EntryInput{{AccountID: testAccountID, AmountMinor: 100, BaseAmountMinor: &explicit}},
	})
	if err != nil || created.Entries[0].BaseAmountMinor != explicit || booking.calls != 1 {
		t.Fatalf("explicit booking = %+v, calls = %d, err = %v", created.Entries, booking.calls, err)
	}
}

func TestCreateRejectsForeignBaseAmountWithOppositeSign(t *testing.T) {
	repository := &transactionRepositoryStub{accountCurrencies: map[string]money.Currency{
		testAccountID: money.USD,
	}}
	explicit := int64(-4900)
	_, err := newTransactionService(repository, nil).Create(
		context.Background(), testWorkspaceID, testUserID,
		WriteInput{
			Kind: KindAdjustment, Status: StatusPosted, TransactionDate: testDate,
			Entries: []EntryInput{{
				AccountID: testAccountID, AmountMinor: 100, BaseAmountMinor: &explicit,
			}},
		},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
	}
}

func TestCreateRejectsUnavailableBookingRateAndInvalidAggregate(t *testing.T) {
	repository := &transactionRepositoryStub{accountCurrencies: map[string]money.Currency{
		testAccountID: money.USD,
	}}
	service := newTransactionService(repository, nil)
	_, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Kind: KindAdjustment, Status: StatusPosted, TransactionDate: testDate,
		Entries: []EntryInput{{AccountID: testAccountID, AmountMinor: 100}},
	})
	if !errors.Is(err, ErrBookingRateUnavailable) {
		t.Fatalf("Create() error = %v, want ErrBookingRateUnavailable", err)
	}
}

func TestUpdatePreservesSourceAndSoftDeleteUsesManagementAccess(t *testing.T) {
	repository := &transactionRepositoryStub{
		accountCurrencies: map[string]money.Currency{testAccountID: money.TRY},
		current: Transaction{
			ID: testTransactionID, WorkspaceID: testWorkspaceID,
			Source: SourceImport, CreatedBy: testUserID,
		},
	}
	service := newTransactionService(repository, nil)
	updated, err := service.Update(
		context.Background(), testWorkspaceID, testUserID, testTransactionID,
		WriteInput{
			Kind: KindAdjustment, Status: StatusPosted, TransactionDate: testDate,
			Entries: []EntryInput{{AccountID: testAccountID, AmountMinor: 100}},
		},
	)
	if err != nil || updated.Source != SourceImport || updated.ID != testTransactionID {
		t.Fatalf("Update() = %+v, err = %v", updated, err)
	}
	if err := service.SoftDelete(context.Background(), testWorkspaceID, testUserID, testTransactionID); err != nil || !repository.deleted {
		t.Fatalf("SoftDelete() error = %v, deleted = %v", err, repository.deleted)
	}
}
