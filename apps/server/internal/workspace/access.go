package workspace

import (
	"context"
	"errors"
)

var ErrForbidden = errors.New("workspace access is forbidden")

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

type MembershipRepository interface {
	MemberRole(context.Context, string, string) (Role, error)
}

type Authorizer struct {
	repository MembershipRepository
}

func (a *Authorizer) Role(ctx context.Context, workspaceID, userID string) (Role, error) {
	role, err := a.repository.MemberRole(ctx, workspaceID, userID)
	if err != nil {
		return "", err
	}
	if !role.Valid() {
		return "", ErrForbidden
	}
	return role, nil
}

func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

func NewAuthorizer(repository MembershipRepository) *Authorizer {
	return &Authorizer{repository: repository}
}

func (a *Authorizer) RequireRead(ctx context.Context, workspaceID, userID string) error {
	role, err := a.Role(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	switch role {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return nil
	default:
		return ErrForbidden
	}
}

func (a *Authorizer) RequireManage(ctx context.Context, workspaceID, userID string) error {
	role, err := a.Role(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	switch role {
	case RoleOwner, RoleAdmin, RoleMember:
		return nil
	default:
		return ErrForbidden
	}
}
