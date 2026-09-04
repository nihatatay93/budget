package category

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/nihatatay93/budget/internal/workspace"
)

var (
	ErrInvalidInput      = errors.New("invalid category input")
	ErrNotFound          = errors.New("category not found")
	ErrProtected         = errors.New("system category is protected")
	ErrHierarchyConflict = errors.New("category hierarchy conflicts with the request")
	ErrKindLocked        = errors.New("category kind is locked by existing relationships")
	ErrHasChildren       = errors.New("category has active children")
)

type Kind string

const (
	KindExpense Kind = "expense"
	KindIncome  Kind = "income"
)

func (k Kind) Valid() bool {
	return k == KindExpense || k == KindIncome
}

type Category struct {
	ID            string
	WorkspaceID   string
	ParentID      *string
	Name          string
	Kind          Kind
	SystemKey     *string
	PredefinedKey *string
	Appearance    Appearance
	Icon          *string
	ArchivedAt    *time.Time
}

type WriteInput struct {
	Name       string
	Kind       Kind
	ParentID   *string
	Icon       *string
	IconType   *string
	IconValue  *string
	ColorKey   *string
	Appearance Appearance
}

type Repository interface {
	List(context.Context, string, bool) ([]Category, error)
	EnsurePredefined(context.Context, string) error
	Get(context.Context, string, string) (Category, error)
	Create(context.Context, Category) (Category, error)
	Update(context.Context, string, string, WriteInput) (Category, error)
	Archive(context.Context, string, string) error
	HasChildren(context.Context, string, string) (bool, error)
	HasAnyChildren(context.Context, string, string) (bool, error)
	HasAllocations(context.Context, string, string) (bool, error)
}

// PredefinedCategory is workspace-owned metadata for a built-in reporting category. Its
// key and appearance use stable semantic values; clients resolve its localized display name.
//
// A predefined category with an empty ParentKey is a group: the heading a client renders its
// members under, and an ordinary category in every other respect, so a workspace can rename,
// recolour, or spend directly against it. Members carry their group's colour on purpose — a
// section reads as one band of colour, and a composition chart ranks the groups rather than
// two dozen equally weighted categories.
type PredefinedCategory struct {
	Key        string
	Kind       Kind
	ParentKey  string
	Appearance Appearance
}

// Groups precede their members, because seeding resolves a member's parent by looking its
// group up in the same workspace. PredefinedCategoriesAreOrdered guards that in a test.
var predefinedCategories = []PredefinedCategory{
	{Key: "group_food", Kind: KindExpense, Appearance: systemAppearance("utensils", "blue")},
	{Key: "groceries", Kind: KindExpense, ParentKey: "group_food", Appearance: systemAppearance("shopping-cart", "blue")},
	{Key: "dining", Kind: KindExpense, ParentKey: "group_food", Appearance: systemAppearance("utensils", "blue")},

	{Key: "group_home", Kind: KindExpense, Appearance: systemAppearance("home", "orange")},
	{Key: "housing", Kind: KindExpense, ParentKey: "group_home", Appearance: systemAppearance("home", "orange")},
	{Key: "utilities", Kind: KindExpense, ParentKey: "group_home", Appearance: systemAppearance("receipt", "orange")},

	// Transportation and Entertainment already named these ideas, so no group is invented to
	// sit above them. A group that only restates its member adds a level without adding
	// meaning — and in Turkish the two rendered identically, as Ulaşım above Ulaşım.
	{Key: "transportation", Kind: KindExpense, Appearance: systemAppearance("car", "purple")},

	{Key: "entertainment", Kind: KindExpense, Appearance: systemAppearance("gamepad", "pink")},
	{Key: "subscriptions", Kind: KindExpense, ParentKey: "entertainment", Appearance: systemAppearance("repeat", "pink")},
	{Key: "travel", Kind: KindExpense, ParentKey: "entertainment", Appearance: systemAppearance("plane", "pink")},

	{Key: "group_lifestyle", Kind: KindExpense, Appearance: systemAppearance("heart", "red")},
	{Key: "health", Kind: KindExpense, ParentKey: "group_lifestyle", Appearance: systemAppearance("heart", "red")},
	{Key: "personal_care", Kind: KindExpense, ParentKey: "group_lifestyle", Appearance: systemAppearance("sparkles", "red")},
	{Key: "shopping", Kind: KindExpense, ParentKey: "group_lifestyle", Appearance: systemAppearance("shopping-bag", "red")},
	{Key: "gifts", Kind: KindExpense, ParentKey: "group_lifestyle", Appearance: systemAppearance("gift", "red")},
	{Key: "education", Kind: KindExpense, ParentKey: "group_lifestyle", Appearance: systemAppearance("graduation-cap", "red")},

	{Key: "other", Kind: KindExpense, Appearance: systemAppearance("ellipsis", "slate")},

	{Key: "group_income", Kind: KindIncome, Appearance: systemAppearance("wallet", "green")},
	{Key: "salary", Kind: KindIncome, ParentKey: "group_income", Appearance: systemAppearance("wallet", "green")},
	{Key: "freelance", Kind: KindIncome, ParentKey: "group_income", Appearance: systemAppearance("laptop", "green")},
	{Key: "investment", Kind: KindIncome, ParentKey: "group_income", Appearance: systemAppearance("trending-up", "green")},
	{Key: "rental_income", Kind: KindIncome, ParentKey: "group_income", Appearance: systemAppearance("building", "green")},
	{Key: "gift_income", Kind: KindIncome, ParentKey: "group_income", Appearance: systemAppearance("gift", "green")},
	{Key: "refund", Kind: KindIncome, ParentKey: "group_income", Appearance: systemAppearance("refund", "green")},
	{Key: "other_income", Kind: KindIncome, ParentKey: "group_income", Appearance: systemAppearance("wallet-more", "green")},
}

func systemAppearance(icon, color string) Appearance {
	return Appearance{IconType: IconTypeSystem, IconValue: icon, ColorKey: color}
}

func PredefinedCategories() []PredefinedCategory {
	return append([]PredefinedCategory(nil), predefinedCategories...)
}

// maxHierarchyDepth bounds ancestor traversal so unexpected stored data cannot loop forever.
const maxHierarchyDepth = 64

type Service struct {
	repository Repository
	access     *workspace.Authorizer
}

func NewService(repository Repository, access *workspace.Authorizer) *Service {
	return &Service{repository: repository, access: access}
}

func (s *Service) List(
	ctx context.Context,
	workspaceID, userID string,
	includeArchived bool,
) ([]Category, error) {
	if !validID(workspaceID) || !validID(userID) {
		return nil, ErrInvalidInput
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	if err := s.repository.EnsurePredefined(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, workspaceID, includeArchived)
}

func (s *Service) Get(ctx context.Context, workspaceID, userID, categoryID string) (Category, error) {
	if !validID(workspaceID) || !validID(userID) || !validID(categoryID) {
		return Category{}, ErrInvalidInput
	}
	if err := s.access.RequireRead(ctx, workspaceID, userID); err != nil {
		return Category{}, err
	}
	return s.repository.Get(ctx, workspaceID, categoryID)
}

func (s *Service) Create(
	ctx context.Context,
	workspaceID, userID string,
	input WriteInput,
) (Category, error) {
	input, err := normalizeWriteInput(input)
	if err != nil || !validID(workspaceID) || !validID(userID) {
		return Category{}, ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return Category{}, err
	}
	if err := s.validateParent(ctx, workspaceID, "", input); err != nil {
		return Category{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Category{}, fmt.Errorf("create category ID: %w", err)
	}
	return s.repository.Create(ctx, Category{
		ID: id.String(), WorkspaceID: workspaceID, ParentID: input.ParentID,
		Name: input.Name, Kind: input.Kind, Appearance: input.Appearance, Icon: &input.Appearance.IconValue,
	})
}

func (s *Service) Update(
	ctx context.Context,
	workspaceID, userID, categoryID string,
	input WriteInput,
) (Category, error) {
	input, err := normalizeWriteInput(input)
	if err != nil || !validID(workspaceID) || !validID(userID) || !validID(categoryID) {
		return Category{}, ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return Category{}, err
	}
	current, err := s.repository.Get(ctx, workspaceID, categoryID)
	if err != nil {
		return Category{}, err
	}
	if current.SystemKey != nil {
		return Category{}, ErrProtected
	}
	if current.PredefinedKey != nil {
		// Built-in categories have localized client-side names and a fixed reporting kind.
		// A workspace may personalize their appearance, but must not alter that identity.
		input.Name = current.Name
		input.Kind = current.Kind
		input.ParentID = current.ParentID
		return s.repository.Update(ctx, workspaceID, categoryID, input)
	}
	if err := s.validateParent(ctx, workspaceID, categoryID, input); err != nil {
		return Category{}, err
	}
	if current.Kind != input.Kind {
		// Archived children still constrain the kind, so this counts children of any state.
		hasChildren, err := s.repository.HasAnyChildren(ctx, workspaceID, categoryID)
		if err != nil {
			return Category{}, err
		}
		hasAllocations, err := s.repository.HasAllocations(ctx, workspaceID, categoryID)
		if err != nil {
			return Category{}, err
		}
		if hasChildren || hasAllocations {
			return Category{}, ErrKindLocked
		}
	}
	return s.repository.Update(ctx, workspaceID, categoryID, input)
}

func (s *Service) Archive(ctx context.Context, workspaceID, userID, categoryID string) error {
	if !validID(workspaceID) || !validID(userID) || !validID(categoryID) {
		return ErrInvalidInput
	}
	if err := s.access.RequireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	current, err := s.repository.Get(ctx, workspaceID, categoryID)
	if err != nil {
		return err
	}
	if current.SystemKey != nil {
		return ErrProtected
	}
	hasChildren, err := s.repository.HasChildren(ctx, workspaceID, categoryID)
	if err != nil {
		return err
	}
	if hasChildren {
		return ErrHasChildren
	}
	return s.repository.Archive(ctx, workspaceID, categoryID)
}

func (s *Service) validateParent(
	ctx context.Context,
	workspaceID, categoryID string,
	input WriteInput,
) error {
	if input.ParentID == nil {
		return nil
	}
	if *input.ParentID == categoryID || !validID(*input.ParentID) {
		return ErrHierarchyConflict
	}
	parent, err := s.repository.Get(ctx, workspaceID, *input.ParentID)
	if errors.Is(err, ErrNotFound) {
		return ErrHierarchyConflict
	}
	if err != nil {
		return err
	}
	if parent.Kind != input.Kind || parent.ArchivedAt != nil {
		return ErrHierarchyConflict
	}
	if categoryID == "" {
		return nil
	}
	// Walking the proposed parent's ancestors rejects deeper cycles, not only direct
	// self-parenting, so the database trigger stays a backstop rather than the only check.
	ancestor := parent
	for depth := 0; depth < maxHierarchyDepth; depth++ {
		if ancestor.ID == categoryID {
			return ErrHierarchyConflict
		}
		if ancestor.ParentID == nil {
			return nil
		}
		ancestor, err = s.repository.Get(ctx, workspaceID, *ancestor.ParentID)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return ErrHierarchyConflict
}

func normalizeWriteInput(input WriteInput) (WriteInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 || !input.Kind.Valid() {
		return WriteInput{}, ErrInvalidInput
	}
	appearance, err := normalizeAppearance(input.IconType, input.IconValue, input.ColorKey, input.Icon)
	if err != nil {
		return WriteInput{}, err
	}
	input.Appearance = appearance
	input.Icon = &appearance.IconValue
	return input, nil
}

func validID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
