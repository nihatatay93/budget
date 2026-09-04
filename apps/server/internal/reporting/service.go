// Package reporting owns derived financial projections over the transaction ledger.
package reporting

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
	ErrInvalidInput   = errors.New("invalid projection input")
	ErrInvalidData    = errors.New("projection source data is invalid")
	ErrAmountOverflow = errors.New("projection amount exceeds int64 minor-unit range")
)

type Query struct {
	FromDate *time.Time
	ToDate   *time.Time
}

type Period struct {
	FromDate     time.Time
	ToDate       time.Time
	Timezone     string
	BaseCurrency money.Currency
}

// Amounts keeps settled and unsettled figures explicit. For account balances Pending is a
// delta; for period activity it is the pending activity in the selected window.
type Amounts struct {
	Posted    int64
	Pending   int64
	Projected int64
}

type Summary struct {
	BalanceBaseMinor  Amounts
	IncomeBaseMinor   Amounts
	SpendingBaseMinor Amounts
}

type Account struct {
	ID         string
	Name       string
	Type       string
	Currency   money.Currency
	ArchivedAt *time.Time
	Native     Amounts
	Base       Amounts
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
	ID            string
	ParentID      *string
	Name          string
	Kind          CategoryKind
	SystemKey     *string
	PredefinedKey *string
	Icon          *string
	IconType      string
	IconValue     string
	ColorKey      string
	ArchivedAt    *time.Time
	Direct        Amounts
	RolledUp      Amounts
}

type Projection struct {
	Period     Period
	Summary    Summary
	Accounts   []Account
	Categories []Category
}

// Snapshot is the repository's consistent, workspace-scoped view of the ledger. Category
// amounts retain ledger signs; the service converts them to reporting orientation.
type Snapshot struct {
	Period     Period
	Accounts   []AccountSnapshot
	Categories []CategorySnapshot
}

type AccountSnapshot struct {
	ID                 string
	Name               string
	Type               string
	Currency           money.Currency
	ArchivedAt         *time.Time
	PostedNativeMinor  int64
	PendingNativeMinor int64
	PostedBaseMinor    int64
	PendingBaseMinor   int64
}

type CategorySnapshot struct {
	ID                       string
	ParentID                 *string
	Name                     string
	Kind                     CategoryKind
	SystemKey                *string
	PredefinedKey            *string
	Icon                     *string
	IconType                 string
	IconValue                string
	ColorKey                 string
	ArchivedAt               *time.Time
	DirectPostedSignedMinor  int64
	DirectPendingSignedMinor int64
	RolledPostedSignedMinor  int64
	RolledPendingSignedMinor int64
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

func (s *Service) Project(
	ctx context.Context,
	workspaceID, userID string,
	query Query,
) (Projection, error) {
	if !validUUID(workspaceID) || !validUUID(userID) || !validQuery(query) {
		return Projection{}, ErrInvalidInput
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return Projection{}, err
	}
	snapshot, err := s.repository.Load(ctx, workspaceID, query, s.now())
	if err != nil {
		return Projection{}, err
	}
	return buildProjection(snapshot)
}

// ResolvePeriod applies the accepted inclusive date-window rules after the repository has
// loaded workspace settings inside its read snapshot.
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
	if query.FromDate == nil {
		localNow := now.In(location)
		toDate := dateOnly(localNow)
		return Period{
			FromDate: time.Date(toDate.Year(), toDate.Month(), 1, 0, 0, 0, 0, time.UTC),
			ToDate:   toDate, Timezone: timezone, BaseCurrency: baseCurrency,
		}, nil
	}
	return Period{
		FromDate: dateOnly(*query.FromDate), ToDate: dateOnly(*query.ToDate),
		Timezone: timezone, BaseCurrency: baseCurrency,
	}, nil
}

func buildProjection(snapshot Snapshot) (Projection, error) {
	if err := validatePeriod(snapshot.Period); err != nil {
		return Projection{}, err
	}
	projection := Projection{
		Period:     snapshot.Period,
		Accounts:   make([]Account, 0, len(snapshot.Accounts)),
		Categories: make([]Category, 0, len(snapshot.Categories)),
	}
	for _, value := range snapshot.Accounts {
		if !validUUID(value.ID) || value.Name == "" || value.Type == "" || !value.Currency.Valid() {
			return Projection{}, ErrInvalidData
		}
		native, err := amounts(value.PostedNativeMinor, value.PendingNativeMinor)
		if err != nil {
			return Projection{}, err
		}
		base, err := amounts(value.PostedBaseMinor, value.PendingBaseMinor)
		if err != nil {
			return Projection{}, err
		}
		projection.Accounts = append(projection.Accounts, Account{
			ID: value.ID, Name: value.Name, Type: value.Type, Currency: value.Currency,
			ArchivedAt: value.ArchivedAt, Native: native, Base: base,
		})
		if err := addAmounts(&projection.Summary.BalanceBaseMinor, base.Posted, base.Pending); err != nil {
			return Projection{}, err
		}
	}
	for _, value := range snapshot.Categories {
		if !validUUID(value.ID) || value.Name == "" || !value.Kind.Valid() ||
			(value.ParentID != nil && !validUUID(*value.ParentID)) {
			return Projection{}, ErrInvalidData
		}
		directPosted, directPending := value.DirectPostedSignedMinor, value.DirectPendingSignedMinor
		rolledPosted, rolledPending := value.RolledPostedSignedMinor, value.RolledPendingSignedMinor
		if value.Kind == CategoryExpense {
			var err error
			directPosted, err = negate(directPosted)
			if err != nil {
				return Projection{}, err
			}
			directPending, err = negate(directPending)
			if err != nil {
				return Projection{}, err
			}
			rolledPosted, err = negate(rolledPosted)
			if err != nil {
				return Projection{}, err
			}
			rolledPending, err = negate(rolledPending)
			if err != nil {
				return Projection{}, err
			}
		}
		direct, err := amounts(directPosted, directPending)
		if err != nil {
			return Projection{}, err
		}
		rolled, err := amounts(rolledPosted, rolledPending)
		if err != nil {
			return Projection{}, err
		}
		projection.Categories = append(projection.Categories, Category{
			ID: value.ID, ParentID: value.ParentID, Name: value.Name, Kind: value.Kind,
			SystemKey: value.SystemKey, PredefinedKey: value.PredefinedKey,
			Icon: value.Icon, IconType: value.IconType, IconValue: value.IconValue,
			ColorKey: value.ColorKey, ArchivedAt: value.ArchivedAt,
			Direct: direct, RolledUp: rolled,
		})
		target := &projection.Summary.IncomeBaseMinor
		if value.Kind == CategoryExpense {
			target = &projection.Summary.SpendingBaseMinor
		}
		if err := addAmounts(target, direct.Posted, direct.Pending); err != nil {
			return Projection{}, err
		}
	}
	return projection, nil
}

func validatePeriod(period Period) error {
	if period.Timezone == "" || !period.BaseCurrency.Valid() || period.FromDate.IsZero() ||
		period.ToDate.IsZero() || dateOnly(period.FromDate).After(dateOnly(period.ToDate)) {
		return ErrInvalidData
	}
	if _, err := time.LoadLocation(period.Timezone); err != nil {
		return ErrInvalidData
	}
	return nil
}

func validQuery(query Query) bool {
	if (query.FromDate == nil) != (query.ToDate == nil) {
		return false
	}
	if query.FromDate == nil {
		return true
	}
	return !query.FromDate.IsZero() && !query.ToDate.IsZero() &&
		!dateOnly(*query.FromDate).After(dateOnly(*query.ToDate))
}

func amounts(posted, pending int64) (Amounts, error) {
	projected, err := add(posted, pending)
	if err != nil {
		return Amounts{}, err
	}
	return Amounts{Posted: posted, Pending: pending, Projected: projected}, nil
}

func addAmounts(target *Amounts, posted, pending int64) error {
	nextPosted, err := add(target.Posted, posted)
	if err != nil {
		return err
	}
	nextPending, err := add(target.Pending, pending)
	if err != nil {
		return err
	}
	nextProjected, err := add(nextPosted, nextPending)
	if err != nil {
		return err
	}
	target.Posted = nextPosted
	target.Pending = nextPending
	target.Projected = nextProjected
	return nil
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
