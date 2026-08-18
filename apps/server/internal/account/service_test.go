package account

import (
	"context"
	"errors"
	"testing"

	"github.com/nihatatay93/budget/internal/workspace"
)

const (
	testWorkspaceID = "018f47da-0af1-7a2f-8c35-165c89772d5b"
	testUserID      = "018f47da-0af1-7a2f-8c35-165c89772d5c"
	testAccountID   = "018f47da-0af1-7a2f-8c35-165c89772d5d"
)

type accountMembershipStub struct{ role workspace.Role }

func (s accountMembershipStub) MemberRole(context.Context, string, string) (workspace.Role, error) {
	return s.role, nil
}

type accountRepositoryStub struct {
	created Account
	updated WriteInput
	err     error
}

func (s *accountRepositoryStub) List(context.Context, string, bool) ([]Account, error) {
	return nil, s.err
}
func (s *accountRepositoryStub) Get(context.Context, string, string) (Account, error) {
	return Account{ID: testAccountID, WorkspaceID: testWorkspaceID}, s.err
}
func (s *accountRepositoryStub) Create(_ context.Context, value Account) (Account, error) {
	s.created = value
	return value, s.err
}
func (s *accountRepositoryStub) Update(_ context.Context, _, _ string, input WriteInput) (Account, error) {
	s.updated = input
	return Account{ID: testAccountID, WorkspaceID: testWorkspaceID}, s.err
}
func (s *accountRepositoryStub) Archive(context.Context, string, string) error { return s.err }

func TestCreateNormalizesAccountInput(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := NewService(
		repository,
		workspace.NewAuthorizer(accountMembershipStub{role: workspace.RoleMember}),
	)
	institution := "  Bank of Budget  "
	created, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Name: "  Daily checking  ", Type: TypeBank, Currency: "try", InstitutionName: &institution,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Name != "Daily checking" || created.Currency != "TRY" ||
		created.InstitutionName == nil || *created.InstitutionName != "Bank of Budget" {
		t.Fatalf("Create() = %+v", created)
	}
}

func TestViewerCannotCreateAccount(t *testing.T) {
	service := NewService(
		&accountRepositoryStub{},
		workspace.NewAuthorizer(accountMembershipStub{role: workspace.RoleViewer}),
	)
	_, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Name: "Cash", Type: TypeCash, Currency: "TRY",
	})
	if !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("Create() error = %v, want ErrForbidden", err)
	}
}

func TestUpdatePreservesCurrencyLockError(t *testing.T) {
	repository := &accountRepositoryStub{err: ErrCurrencyLocked}
	service := NewService(
		repository,
		workspace.NewAuthorizer(accountMembershipStub{role: workspace.RoleOwner}),
	)
	_, err := service.Update(context.Background(), testWorkspaceID, testUserID, testAccountID, WriteInput{
		Name: "Cash", Type: TypeCash, Currency: "USD",
	})
	if !errors.Is(err, ErrCurrencyLocked) {
		t.Fatalf("Update() error = %v, want ErrCurrencyLocked", err)
	}
}

// Every read and write path must consult membership before touching the repository.
func newAccountService(repository Repository) *Service {
	return NewService(
		repository,
		workspace.NewAuthorizer(accountMembershipStub{role: workspace.RoleMember}),
	)
}

func TestServiceDeniesViewerWritesAndNonMembersEntirely(t *testing.T) {
	ctx := context.Background()
	input := WriteInput{Name: "Checking", Type: TypeBank, Currency: "TRY"}

	viewer := NewService(
		&accountRepositoryStub{}, workspace.NewAuthorizer(accountMembershipStub{role: workspace.RoleViewer}),
	)
	if err := viewer.Archive(ctx, testWorkspaceID, testUserID, testAccountID); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("viewer Archive() error = %v, want ErrForbidden", err)
	}
	if _, err := viewer.Update(ctx, testWorkspaceID, testUserID, testAccountID, input); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("viewer Update() error = %v, want ErrForbidden", err)
	}
	if _, err := viewer.List(ctx, testWorkspaceID, testUserID, false); err != nil {
		t.Fatalf("viewer List() error = %v, want success", err)
	}

	stranger := NewService(
		&accountRepositoryStub{}, workspace.NewAuthorizer(accountMembershipStub{role: ""}),
	)
	if _, err := stranger.List(ctx, testWorkspaceID, testUserID, false); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("non-member List() error = %v, want ErrForbidden", err)
	}
	if _, err := stranger.Get(ctx, testWorkspaceID, testUserID, testAccountID); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("non-member Get() error = %v, want ErrForbidden", err)
	}
	if err := stranger.Archive(ctx, testWorkspaceID, testUserID, testAccountID); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("non-member Archive() error = %v, want ErrForbidden", err)
	}
}

// Identifiers are rejected before authorization so a malformed request cannot probe
// membership or reach the database.
func TestServiceRejectsMalformedIdentifiers(t *testing.T) {
	ctx := context.Background()
	service := newAccountService(&accountRepositoryStub{})
	input := WriteInput{Name: "Checking", Type: TypeBank, Currency: "TRY"}

	for _, id := range []string{"", "not-a-uuid", testAccountID + "x"} {
		if _, err := service.List(ctx, id, testUserID, false); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("List(%q) error = %v, want ErrInvalidInput", id, err)
		}
		if _, err := service.Get(ctx, testWorkspaceID, testUserID, id); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Get(%q) error = %v, want ErrInvalidInput", id, err)
		}
		if _, err := service.Create(ctx, id, testUserID, input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Create(%q) error = %v, want ErrInvalidInput", id, err)
		}
		if err := service.Archive(ctx, testWorkspaceID, testUserID, id); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Archive(%q) error = %v, want ErrInvalidInput", id, err)
		}
	}
}

func TestNormalizeWriteInputRejectsInvalidValues(t *testing.T) {
	longName := ""
	for range 101 {
		longName += "a"
	}
	blank := "   "
	tests := map[string]WriteInput{
		"empty name":       {Name: "  ", Type: TypeBank, Currency: "TRY"},
		"name too long":    {Name: longName, Type: TypeBank, Currency: "TRY"},
		"unknown type":     {Name: "Checking", Type: Type("crypto"), Currency: "TRY"},
		"empty type":       {Name: "Checking", Type: "", Currency: "TRY"},
		"unsupported code": {Name: "Checking", Type: TypeBank, Currency: "GBP"},
		"invalid code":     {Name: "Checking", Type: TypeBank, Currency: "XYZ"},
		"blank currency":   {Name: "Checking", Type: TypeBank, Currency: ""},
		"blank institution": {
			Name: "Checking", Type: TypeBank, Currency: "TRY", InstitutionName: &blank,
		},
		"institution too long": {
			Name: "Checking", Type: TypeBank, Currency: "TRY", InstitutionName: &longName,
		},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeWriteInput(input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("normalizeWriteInput() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestNormalizeWriteInputTrimsAndUppercases(t *testing.T) {
	institution := "  Garanti  "
	got, err := normalizeWriteInput(WriteInput{
		Name: "  Checking  ", Type: TypeBank, Currency: " try ", InstitutionName: &institution,
	})
	if err != nil {
		t.Fatalf("normalizeWriteInput() error = %v", err)
	}
	if got.Name != "Checking" || got.Currency != "TRY" || *got.InstitutionName != "Garanti" {
		t.Fatalf("normalizeWriteInput() = %#v", got)
	}
}

func TestTypeValidCoversEveryDeclaredType(t *testing.T) {
	for _, accountType := range []Type{
		TypeBank, TypeCash, TypeCreditCard, TypeSavings, TypeInvestment, TypeOther,
	} {
		if !accountType.Valid() {
			t.Fatalf("Valid(%q) = false", accountType)
		}
	}
	for _, accountType := range []Type{"", "crypto", "Bank"} {
		if accountType.Valid() {
			t.Fatalf("Valid(%q) = true", accountType)
		}
	}
}
