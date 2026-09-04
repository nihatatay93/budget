package category

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/workspace"
)

const (
	testWorkspaceID = "018f47da-0af1-7a2f-8c35-165c89772d5b"
	testUserID      = "018f47da-0af1-7a2f-8c35-165c89772d5c"
	testCategoryID  = "018f47da-0af1-7a2f-8c35-165c89772d5d"
	testParentID    = "018f47da-0af1-7a2f-8c35-165c89772d5e"
)

type categoryMembershipStub struct{ role workspace.Role }

func (s categoryMembershipStub) MemberRole(context.Context, string, string) (workspace.Role, error) {
	return s.role, nil
}

type categoryRepositoryStub struct {
	current        Category
	parent         Category
	hasChildren    bool
	hasAnyChildren bool
	hasAllocations bool
	archived       bool
	ensured        int
	updated        *WriteInput
}

func (s *categoryRepositoryStub) List(context.Context, string, bool) ([]Category, error) {
	return nil, nil
}
func (s *categoryRepositoryStub) EnsurePredefined(context.Context, string) error {
	s.ensured++
	return nil
}
func (s *categoryRepositoryStub) Get(_ context.Context, _ string, id string) (Category, error) {
	if id == testParentID {
		return s.parent, nil
	}
	return s.current, nil
}
func (s *categoryRepositoryStub) Create(_ context.Context, value Category) (Category, error) {
	return value, nil
}
func (s *categoryRepositoryStub) Update(
	_ context.Context, _, _ string, input WriteInput,
) (Category, error) {
	s.updated = &input
	return Category{Name: input.Name, Kind: input.Kind, ParentID: input.ParentID}, nil
}
func (s *categoryRepositoryStub) Archive(context.Context, string, string) error {
	s.archived = true
	return nil
}
func (s *categoryRepositoryStub) HasChildren(context.Context, string, string) (bool, error) {
	return s.hasChildren, nil
}
func (s *categoryRepositoryStub) HasAnyChildren(context.Context, string, string) (bool, error) {
	return s.hasAnyChildren, nil
}
func (s *categoryRepositoryStub) HasAllocations(context.Context, string, string) (bool, error) {
	return s.hasAllocations, nil
}

func newCategoryService(repository Repository) *Service {
	return NewService(
		repository,
		workspace.NewAuthorizer(categoryMembershipStub{role: workspace.RoleOwner}),
	)
}

func TestListEnsuresPredefinedCategories(t *testing.T) {
	repository := &categoryRepositoryStub{}
	_, err := newCategoryService(repository).List(context.Background(), testWorkspaceID, testUserID, false)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repository.ensured != 1 {
		t.Fatalf("EnsurePredefined() calls = %d, want 1", repository.ensured)
	}
}

func TestPredefinedCategoriesUseStableSemanticMetadata(t *testing.T) {
	values := PredefinedCategories()
	if len(values) != 25 {
		t.Fatalf("predefined category count = %d, want 25", len(values))
	}
	for _, value := range values {
		if value.Key == "" || !value.Kind.Valid() || !value.Appearance.Valid() {
			t.Fatalf("invalid predefined category: %+v", value)
		}
	}
	appearances := map[string]Appearance{}
	for _, value := range values {
		appearances[value.Key] = value.Appearance
	}
	for key, want := range map[string]Appearance{
		"group_food":    systemAppearance("utensils", "blue"),
		"housing":       systemAppearance("home", "orange"),
		"personal_care": systemAppearance("sparkles", "red"),
		"investment":    systemAppearance("trending-up", "green"),
		"refund":        systemAppearance("refund", "green"),
		"other_income":  systemAppearance("wallet-more", "green"),
	} {
		if got := appearances[key]; got != want {
			t.Fatalf("%s appearance = %#v, want %#v", key, got, want)
		}
	}
	values[0].Key = "changed"
	if PredefinedCategories()[0].Key == "changed" {
		t.Fatal("PredefinedCategories() returned the internal slice")
	}
}

// Seeding resolves a member's parent by looking the group up in the same workspace, so a group
// listed after one of its members would silently produce a category with no section.
func TestPredefinedCategoriesListEachGroupBeforeItsMembers(t *testing.T) {
	seen := map[string]Kind{}
	groups := 0
	for _, value := range PredefinedCategories() {
		if value.ParentKey == "" {
			groups++
			seen[value.Key] = value.Kind
			continue
		}
		kind, ok := seen[value.ParentKey]
		if !ok {
			t.Fatalf("%s names group %s, which is not listed before it", value.Key, value.ParentKey)
		}
		// A parent must share its children's kind, which is the rule the category service
		// enforces for a hierarchy a person builds by hand.
		if kind != value.Kind {
			t.Fatalf("%s is %s but its group %s is %s", value.Key, value.Kind, value.ParentKey, kind)
		}
	}
	// Four invented groups plus Entertainment, which was already the name of this idea, and
	// the two categories that stand alone at the top level.
	if groups != 7 {
		t.Fatalf("root count = %d, want 7", groups)
	}
}

// Every member wears its group's colour, so a section reads as one band rather than a scatter.
func TestPredefinedMembersCarryTheirGroupColour(t *testing.T) {
	colours := map[string]string{}
	for _, value := range PredefinedCategories() {
		if value.ParentKey == "" {
			colours[value.Key] = value.Appearance.ColorKey
		}
	}
	for _, value := range PredefinedCategories() {
		if value.ParentKey == "" {
			continue
		}
		if got, want := value.Appearance.ColorKey, colours[value.ParentKey]; got != want {
			t.Fatalf("%s colour = %s, want its group %s colour %s", value.Key, got, value.ParentKey, want)
		}
	}
}

func TestCreateRejectsParentWithDifferentKind(t *testing.T) {
	repository := &categoryRepositoryStub{
		parent: Category{ID: testParentID, WorkspaceID: testWorkspaceID, Kind: KindIncome},
	}
	service := newCategoryService(repository)
	parentID := testParentID
	_, err := service.Create(context.Background(), testWorkspaceID, testUserID, WriteInput{
		Name: "Dining", Kind: KindExpense, ParentID: &parentID,
	})
	if !errors.Is(err, ErrHierarchyConflict) {
		t.Fatalf("Create() error = %v, want ErrHierarchyConflict", err)
	}
}

func TestUpdateRejectsProtectedCategory(t *testing.T) {
	systemKey := "uncategorized_expense"
	repository := &categoryRepositoryStub{current: Category{
		ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense, SystemKey: &systemKey,
	}}
	service := newCategoryService(repository)
	_, err := service.Update(context.Background(), testWorkspaceID, testUserID, testCategoryID, WriteInput{
		Name: "Anything", Kind: KindExpense,
	})
	if !errors.Is(err, ErrProtected) {
		t.Fatalf("Update() error = %v, want ErrProtected", err)
	}
}

func TestUpdatePredefinedCategoryPreservesIdentityAndChangesAppearance(t *testing.T) {
	predefinedKey := "groceries"
	parentID := testParentID
	iconType, iconValue, colorKey := "emoji", "🛒", "purple"
	repository := &categoryRepositoryStub{current: Category{
		ID: testCategoryID, WorkspaceID: testWorkspaceID, Name: "Groceries", Kind: KindExpense,
		PredefinedKey: &predefinedKey,
	}}

	_, err := newCategoryService(repository).Update(
		context.Background(), testWorkspaceID, testUserID, testCategoryID,
		WriteInput{
			Name: "Renamed", Kind: KindIncome, ParentID: &parentID,
			IconType: &iconType, IconValue: &iconValue, ColorKey: &colorKey,
		},
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repository.updated == nil {
		t.Fatal("Update() did not persist the appearance")
	}
	if repository.updated.Name != "Groceries" || repository.updated.Kind != KindExpense || repository.updated.ParentID != nil {
		t.Fatalf("predefined identity changed: %#v", repository.updated)
	}
	if got := repository.updated.Appearance; got != (Appearance{IconType: IconTypeEmoji, IconValue: "🛒", ColorKey: "purple"}) {
		t.Fatalf("appearance = %#v", got)
	}
}

func TestArchiveRejectsCategoryWithActiveChildren(t *testing.T) {
	repository := &categoryRepositoryStub{
		current:     Category{ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
		hasChildren: true,
	}
	service := newCategoryService(repository)
	err := service.Archive(context.Background(), testWorkspaceID, testUserID, testCategoryID)
	if !errors.Is(err, ErrHasChildren) || repository.archived {
		t.Fatalf("Archive() error = %v, archived = %v", err, repository.archived)
	}
}

func TestUpdateRejectsKindChangeWithArchivedChildren(t *testing.T) {
	repository := &categoryRepositoryStub{
		current:        Category{ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
		hasAnyChildren: true,
	}
	service := newCategoryService(repository)
	_, err := service.Update(context.Background(), testWorkspaceID, testUserID, testCategoryID, WriteInput{
		Name: "Consulting", Kind: KindIncome,
	})
	if !errors.Is(err, ErrKindLocked) {
		t.Fatalf("Update() error = %v, want ErrKindLocked", err)
	}
}

func TestUpdateRejectsParentThatIsADescendant(t *testing.T) {
	categoryID := testCategoryID
	repository := &categoryRepositoryStub{
		current: Category{ID: categoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
		parent: Category{
			ID: testParentID, WorkspaceID: testWorkspaceID, Kind: KindExpense, ParentID: &categoryID,
		},
	}
	service := newCategoryService(repository)
	parentID := testParentID
	_, err := service.Update(context.Background(), testWorkspaceID, testUserID, categoryID, WriteInput{
		Name: "Food", Kind: KindExpense, ParentID: &parentID,
	})
	if !errors.Is(err, ErrHierarchyConflict) {
		t.Fatalf("Update() error = %v, want ErrHierarchyConflict", err)
	}
}

func TestUpdateRejectsKindChangeWithAllocations(t *testing.T) {
	repository := &categoryRepositoryStub{
		current:        Category{ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
		hasAllocations: true,
	}
	service := newCategoryService(repository)
	_, err := service.Update(context.Background(), testWorkspaceID, testUserID, testCategoryID, WriteInput{
		Name: "Consulting", Kind: KindIncome,
	})
	if !errors.Is(err, ErrKindLocked) {
		t.Fatalf("Update() error = %v, want ErrKindLocked", err)
	}
}

func newCategoryServiceAs(repository Repository, role workspace.Role) *Service {
	return NewService(repository, workspace.NewAuthorizer(categoryMembershipStub{role: role}))
}

// Reads require membership; writes require a managing role.
func TestCategoryServiceEnforcesMembershipOnEveryPath(t *testing.T) {
	ctx := context.Background()
	input := WriteInput{Name: "Food", Kind: KindExpense}
	repository := &categoryRepositoryStub{
		current: Category{ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
	}

	viewer := newCategoryServiceAs(repository, workspace.RoleViewer)
	if _, err := viewer.List(ctx, testWorkspaceID, testUserID, false); err != nil {
		t.Fatalf("viewer List() error = %v, want success", err)
	}
	if _, err := viewer.Create(ctx, testWorkspaceID, testUserID, input); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("viewer Create() error = %v, want ErrForbidden", err)
	}
	if err := viewer.Archive(ctx, testWorkspaceID, testUserID, testCategoryID); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("viewer Archive() error = %v, want ErrForbidden", err)
	}

	stranger := newCategoryServiceAs(repository, "")
	if _, err := stranger.List(ctx, testWorkspaceID, testUserID, false); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("non-member List() error = %v, want ErrForbidden", err)
	}
	if _, err := stranger.Get(ctx, testWorkspaceID, testUserID, testCategoryID); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("non-member Get() error = %v, want ErrForbidden", err)
	}
}

func TestCategoryServiceRejectsMalformedIdentifiers(t *testing.T) {
	ctx := context.Background()
	service := newCategoryService(&categoryRepositoryStub{})
	input := WriteInput{Name: "Food", Kind: KindExpense}

	for _, id := range []string{"", "not-a-uuid"} {
		if _, err := service.List(ctx, id, testUserID, false); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("List(%q) error = %v, want ErrInvalidInput", id, err)
		}
		if _, err := service.Get(ctx, testWorkspaceID, testUserID, id); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Get(%q) error = %v, want ErrInvalidInput", id, err)
		}
		if _, err := service.Create(ctx, id, testUserID, input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Create(%q) error = %v, want ErrInvalidInput", id, err)
		}
		if err := service.Archive(ctx, testWorkspaceID, testUserID, id); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Archive(%q) error = %v, want ErrInvalidInput", id, err)
		}
	}
}

func TestCategoryNormalizeWriteInputRejectsInvalidValues(t *testing.T) {
	longName := ""
	for range 101 {
		longName += "a"
	}
	longIcon := ""
	for range 65 {
		longIcon += "i"
	}
	tests := map[string]WriteInput{
		"empty name":    {Name: "   ", Kind: KindExpense},
		"name too long": {Name: longName, Kind: KindExpense},
		"unknown kind":  {Name: "Food", Kind: Kind("asset")},
		"empty kind":    {Name: "Food", Kind: ""},
		"icon too long": {Name: "Food", Kind: KindExpense, Icon: &longIcon},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeWriteInput(input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("normalizeWriteInput() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCategoryNormalizeWriteInputTrims(t *testing.T) {
	icon := "  🍔  "
	got, err := normalizeWriteInput(WriteInput{Name: "  Food  ", Kind: KindExpense, Icon: &icon})
	if err != nil {
		t.Fatalf("normalizeWriteInput() error = %v", err)
	}
	if got.Name != "Food" || *got.Icon != "🍔" || got.Appearance.IconType != IconTypeEmoji || got.Appearance.ColorKey != "slate" {
		t.Fatalf("normalizeWriteInput() = %#v", got)
	}
}

func TestCategoryAppearanceValidation(t *testing.T) {
	iconType, iconValue, color := "system", "home", "blue"
	got, err := normalizeWriteInput(WriteInput{
		Name: "Housing", Kind: KindExpense, IconType: &iconType, IconValue: &iconValue, ColorKey: &color,
	})
	if err != nil || got.Appearance != (Appearance{IconType: IconTypeSystem, IconValue: "home", ColorKey: "blue"}) {
		t.Fatalf("system appearance = %#v, %v", got.Appearance, err)
	}

	emojiType, emojiValue, emojiColor := "emoji", "👩🏽‍💻", "purple"
	got, err = normalizeWriteInput(WriteInput{
		Name: "Work", Kind: KindIncome, IconType: &emojiType, IconValue: &emojiValue, ColorKey: &emojiColor,
	})
	if err != nil || got.Appearance.IconType != IconTypeEmoji || got.Appearance.IconValue != emojiValue {
		t.Fatalf("emoji appearance = %#v, %v", got.Appearance, err)
	}

	unsupportedIcon, unsupportedColor := "rocket", "beige"
	for _, input := range []WriteInput{
		{Name: "Bad", Kind: KindExpense, IconType: &iconType, IconValue: &unsupportedIcon, ColorKey: &color},
		{Name: "Bad", Kind: KindExpense, IconType: &iconType, IconValue: &iconValue, ColorKey: &unsupportedColor},
	} {
		if _, err := normalizeWriteInput(input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("unsupported appearance error = %v, want ErrInvalidInput", err)
		}
	}
}

func TestValidateParentRejectsSelfAndMissingParents(t *testing.T) {
	ctx := context.Background()
	repository := &categoryRepositoryStub{
		current: Category{ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
		parent:  Category{ID: testParentID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
	}
	service := newCategoryService(repository)

	self := testCategoryID
	if _, err := service.Update(ctx, testWorkspaceID, testUserID, testCategoryID, WriteInput{
		Name: "Food", Kind: KindExpense, ParentID: &self,
	}); !errors.Is(err, ErrHierarchyConflict) {
		t.Fatalf("self-parent error = %v, want ErrHierarchyConflict", err)
	}

	malformed := "not-a-uuid"
	if _, err := service.Update(ctx, testWorkspaceID, testUserID, testCategoryID, WriteInput{
		Name: "Food", Kind: KindExpense, ParentID: &malformed,
	}); !errors.Is(err, ErrHierarchyConflict) {
		t.Fatalf("malformed parent error = %v, want ErrHierarchyConflict", err)
	}
}

// An archived parent cannot adopt children: the database enforces it too, but the domain
// must reject it first so the caller gets a precise conflict.
func TestValidateParentRejectsArchivedParent(t *testing.T) {
	archived := time.Now()
	repository := &categoryRepositoryStub{
		current: Category{ID: testCategoryID, WorkspaceID: testWorkspaceID, Kind: KindExpense},
		parent: Category{
			ID: testParentID, WorkspaceID: testWorkspaceID, Kind: KindExpense, ArchivedAt: &archived,
		},
	}
	parentID := testParentID
	_, err := newCategoryService(repository).Create(
		context.Background(), testWorkspaceID, testUserID,
		WriteInput{Name: "Groceries", Kind: KindExpense, ParentID: &parentID},
	)
	if !errors.Is(err, ErrHierarchyConflict) {
		t.Fatalf("archived parent error = %v, want ErrHierarchyConflict", err)
	}
}

func TestKindValidCoversEveryDeclaredKind(t *testing.T) {
	for _, kind := range []Kind{KindExpense, KindIncome} {
		if !kind.Valid() {
			t.Fatalf("Valid(%q) = false", kind)
		}
	}
	for _, kind := range []Kind{"", "asset", "Expense"} {
		if kind.Valid() {
			t.Fatalf("Valid(%q) = true", kind)
		}
	}
}
