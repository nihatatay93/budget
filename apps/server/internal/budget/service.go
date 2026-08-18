// Package budget owns monthly budget definitions and posted-allocation-derived usage.
package budget

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/money"
	"github.com/nihatatay93/budget/internal/workspace"
)

var (
	ErrInvalidInput      = errors.New("invalid budget input")
	ErrInvalidData       = errors.New("budget source data is invalid")
	ErrNotFound          = errors.New("budget not found")
	ErrCategoryNotFound  = errors.New("budget category not found")
	ErrCategoryKind      = errors.New("budget category must be an expense category")
	ErrCategoryArchived  = errors.New("archived category cannot be newly budgeted")
	ErrCategoryDuplicate = errors.New("budget category is duplicated")
	ErrCategoryOverlap   = errors.New("budget category branches overlap")
	ErrAmountOverflow    = errors.New("budget amount exceeds int64 minor-unit range")
)

const maxItems = 200

type Month struct {
	Year  int
	Month time.Month
}

func ParseMonth(value string) (Month, error) {
	parsed, err := time.Parse("2006-01", value)
	if err != nil || parsed.Year() < 1 || parsed.Year() > 9999 {
		return Month{}, ErrInvalidInput
	}
	return Month{Year: parsed.Year(), Month: parsed.Month()}, nil
}

func (m Month) String() string {
	return fmt.Sprintf("%04d-%02d", m.Year, m.Month)
}

func (m Month) StartDate() time.Time {
	return time.Date(m.Year, m.Month, 1, 0, 0, 0, 0, time.UTC)
}

type WorkspaceSettings struct {
	Timezone     string
	BaseCurrency money.Currency
}

type ItemInput struct {
	CategoryID      string
	AmountBaseMinor int64
}

type WriteInput struct {
	Name  string
	Items []ItemInput
}

type CategoryRule struct {
	ID              string
	ParentID        *string
	Kind            category.Kind
	ArchivedAt      *time.Time
	AlreadyBudgeted bool
}

type ReplaceItem struct {
	ID              string
	CategoryID      string
	AmountBaseMinor int64
}

type ReplaceCommand struct {
	NewBudgetID string
	Name        string
	Items       []ReplaceItem
}

type ItemSnapshot struct {
	ID                        string
	CategoryID                string
	CategoryName              string
	CategoryIcon              *string
	CategoryArchivedAt        *time.Time
	PlannedBaseMinor          int64
	SignedAllocationBaseMinor int64
}

type Snapshot struct {
	ID           string
	WorkspaceID  string
	Name         string
	StartsOn     time.Time
	Timezone     string
	BaseCurrency money.Currency
	Items        []ItemSnapshot
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Item struct {
	ID                 string
	CategoryID         string
	CategoryName       string
	CategoryIcon       *string
	CategoryArchivedAt *time.Time
	PlannedBaseMinor   int64
	UsedBaseMinor      int64
	RemainingBaseMinor int64
}

type Budget struct {
	ID                 string
	WorkspaceID        string
	Name               string
	Month              string
	Timezone           string
	BaseCurrency       money.Currency
	PlannedBaseMinor   int64
	UsedBaseMinor      int64
	RemainingBaseMinor int64
	Items              []Item
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Repository interface {
	WorkspaceSettings(context.Context, string) (WorkspaceSettings, error)
	CategoriesForMonth(context.Context, string, Month) ([]CategoryRule, error)
	Get(context.Context, string, Month) (Snapshot, error)
	Replace(context.Context, string, Month, ReplaceCommand) (Snapshot, error)
}

type Service struct {
	repository Repository
	access     *workspace.Authorizer
	now        func() time.Time
}

func NewService(repository Repository, access *workspace.Authorizer, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, access: access, now: now}
}

func (s *Service) Get(
	ctx context.Context,
	workspaceID, userID string,
	requestedMonth *string,
) (Budget, error) {
	if !validID(workspaceID) || !validID(userID) {
		return Budget{}, ErrInvalidInput
	}
	var month Month
	if requestedMonth != nil {
		var err error
		month, err = ParseMonth(*requestedMonth)
		if err != nil {
			return Budget{}, err
		}
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return Budget{}, err
	}
	if requestedMonth == nil {
		settings, err := s.repository.WorkspaceSettings(ctx, workspaceID)
		if err != nil {
			return Budget{}, err
		}
		month, err = currentMonth(settings, s.now())
		if err != nil {
			return Budget{}, err
		}
	}
	snapshot, err := s.repository.Get(ctx, workspaceID, month)
	if err != nil {
		return Budget{}, err
	}
	return buildBudget(snapshot, workspaceID, month)
}

func (s *Service) Replace(
	ctx context.Context,
	workspaceID, userID, monthValue string,
	input WriteInput,
) (Budget, error) {
	month, err := ParseMonth(monthValue)
	if err != nil || !validID(workspaceID) || !validID(userID) {
		return Budget{}, ErrInvalidInput
	}
	input, err = normalizeWriteInput(input)
	if err != nil {
		return Budget{}, err
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return Budget{}, err
	}
	rules, err := s.repository.CategoriesForMonth(ctx, workspaceID, month)
	if err != nil {
		return Budget{}, err
	}
	if err := validateCategories(input.Items, rules); err != nil {
		return Budget{}, err
	}
	command, err := replaceCommand(input)
	if err != nil {
		return Budget{}, err
	}
	snapshot, err := s.repository.Replace(ctx, workspaceID, month, command)
	if err != nil {
		return Budget{}, err
	}
	return buildBudget(snapshot, workspaceID, month)
}

func currentMonth(settings WorkspaceSettings, now time.Time) (Month, error) {
	if !settings.BaseCurrency.Valid() || settings.Timezone == "" {
		return Month{}, ErrInvalidData
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return Month{}, ErrInvalidData
	}
	local := now.In(location)
	return Month{Year: local.Year(), Month: local.Month()}, nil
}

func normalizeWriteInput(input WriteInput) (WriteInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 ||
		len(input.Items) == 0 || len(input.Items) > maxItems {
		return WriteInput{}, ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(input.Items))
	var plannedTotal int64
	for _, item := range input.Items {
		if !validID(item.CategoryID) || item.AmountBaseMinor <= 0 {
			return WriteInput{}, ErrInvalidInput
		}
		if _, exists := seen[item.CategoryID]; exists {
			return WriteInput{}, ErrCategoryDuplicate
		}
		seen[item.CategoryID] = struct{}{}
		if err := add(&plannedTotal, item.AmountBaseMinor); err != nil {
			return WriteInput{}, err
		}
	}
	return input, nil
}

func validateCategories(items []ItemInput, rules []CategoryRule) error {
	byID := make(map[string]CategoryRule, len(rules))
	for _, rule := range rules {
		if !validID(rule.ID) || (rule.ParentID != nil && !validID(*rule.ParentID)) ||
			!rule.Kind.Valid() {
			return ErrInvalidData
		}
		if _, exists := byID[rule.ID]; exists {
			return ErrInvalidData
		}
		byID[rule.ID] = rule
	}
	selected := make(map[string]struct{}, len(items))
	for _, item := range items {
		rule, exists := byID[item.CategoryID]
		if !exists {
			return ErrCategoryNotFound
		}
		if rule.Kind != category.KindExpense {
			return ErrCategoryKind
		}
		if rule.ArchivedAt != nil && !rule.AlreadyBudgeted {
			return ErrCategoryArchived
		}
		selected[item.CategoryID] = struct{}{}
	}
	for categoryID := range selected {
		visited := map[string]struct{}{categoryID: {}}
		parentID := byID[categoryID].ParentID
		for parentID != nil {
			if _, exists := selected[*parentID]; exists {
				return ErrCategoryOverlap
			}
			if _, exists := visited[*parentID]; exists {
				return ErrInvalidData
			}
			visited[*parentID] = struct{}{}
			parent, exists := byID[*parentID]
			if !exists {
				return ErrInvalidData
			}
			parentID = parent.ParentID
		}
	}
	return nil
}

func replaceCommand(input WriteInput) (ReplaceCommand, error) {
	budgetID, err := uuid.NewV7()
	if err != nil {
		return ReplaceCommand{}, fmt.Errorf("create budget ID: %w", err)
	}
	command := ReplaceCommand{
		NewBudgetID: budgetID.String(), Name: input.Name,
		Items: make([]ReplaceItem, 0, len(input.Items)),
	}
	for _, item := range input.Items {
		itemID, err := uuid.NewV7()
		if err != nil {
			return ReplaceCommand{}, fmt.Errorf("create budget item ID: %w", err)
		}
		command.Items = append(command.Items, ReplaceItem{
			ID: itemID.String(), CategoryID: item.CategoryID,
			AmountBaseMinor: item.AmountBaseMinor,
		})
	}
	return command, nil
}

func buildBudget(snapshot Snapshot, workspaceID string, month Month) (Budget, error) {
	if !validID(snapshot.ID) || snapshot.WorkspaceID != workspaceID || snapshot.Name == "" ||
		!snapshot.BaseCurrency.Valid() || snapshot.Timezone == "" || snapshot.CreatedAt.IsZero() ||
		snapshot.UpdatedAt.IsZero() || !sameDate(snapshot.StartsOn, month.StartDate()) {
		return Budget{}, ErrInvalidData
	}
	if _, err := time.LoadLocation(snapshot.Timezone); err != nil {
		return Budget{}, ErrInvalidData
	}
	result := Budget{
		ID: snapshot.ID, WorkspaceID: snapshot.WorkspaceID, Name: snapshot.Name,
		Month: month.String(), Timezone: snapshot.Timezone, BaseCurrency: snapshot.BaseCurrency,
		Items: make([]Item, 0, len(snapshot.Items)), CreatedAt: snapshot.CreatedAt,
		UpdatedAt: snapshot.UpdatedAt,
	}
	seen := make(map[string]struct{}, len(snapshot.Items))
	for _, value := range snapshot.Items {
		if !validID(value.ID) || !validID(value.CategoryID) || value.CategoryName == "" ||
			value.PlannedBaseMinor <= 0 {
			return Budget{}, ErrInvalidData
		}
		if _, exists := seen[value.CategoryID]; exists {
			return Budget{}, ErrInvalidData
		}
		seen[value.CategoryID] = struct{}{}
		used, err := negate(value.SignedAllocationBaseMinor)
		if err != nil {
			return Budget{}, err
		}
		remaining, err := subtract(value.PlannedBaseMinor, used)
		if err != nil {
			return Budget{}, err
		}
		result.Items = append(result.Items, Item{
			ID: value.ID, CategoryID: value.CategoryID, CategoryName: value.CategoryName,
			CategoryIcon: value.CategoryIcon, CategoryArchivedAt: value.CategoryArchivedAt,
			PlannedBaseMinor: value.PlannedBaseMinor, UsedBaseMinor: used,
			RemainingBaseMinor: remaining,
		})
		if err := add(&result.PlannedBaseMinor, value.PlannedBaseMinor); err != nil {
			return Budget{}, err
		}
		if err := add(&result.UsedBaseMinor, used); err != nil {
			return Budget{}, err
		}
	}
	remaining, err := subtract(result.PlannedBaseMinor, result.UsedBaseMinor)
	if err != nil {
		return Budget{}, err
	}
	result.RemainingBaseMinor = remaining
	return result, nil
}

func add(target *int64, value int64) error {
	if (value > 0 && *target > math.MaxInt64-value) ||
		(value < 0 && *target < math.MinInt64-value) {
		return ErrAmountOverflow
	}
	*target += value
	return nil
}

func subtract(left, right int64) (int64, error) {
	if (right < 0 && left > math.MaxInt64+right) ||
		(right > 0 && left < math.MinInt64+right) {
		return 0, ErrAmountOverflow
	}
	return left - right, nil
}

func negate(value int64) (int64, error) {
	if value == math.MinInt64 {
		return 0, ErrAmountOverflow
	}
	return -value, nil
}

func sameDate(left, right time.Time) bool {
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
}

func validID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
