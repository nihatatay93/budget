package budget

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

const (
	budgetWorkspaceID = "0198c307-75e7-7899-a5c0-4839cc802bd5"
	budgetUserID      = "0198c307-75e7-7899-a5c0-4839cc802bd6"
	budgetID          = "0198c307-75e7-7899-a5c0-4839cc802bd7"
	budgetItemID      = "0198c307-75e7-7899-a5c0-4839cc802bd8"
	budgetParentID    = "0198c307-75e7-7899-a5c0-4839cc802bd9"
	budgetChildID     = "0198c307-75e7-7899-a5c0-4839cc802bda"
	budgetRefundID    = "0198c307-75e7-7899-a5c0-4839cc802bdb"
)

type budgetMembershipStub struct {
	role workspace.Role
	err  error
}

func (s budgetMembershipStub) MemberRole(context.Context, string, string) (workspace.Role, error) {
	return s.role, s.err
}

type budgetRepositoryStub struct {
	settings       WorkspaceSettings
	rules          []CategoryRule
	snapshot       Snapshot
	err            error
	settingsCalled bool
	rulesCalled    bool
	replaceCalled  bool
	requestedMonth Month
	replaceCommand ReplaceCommand
}

func (s *budgetRepositoryStub) WorkspaceSettings(context.Context, string) (WorkspaceSettings, error) {
	s.settingsCalled = true
	return s.settings, s.err
}

func (s *budgetRepositoryStub) CategoriesForMonth(
	context.Context,
	string,
	Month,
) ([]CategoryRule, error) {
	s.rulesCalled = true
	return s.rules, s.err
}

func (s *budgetRepositoryStub) Get(_ context.Context, _ string, month Month) (Snapshot, error) {
	s.requestedMonth = month
	return s.snapshot, s.err
}

func (s *budgetRepositoryStub) Replace(
	_ context.Context,
	_ string,
	month Month,
	command ReplaceCommand,
) (Snapshot, error) {
	s.replaceCalled = true
	s.requestedMonth = month
	s.replaceCommand = command
	return s.snapshot, s.err
}

func TestGetDefaultsToWorkspaceLocalCurrentMonth(t *testing.T) {
	now := time.Date(2026, time.August, 31, 21, 30, 0, 0, time.UTC)
	repository := &budgetRepositoryStub{
		settings: WorkspaceSettings{Timezone: "Europe/Istanbul", BaseCurrency: money.TRY},
		snapshot: validBudgetSnapshot(Month{Year: 2026, Month: time.September}),
	}
	service := newBudgetService(repository, workspace.RoleViewer, func() time.Time { return now })

	result, err := service.Get(context.Background(), budgetWorkspaceID, budgetUserID, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !repository.settingsCalled || repository.requestedMonth.String() != "2026-09" {
		t.Fatalf("requested month = %s, settings called = %v", repository.requestedMonth, repository.settingsCalled)
	}
	if result.Month != "2026-09" || result.Timezone != "Europe/Istanbul" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGetNormalizesPurchaseAndRefundUsageWithoutClamping(t *testing.T) {
	month := Month{Year: 2026, Month: time.August}
	snapshot := validBudgetSnapshot(month)
	snapshot.Items = []ItemSnapshot{
		{
			ID: budgetItemID, CategoryID: budgetChildID, CategoryName: "Restaurants",
			PlannedBaseMinor: 10_000, SignedAllocationBaseMinor: -12_000,
		},
		{
			ID: budgetRefundID, CategoryID: budgetRefundID, CategoryName: "Shopping",
			PlannedBaseMinor: 5_000, SignedAllocationBaseMinor: 500,
		},
	}
	repository := &budgetRepositoryStub{snapshot: snapshot}
	service := newBudgetService(repository, workspace.RoleViewer, nil)
	requested := "2026-08"

	result, err := service.Get(context.Background(), budgetWorkspaceID, budgetUserID, &requested)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if repository.settingsCalled {
		t.Fatal("explicit month unexpectedly loaded workspace settings")
	}
	if result.Items[0].UsedBaseMinor != 12_000 || result.Items[0].RemainingBaseMinor != -2_000 {
		t.Fatalf("purchase item = %#v", result.Items[0])
	}
	if result.Items[1].UsedBaseMinor != -500 || result.Items[1].RemainingBaseMinor != 5_500 {
		t.Fatalf("refund item = %#v", result.Items[1])
	}
	if result.PlannedBaseMinor != 15_000 || result.UsedBaseMinor != 11_500 ||
		result.RemainingBaseMinor != 3_500 {
		t.Fatalf("totals = planned %d used %d remaining %d", result.PlannedBaseMinor, result.UsedBaseMinor, result.RemainingBaseMinor)
	}
}

func TestReplaceRejectsOverlappingCategoryBranches(t *testing.T) {
	repository := &budgetRepositoryStub{rules: []CategoryRule{
		{ID: budgetParentID, Kind: category.KindExpense},
		{ID: budgetChildID, ParentID: stringPointer(budgetParentID), Kind: category.KindExpense},
	}}
	service := newBudgetService(repository, workspace.RoleMember, nil)

	_, err := service.Replace(
		context.Background(), budgetWorkspaceID, budgetUserID, "2026-08",
		WriteInput{Name: "August", Items: []ItemInput{
			{CategoryID: budgetParentID, AmountBaseMinor: 10_000},
			{CategoryID: budgetChildID, AmountBaseMinor: 5_000},
		}},
	)
	if !errors.Is(err, ErrCategoryOverlap) || repository.replaceCalled {
		t.Fatalf("Replace() error = %v, replace called = %v", err, repository.replaceCalled)
	}
}

func TestReplaceAllowsAnAlreadyBudgetedArchivedCategory(t *testing.T) {
	archivedAt := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	month := Month{Year: 2026, Month: time.August}
	repository := &budgetRepositoryStub{
		rules: []CategoryRule{{
			ID: budgetChildID, Kind: category.KindExpense,
			ArchivedAt: &archivedAt, AlreadyBudgeted: true,
		}},
		snapshot: validBudgetSnapshot(month),
	}
	repository.snapshot.Items = []ItemSnapshot{{
		ID: budgetItemID, CategoryID: budgetChildID, CategoryName: "Restaurants",
		CategoryArchivedAt: &archivedAt, PlannedBaseMinor: 10_000,
	}}
	service := newBudgetService(repository, workspace.RoleMember, nil)

	result, err := service.Replace(
		context.Background(), budgetWorkspaceID, budgetUserID, "2026-08",
		WriteInput{Name: "  August plan  ", Items: []ItemInput{{
			CategoryID: budgetChildID, AmountBaseMinor: 10_000,
		}}},
	)
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if !repository.replaceCalled || repository.replaceCommand.Name != "August plan" ||
		!validID(repository.replaceCommand.NewBudgetID) ||
		!validID(repository.replaceCommand.Items[0].ID) {
		t.Fatalf("replace command = %#v", repository.replaceCommand)
	}
	if result.Items[0].CategoryArchivedAt == nil {
		t.Fatalf("result item = %#v", result.Items[0])
	}
}

func TestReplaceRejectsIneligibleCategories(t *testing.T) {
	archivedAt := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		rules []CategoryRule
		want  error
	}{
		{
			name:  "income",
			rules: []CategoryRule{{ID: budgetChildID, Kind: category.KindIncome}},
			want:  ErrCategoryKind,
		},
		{
			name: "new archived",
			rules: []CategoryRule{{
				ID: budgetChildID, Kind: category.KindExpense, ArchivedAt: &archivedAt,
			}},
			want: ErrCategoryArchived,
		},
		{name: "missing", want: ErrCategoryNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &budgetRepositoryStub{rules: test.rules}
			service := newBudgetService(repository, workspace.RoleMember, nil)
			_, err := service.Replace(
				context.Background(), budgetWorkspaceID, budgetUserID, "2026-08",
				WriteInput{Name: "August", Items: []ItemInput{{
					CategoryID: budgetChildID, AmountBaseMinor: 1,
				}}},
			)
			if !errors.Is(err, test.want) || repository.replaceCalled {
				t.Fatalf("Replace() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestReplaceRejectsDuplicateCategoryBeforeAuthorization(t *testing.T) {
	repository := &budgetRepositoryStub{}
	service := newBudgetService(repository, workspace.RoleMember, nil)
	_, err := service.Replace(
		context.Background(), budgetWorkspaceID, budgetUserID, "2026-08",
		WriteInput{Name: "August", Items: []ItemInput{
			{CategoryID: budgetChildID, AmountBaseMinor: 1},
			{CategoryID: budgetChildID, AmountBaseMinor: 2},
		}},
	)
	if !errors.Is(err, ErrCategoryDuplicate) || repository.rulesCalled {
		t.Fatalf("Replace() error = %v, rules called = %v", err, repository.rulesCalled)
	}
}

func TestReplaceRejectsPlannedTotalOverflowBeforeAuthorization(t *testing.T) {
	repository := &budgetRepositoryStub{}
	service := newBudgetService(repository, workspace.RoleMember, nil)
	_, err := service.Replace(
		context.Background(), budgetWorkspaceID, budgetUserID, "2026-08",
		WriteInput{Name: "August", Items: []ItemInput{
			{CategoryID: budgetChildID, AmountBaseMinor: math.MaxInt64},
			{CategoryID: budgetRefundID, AmountBaseMinor: 1},
		}},
	)
	if !errors.Is(err, ErrAmountOverflow) || repository.rulesCalled {
		t.Fatalf("Replace() error = %v, rules called = %v", err, repository.rulesCalled)
	}
}

func TestReplaceRequiresManageAccessBeforeCategoryLookup(t *testing.T) {
	repository := &budgetRepositoryStub{}
	service := newBudgetService(repository, workspace.RoleViewer, nil)

	_, err := service.Replace(
		context.Background(), budgetWorkspaceID, budgetUserID, "2026-08",
		WriteInput{Name: "August", Items: []ItemInput{{
			CategoryID: budgetChildID, AmountBaseMinor: 1,
		}}},
	)
	if !errors.Is(err, workspace.ErrForbidden) || repository.rulesCalled {
		t.Fatalf("Replace() error = %v, rules called = %v", err, repository.rulesCalled)
	}
}

func TestBuildBudgetRejectsAmountOverflow(t *testing.T) {
	month := Month{Year: 2026, Month: time.August}
	snapshot := validBudgetSnapshot(month)
	snapshot.Items = []ItemSnapshot{{
		ID: budgetItemID, CategoryID: budgetChildID, CategoryName: "Restaurants",
		PlannedBaseMinor: 1, SignedAllocationBaseMinor: math.MinInt64,
	}}
	_, err := buildBudget(snapshot, budgetWorkspaceID, month)
	if !errors.Is(err, ErrAmountOverflow) {
		t.Fatalf("buildBudget() error = %v", err)
	}
}

func validBudgetSnapshot(month Month) Snapshot {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	return Snapshot{
		ID: budgetID, WorkspaceID: budgetWorkspaceID, Name: "Monthly budget",
		StartsOn: month.StartDate(), Timezone: "Europe/Istanbul", BaseCurrency: money.TRY,
		Items: []ItemSnapshot{{
			ID: budgetItemID, CategoryID: budgetChildID, CategoryName: "Restaurants",
			PlannedBaseMinor: 10_000,
		}},
		CreatedAt: now, UpdatedAt: now,
	}
}

func newBudgetService(
	repository Repository,
	role workspace.Role,
	now func() time.Time,
) *Service {
	return NewService(
		repository,
		workspace.NewAuthorizer(budgetMembershipStub{role: role}),
		now,
	)
}

func stringPointer(value string) *string { return &value }
