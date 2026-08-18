package httpapi

import (
	"context"

	"github.com/nihatatay93/budget/internal/account"
	"github.com/nihatatay93/budget/internal/budget"
	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/exchange"
	"github.com/nihatatay93/budget/internal/reporting"
	"github.com/nihatatay93/budget/internal/transaction"
	"github.com/nihatatay93/budget/internal/workspace"
)

type accountService interface {
	List(context.Context, string, string, bool) ([]account.Account, error)
	Get(context.Context, string, string, string) (account.Account, error)
	Create(context.Context, string, string, account.WriteInput) (account.Account, error)
	Update(context.Context, string, string, string, account.WriteInput) (account.Account, error)
	Archive(context.Context, string, string, string) error
}

// exchangeRateService is nil when the operator has not enabled rate fetching.
type exchangeRateService interface {
	Rates(context.Context, string, string) ([]exchange.Rate, error)
}

type categoryService interface {
	List(context.Context, string, string, bool) ([]category.Category, error)
	Get(context.Context, string, string, string) (category.Category, error)
	Create(context.Context, string, string, category.WriteInput) (category.Category, error)
	Update(context.Context, string, string, string, category.WriteInput) (category.Category, error)
	Archive(context.Context, string, string, string) error
}

type transactionService interface {
	List(context.Context, string, string, transaction.ListFilter) ([]transaction.Transaction, error)
	Get(context.Context, string, string, string) (transaction.Transaction, error)
	Create(context.Context, string, string, transaction.WriteInput) (transaction.Transaction, error)
	Update(context.Context, string, string, string, transaction.WriteInput) (transaction.Transaction, error)
	SoftDelete(context.Context, string, string, string) error
}

type reportingService interface {
	Project(context.Context, string, string, reporting.Query) (reporting.Projection, error)
}

type budgetService interface {
	Get(context.Context, string, string, *string) (budget.Budget, error)
	Replace(context.Context, string, string, string, budget.WriteInput) (budget.Budget, error)
}

type collaborationService interface {
	ListMembers(context.Context, string, string) ([]workspace.Member, error)
	ListInvitations(context.Context, string, string) ([]workspace.Invitation, error)
	CreateInvitation(
		context.Context, string, string, workspace.InvitationInput,
	) (workspace.InvitationCredential, error)
	RevokeInvitation(context.Context, string, string, string) error
	UpdateMemberRole(context.Context, string, string, string, workspace.Role) (workspace.Member, error)
	RemoveMember(context.Context, string, string, string) error
	AcceptInvitation(context.Context, string, string) (workspace.Acceptance, error)
}
