// Package analysis owns posted-only spending and income analytics over the transaction
// ledger. It answers where money went, when it went, and how that changed, without
// re-deriving balances: those remain the reporting package's responsibility.
//
// Every figure is posted, expressed in workspace base-currency minor units, and oriented for
// reading rather than for bookkeeping. Spending is positive when money left the workspace.
// Transfers carry no allocations and therefore never appear as spending or income.
package analysis

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

var (
	ErrInvalidInput   = errors.New("invalid analysis input")
	ErrInvalidData    = errors.New("analysis source data is invalid")
	ErrAmountOverflow = errors.New("analysis amount exceeds int64 minor-unit range")
)

// defaultWindowMonths is the trailing span used when the caller supplies no dates. A year of
// completed months is enough to read seasonality without asking the operator to choose.
const defaultWindowMonths = 12

// payeeLimit caps the payee ranking. The list is a leaderboard, not an export.
const payeeLimit = 25

// The series is generated one bucket per calendar step, so an analysis costs what its window
// spans rather than what happened inside it: an empty workspace queried across a century
// still produces a bucket a day. Both bounds below exist to keep that cost tied to something
// a person could plausibly want to read.
//
// MaxWindowDays bounds the window itself, which also bounds the equal-length comparison
// window read alongside it. MaxSeriesBuckets bounds the resolved series, which is what stops
// a decade being requested one day at a time.
const (
	MaxWindowDays    = 3653 // ten years, allowing for leap days
	MaxSeriesBuckets = 750  // two years of days, a decade of weeks, sixty years of months
)

type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
)

func (g Granularity) Valid() bool {
	return g == GranularityDay || g == GranularityWeek || g == GranularityMonth
}

// Query is the caller's request. Dates are supplied as an inclusive pair or not at all.
// An empty Granularity asks the server to pick one that suits the window length.
type Query struct {
	FromDate    *time.Time
	ToDate      *time.Time
	Granularity Granularity
}

// Period is the resolved window, its equal-length predecessor, and the workspace settings
// every amount in the analysis is expressed against.
type Period struct {
	FromDate           time.Time
	ToDate             time.Time
	ComparisonFromDate time.Time
	ComparisonToDate   time.Time
	Granularity        Granularity
	Timezone           string
	BaseCurrency       money.Currency
}

// DayCount is the inclusive length of the window in calendar days.
func (p Period) DayCount() int64 {
	return int64(dateOnly(p.ToDate).Sub(dateOnly(p.FromDate))/(24*time.Hour)) + 1
}

type Totals struct {
	IncomeBaseMinor             int64
	SpendingBaseMinor           int64
	NetBaseMinor                int64
	ComparisonIncomeBaseMinor   int64
	ComparisonSpendingBaseMinor int64
	ComparisonNetBaseMinor      int64
	TransactionCount            int64
	SpendingTransactionCount    int64
	LargestSpendingBaseMinor    int64
	SpendingDayCount            int64
	DayCount                    int64
}

type Bucket struct {
	StartDate         time.Time
	EndDate           time.Time
	IncomeBaseMinor   int64
	SpendingBaseMinor int64
	NetBaseMinor      int64
	TransactionCount  int64
}

type CategoryKind string

const (
	CategoryExpense CategoryKind = "expense"
	CategoryIncome  CategoryKind = "income"
)

func (k CategoryKind) Valid() bool {
	return k == CategoryExpense || k == CategoryIncome
}

type Category struct {
	ID                          string
	ParentID                    *string
	Name                        string
	Kind                        CategoryKind
	SystemKey                   *string
	PredefinedKey               *string
	IconType                    string
	IconValue                   string
	ColorKey                    string
	ArchivedAt                  *time.Time
	DirectBaseMinor             int64
	RolledUpBaseMinor           int64
	ComparisonDirectBaseMinor   int64
	ComparisonRolledUpBaseMinor int64
	TransactionCount            int64
	RolledUpTransactionCount    int64
	LargestBaseMinor            int64
	FirstDate                   *time.Time
	LastDate                    *time.Time
}

// CategoryPoint is one category's direct activity inside one bucket. Points exist only where
// activity does, so a long window of sparse categories stays small on the wire.
type CategoryPoint struct {
	CategoryID string
	StartDate  time.Time
	BaseMinor  int64
}

type Weekday struct {
	// Weekday is the ISO weekday, where 1 is Monday and 7 is Sunday.
	Weekday           int
	IncomeBaseMinor   int64
	SpendingBaseMinor int64
	TransactionCount  int64
}

type Day struct {
	Date              time.Time
	IncomeBaseMinor   int64
	SpendingBaseMinor int64
	TransactionCount  int64
}

type Payee struct {
	Payee             string
	SpendingBaseMinor int64
	IncomeBaseMinor   int64
	TransactionCount  int64
	FirstDate         time.Time
	LastDate          time.Time
}

type Account struct {
	ID               string
	Name             string
	Type             string
	Currency         money.Currency
	ArchivedAt       *time.Time
	OutflowBaseMinor int64
	InflowBaseMinor  int64
	TransactionCount int64
}

type Analysis struct {
	Period         Period
	Totals         Totals
	Series         []Bucket
	Categories     []Category
	CategorySeries []CategoryPoint
	Weekdays       []Weekday
	Days           []Day
	Payees         []Payee
	Accounts       []Account
}

// Snapshot is the repository's consistent, workspace-scoped view. Amounts retain ledger
// signs, so expense activity arrives negative; the service applies reporting orientation.
type Snapshot struct {
	Period         Period
	Totals         TotalsSnapshot
	Series         []BucketSnapshot
	Categories     []CategorySnapshot
	CategorySeries []CategoryPointSnapshot
	Weekdays       []WeekdaySnapshot
	Days           []DaySnapshot
	Payees         []PayeeSnapshot
	Accounts       []AccountSnapshot
}

type TotalsSnapshot struct {
	IncomeSignedMinor            int64
	ExpenseSignedMinor           int64
	ComparisonIncomeSignedMinor  int64
	ComparisonExpenseSignedMinor int64
	TransactionCount             int64
	SpendingTransactionCount     int64
	// SmallestExpenseSignedMinor is the most negative per-transaction expense total, which
	// is the largest single charge once oriented for reading.
	SmallestExpenseSignedMinor int64
	SpendingDayCount           int64
}

type BucketSnapshot struct {
	StartDate          time.Time
	EndDate            time.Time
	IncomeSignedMinor  int64
	ExpenseSignedMinor int64
	TransactionCount   int64
}

type CategorySnapshot struct {
	ID                          string
	ParentID                    *string
	Name                        string
	Kind                        CategoryKind
	SystemKey                   *string
	PredefinedKey               *string
	IconType                    string
	IconValue                   string
	ColorKey                    string
	ArchivedAt                  *time.Time
	DirectSignedMinor           int64
	RolledSignedMinor           int64
	ComparisonDirectSignedMinor int64
	ComparisonRolledSignedMinor int64
	TransactionCount            int64
	RolledTransactionCount      int64
	// SmallestSignedMinor and LargestSignedMinor bound the window's allocations. Which one
	// is the largest reading depends on the category kind.
	SmallestSignedMinor int64
	LargestSignedMinor  int64
	FirstDate           *time.Time
	LastDate            *time.Time
}

type CategoryPointSnapshot struct {
	CategoryID  string
	StartDate   time.Time
	SignedMinor int64
}

type WeekdaySnapshot struct {
	Weekday            int
	IncomeSignedMinor  int64
	ExpenseSignedMinor int64
	TransactionCount   int64
}

type DaySnapshot struct {
	Date               time.Time
	IncomeSignedMinor  int64
	ExpenseSignedMinor int64
	TransactionCount   int64
}

type PayeeSnapshot struct {
	Payee              string
	ExpenseSignedMinor int64
	IncomeSignedMinor  int64
	TransactionCount   int64
	FirstDate          time.Time
	LastDate           time.Time
}

type AccountSnapshot struct {
	ID                 string
	Name               string
	Type               string
	Currency           money.Currency
	ArchivedAt         *time.Time
	OutflowSignedMinor int64
	InflowSignedMinor  int64
	TransactionCount   int64
}

type Repository interface {
	Load(context.Context, string, Query, time.Time) (Snapshot, error)
}

type Service struct {
	repository Repository
	access     *workspace.Authorizer
	now        func() time.Time
}

func NewService(
	repository Repository,
	access *workspace.Authorizer,
	now func() time.Time,
) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, access: access, now: now}
}

func (s *Service) Analyze(
	ctx context.Context,
	workspaceID, userID string,
	query Query,
) (Analysis, error) {
	if !validUUID(workspaceID) || !validUUID(userID) || !validQuery(query) {
		return Analysis{}, ErrInvalidInput
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return Analysis{}, err
	}
	snapshot, err := s.repository.Load(ctx, workspaceID, query, s.now())
	if err != nil {
		return Analysis{}, err
	}
	return buildAnalysis(snapshot)
}

// PayeeLimit is the number of payees the repository should rank.
func PayeeLimit() int32 { return payeeLimit }

// ResolvePeriod applies the accepted window rules once the repository has loaded workspace
// settings inside its read snapshot. The comparison window always ends the day before the
// analysis window and matches its length, so movement between the two is like-for-like.
func ResolvePeriod(
	query Query,
	timezone string,
	baseCurrency money.Currency,
	now time.Time,
) (Period, error) {
	if !validQuery(query) {
		return Period{}, ErrInvalidInput
	}
	if !baseCurrency.Valid() {
		return Period{}, ErrInvalidData
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Period{}, ErrInvalidData
	}
	var fromDate, toDate time.Time
	if query.FromDate == nil {
		toDate = dateOnly(now.In(location))
		// Anchor the trailing window on a month boundary so month buckets are whole and
		// year-over-year reading is not skewed by a ragged first bucket.
		fromDate = time.Date(
			toDate.Year(), toDate.Month()-defaultWindowMonths+1, 1, 0, 0, 0, 0, time.UTC,
		)
	} else {
		fromDate, toDate = dateOnly(*query.FromDate), dateOnly(*query.ToDate)
	}
	granularity := query.Granularity
	if granularity == "" {
		granularity = suggestGranularity(fromDate, toDate)
	}
	// Checked here rather than in validQuery because a defaulted window is not known until
	// the workspace timezone has resolved it, and the bucket count depends on both.
	if seriesBucketCount(fromDate, toDate, granularity) > MaxSeriesBuckets {
		return Period{}, ErrInvalidInput
	}
	comparisonToDate := fromDate.AddDate(0, 0, -1)
	days := int(toDate.Sub(fromDate)/(24*time.Hour)) + 1
	return Period{
		FromDate:           fromDate,
		ToDate:             toDate,
		ComparisonFromDate: fromDate.AddDate(0, 0, -days),
		ComparisonToDate:   comparisonToDate,
		Granularity:        granularity,
		Timezone:           timezone,
		BaseCurrency:       baseCurrency,
	}, nil
}

// suggestGranularity keeps a chart readable: roughly a hundred points at most, and a bucket
// width people already think in.
func suggestGranularity(fromDate, toDate time.Time) Granularity {
	days := int(toDate.Sub(fromDate)/(24*time.Hour)) + 1
	switch {
	case days <= 62:
		return GranularityDay
	case days <= 200:
		return GranularityWeek
	default:
		return GranularityMonth
	}
}

func buildAnalysis(snapshot Snapshot) (Analysis, error) {
	if err := validatePeriod(snapshot.Period); err != nil {
		return Analysis{}, err
	}
	analysis := Analysis{
		Period:         snapshot.Period,
		Series:         make([]Bucket, 0, len(snapshot.Series)),
		Categories:     make([]Category, 0, len(snapshot.Categories)),
		CategorySeries: make([]CategoryPoint, 0, len(snapshot.CategorySeries)),
		Weekdays:       make([]Weekday, 0, len(snapshot.Weekdays)),
		Days:           make([]Day, 0, len(snapshot.Days)),
		Payees:         make([]Payee, 0, len(snapshot.Payees)),
		Accounts:       make([]Account, 0, len(snapshot.Accounts)),
	}

	totals, err := buildTotals(snapshot)
	if err != nil {
		return Analysis{}, err
	}
	analysis.Totals = totals

	for _, value := range snapshot.Series {
		if value.StartDate.IsZero() || value.EndDate.IsZero() ||
			dateOnly(value.StartDate).After(dateOnly(value.EndDate)) {
			return Analysis{}, ErrInvalidData
		}
		spending, err := negate(value.ExpenseSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		net, err := subtract(value.IncomeSignedMinor, spending)
		if err != nil {
			return Analysis{}, err
		}
		analysis.Series = append(analysis.Series, Bucket{
			StartDate: dateOnly(value.StartDate), EndDate: dateOnly(value.EndDate),
			IncomeBaseMinor: value.IncomeSignedMinor, SpendingBaseMinor: spending,
			NetBaseMinor: net, TransactionCount: value.TransactionCount,
		})
	}

	kinds := make(map[string]CategoryKind, len(snapshot.Categories))
	for _, value := range snapshot.Categories {
		if !validUUID(value.ID) || value.Name == "" || !value.Kind.Valid() ||
			(value.ParentID != nil && !validUUID(*value.ParentID)) {
			return Analysis{}, ErrInvalidData
		}
		kinds[value.ID] = value.Kind
		direct, err := orient(value.Kind, value.DirectSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		rolled, err := orient(value.Kind, value.RolledSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		comparisonDirect, err := orient(value.Kind, value.ComparisonDirectSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		comparisonRolled, err := orient(value.Kind, value.ComparisonRolledSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		// An expense category's largest reading is its most negative allocation; an income
		// category's is its most positive one. Refunds sit on the opposite end of each.
		largestSigned := value.LargestSignedMinor
		if value.Kind == CategoryExpense {
			largestSigned = value.SmallestSignedMinor
		}
		largest, err := orient(value.Kind, largestSigned)
		if err != nil {
			return Analysis{}, err
		}
		// A category whose only activity was a refund has no largest reading to show.
		if largest < 0 {
			largest = 0
		}
		analysis.Categories = append(analysis.Categories, Category{
			ID: value.ID, ParentID: value.ParentID, Name: value.Name, Kind: value.Kind,
			SystemKey: value.SystemKey, PredefinedKey: value.PredefinedKey,
			IconType: value.IconType, IconValue: value.IconValue, ColorKey: value.ColorKey,
			ArchivedAt:                  value.ArchivedAt,
			DirectBaseMinor:             direct,
			RolledUpBaseMinor:           rolled,
			ComparisonDirectBaseMinor:   comparisonDirect,
			ComparisonRolledUpBaseMinor: comparisonRolled,
			TransactionCount:            value.TransactionCount,
			RolledUpTransactionCount:    value.RolledTransactionCount,
			LargestBaseMinor:            largest,
			FirstDate:                   value.FirstDate,
			LastDate:                    value.LastDate,
		})
	}

	for _, value := range snapshot.CategorySeries {
		kind, ok := kinds[value.CategoryID]
		if !ok || value.StartDate.IsZero() {
			return Analysis{}, ErrInvalidData
		}
		amount, err := orient(kind, value.SignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		analysis.CategorySeries = append(analysis.CategorySeries, CategoryPoint{
			CategoryID: value.CategoryID, StartDate: dateOnly(value.StartDate),
			BaseMinor: amount,
		})
	}

	for _, value := range snapshot.Weekdays {
		if value.Weekday < 1 || value.Weekday > 7 {
			return Analysis{}, ErrInvalidData
		}
		spending, err := negate(value.ExpenseSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		analysis.Weekdays = append(analysis.Weekdays, Weekday{
			Weekday: value.Weekday, IncomeBaseMinor: value.IncomeSignedMinor,
			SpendingBaseMinor: spending, TransactionCount: value.TransactionCount,
		})
	}

	for _, value := range snapshot.Days {
		if value.Date.IsZero() {
			return Analysis{}, ErrInvalidData
		}
		spending, err := negate(value.ExpenseSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		analysis.Days = append(analysis.Days, Day{
			Date: dateOnly(value.Date), IncomeBaseMinor: value.IncomeSignedMinor,
			SpendingBaseMinor: spending, TransactionCount: value.TransactionCount,
		})
	}

	for _, value := range snapshot.Payees {
		if value.Payee == "" || value.FirstDate.IsZero() || value.LastDate.IsZero() {
			return Analysis{}, ErrInvalidData
		}
		spending, err := negate(value.ExpenseSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		analysis.Payees = append(analysis.Payees, Payee{
			Payee: value.Payee, SpendingBaseMinor: spending,
			IncomeBaseMinor: value.IncomeSignedMinor, TransactionCount: value.TransactionCount,
			FirstDate: dateOnly(value.FirstDate), LastDate: dateOnly(value.LastDate),
		})
	}

	for _, value := range snapshot.Accounts {
		if !validUUID(value.ID) || value.Name == "" || value.Type == "" ||
			!value.Currency.Valid() {
			return Analysis{}, ErrInvalidData
		}
		outflow, err := negate(value.OutflowSignedMinor)
		if err != nil {
			return Analysis{}, err
		}
		analysis.Accounts = append(analysis.Accounts, Account{
			ID: value.ID, Name: value.Name, Type: value.Type, Currency: value.Currency,
			ArchivedAt: value.ArchivedAt, OutflowBaseMinor: outflow,
			InflowBaseMinor: value.InflowSignedMinor, TransactionCount: value.TransactionCount,
		})
	}

	return analysis, nil
}

func buildTotals(snapshot Snapshot) (Totals, error) {
	source := snapshot.Totals
	spending, err := negate(source.ExpenseSignedMinor)
	if err != nil {
		return Totals{}, err
	}
	net, err := subtract(source.IncomeSignedMinor, spending)
	if err != nil {
		return Totals{}, err
	}
	comparisonSpending, err := negate(source.ComparisonExpenseSignedMinor)
	if err != nil {
		return Totals{}, err
	}
	comparisonNet, err := subtract(source.ComparisonIncomeSignedMinor, comparisonSpending)
	if err != nil {
		return Totals{}, err
	}
	largest, err := negate(source.SmallestExpenseSignedMinor)
	if err != nil {
		return Totals{}, err
	}
	// A window whose only expense transaction is a net refund has no largest charge to show.
	if largest < 0 {
		largest = 0
	}
	return Totals{
		IncomeBaseMinor:             source.IncomeSignedMinor,
		SpendingBaseMinor:           spending,
		NetBaseMinor:                net,
		ComparisonIncomeBaseMinor:   source.ComparisonIncomeSignedMinor,
		ComparisonSpendingBaseMinor: comparisonSpending,
		ComparisonNetBaseMinor:      comparisonNet,
		TransactionCount:            source.TransactionCount,
		SpendingTransactionCount:    source.SpendingTransactionCount,
		LargestSpendingBaseMinor:    largest,
		SpendingDayCount:            source.SpendingDayCount,
		DayCount:                    snapshot.Period.DayCount(),
	}, nil
}

func validatePeriod(period Period) error {
	if period.Timezone == "" || !period.BaseCurrency.Valid() || !period.Granularity.Valid() ||
		period.FromDate.IsZero() || period.ToDate.IsZero() ||
		period.ComparisonFromDate.IsZero() || period.ComparisonToDate.IsZero() ||
		dateOnly(period.FromDate).After(dateOnly(period.ToDate)) ||
		!dateOnly(period.ComparisonToDate).Before(dateOnly(period.FromDate)) ||
		dateOnly(period.ComparisonFromDate).After(dateOnly(period.ComparisonToDate)) {
		return ErrInvalidData
	}
	if _, err := time.LoadLocation(period.Timezone); err != nil {
		return ErrInvalidData
	}
	return nil
}

func validQuery(query Query) bool {
	if query.Granularity != "" && !query.Granularity.Valid() {
		return false
	}
	if (query.FromDate == nil) != (query.ToDate == nil) {
		return false
	}
	if query.FromDate == nil {
		return true
	}
	if query.FromDate.IsZero() || query.ToDate.IsZero() {
		return false
	}
	from, to := dateOnly(*query.FromDate), dateOnly(*query.ToDate)
	if from.After(to) {
		return false
	}
	return inclusiveDays(from, to) <= MaxWindowDays
}

func inclusiveDays(from, to time.Time) int64 {
	return int64(to.Sub(from)/(24*time.Hour)) + 1
}

// seriesBucketCount mirrors the steps the repository's generate_series will take, so the
// bound is checked against the series that would actually be produced.
func seriesBucketCount(from, to time.Time, granularity Granularity) int64 {
	switch granularity {
	case GranularityDay:
		return inclusiveDays(from, to)
	case GranularityWeek:
		start, end := startOfISOWeek(from), startOfISOWeek(to)
		return int64(end.Sub(start)/(7*24*time.Hour)) + 1
	case GranularityMonth:
		return int64((to.Year()-from.Year())*12+int(to.Month())-int(from.Month())) + 1
	default:
		return 0
	}
}

// startOfISOWeek matches PostgreSQL's date_trunc('week', ...), which anchors on Monday.
func startOfISOWeek(value time.Time) time.Time {
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return value.AddDate(0, 0, -(weekday - 1))
}

// orient turns a ledger-signed amount into a reading: spending counts up for expense
// categories, and income counts up for income categories.
func orient(kind CategoryKind, signed int64) (int64, error) {
	if kind == CategoryExpense {
		return negate(signed)
	}
	return signed, nil
}

func subtract(left, right int64) (int64, error) {
	negated, err := negate(right)
	if err != nil {
		return 0, err
	}
	return add(left, negated)
}

func add(left, right int64) (int64, error) {
	if (right > 0 && left > math.MaxInt64-right) ||
		(right < 0 && left < math.MinInt64-right) {
		return 0, ErrAmountOverflow
	}
	return left + right, nil
}

func negate(value int64) (int64, error) {
	if value == math.MinInt64 {
		return 0, ErrAmountOverflow
	}
	return -value, nil
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
