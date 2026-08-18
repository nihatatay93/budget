package httpapi

import (
	"testing"

	"github.com/nihatatay93/budget/internal/account"
	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

// The domain and the contract keep separate vocabularies so neither drives the other. That
// separation only holds while every value one accepts is a value the other names: a currency
// the domain supports but the contract omits is unreachable, and one the contract names but
// the domain rejects is a 400 the client cannot anticipate.
//
// Each check compares whole sets rather than sampling, so adding a member on one side
// without the other fails here rather than in a client.

func TestContractCurrenciesMatchTheDomain(t *testing.T) {
	contract := []openapi.Currency{openapi.TRY, openapi.USD, openapi.EUR}
	assertSameMembers(t, "currency",
		toStrings(money.Supported(), func(c money.Currency) string { return c.String() }),
		toStrings(contract, func(c openapi.Currency) string { return string(c) }),
	)
	// Every contract value must also survive domain parsing.
	for _, value := range contract {
		if _, ok := money.Parse(string(value)); !ok {
			t.Fatalf("contract currency %q is rejected by the domain", value)
		}
	}
}

func TestContractWorkspaceRolesMatchTheDomain(t *testing.T) {
	domain := []workspace.Role{
		workspace.RoleOwner, workspace.RoleAdmin, workspace.RoleMember, workspace.RoleViewer,
	}
	contract := []openapi.WorkspaceRole{
		openapi.WorkspaceRoleOwner, openapi.WorkspaceRoleAdmin,
		openapi.WorkspaceRoleMember, openapi.WorkspaceRoleViewer,
	}
	assertSameMembers(t, "workspace role",
		toStrings(domain, func(r workspace.Role) string { return string(r) }),
		toStrings(contract, func(r openapi.WorkspaceRole) string { return string(r) }),
	)
	for _, value := range domain {
		if !value.Valid() {
			t.Fatalf("domain role %q fails its own validation", value)
		}
	}
}

// Invitation roles are deliberately a subset: an invitation never confers ownership.
func TestContractInvitationRolesExcludeOwner(t *testing.T) {
	contract := []openapi.WorkspaceInvitationRole{
		openapi.WorkspaceInvitationRoleAdmin,
		openapi.WorkspaceInvitationRoleMember,
		openapi.WorkspaceInvitationRoleViewer,
	}
	for _, value := range contract {
		if workspace.Role(value) == workspace.RoleOwner {
			t.Fatal("the contract offers an owner invitation")
		}
		if !workspace.Role(value).Valid() {
			t.Fatalf("invitation role %q is not a domain role", value)
		}
		// No actor may invite an owner, so the set the contract exposes is the set the
		// policy can actually grant.
		if !workspace.CanInvite(workspace.RoleOwner, workspace.Role(value)) {
			t.Fatalf("an owner cannot grant contract role %q", value)
		}
	}
	if workspace.CanInvite(workspace.RoleOwner, workspace.RoleOwner) {
		t.Fatal("policy allows inviting an owner, which the contract cannot express")
	}
}

func TestContractAccountTypesMatchTheDomain(t *testing.T) {
	domain := []account.Type{
		account.TypeBank, account.TypeCash, account.TypeCreditCard,
		account.TypeSavings, account.TypeInvestment, account.TypeOther,
	}
	contract := []openapi.AccountType{
		openapi.Bank, openapi.Cash, openapi.CreditCard,
		openapi.Savings, openapi.Investment, openapi.Other,
	}
	assertSameMembers(t, "account type",
		toStrings(domain, func(t account.Type) string { return string(t) }),
		toStrings(contract, func(t openapi.AccountType) string { return string(t) }),
	)
	for _, value := range contract {
		if !account.Type(value).Valid() {
			t.Fatalf("contract account type %q is rejected by the domain", value)
		}
	}
}

func TestContractCategoryKindsMatchTheDomain(t *testing.T) {
	contract := []openapi.CategoryKind{openapi.Expense, openapi.Income}
	assertSameMembers(t, "category kind",
		[]string{string(category.KindExpense), string(category.KindIncome)},
		toStrings(contract, func(k openapi.CategoryKind) string { return string(k) }),
	)
	for _, value := range contract {
		if !category.Kind(value).Valid() {
			t.Fatalf("contract category kind %q is rejected by the domain", value)
		}
	}
}

func TestContractTransactionKindsAndStatusesMatchTheDomain(t *testing.T) {
	kinds := []openapi.TransactionKind{
		openapi.Standard, openapi.Transfer, openapi.Adjustment,
	}
	assertSameMembers(t, "transaction kind",
		[]string{
			string(transaction.KindStandard), string(transaction.KindTransfer),
			string(transaction.KindAdjustment),
		},
		toStrings(kinds, func(k openapi.TransactionKind) string { return string(k) }),
	)
	for _, value := range kinds {
		if !transaction.Kind(value).Valid() {
			t.Fatalf("contract transaction kind %q is rejected by the domain", value)
		}
	}

	statuses := []openapi.TransactionStatus{openapi.Pending, openapi.Posted}
	assertSameMembers(t, "transaction status",
		[]string{string(transaction.StatusPending), string(transaction.StatusPosted)},
		toStrings(statuses, func(s openapi.TransactionStatus) string { return string(s) }),
	)
	for _, value := range statuses {
		if !transaction.Status(value).Valid() {
			t.Fatalf("contract transaction status %q is rejected by the domain", value)
		}
	}
}

func toStrings[T any](values []T, render func(T) string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, render(value))
	}
	return result
}

func assertSameMembers(t *testing.T, subject string, domain, contract []string) {
	t.Helper()
	inDomain := map[string]bool{}
	for _, value := range domain {
		inDomain[value] = true
	}
	inContract := map[string]bool{}
	for _, value := range contract {
		inContract[value] = true
	}
	for value := range inDomain {
		if !inContract[value] {
			t.Fatalf("%s %q exists in the domain but not the contract, so it is unreachable",
				subject, value)
		}
	}
	for value := range inContract {
		if !inDomain[value] {
			t.Fatalf("%s %q exists in the contract but not the domain, so clients can send it "+
				"and be rejected", subject, value)
		}
	}
}
