package workspace

import (
	"context"
	"errors"
	"testing"
)

type membershipStub struct {
	role Role
	err  error
}

func (s membershipStub) MemberRole(context.Context, string, string) (Role, error) {
	return s.role, s.err
}

func TestAuthorizerViewerCanReadButCannotManage(t *testing.T) {
	authorizer := NewAuthorizer(membershipStub{role: RoleViewer})
	if err := authorizer.RequireRead(context.Background(), "workspace", "user"); err != nil {
		t.Fatalf("RequireRead() error = %v", err)
	}
	if err := authorizer.RequireManage(context.Background(), "workspace", "user"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RequireManage() error = %v, want ErrForbidden", err)
	}
}

func TestAuthorizerMembersCanManageSetup(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin, RoleMember} {
		t.Run(string(role), func(t *testing.T) {
			authorizer := NewAuthorizer(membershipStub{role: role})
			if err := authorizer.RequireManage(context.Background(), "workspace", "user"); err != nil {
				t.Fatalf("RequireManage() error = %v", err)
			}
		})
	}
}

func TestAuthorizerDeniesNonMembers(t *testing.T) {
	// An empty role is what MemberRole yields for a user with no membership row.
	for _, role := range []Role{"", "guest", "Owner", "viewer "} {
		t.Run(string(role), func(t *testing.T) {
			authorizer := NewAuthorizer(membershipStub{role: role})
			if err := authorizer.RequireRead(context.Background(), "workspace", "user"); !errors.Is(err, ErrForbidden) {
				t.Fatalf("RequireRead(%q) error = %v, want ErrForbidden", role, err)
			}
			if err := authorizer.RequireManage(context.Background(), "workspace", "user"); !errors.Is(err, ErrForbidden) {
				t.Fatalf("RequireManage(%q) error = %v, want ErrForbidden", role, err)
			}
		})
	}
}

// A lookup failure must not be mistaken for a granted role: the repository error propagates
// so the caller reports a fault rather than silently allowing or denying access.
func TestAuthorizerPropagatesLookupFailure(t *testing.T) {
	lookupErr := errors.New("database unavailable")
	authorizer := NewAuthorizer(membershipStub{role: RoleOwner, err: lookupErr})

	if err := authorizer.RequireRead(context.Background(), "workspace", "user"); !errors.Is(err, lookupErr) {
		t.Fatalf("RequireRead() error = %v, want the lookup failure", err)
	}
	if err := authorizer.RequireManage(context.Background(), "workspace", "user"); !errors.Is(err, lookupErr) {
		t.Fatalf("RequireManage() error = %v, want the lookup failure", err)
	}
	if _, err := authorizer.Role(context.Background(), "workspace", "user"); !errors.Is(err, lookupErr) {
		t.Fatalf("Role() error = %v, want the lookup failure", err)
	}
}

func TestAuthorizerRoleReturnsMembership(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin, RoleMember, RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			authorizer := NewAuthorizer(membershipStub{role: role})
			got, err := authorizer.Role(context.Background(), "workspace", "user")
			if err != nil || got != role {
				t.Fatalf("Role() = %q, %v", got, err)
			}
		})
	}
}

func TestAuthorizerRoleRejectsUnknownMembership(t *testing.T) {
	authorizer := NewAuthorizer(membershipStub{role: "auditor"})
	if _, err := authorizer.Role(context.Background(), "workspace", "user"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Role() error = %v, want ErrForbidden", err)
	}
}

func TestRoleValidCoversEveryDeclaredRole(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin, RoleMember, RoleViewer} {
		if !role.Valid() {
			t.Fatalf("Valid(%q) = false", role)
		}
	}
	for _, role := range []Role{"", "auditor", "OWNER"} {
		if role.Valid() {
			t.Fatalf("Valid(%q) = true", role)
		}
	}
}
