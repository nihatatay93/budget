package reporting

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

const (
	reportingWorkspaceID = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97b"
	reportingUserID      = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97c"
	reportingAccountID   = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97d"
	reportingParentID    = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97e"
	reportingChildID     = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97f"
	reportingIncomeID    = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b980"
)

type reportingMembershipStub struct {
	role workspace.Role
	err  error
}

func (s reportingMembershipStub) MemberRole(context.Context, string, string) (workspace.Role, error) {
	return s.role, s.err
}

type reportingRepositoryStub struct {
	snapshot Snapshot
	err      error
	called   bool
	query    Query
	now      time.Time
}

func (s *reportingRepositoryStub) Load(
	_ context.Context,
	_ string,
	query Query,
	now time.Time,
) (Snapshot, error) {
	s.called = true
	s.query = query
	s.now = now
	return s.snapshot, s.err
}

func TestResolvePeriodUsesWorkspaceLocalMonthToDate(t *testing.T) {
	now := time.Date(2026, time.August, 31, 21, 30, 0, 0, time.UTC)
	period, err := ResolvePeriod(Query{}, "Europe/Istanbul", money.TRY, now)
	if err != nil {
		t.Fatalf("ResolvePeriod() error = %v", err)
	}
	assertDate(t, period.FromDate, 2026, time.September, 1)
	assertDate(t, period.ToDate, 2026, time.September, 1)

	from := time.Date(2026, time.July, 2, 14, 0, 0, 0, time.FixedZone("custom", 3*60*60))
	to := time.Date(2026, time.July, 9, 23, 0, 0, 0, time.FixedZone("custom", 3*60*60))
	period, err = ResolvePeriod(
		Query{FromDate: &from, ToDate: &to}, "Europe/Istanbul", money.TRY, now,
	)
	if err != nil {
		t.Fatalf("ResolvePeriod(explicit) error = %v", err)
	}
	assertDate(t, period.FromDate, 2026, time.July, 2)
	assertDate(t, period.ToDate, 2026, time.July, 9)
}

func TestProjectNormalizesSignsAndDoesNotDoubleCountRollups(t *testing.T) {
	period := Period{
		FromDate: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		ToDate:   time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC),
		Timezone: "Europe/Istanbul", BaseCurrency: money.TRY,
	}
	repository := &reportingRepositoryStub{snapshot: Snapshot{
		Period: period,
		Accounts: []AccountSnapshot{{
			ID: reportingAccountID, Name: "Checking", Type: "bank", Currency: money.TRY,
			PostedNativeMinor: 8000, PendingNativeMinor: -300,
			PostedBaseMinor: 8000, PendingBaseMinor: -300,
		}},
		Categories: []CategorySnapshot{
			{
				ID: reportingParentID, Name: "Food", Kind: CategoryExpense,
				RolledPostedSignedMinor: -1300, RolledPendingSignedMinor: -300,
			},
			{
				ID: reportingChildID, ParentID: stringPointer(reportingParentID),
				Name: "Restaurants", Kind: CategoryExpense,
				DirectPostedSignedMinor: -1300, DirectPendingSignedMinor: -300,
				RolledPostedSignedMinor: -1300, RolledPendingSignedMinor: -300,
			},
			{
				ID: reportingIncomeID, Name: "Salary", Kind: CategoryIncome,
				DirectPostedSignedMinor: 5000, DirectPendingSignedMinor: -200,
				RolledPostedSignedMinor: 5000, RolledPendingSignedMinor: -200,
			},
		},
	}}
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	service := NewService(
		repository,
		workspace.NewAuthorizer(reportingMembershipStub{role: workspace.RoleViewer}),
		func() time.Time { return now },
	)
	projection, err := service.Project(
		context.Background(), reportingWorkspaceID, reportingUserID, Query{},
	)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if !repository.called || !repository.now.Equal(now) {
		t.Fatalf("repository call = %#v at %v", repository.called, repository.now)
	}
	if projection.Summary.BalanceBaseMinor != (Amounts{Posted: 8000, Pending: -300, Projected: 7700}) {
		t.Fatalf("balance summary = %#v", projection.Summary.BalanceBaseMinor)
	}
	if projection.Summary.SpendingBaseMinor != (Amounts{Posted: 1300, Pending: 300, Projected: 1600}) {
		t.Fatalf("spending summary = %#v", projection.Summary.SpendingBaseMinor)
	}
	if projection.Summary.IncomeBaseMinor != (Amounts{Posted: 5000, Pending: -200, Projected: 4800}) {
		t.Fatalf("income summary = %#v", projection.Summary.IncomeBaseMinor)
	}
	if projection.Categories[0].Direct != (Amounts{}) ||
		projection.Categories[0].RolledUp != (Amounts{Posted: 1300, Pending: 300, Projected: 1600}) {
		t.Fatalf("parent category = %#v", projection.Categories[0])
	}
}

func TestProjectRejectsInvalidInputBeforeAuthorization(t *testing.T) {
	repository := &reportingRepositoryStub{}
	service := NewService(
		repository,
		workspace.NewAuthorizer(reportingMembershipStub{role: workspace.RoleViewer}),
		nil,
	)
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	_, err := service.Project(
		context.Background(), reportingWorkspaceID, reportingUserID,
		Query{FromDate: &from},
	)
	if !errors.Is(err, ErrInvalidInput) || repository.called {
		t.Fatalf("Project() error = %v, repository called = %v", err, repository.called)
	}
}

func TestProjectRequiresWorkspaceReadAccess(t *testing.T) {
	repository := &reportingRepositoryStub{}
	service := NewService(
		repository,
		workspace.NewAuthorizer(reportingMembershipStub{}),
		nil,
	)
	_, err := service.Project(
		context.Background(), reportingWorkspaceID, reportingUserID, Query{},
	)
	if !errors.Is(err, workspace.ErrForbidden) || repository.called {
		t.Fatalf("Project() error = %v, repository called = %v", err, repository.called)
	}
}

func TestBuildProjectionRejectsOverflow(t *testing.T) {
	_, err := buildProjection(Snapshot{
		Period: Period{
			FromDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			ToDate:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			Timezone: "UTC", BaseCurrency: money.TRY,
		},
		Accounts: []AccountSnapshot{{
			ID: reportingAccountID, Name: "Checking", Type: "bank", Currency: money.TRY,
			PostedNativeMinor: math.MaxInt64, PendingNativeMinor: 1,
		}},
	})
	if !errors.Is(err, ErrAmountOverflow) {
		t.Fatalf("buildProjection() error = %v, want ErrAmountOverflow", err)
	}
}

func assertDate(t *testing.T, value time.Time, year int, month time.Month, day int) {
	t.Helper()
	if value.Year() != year || value.Month() != month || value.Day() != day ||
		value.Location() != time.UTC {
		t.Fatalf("date = %v, want %04d-%02d-%02d UTC", value, year, month, day)
	}
}

func stringPointer(value string) *string { return &value }
