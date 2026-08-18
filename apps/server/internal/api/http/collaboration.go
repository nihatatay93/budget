package httpapi

import (
	"context"
	"errors"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/workspace"
)

func (s *server) ListWorkspaceMembers(
	ctx context.Context,
	request openapi.ListWorkspaceMembersRequestObject,
) (openapi.ListWorkspaceMembersResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	values, err := s.Collaboration.ListMembers(
		ctx, request.WorkspaceId.String(), principal.User.ID,
	)
	if err != nil {
		return nil, err
	}
	members, err := workspaceMemberResponses(values)
	if err != nil {
		return nil, err
	}
	return openapi.ListWorkspaceMembers200JSONResponse{
		Body:    openapi.WorkspaceMemberListResponse{Members: members},
		Headers: openapi.ListWorkspaceMembers200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) ListWorkspaceInvitations(
	ctx context.Context,
	request openapi.ListWorkspaceInvitationsRequestObject,
) (openapi.ListWorkspaceInvitationsResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	values, err := s.Collaboration.ListInvitations(
		ctx, request.WorkspaceId.String(), principal.User.ID,
	)
	if err != nil {
		return nil, err
	}
	invitations, err := workspaceInvitationResponses(values)
	if err != nil {
		return nil, err
	}
	return openapi.ListWorkspaceInvitations200JSONResponse{
		Body:    openapi.WorkspaceInvitationListResponse{Invitations: invitations},
		Headers: openapi.ListWorkspaceInvitations200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) CreateWorkspaceInvitation(
	ctx context.Context,
	request openapi.CreateWorkspaceInvitationRequestObject,
) (openapi.CreateWorkspaceInvitationResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.CreateWorkspaceInvitation400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	}
	value, err := s.Collaboration.CreateInvitation(
		ctx, request.WorkspaceId.String(), principal.User.ID,
		workspace.InvitationInput{
			Email: string(request.Body.Email), Role: workspace.Role(request.Body.Role),
		},
	)
	switch {
	case errors.Is(err, workspace.ErrInvalidInput):
		return openapi.CreateWorkspaceInvitation400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	case errors.Is(err, workspace.ErrAlreadyMember):
		return openapi.CreateWorkspaceInvitation409JSONResponse{
			ConflictJSONResponse: conflict(
				requestID, "already_member", "That email address already belongs to an active workspace member.",
			),
		}, nil
	case err != nil:
		return nil, err
	}
	invitation, err := workspaceInvitationResponse(value.Invitation)
	if err != nil {
		return nil, err
	}
	return openapi.CreateWorkspaceInvitation201JSONResponse{
		Body: openapi.CreateWorkspaceInvitationResponse{
			Invitation: invitation, AcceptanceToken: value.Token,
		},
		Headers: openapi.CreateWorkspaceInvitation201ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) RevokeWorkspaceInvitation(
	ctx context.Context,
	request openapi.RevokeWorkspaceInvitationRequestObject,
) (openapi.RevokeWorkspaceInvitationResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	err = s.Collaboration.RevokeInvitation(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.InvitationId.String(),
	)
	switch {
	case errors.Is(err, workspace.ErrInvitationUnavailable):
		return openapi.RevokeWorkspaceInvitation409JSONResponse{
			ConflictJSONResponse: conflict(
				requestID, "invitation_not_pending", "The invitation is no longer pending.",
			),
		}, nil
	case err != nil:
		return nil, err
	}
	return openapi.RevokeWorkspaceInvitation204Response{
		Headers: openapi.RevokeWorkspaceInvitation204ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) UpdateWorkspaceMemberRole(
	ctx context.Context,
	request openapi.UpdateWorkspaceMemberRoleRequestObject,
) (openapi.UpdateWorkspaceMemberRoleResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.UpdateWorkspaceMemberRole400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	}
	value, err := s.Collaboration.UpdateMemberRole(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.UserId.String(),
		workspace.Role(request.Body.Role),
	)
	switch {
	case errors.Is(err, workspace.ErrInvalidInput):
		return openapi.UpdateWorkspaceMemberRole400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	case errors.Is(err, workspace.ErrLastOwner):
		return openapi.UpdateWorkspaceMemberRole409JSONResponse{
			ConflictJSONResponse: lastOwnerConflict(requestID),
		}, nil
	case err != nil:
		return nil, err
	}
	body, err := workspaceMemberResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.UpdateWorkspaceMemberRole200JSONResponse{
		Body:    body,
		Headers: openapi.UpdateWorkspaceMemberRole200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) RemoveWorkspaceMember(
	ctx context.Context,
	request openapi.RemoveWorkspaceMemberRequestObject,
) (openapi.RemoveWorkspaceMemberResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	err = s.Collaboration.RemoveMember(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.UserId.String(),
	)
	switch {
	case errors.Is(err, workspace.ErrLastOwner):
		return openapi.RemoveWorkspaceMember409JSONResponse{
			ConflictJSONResponse: lastOwnerConflict(requestID),
		}, nil
	case err != nil:
		return nil, err
	}
	return openapi.RemoveWorkspaceMember204Response{
		Headers: openapi.RemoveWorkspaceMember204ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) AcceptWorkspaceInvitation(
	ctx context.Context,
	request openapi.AcceptWorkspaceInvitationRequestObject,
) (openapi.AcceptWorkspaceInvitationResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.AcceptWorkspaceInvitation400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	}
	value, err := s.Collaboration.AcceptInvitation(ctx, principal.User.ID, request.Body.Token)
	switch {
	case errors.Is(err, workspace.ErrInvalidInput):
		return openapi.AcceptWorkspaceInvitation400JSONResponse{
			BadRequestJSONResponse: badRequest(requestID),
		}, nil
	case errors.Is(err, workspace.ErrAlreadyMember):
		return openapi.AcceptWorkspaceInvitation409JSONResponse{
			ConflictJSONResponse: conflict(
				requestID, "already_member", "You are already an active member of this workspace.",
			),
		}, nil
	case errors.Is(err, workspace.ErrInvitationUnavailable):
		return openapi.AcceptWorkspaceInvitation410JSONResponse{
			GoneJSONResponse: gone(
				requestID, "invitation_unavailable", "The invitation has expired or is no longer available.",
			),
		}, nil
	case err != nil:
		return nil, err
	}
	body, err := workspaceAcceptanceResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.AcceptWorkspaceInvitation200JSONResponse{
		Body:    body,
		Headers: openapi.AcceptWorkspaceInvitation200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func workspaceMemberResponses(values []workspace.Member) ([]openapi.WorkspaceMember, error) {
	result := make([]openapi.WorkspaceMember, 0, len(values))
	for _, value := range values {
		member, err := workspaceMemberResponse(value)
		if err != nil {
			return nil, err
		}
		result = append(result, member)
	}
	return result, nil
}

func workspaceMemberResponse(value workspace.Member) (openapi.WorkspaceMember, error) {
	userID, err := uuid.Parse(value.UserID)
	if err != nil {
		return openapi.WorkspaceMember{}, err
	}
	return openapi.WorkspaceMember{
		UserId: userID, Email: openapi_types.Email(value.Email), DisplayName: value.DisplayName,
		Role: openapi.WorkspaceRole(value.Role), JoinedAt: value.JoinedAt,
	}, nil
}

func workspaceInvitationResponses(
	values []workspace.Invitation,
) ([]openapi.WorkspaceInvitation, error) {
	result := make([]openapi.WorkspaceInvitation, 0, len(values))
	for _, value := range values {
		invitation, err := workspaceInvitationResponse(value)
		if err != nil {
			return nil, err
		}
		result = append(result, invitation)
	}
	return result, nil
}

func workspaceInvitationResponse(
	value workspace.Invitation,
) (openapi.WorkspaceInvitation, error) {
	id, err := uuid.Parse(value.ID)
	if err != nil {
		return openapi.WorkspaceInvitation{}, err
	}
	workspaceID, err := uuid.Parse(value.WorkspaceID)
	if err != nil {
		return openapi.WorkspaceInvitation{}, err
	}
	invitedBy, err := uuid.Parse(value.InvitedBy)
	if err != nil {
		return openapi.WorkspaceInvitation{}, err
	}
	return openapi.WorkspaceInvitation{
		Id: id, WorkspaceId: workspaceID, Email: openapi_types.Email(value.Email),
		Role: openapi.WorkspaceInvitationRole(value.Role), InvitedBy: invitedBy,
		InviterDisplayName: value.InviterDisplayName,
		ExpiresAt:          value.ExpiresAt, CreatedAt: value.CreatedAt,
	}, nil
}

func workspaceAcceptanceResponse(
	value workspace.Acceptance,
) (openapi.WorkspaceMembershipAcceptance, error) {
	member, err := workspaceMemberResponse(value.Member)
	if err != nil {
		return openapi.WorkspaceMembershipAcceptance{}, err
	}
	workspaceID, err := uuid.Parse(value.Workspace.ID)
	if err != nil {
		return openapi.WorkspaceMembershipAcceptance{}, err
	}
	return openapi.WorkspaceMembershipAcceptance{
		Workspace: openapi.WorkspaceSummary{
			Id: workspaceID, Name: value.Workspace.Name,
			BaseCurrency: openapi.Currency(value.Workspace.BaseCurrency),
			Timezone:     value.Workspace.Timezone, Role: openapi.WorkspaceRole(value.Workspace.Role),
		},
		Member: member,
	}, nil
}

func lastOwnerConflict(requestID string) openapi.ConflictJSONResponse {
	return conflict(
		requestID, "last_owner", "A workspace must retain at least one active owner.",
	)
}
