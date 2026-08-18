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
	ID          string
	WorkspaceID string
	ParentID    *string
	Name        string
	Kind        Kind
	SystemKey   *string
	Icon        *string
	ArchivedAt  *time.Time
}

type WriteInput struct {
	Name     string
	Kind     Kind
	ParentID *string
	Icon     *string
}

type Repository interface {
	List(context.Context, string, bool) ([]Category, error)
	Get(context.Context, string, string) (Category, error)
	Create(context.Context, Category) (Category, error)
	Update(context.Context, string, string, WriteInput) (Category, error)
	Archive(context.Context, string, string) error
	HasChildren(context.Context, string, string) (bool, error)
	HasAnyChildren(context.Context, string, string) (bool, error)
	HasAllocations(context.Context, string, string) (bool, error)
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
		Name: input.Name, Kind: input.Kind, Icon: input.Icon,
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
	if input.Icon != nil {
		icon := strings.TrimSpace(*input.Icon)
		if icon == "" || utf8.RuneCountInString(icon) > 64 {
			return WriteInput{}, ErrInvalidInput
		}
		input.Icon = &icon
	}
	return input, nil
}

func validID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
