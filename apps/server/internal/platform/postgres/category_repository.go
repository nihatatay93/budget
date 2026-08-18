package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/category"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
)

type CategoryRepository struct {
	pool *pgxpool.Pool
}

func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

func (r *CategoryRepository) List(
	ctx context.Context,
	workspaceID string,
	includeArchived bool,
) ([]category.Category, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlc.New(r.pool).ListCategories(ctx, sqlc.ListCategoriesParams{
		WorkspaceID: workspaceUUID, IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	categories := make([]category.Category, 0, len(rows))
	for _, row := range rows {
		value, err := categoryFromDatabase(
			row.ID, row.WorkspaceID, row.ParentID, row.Name, row.Kind,
			row.SystemKey, row.Icon, row.ArchivedAt,
		)
		if err != nil {
			return nil, err
		}
		categories = append(categories, value)
	}
	return categories, nil
}

func (r *CategoryRepository) Get(
	ctx context.Context,
	workspaceID, categoryID string,
) (category.Category, error) {
	workspaceUUID, categoryUUID, err := resourceUUIDs(workspaceID, categoryID)
	if err != nil {
		return category.Category{}, err
	}
	row, err := sqlc.New(r.pool).GetCategory(ctx, sqlc.GetCategoryParams{
		WorkspaceID: workspaceUUID, ID: categoryUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return category.Category{}, category.ErrNotFound
	}
	if err != nil {
		return category.Category{}, fmt.Errorf("get category: %w", err)
	}
	return categoryFromDatabase(
		row.ID, row.WorkspaceID, row.ParentID, row.Name, row.Kind,
		row.SystemKey, row.Icon, row.ArchivedAt,
	)
}

func (r *CategoryRepository) Create(ctx context.Context, value category.Category) (category.Category, error) {
	workspaceUUID, categoryUUID, err := resourceUUIDs(value.WorkspaceID, value.ID)
	if err != nil {
		return category.Category{}, err
	}
	if err := sqlc.New(r.pool).CreateCategory(ctx, sqlc.CreateCategoryParams{
		ID: categoryUUID, WorkspaceID: workspaceUUID, ParentID: postgresUUIDPointer(value.ParentID),
		Name: value.Name, Kind: string(value.Kind), Icon: postgresText(value.Icon),
	}); err != nil {
		return category.Category{}, mapCategoryError("create category", err)
	}
	return r.Get(ctx, value.WorkspaceID, value.ID)
}

func (r *CategoryRepository) Update(
	ctx context.Context,
	workspaceID, categoryID string,
	input category.WriteInput,
) (category.Category, error) {
	workspaceUUID, categoryUUID, err := resourceUUIDs(workspaceID, categoryID)
	if err != nil {
		return category.Category{}, err
	}
	rows, err := sqlc.New(r.pool).UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		ParentID: postgresUUIDPointer(input.ParentID), Name: input.Name, Kind: string(input.Kind),
		Icon: postgresText(input.Icon), WorkspaceID: workspaceUUID, ID: categoryUUID,
	})
	if err != nil {
		return category.Category{}, mapCategoryError("update category", err)
	}
	if rows == 0 {
		return category.Category{}, category.ErrNotFound
	}
	return r.Get(ctx, workspaceID, categoryID)
}

func (r *CategoryRepository) Archive(ctx context.Context, workspaceID, categoryID string) error {
	workspaceUUID, categoryUUID, err := resourceUUIDs(workspaceID, categoryID)
	if err != nil {
		return err
	}
	rows, err := sqlc.New(r.pool).ArchiveCategory(ctx, sqlc.ArchiveCategoryParams{
		WorkspaceID: workspaceUUID, ID: categoryUUID,
	})
	if err != nil {
		return mapCategoryError("archive category", err)
	}
	if rows == 0 {
		return category.ErrNotFound
	}
	return nil
}

func (r *CategoryRepository) HasChildren(ctx context.Context, workspaceID, categoryID string) (bool, error) {
	workspaceUUID, categoryUUID, err := resourceUUIDs(workspaceID, categoryID)
	if err != nil {
		return false, err
	}
	result, err := sqlc.New(r.pool).CategoryHasChildren(ctx, sqlc.CategoryHasChildrenParams{
		WorkspaceID: workspaceUUID, ParentID: categoryUUID,
	})
	if err != nil {
		return false, fmt.Errorf("check category children: %w", err)
	}
	return result, nil
}

func (r *CategoryRepository) HasAnyChildren(ctx context.Context, workspaceID, categoryID string) (bool, error) {
	workspaceUUID, categoryUUID, err := resourceUUIDs(workspaceID, categoryID)
	if err != nil {
		return false, err
	}
	result, err := sqlc.New(r.pool).CategoryHasAnyChildren(ctx, sqlc.CategoryHasAnyChildrenParams{
		WorkspaceID: workspaceUUID, ParentID: categoryUUID,
	})
	if err != nil {
		return false, fmt.Errorf("check any category children: %w", err)
	}
	return result, nil
}

func (r *CategoryRepository) HasAllocations(ctx context.Context, workspaceID, categoryID string) (bool, error) {
	workspaceUUID, categoryUUID, err := resourceUUIDs(workspaceID, categoryID)
	if err != nil {
		return false, err
	}
	result, err := sqlc.New(r.pool).CategoryHasAllocations(ctx, sqlc.CategoryHasAllocationsParams{
		WorkspaceID: workspaceUUID, CategoryID: categoryUUID,
	})
	if err != nil {
		return false, fmt.Errorf("check category allocations: %w", err)
	}
	return result, nil
}

func categoryFromDatabase(
	id, workspaceID, parentID pgtype.UUID,
	name, kind string,
	systemKey, icon pgtype.Text,
	archived pgtype.Timestamptz,
) (category.Category, error) {
	idString, err := stringUUID(id)
	if err != nil {
		return category.Category{}, err
	}
	workspaceIDString, err := stringUUID(workspaceID)
	if err != nil {
		return category.Category{}, err
	}
	return category.Category{
		ID: idString, WorkspaceID: workspaceIDString, ParentID: uuidStringPointer(parentID),
		Name: name, Kind: category.Kind(kind), SystemKey: stringPointer(systemKey),
		Icon: stringPointer(icon), ArchivedAt: timePointer(archived),
	}, nil
}

func postgresUUIDPointer(value *string) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	id, err := postgresUUID(*value)
	if err != nil {
		return pgtype.UUID{}
	}
	return id
}

func uuidStringPointer(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	result, err := stringUUID(value)
	if err != nil {
		return nil
	}
	return &result
}

func mapCategoryError(action string, err error) error {
	switch {
	case constraintViolation(err, "categories_system_protected"):
		return category.ErrProtected
	case constraintViolation(err, "categories_parent_compatible"),
		constraintViolation(err, "categories_hierarchy_acyclic"),
		constraintViolation(err, "budget_items_non_overlapping_categories"):
		return category.ErrHierarchyConflict
	case constraintViolation(err, "categories_kind_with_relationships"):
		return category.ErrKindLocked
	case constraintViolation(err, "categories_active_children"):
		return category.ErrHasChildren
	default:
		return fmt.Errorf("%s: %w", action, err)
	}
}
