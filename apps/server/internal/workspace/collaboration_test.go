package workspace

import (
	"context"
	"errors"
	"testing"
	"time"
)

type collaborationRepositoryStub struct {
	members      map[string]Member
	owners       int64
	invitation   Invitation
	create       CreateInvitationCommand
	acceptedHash []byte
	err          error
}

func (s *collaborationRepositoryStub) ListMembers(context.Context, string) ([]Member, error) {
	return nil, s.err
}

func (s *collaborationRepositoryStub) Member(_ context.Context, _, userID string) (Member, error) {
	value, ok := s.members[userID]
	if !ok {
		return Member{}, ErrNotFound
	}
	return value, s.err
}

func (s *collaborationRepositoryStub) ActiveOwnerCount(context.Context, string) (int64, error) {
	return s.owners, s.err
}

func (s *collaborationRepositoryStub) ListInvitations(
	context.Context, string, time.Time,
) ([]Invitation, error) {
	return nil, s.err
}

func (s *collaborationRepositoryStub) Invitation(
	context.Context, string, string, time.Time,
) (Invitation, error) {
	return s.invitation, s.err
}

func (s *collaborationRepositoryStub) CreateInvitation(
	_ context.Context, command CreateInvitationCommand,
) (Invitation, error) {
	s.create = command
	return Invitation{
		ID: command.ID, WorkspaceID: command.WorkspaceID, Email: command.Email,
		Role: command.Role, InvitedBy: command.ActorID,
		ExpiresAt: command.ExpiresAt, CreatedAt: command.Now,
	}, s.err
}

func (s *collaborationRepositoryStub) RevokeInvitation(
	context.Context, string, string, string, time.Time,
) error {
	return s.err
}

func (s *collaborationRepositoryStub) UpdateMemberRole(
	_ context.Context, _, _, targetID string, role Role,
) (Member, error) {
	value := s.members[targetID]
	value.Role = role
	return value, s.err
}

func (s *collaborationRepositoryStub) RemoveMember(
	context.Context, string, string, string, time.Time,
) error {
	return s.err
}

func (s *collaborationRepositoryStub) AcceptInvitation(
	_ context.Context, tokenHash []byte, _ string, _ time.Time,
) (Acceptance, error) {
	s.acceptedHash = tokenHash
	return Acceptance{}, s.err
}

func TestCollaborationRolePolicy(t *testing.T) {
	if !CanInvite(RoleOwner, RoleAdmin) || !CanInvite(RoleAdmin, RoleMember) {
		t.Fatal("owner/admin invitation policy rejected a permitted role")
	}
	if CanInvite(RoleAdmin, RoleAdmin) || CanInvite(RoleMember, RoleViewer) || CanInvite(RoleOwner, RoleOwner) {
		t.Fatal("invitation policy allowed a forbidden role")
	}
	if !CanChangeRole(RoleOwner, RoleOwner, RoleViewer) ||
		!CanChangeRole(RoleAdmin, RoleViewer, RoleMember) {
		t.Fatal("role-change policy rejected a permitted transition")
	}
	if CanChangeRole(RoleAdmin, RoleAdmin, RoleMember) ||
		CanChangeRole(RoleAdmin, RoleMember, RoleAdmin) {
		t.Fatal("administrator role-change policy crossed its boundary")
	}
	if !CanRemoveMember("same", "same", RoleViewer, RoleViewer) {
		t.Fatal("viewer could not leave the workspace")
	}
	if CanRemoveMember("admin", "owner", RoleAdmin, RoleOwner) ||
		CanRemoveMember("member", "viewer", RoleMember, RoleViewer) {
		t.Fatal("member-removal policy crossed its boundary")
	}
}

func TestCreateInvitationNormalizesAndExpiresCredential(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.FixedZone("local", 3*60*60))
	repository := &collaborationRepositoryStub{}
	service := NewCollaborationService(
		repository,
		NewAuthorizer(membershipStub{role: RoleOwner}),
		func() time.Time { return now },
	)
	result, err := service.CreateInvitation(
		context.Background(), "workspace", "owner", InvitationInput{
			Email: "  Person@Example.COM ", Role: RoleAdmin,
		},
	)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if repository.create.Email != "person@example.com" || result.Invitation.Email != "person@example.com" {
		t.Fatalf("normalized email = %q", repository.create.Email)
	}
	if len(result.Token) != 43 || len(repository.create.TokenHash) != 32 {
		t.Fatalf("credential lengths = token %d, hash %d", len(result.Token), len(repository.create.TokenHash))
	}
	wantExpiry := now.UTC().Add(InvitationLifetime)
	if !repository.create.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expiry = %v, want %v", repository.create.ExpiresAt, wantExpiry)
	}
}

func TestCollaborationServiceProtectsRolesAndLastOwner(t *testing.T) {
	repository := &collaborationRepositoryStub{
		members: map[string]Member{
			"owner": {UserID: "owner", Role: RoleOwner},
			"admin": {UserID: "admin", Role: RoleAdmin},
		},
		owners: 1,
	}
	adminService := NewCollaborationService(
		repository,
		NewAuthorizer(membershipStub{role: RoleAdmin}),
		time.Now,
	)
	_, err := adminService.UpdateMemberRole(
		context.Background(), "workspace", "admin", "owner", RoleMember,
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("admin owner change error = %v, want ErrForbidden", err)
	}
	ownerService := NewCollaborationService(
		repository,
		NewAuthorizer(membershipStub{role: RoleOwner}),
		time.Now,
	)
	_, err = ownerService.UpdateMemberRole(
		context.Background(), "workspace", "owner", "owner", RoleMember,
	)
	if !errors.Is(err, ErrLastOwner) {
		t.Fatalf("last-owner demotion error = %v, want ErrLastOwner", err)
	}
	if err := ownerService.RemoveMember(
		context.Background(), "workspace", "owner", "owner",
	); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("last-owner departure error = %v, want ErrLastOwner", err)
	}
}

func TestAcceptInvitationRejectsMalformedToken(t *testing.T) {
	repository := &collaborationRepositoryStub{}
	service := NewCollaborationService(
		repository,
		NewAuthorizer(membershipStub{role: RoleOwner}),
		time.Now,
	)
	_, err := service.AcceptInvitation(context.Background(), "user", "not-a-token")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AcceptInvitation() error = %v, want ErrInvalidInput", err)
	}
	if repository.acceptedHash != nil {
		t.Fatal("malformed token reached repository")
	}
}

func TestCanListInvitationsRestrictsToOwnersAndAdmins(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin} {
		if !CanListInvitations(role) {
			t.Fatalf("CanListInvitations(%q) = false", role)
		}
	}
	// Pending invitations expose the email addresses of people not yet in the workspace.
	for _, role := range []Role{RoleMember, RoleViewer, "", "auditor"} {
		if CanListInvitations(role) {
			t.Fatalf("CanListInvitations(%q) = true", role)
		}
	}
}

func TestCanChangeRolePolicy(t *testing.T) {
	tests := []struct {
		name                   string
		actor, target, newRole Role
		want                   bool
	}{
		{"owner promotes member to admin", RoleOwner, RoleMember, RoleAdmin, true},
		{"owner transfers ownership", RoleOwner, RoleMember, RoleOwner, true},
		{"owner demotes another owner", RoleOwner, RoleOwner, RoleMember, true},
		{"admin adjusts a member", RoleAdmin, RoleMember, RoleViewer, true},
		{"admin adjusts a viewer", RoleAdmin, RoleViewer, RoleMember, true},
		// An admin must not create a peer or a superior, nor touch one.
		{"admin promotes to admin", RoleAdmin, RoleMember, RoleAdmin, false},
		{"admin promotes to owner", RoleAdmin, RoleMember, RoleOwner, false},
		{"admin changes another admin", RoleAdmin, RoleAdmin, RoleMember, false},
		{"admin changes the owner", RoleAdmin, RoleOwner, RoleMember, false},
		{"member changes anyone", RoleMember, RoleViewer, RoleMember, false},
		{"viewer changes anyone", RoleViewer, RoleMember, RoleViewer, false},
		{"unknown target role", RoleOwner, RoleMember, "auditor", false},
		{"empty target role", RoleOwner, RoleMember, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanChangeRole(test.actor, test.target, test.newRole); got != test.want {
				t.Fatalf("CanChangeRole(%q,%q,%q) = %v, want %v",
					test.actor, test.target, test.newRole, got, test.want)
			}
		})
	}
}

func TestCanRemoveMemberPolicy(t *testing.T) {
	const actorID, targetID = "actor", "target"
	tests := []struct {
		name          string
		actorID       string
		actor, target Role
		want          bool
	}{
		// Anyone may leave a workspace, whatever their role.
		{"viewer removes self", targetID, RoleViewer, RoleViewer, true},
		{"owner removes self", targetID, RoleOwner, RoleOwner, true},
		{"owner removes an admin", actorID, RoleOwner, RoleAdmin, true},
		{"admin removes a member", actorID, RoleAdmin, RoleMember, true},
		{"admin removes a viewer", actorID, RoleAdmin, RoleViewer, true},
		{"admin removes another admin", actorID, RoleAdmin, RoleAdmin, false},
		{"admin removes the owner", actorID, RoleAdmin, RoleOwner, false},
		{"member removes a viewer", actorID, RoleMember, RoleViewer, false},
		{"viewer removes a member", actorID, RoleViewer, RoleMember, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CanRemoveMember(test.actorID, targetID, test.actor, test.target)
			if got != test.want {
				t.Fatalf("CanRemoveMember(%q,%q,%q,%q) = %v, want %v",
					test.actorID, targetID, test.actor, test.target, got, test.want)
			}
		})
	}
}

// The token is the entire authorization for accepting an invitation, so anything that is not
// exactly a 32-byte base64url value is rejected before it reaches a database lookup.
func TestInvitationTokenHashRejectsMalformedTokens(t *testing.T) {
	for _, token := range []string{
		"",
		"short",
		"not-base64-but-exactly-forty-three-chars!!!",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",   // 42 characters
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", // 44 characters
	} {
		if _, err := invitationTokenHash(token); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invitationTokenHash(%q) error = %v, want ErrInvalidInput", token, err)
		}
	}
}

func TestInvitationTokenHashIsStableAndOpaque(t *testing.T) {
	_, token, storedHash, err := newInvitationCredential()
	if err != nil {
		t.Fatalf("newInvitationCredential() error = %v", err)
	}
	hash, err := invitationTokenHash(token)
	if err != nil {
		t.Fatalf("invitationTokenHash() error = %v", err)
	}
	if len(hash) != 32 {
		t.Fatalf("hash length = %d, want 32", len(hash))
	}
	if string(hash) == token {
		t.Fatal("hash equals the raw token")
	}
	// Acceptance looks the invitation up by hash, so issuing and verifying must agree.
	if string(hash) != string(storedHash) {
		t.Fatal("issued hash does not match the verification hash")
	}
	repeat, err := invitationTokenHash(token)
	if err != nil || string(repeat) != string(hash) {
		t.Fatalf("invitationTokenHash() is not stable: %v", err)
	}
}

type invitationNotifierStub struct {
	err           error
	calls         int
	token         string
	workspaceName string
}

func (s *invitationNotifierStub) NotifyInvitation(
	_ context.Context, _ Invitation, workspaceName, token string,
) error {
	s.calls++
	s.token = token
	s.workspaceName = workspaceName
	return s.err
}

type workspaceNameStub struct {
	name string
	err  error
}

func (s workspaceNameStub) WorkspaceName(context.Context, string) (string, error) {
	return s.name, s.err
}

func TestCreateInvitationDeliversTheTokenOnce(t *testing.T) {
	notifier := &invitationNotifierStub{}
	service := newCollaborationServiceForNotification(notifier, workspaceNameStub{name: "Atay Family"})

	credential, err := service.CreateInvitation(
		context.Background(), "workspace", "owner",
		InvitationInput{Email: "invited@example.com", Role: RoleMember},
	)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if notifier.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1", notifier.calls)
	}
	// The repository stores only a hash, so delivery is the one chance to send the token.
	if notifier.token != credential.Token || notifier.token == "" {
		t.Fatalf("delivered token = %q, credential token = %q", notifier.token, credential.Token)
	}
	if notifier.workspaceName != "Atay Family" {
		t.Fatalf("workspace name = %q", notifier.workspaceName)
	}
}

// A relay failure must not discard a stored invitation: the token is still returned so the
// inviter can share it directly, which is the same flow that runs with SMTP switched off.
func TestCreateInvitationSurvivesADeliveryFailure(t *testing.T) {
	notifier := &invitationNotifierStub{err: errors.New("relay unreachable")}
	service := newCollaborationServiceForNotification(notifier, workspaceNameStub{name: "Atay Family"})

	credential, err := service.CreateInvitation(
		context.Background(), "workspace", "owner",
		InvitationInput{Email: "invited@example.com", Role: RoleMember},
	)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v, want the invitation to survive", err)
	}
	if credential.Token == "" {
		t.Fatal("no token was returned for the inviter to share")
	}
}

// A workspace-name lookup failure is equally non-fatal, and must not send a message naming
// the wrong workspace.
func TestCreateInvitationSkipsDeliveryWhenTheWorkspaceIsUnknown(t *testing.T) {
	notifier := &invitationNotifierStub{}
	service := newCollaborationServiceForNotification(
		notifier, workspaceNameStub{err: errors.New("not found")},
	)

	if _, err := service.CreateInvitation(
		context.Background(), "workspace", "owner",
		InvitationInput{Email: "invited@example.com", Role: RoleMember},
	); err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if notifier.calls != 0 {
		t.Fatal("a message was sent without a workspace name")
	}
}

// With no notifier configured the service must not reach for one.
func TestCreateInvitationWithoutANotifierStillIssuesTheToken(t *testing.T) {
	service := newCollaborationServiceForNotification(nil, nil)
	credential, err := service.CreateInvitation(
		context.Background(), "workspace", "owner",
		InvitationInput{Email: "invited@example.com", Role: RoleMember},
	)
	if err != nil || credential.Token == "" {
		t.Fatalf("CreateInvitation() = %#v, error = %v", credential, err)
	}
}

func newCollaborationServiceForNotification(
	notifier InvitationNotifier, workspaces WorkspaceNameLookup,
) *CollaborationService {
	service := NewCollaborationService(
		&collaborationRepositoryStub{},
		NewAuthorizer(membershipStub{role: RoleOwner}),
		func() time.Time { return time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC) },
	)
	if notifier == nil {
		return service
	}
	return service.WithInvitationNotifier(notifier, workspaces, nil)
}
