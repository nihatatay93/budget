package analysis

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
	analysisWorkspaceID = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97b"
	analysisUserID      = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97c"
	analysisAccountID   = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97d"
	analysisExpenseID   = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97e"
	analysisChildID     = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97f"
	analysisIncomeID    = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b980"
)

type membershipStub struct {
	role workspace.Role
	err  error
}

func (s membershipStub) MemberRole(context.Context, string, string) (workspace.Role, error) {
	return s.role, s.err
}

type repositoryStub struct {
	snapshot Snapshot
	err      error
	called   bool
	query    Query
	now      time.Time
}

func (s *repositoryStub) Load(
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

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func assertDate(t *testing.T, value time.Time, year int, month time.Month, day int) {
	t.Helper()
	want := date(year, month, day)
	if !value.Equal(want) {
		t.Fatalf("date = %s, want %s", value.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}

func basePeriod() Period {
	return Period{
		FromDate: date(2026, time.August, 1), ToDate: date(2026, time.August, 31),
		ComparisonFromDate: date(2026, time.July, 1), ComparisonToDate: date(2026, time.July, 31),
		Granularity: GranularityWeek, Timezone: "Europe/Istanbul", BaseCurrency: money.TRY,
	}
}

func TestResolvePeriodDefaultsToATrailingYearAnchoredOnAMonth(t *testing.T) {
	now := time.Date(2026, time.August, 25, 21, 30, 0, 0, time.UTC)

	period, err := ResolvePeriod(Query{}, "Europe/Istanbul", money.TRY, now)
	if err != nil {
		t.Fatalf("ResolvePeriod() error = %v", err)
	}
	assertDate(t, period.FromDate, 2025, time.September, 1)
	assertDate(t, period.ToDate, 2026, time.August, 26)
	if period.Granularity != GranularityMonth {
		t.Fatalf("granularity = %q, want month for a year-long window", period.Granularity)
	}
}

// The comparison window has to be the same length and end the day before the analysis window,
// otherwise a period-over-period reading compares unequal spans.
func TestResolvePeriodDerivesAnEqualLengthComparisonWindow(t *testing.T) {
	from, to := date(2026, time.August, 1), date(2026, time.August, 31)
	period, err := ResolvePeriod(
		Query{FromDate: &from, ToDate: &to}, "UTC", money.TRY, date(2026, time.September, 5),
	)
	if err != nil {
		t.Fatalf("ResolvePeriod() error = %v", err)
	}
	assertDate(t, period.ComparisonToDate, 2026, time.July, 31)
	assertDate(t, period.ComparisonFromDate, 2026, time.July, 1)
	if got := period.DayCount(); got != 31 {
		t.Fatalf("day count = %d, want 31", got)
	}
	comparisonDays := int(period.ComparisonToDate.Sub(period.ComparisonFromDate)/(24*time.Hour)) + 1
	if comparisonDays != int(period.DayCount()) {
		t.Fatalf("comparison span = %d days, want %d", comparisonDays, period.DayCount())
	}
}

func TestResolvePeriodChoosesBucketWidthFromWindowLength(t *testing.T) {
	tests := []struct {
		name string
		from time.Time
		to   time.Time
		want Granularity
	}{
		{"one month", date(2026, time.August, 1), date(2026, time.August, 31), GranularityDay},
		{"one quarter", date(2026, time.June, 1), date(2026, time.August, 31), GranularityWeek},
		{"one year", date(2025, time.September, 1), date(2026, time.August, 31), GranularityMonth},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			period, err := ResolvePeriod(
				Query{FromDate: &test.from, ToDate: &test.to}, "UTC", money.TRY, test.to,
			)
			if err != nil {
				t.Fatalf("ResolvePeriod() error = %v", err)
			}
			if period.Granularity != test.want {
				t.Fatalf("granularity = %q, want %q", period.Granularity, test.want)
			}
		})
	}
}

// The series is generated per calendar step, so its size follows the window rather than the
// activity in it. Without a bound, one authenticated request can ask for a bucket a day
// across centuries and be answered.
func TestResolvePeriodRejectsAWindowLongerThanTheBound(t *testing.T) {
	from := date(1900, time.January, 1)
	to := date(2100, time.January, 1)

	_, err := ResolvePeriod(Query{FromDate: &from, ToDate: &to}, "UTC", money.TRY, to)

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ResolvePeriod() error = %v, want invalid input", err)
	}
}

func TestResolvePeriodAcceptsTheLongestAllowedWindow(t *testing.T) {
	from := date(2020, time.January, 1)
	to := from.AddDate(0, 0, int(MaxWindowDays)-1)

	period, err := ResolvePeriod(
		Query{FromDate: &from, ToDate: &to, Granularity: GranularityMonth},
		"UTC", money.TRY, to,
	)
	if err != nil {
		t.Fatalf("ResolvePeriod() error = %v, want the boundary window accepted", err)
	}
	if got := period.DayCount(); got != MaxWindowDays {
		t.Fatalf("day count = %d, want %d", got, MaxWindowDays)
	}
}

// A window inside the bound can still ask for more buckets than the series should carry, so
// the granularity is bounded separately from the span.
func TestResolvePeriodRejectsAGranularityTooFineForTheWindow(t *testing.T) {
	from := date(2020, time.January, 1)
	to := from.AddDate(0, 0, int(MaxWindowDays)-1)

	if _, err := ResolvePeriod(
		Query{FromDate: &from, ToDate: &to, Granularity: GranularityDay},
		"UTC", money.TRY, to,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ResolvePeriod() error = %v, want invalid input for a decade of days", err)
	}
	// The same decade is fine once the buckets are coarse enough to read.
	if _, err := ResolvePeriod(
		Query{FromDate: &from, ToDate: &to, Granularity: GranularityWeek},
		"UTC", money.TRY, to,
	); err != nil {
		t.Fatalf("ResolvePeriod() error = %v, want a decade of weeks accepted", err)
	}
}

// An omitted granularity must never be resolved into a rejection: the server picks the width
// itself, so the default request has to stay answerable at every allowed window length.
func TestResolvePeriodNeverSuggestsAGranularityItWouldReject(t *testing.T) {
	from := date(2020, time.January, 1)
	for days := 1; days <= int(MaxWindowDays); days++ {
		to := from.AddDate(0, 0, days-1)
		if _, err := ResolvePeriod(
			Query{FromDate: &from, ToDate: &to}, "UTC", money.TRY, to,
		); err != nil {
			t.Fatalf("ResolvePeriod() over %d days error = %v, want an accepted window",
				days, err)
		}
	}
}

func TestSeriesBucketCountMatchesTheGeneratedSeries(t *testing.T) {
	tests := []struct {
		name        string
		from, to    time.Time
		granularity Granularity
		want        int64
	}{
		{"one day", date(2026, time.August, 1), date(2026, time.August, 1), GranularityDay, 1},
		{"a fortnight", date(2026, time.August, 1), date(2026, time.August, 14), GranularityDay, 14},
		// 2026-08-01 is a Saturday, so a window into the next Monday spans three week anchors.
		{"across weeks", date(2026, time.August, 1), date(2026, time.August, 10), GranularityWeek, 3},
		{"within one week", date(2026, time.August, 3), date(2026, time.August, 9), GranularityWeek, 1},
		{"across a year end", date(2025, time.December, 20), date(2026, time.January, 5), GranularityMonth, 2},
		{"twelve months", date(2025, time.September, 1), date(2026, time.August, 31), GranularityMonth, 12},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := seriesBucketCount(test.from, test.to, test.granularity); got != test.want {
				t.Fatalf("seriesBucketCount() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestResolvePeriodKeepsAnExplicitGranularity(t *testing.T) {
	from, to := date(2025, time.September, 1), date(2026, time.August, 31)
	period, err := ResolvePeriod(
		Query{FromDate: &from, ToDate: &to, Granularity: GranularityDay},
		"UTC", money.TRY, to,
	)
	if err != nil {
		t.Fatalf("ResolvePeriod() error = %v", err)
	}
	if period.Granularity != GranularityDay {
		t.Fatalf("granularity = %q, want the requested day", period.Granularity)
	}
}

func TestAnalyzeRequiresReadAccessBeforeReadingTheLedger(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(
		repository,
		workspace.NewAuthorizer(membershipStub{err: workspace.ErrForbidden}),
		func() time.Time { return date(2026, time.August, 25) },
	)

	_, err := service.Analyze(context.Background(), analysisWorkspaceID, analysisUserID, Query{})
	if !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("Analyze() error = %v, want forbidden", err)
	}
	if repository.called {
		t.Fatal("Analyze() read the ledger for a caller without access")
	}
}

func TestAnalyzeRejectsMalformedInput(t *testing.T) {
	from, to := date(2026, time.August, 31), date(2026, time.August, 1)
	tests := []struct {
		name  string
		query Query
		id    string
	}{
		{name: "inverted range", query: Query{FromDate: &from, ToDate: &to}, id: analysisWorkspaceID},
		{name: "half range", query: Query{FromDate: &to}, id: analysisWorkspaceID},
		{
			name:  "unknown granularity",
			query: Query{Granularity: Granularity("century")},
			id:    analysisWorkspaceID,
		},
		{name: "malformed workspace", query: Query{}, id: "not-a-uuid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			service := NewService(
				repository,
				workspace.NewAuthorizer(membershipStub{role: workspace.RoleOwner}),
				func() time.Time { return date(2026, time.August, 25) },
			)
			_, err := service.Analyze(context.Background(), test.id, analysisUserID, test.query)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Analyze() error = %v, want invalid input", err)
			}
			if repository.called {
				t.Fatal("Analyze() reached the repository with invalid input")
			}
		})
	}
}

// Expense activity is stored as negative allocations. Every expense figure the analysis
// publishes has to read as a positive amount of money spent, and income has to stay as it is.
func TestBuildAnalysisOrientsExpenseActivityForReading(t *testing.T) {
	snapshot := Snapshot{
		Period: basePeriod(),
		Totals: TotalsSnapshot{
			IncomeSignedMinor: 500000, ExpenseSignedMinor: -320000,
			ComparisonIncomeSignedMinor: 480000, ComparisonExpenseSignedMinor: -350000,
			TransactionCount: 42, SpendingTransactionCount: 33,
			SmallestExpenseSignedMinor: -90000, SpendingDayCount: 18,
		},
		Series: []BucketSnapshot{{
			StartDate: date(2026, time.August, 1), EndDate: date(2026, time.August, 7),
			IncomeSignedMinor: 500000, ExpenseSignedMinor: -120000, TransactionCount: 12,
		}},
		Categories: []CategorySnapshot{
			{
				ID: analysisExpenseID, Name: "Food", Kind: CategoryExpense,
				IconType: "system", IconValue: "utensils", ColorKey: "orange",
				DirectSignedMinor: -200000, RolledSignedMinor: -260000,
				ComparisonDirectSignedMinor: -150000, ComparisonRolledSignedMinor: -190000,
				SmallestSignedMinor: -45000, LargestSignedMinor: 3000,
			},
			{
				ID: analysisIncomeID, Name: "Salary", Kind: CategoryIncome,
				IconType: "system", IconValue: "wallet", ColorKey: "green",
				DirectSignedMinor: 500000, RolledSignedMinor: 500000,
				SmallestSignedMinor: 100000, LargestSignedMinor: 400000,
			},
		},
		CategorySeries: []CategoryPointSnapshot{
			{
				CategoryID: analysisExpenseID, StartDate: date(2026, time.August, 1),
				SignedMinor: -200000,
			},
			{
				CategoryID: analysisIncomeID, StartDate: date(2026, time.August, 1),
				SignedMinor: 500000,
			},
		},
		Weekdays: []WeekdaySnapshot{
			{Weekday: 6, IncomeSignedMinor: 0, ExpenseSignedMinor: -140000, TransactionCount: 9},
		},
		Days: []DaySnapshot{{
			Date: date(2026, time.August, 1), IncomeSignedMinor: 500000,
			ExpenseSignedMinor: -4000, TransactionCount: 2,
		}},
		Payees: []PayeeSnapshot{{
			Payee: "Migros", ExpenseSignedMinor: -88000, TransactionCount: 7,
			FirstDate: date(2026, time.August, 1), LastDate: date(2026, time.August, 31),
		}},
		Accounts: []AccountSnapshot{{
			ID: analysisAccountID, Name: "Checking", Type: "bank", Currency: money.TRY,
			OutflowSignedMinor: -320000, InflowSignedMinor: 500000, TransactionCount: 42,
		}},
	}

	result, err := buildAnalysis(snapshot)
	if err != nil {
		t.Fatalf("buildAnalysis() error = %v", err)
	}
	if result.Totals.SpendingBaseMinor != 320000 || result.Totals.IncomeBaseMinor != 500000 {
		t.Fatalf("totals = %+v, want positive spending and unchanged income", result.Totals)
	}
	if result.Totals.NetBaseMinor != 180000 || result.Totals.ComparisonNetBaseMinor != 130000 {
		t.Fatalf("net = %d/%d, want income minus spending in both windows",
			result.Totals.NetBaseMinor, result.Totals.ComparisonNetBaseMinor)
	}
	if result.Totals.ComparisonSpendingBaseMinor != 350000 {
		t.Fatalf("comparison spending = %d, want 350000", result.Totals.ComparisonSpendingBaseMinor)
	}
	if result.Totals.LargestSpendingBaseMinor != 90000 {
		t.Fatalf("largest spending = %d, want 90000", result.Totals.LargestSpendingBaseMinor)
	}
	if result.Totals.DayCount != 31 {
		t.Fatalf("day count = %d, want the inclusive window length", result.Totals.DayCount)
	}
	if result.Series[0].SpendingBaseMinor != 120000 || result.Series[0].NetBaseMinor != 380000 {
		t.Fatalf("bucket = %+v, want oriented spending and net", result.Series[0])
	}
	expense, income := result.Categories[0], result.Categories[1]
	if expense.DirectBaseMinor != 200000 || expense.RolledUpBaseMinor != 260000 ||
		expense.ComparisonDirectBaseMinor != 150000 ||
		expense.ComparisonRolledUpBaseMinor != 190000 {
		t.Fatalf("expense category = %+v, want positive spending figures", expense)
	}
	// The largest expense reading is the most negative allocation, not the most positive
	// one, which for an expense category is a refund.
	if expense.LargestBaseMinor != 45000 {
		t.Fatalf("largest expense allocation = %d, want 45000", expense.LargestBaseMinor)
	}
	if income.DirectBaseMinor != 500000 || income.LargestBaseMinor != 400000 {
		t.Fatalf("income category = %+v, want unchanged income figures", income)
	}
	if result.CategorySeries[0].BaseMinor != 200000 || result.CategorySeries[1].BaseMinor != 500000 {
		t.Fatalf("category series = %+v, want per-kind orientation", result.CategorySeries)
	}
	if result.Weekdays[0].SpendingBaseMinor != 140000 {
		t.Fatalf("weekday = %+v, want positive spending", result.Weekdays[0])
	}
	if result.Days[0].SpendingBaseMinor != 4000 {
		t.Fatalf("day = %+v, want positive spending", result.Days[0])
	}
	if result.Payees[0].SpendingBaseMinor != 88000 {
		t.Fatalf("payee = %+v, want positive spending", result.Payees[0])
	}
	if result.Accounts[0].OutflowBaseMinor != 320000 {
		t.Fatalf("account = %+v, want positive outflow", result.Accounts[0])
	}
}

// A category whose only activity was a refund has no largest charge. Reporting a negative
// "largest" would read as a charge that never happened.
func TestBuildAnalysisReportsNoLargestChargeForRefundOnlyActivity(t *testing.T) {
	snapshot := Snapshot{
		Period: basePeriod(),
		Totals: TotalsSnapshot{SmallestExpenseSignedMinor: 2500},
		Categories: []CategorySnapshot{{
			ID: analysisExpenseID, Name: "Food", Kind: CategoryExpense,
			IconType: "system", IconValue: "utensils", ColorKey: "orange",
			DirectSignedMinor: 2500, RolledSignedMinor: 2500,
			SmallestSignedMinor: 2500, LargestSignedMinor: 2500,
		}},
	}

	result, err := buildAnalysis(snapshot)
	if err != nil {
		t.Fatalf("buildAnalysis() error = %v", err)
	}
	if result.Totals.LargestSpendingBaseMinor != 0 {
		t.Fatalf("largest spending = %d, want 0", result.Totals.LargestSpendingBaseMinor)
	}
	if result.Categories[0].LargestBaseMinor != 0 {
		t.Fatalf("largest allocation = %d, want 0", result.Categories[0].LargestBaseMinor)
	}
	if result.Categories[0].DirectBaseMinor != -2500 {
		t.Fatalf("direct = %d, want a negative spending total for a net refund",
			result.Categories[0].DirectBaseMinor)
	}
}

func TestBuildAnalysisRejectsUnsoundSnapshots(t *testing.T) {
	invertedBucket := basePeriod()
	tests := []struct {
		name     string
		snapshot Snapshot
		want     error
	}{
		{
			name:     "unresolved period",
			snapshot: Snapshot{Period: Period{}},
			want:     ErrInvalidData,
		},
		{
			name: "comparison window overlaps the analysis window",
			snapshot: func() Snapshot {
				period := basePeriod()
				period.ComparisonToDate = period.FromDate
				return Snapshot{Period: period}
			}(),
			want: ErrInvalidData,
		},
		{
			name: "bucket ends before it starts",
			snapshot: Snapshot{Period: invertedBucket, Series: []BucketSnapshot{{
				StartDate: date(2026, time.August, 7), EndDate: date(2026, time.August, 1),
			}}},
			want: ErrInvalidData,
		},
		{
			name:     "weekday outside the ISO range",
			snapshot: Snapshot{Period: basePeriod(), Weekdays: []WeekdaySnapshot{{Weekday: 8}}},
			want:     ErrInvalidData,
		},
		{
			name: "category point without a category",
			snapshot: Snapshot{Period: basePeriod(), CategorySeries: []CategoryPointSnapshot{{
				CategoryID: analysisChildID, StartDate: date(2026, time.August, 1),
			}}},
			want: ErrInvalidData,
		},
		{
			name: "amount beyond the minor-unit range",
			snapshot: Snapshot{
				Period: basePeriod(),
				Totals: TotalsSnapshot{ExpenseSignedMinor: math.MinInt64},
			},
			want: ErrAmountOverflow,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildAnalysis(test.snapshot); !errors.Is(err, test.want) {
				t.Fatalf("buildAnalysis() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestAnalyzeReturnsTheOrientedAnalysis(t *testing.T) {
	repository := &repositoryStub{snapshot: Snapshot{
		Period: basePeriod(),
		Totals: TotalsSnapshot{IncomeSignedMinor: 1000, ExpenseSignedMinor: -400},
	}}
	now := date(2026, time.August, 25)
	service := NewService(
		repository,
		workspace.NewAuthorizer(membershipStub{role: workspace.RoleViewer}),
		func() time.Time { return now },
	)

	result, err := service.Analyze(
		context.Background(), analysisWorkspaceID, analysisUserID,
		Query{Granularity: GranularityMonth},
	)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !repository.now.Equal(now) {
		t.Fatalf("repository now = %s, want the service clock", repository.now)
	}
	if repository.query.Granularity != GranularityMonth {
		t.Fatalf("repository granularity = %q, want month", repository.query.Granularity)
	}
	if result.Totals.SpendingBaseMinor != 400 || result.Totals.NetBaseMinor != 600 {
		t.Fatalf("totals = %+v, want oriented spending and net", result.Totals)
	}
}
