package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const InvitationLifetime = 7 * 24 * time.Hour

var (
	ErrInvalidInput          = errors.New("workspace collaboration input is invalid")
	ErrNotFound              = errors.New("workspace collaboration resource was not found")
	ErrAlreadyMember         = errors.New("user is already an active workspace member")
	ErrInvitationUnavailable = errors.New("workspace invitation is no longer available")
	ErrLastOwner             = errors.New("workspace must retain an active owner")
)

type Member struct {
	UserID      string
	Email       string
	DisplayName string
	Role        Role
	JoinedAt    time.Time
}

type Invitation struct {
	ID                 string
	WorkspaceID        string
	Email              string
	Role               Role
	InvitedBy          string
	InviterDisplayName string
	ExpiresAt          time.Time
	CreatedAt          time.Time
}

type WorkspaceSummary struct {
	ID           string
	Name         string
	BaseCurrency string
	Timezone     string
	Role         Role
}

type InvitationInput struct {
	Email string
	Role  Role
}

type CreateInvitationCommand struct {
	ID          string
	WorkspaceID string
	ActorID     string
	Email       string
	Role        Role
	TokenHash   []byte
	ExpiresAt   time.Time
	Now         time.Time
}

type InvitationCredential struct {
	Invitation Invitation
	Token      string
}

type Acceptance struct {
	Workspace WorkspaceSummary
	Member    Member
}

type CollaborationRepository interface {
	ListMembers(context.Context, string) ([]Member, error)
	Member(context.Context, string, string) (Member, error)
	ActiveOwnerCount(context.Context, string) (int64, error)
	ListInvitations(context.Context, string, time.Time) ([]Invitation, error)
	Invitation(context.Context, string, string, time.Time) (Invitation, error)
	CreateInvitation(context.Context, CreateInvitationCommand) (Invitation, error)
	RevokeInvitation(context.Context, string, string, string, time.Time) error
	UpdateMemberRole(context.Context, string, string, string, Role) (Member, error)
	RemoveMember(context.Context, string, string, string, time.Time) error
	AcceptInvitation(context.Context, []byte, string, time.Time) (Acceptance, error)
}

type CollaborationService struct {
	repository CollaborationRepository
	authorizer *Authorizer
	now        func() time.Time
	// notifier is nil when the operator has not configured email delivery. Invitations still
	// work in that case: the token is returned to the inviter to share directly.
	notifier   InvitationNotifier
	workspaces WorkspaceNameLookup
	logger     *slog.Logger
}

func NewCollaborationService(
	repository CollaborationRepository,
	authorizer *Authorizer,
	now func() time.Time,
) *CollaborationService {
	return &CollaborationService{
		repository: repository, authorizer: authorizer, now: now,
		logger: slog.New(slog.DiscardHandler),
	}
}

// WithInvitationNotifier enables delivery of invitation emails. Delivery is best effort: see
// CreateInvitation.
func (s *CollaborationService) WithInvitationNotifier(
	notifier InvitationNotifier, workspaces WorkspaceNameLookup, logger *slog.Logger,
) *CollaborationService {
	s.notifier = notifier
	s.workspaces = workspaces
	if logger != nil {
		s.logger = logger
	}
	return s
}

func (s *CollaborationService) ListMembers(
	ctx context.Context, workspaceID, userID string,
) ([]Member, error) {
	if err := s.authorizer.RequireRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repository.ListMembers(ctx, workspaceID)
}

func (s *CollaborationService) ListInvitations(
	ctx context.Context, workspaceID, userID string,
) ([]Invitation, error) {
	actorRole, err := s.authorizer.Role(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !CanListInvitations(actorRole) {
		return nil, ErrForbidden
	}
	return s.repository.ListInvitations(ctx, workspaceID, s.now().UTC())
}

func (s *CollaborationService) CreateInvitation(
	ctx context.Context,
	workspaceID, userID string,
	input InvitationInput,
) (InvitationCredential, error) {
	email, err := normalizeInvitationEmail(input.Email)
	if err != nil || !input.Role.Valid() || input.Role == RoleOwner {
		return InvitationCredential{}, ErrInvalidInput
	}
	actorRole, err := s.authorizer.Role(ctx, workspaceID, userID)
	if err != nil {
		return InvitationCredential{}, err
	}
	if !CanInvite(actorRole, input.Role) {
		return InvitationCredential{}, ErrForbidden
	}
	id, token, tokenHash, err := newInvitationCredential()
	if err != nil {
		return InvitationCredential{}, err
	}
	now := s.now().UTC()
	invitation, err := s.repository.CreateInvitation(ctx, CreateInvitationCommand{
		ID: id, WorkspaceID: workspaceID, ActorID: userID, Email: email,
		Role: input.Role, TokenHash: tokenHash, ExpiresAt: now.Add(InvitationLifetime), Now: now,
	})
	if err != nil {
		return InvitationCredential{}, err
	}
	s.notifyInvitation(ctx, invitation, token)
	return InvitationCredential{Invitation: invitation, Token: token}, nil
}

// notifyInvitation sends the invitation if delivery is configured.
//
// A delivery failure never fails the request. The invitation is already stored and its token
// is returned to the inviter, so a bounced or misconfigured relay degrades to the same
// share-it-yourself flow that runs when email is switched off entirely. Failing here would
// discard a valid invitation over a transport problem.
func (s *CollaborationService) notifyInvitation(
	ctx context.Context, invitation Invitation, token string,
) {
	if s.notifier == nil || s.workspaces == nil {
		return
	}
	name, err := s.workspaces.WorkspaceName(ctx, invitation.WorkspaceID)
	if err != nil {
		s.logger.WarnContext(ctx, "resolve workspace name for invitation",
			"error", err, "invitation_id", invitation.ID)
		return
	}
	if err := s.notifier.NotifyInvitation(ctx, invitation, name, token); err != nil {
		// The token is never logged: the error is about transport, not content.
		s.logger.WarnContext(ctx, "deliver invitation email",
			"error", err, "invitation_id", invitation.ID)
	}
}

func (s *CollaborationService) RevokeInvitation(
	ctx context.Context, workspaceID, userID, invitationID string,
) error {
	actorRole, err := s.authorizer.Role(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	invitation, err := s.repository.Invitation(ctx, workspaceID, invitationID, now)
	if err != nil {
		return err
	}
	if !CanInvite(actorRole, invitation.Role) {
		return ErrForbidden
	}
	return s.repository.RevokeInvitation(ctx, workspaceID, userID, invitationID, now)
}

func (s *CollaborationService) UpdateMemberRole(
	ctx context.Context,
	workspaceID, userID, targetUserID string,
	newRole Role,
) (Member, error) {
	if !newRole.Valid() {
		return Member{}, ErrInvalidInput
	}
	actorRole, err := s.authorizer.Role(ctx, workspaceID, userID)
	if err != nil {
		return Member{}, err
	}
	target, err := s.repository.Member(ctx, workspaceID, targetUserID)
	if err != nil {
		return Member{}, err
	}
	if !CanChangeRole(actorRole, target.Role, newRole) {
		return Member{}, ErrForbidden
	}
	if target.Role == RoleOwner && newRole != RoleOwner {
		if err := s.requireAnotherOwner(ctx, workspaceID); err != nil {
			return Member{}, err
		}
	}
	return s.repository.UpdateMemberRole(ctx, workspaceID, userID, targetUserID, newRole)
}

func (s *CollaborationService) RemoveMember(
	ctx context.Context,
	workspaceID, userID, targetUserID string,
) error {
	actorRole, err := s.authorizer.Role(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	target, err := s.repository.Member(ctx, workspaceID, targetUserID)
	if err != nil {
		return err
	}
	if !CanRemoveMember(userID, targetUserID, actorRole, target.Role) {
		return ErrForbidden
	}
	if target.Role == RoleOwner {
		if err := s.requireAnotherOwner(ctx, workspaceID); err != nil {
			return err
		}
	}
	return s.repository.RemoveMember(ctx, workspaceID, userID, targetUserID, s.now().UTC())
}

func (s *CollaborationService) AcceptInvitation(
	ctx context.Context, userID, token string,
) (Acceptance, error) {
	tokenHash, err := invitationTokenHash(token)
	if err != nil {
		return Acceptance{}, ErrInvalidInput
	}
	return s.repository.AcceptInvitation(ctx, tokenHash, userID, s.now().UTC())
}

func (s *CollaborationService) requireAnotherOwner(ctx context.Context, workspaceID string) error {
	count, err := s.repository.ActiveOwnerCount(ctx, workspaceID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastOwner
	}
	return nil
}

func CanListInvitations(role Role) bool {
	return role == RoleOwner || role == RoleAdmin
}

func CanInvite(actorRole, invitationRole Role) bool {
	switch actorRole {
	case RoleOwner:
		return invitationRole == RoleAdmin || invitationRole == RoleMember || invitationRole == RoleViewer
	case RoleAdmin:
		return invitationRole == RoleMember || invitationRole == RoleViewer
	default:
		return false
	}
}

func CanChangeRole(actorRole, targetRole, newRole Role) bool {
	if !newRole.Valid() {
		return false
	}
	if actorRole == RoleOwner {
		return true
	}
	return actorRole == RoleAdmin &&
		(targetRole == RoleMember || targetRole == RoleViewer) &&
		(newRole == RoleMember || newRole == RoleViewer)
}

func CanRemoveMember(actorID, targetID string, actorRole, targetRole Role) bool {
	if actorID == targetID {
		return true
	}
	if actorRole == RoleOwner {
		return true
	}
	return actorRole == RoleAdmin && (targetRole == RoleMember || targetRole == RoleViewer)
}

func normalizeInvitationEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized || len(normalized) > 254 {
		return "", ErrInvalidInput
	}
	return normalized, nil
}

func newInvitationCredential() (string, string, []byte, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", "", nil, fmt.Errorf("create invitation UUID: %w", err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", nil, fmt.Errorf("create invitation token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return id.String(), token, hash[:], nil
}

func invitationTokenHash(token string) ([]byte, error) {
	if len(token) != 43 {
		return nil, ErrInvalidInput
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 {
		return nil, ErrInvalidInput
	}
	hash := sha256.Sum256([]byte(token))
	return hash[:], nil
}
