package httpapi

import (
	"context"
	"errors"

	"github.com/google/uuid"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/category"
)

func (s *server) ListCategories(
	ctx context.Context,
	request openapi.ListCategoriesRequestObject,
) (openapi.ListCategoriesResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	includeArchived := request.Params.IncludeArchived != nil && bool(*request.Params.IncludeArchived)
	values, err := s.Categories.List(
		ctx, request.WorkspaceId.String(), principal.User.ID, includeArchived,
	)
	if err != nil {
		return nil, err
	}
	response := make([]openapi.Category, 0, len(values))
	for _, value := range values {
		converted, err := categoryResponse(value)
		if err != nil {
			return nil, err
		}
		response = append(response, converted)
	}
	return openapi.ListCategories200JSONResponse{
		Body:    openapi.CategoryListResponse{Categories: response},
		Headers: openapi.ListCategories200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) CreateCategory(
	ctx context.Context,
	request openapi.CreateCategoryRequestObject,
) (openapi.CreateCategoryResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.CreateCategory400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	value, err := s.Categories.Create(
		ctx, request.WorkspaceId.String(), principal.User.ID, categoryWriteInput(*request.Body),
	)
	switch {
	case errors.Is(err, category.ErrInvalidInput):
		return openapi.CreateCategory400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	case errors.Is(err, category.ErrHierarchyConflict):
		return openapi.CreateCategory409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "category_hierarchy_conflict", "The selected parent is not valid for this category.",
		)}, nil
	case err != nil:
		return nil, err
	}
	converted, err := categoryResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.CreateCategory201JSONResponse{
		Body: converted, Headers: openapi.CreateCategory201ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) GetCategory(
	ctx context.Context,
	request openapi.GetCategoryRequestObject,
) (openapi.GetCategoryResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	value, err := s.Categories.Get(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.CategoryId.String(),
	)
	if err != nil {
		return nil, err
	}
	converted, err := categoryResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.GetCategory200JSONResponse{
		Body: converted, Headers: openapi.GetCategory200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) UpdateCategory(
	ctx context.Context,
	request openapi.UpdateCategoryRequestObject,
) (openapi.UpdateCategoryResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.UpdateCategory400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	value, err := s.Categories.Update(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.CategoryId.String(),
		categoryWriteInput(*request.Body),
	)
	switch {
	case errors.Is(err, category.ErrInvalidInput):
		return openapi.UpdateCategory400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	case errors.Is(err, category.ErrProtected):
		return openapi.UpdateCategory409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "system_category_protected", "System categories cannot be changed.",
		)}, nil
	case errors.Is(err, category.ErrHierarchyConflict):
		return openapi.UpdateCategory409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "category_hierarchy_conflict", "The selected parent is not valid for this category.",
		)}, nil
	case errors.Is(err, category.ErrKindLocked):
		return openapi.UpdateCategory409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "category_kind_locked", "Category kind cannot change while relationships exist.",
		)}, nil
	case err != nil:
		return nil, err
	}
	converted, err := categoryResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.UpdateCategory200JSONResponse{
		Body: converted, Headers: openapi.UpdateCategory200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) ArchiveCategory(
	ctx context.Context,
	request openapi.ArchiveCategoryRequestObject,
) (openapi.ArchiveCategoryResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	err = s.Categories.Archive(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.CategoryId.String(),
	)
	switch {
	case errors.Is(err, category.ErrProtected):
		return openapi.ArchiveCategory409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "system_category_protected", "System categories cannot be archived.",
		)}, nil
	case errors.Is(err, category.ErrHasChildren):
		return openapi.ArchiveCategory409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "category_has_children", "Archive child categories before their parent.",
		)}, nil
	case err != nil:
		return nil, err
	}
	return openapi.ArchiveCategory204Response{
		Headers: openapi.ArchiveCategory204ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func categoryWriteInput(input openapi.CategoryWriteRequest) category.WriteInput {
	var parentID *string
	if input.ParentId != nil {
		value := input.ParentId.String()
		parentID = &value
	}
	return category.WriteInput{
		Name: input.Name, Kind: category.Kind(input.Kind), ParentID: parentID, Icon: input.Icon,
	}
}

func categoryResponse(value category.Category) (openapi.Category, error) {
	id, err := uuid.Parse(value.ID)
	if err != nil {
		return openapi.Category{}, err
	}
	workspaceID, err := uuid.Parse(value.WorkspaceID)
	if err != nil {
		return openapi.Category{}, err
	}
	response := openapi.Category{
		Id: id, WorkspaceId: workspaceID, Name: value.Name,
		Kind: openapi.CategoryKind(value.Kind), Icon: value.Icon, ArchivedAt: value.ArchivedAt,
	}
	if value.ParentID != nil {
		parentID, err := uuid.Parse(*value.ParentID)
		if err != nil {
			return openapi.Category{}, err
		}
		response.ParentId = &parentID
	}
	if value.SystemKey != nil {
		key := openapi.SystemCategoryKey(*value.SystemKey)
		response.SystemKey = &key
	}
	return response, nil
}
