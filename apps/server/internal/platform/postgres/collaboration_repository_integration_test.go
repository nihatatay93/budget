//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nihatatay93/budget/internal/auth"
	cryptoplatform "github.com/nihatatay93/budget/internal/platform/crypto"
	"github.com/nihatatay93/budget/internal/workspace"
)

func TestCollaborationRepositoryInvitationAndMembershipLifecycle(t *testing.T) {
	ctx := context.Background()
	container, err := postgrescontainer.Run(
		ctx,
		"postgres:18-alpine",
		postgrescontainer.WithDatabase("budget_test"),
		postgrescontainer.WithUsername("budget"),
		postgrescontainer.WithPassword("budget"),
		postgrescontainer.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	testcontainers.CleanupContainer(t, container)
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}
	if err := Migrate(ctx, databaseURL); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(store.Close)

	authentication, err := auth.NewService(
		NewAuthRepository(store.Pool()), cryptoplatform.PasswordHasher{}, 24*time.Hour,
	)
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	owner := registerCollaborationUser(t, ctx, authentication, "owner@example.com", "Owner")
	first := registerCollaborationUser(t, ctx, authentication, "first@example.com", "First")
	second := registerCollaborationUser(t, ctx, authentication, "second@example.com", "Second")
	workspaceID := owner.Workspaces[0].ID
	ownerID := owner.Principal.User.ID
	firstID := first.Principal.User.ID
	secondID := second.Principal.User.ID

	access := workspace.NewAuthorizer(NewWorkspaceRepository(store.Pool()))
	service := workspace.NewCollaborationService(
		NewCollaborationRepository(store.Pool()), access, time.Now,
	)

	firstCredential, err := service.CreateInvitation(
		ctx, workspaceID, ownerID,
		workspace.InvitationInput{Email: "intended@example.com", Role: workspace.RoleMember},
	)
	if err != nil {
		t.Fatalf("create first invitation: %v", err)
	}
	replacement, err := service.CreateInvitation(
		ctx, workspaceID, ownerID,
		workspace.InvitationInput{Email: "INTENDED@example.com", Role: workspace.RoleViewer},
	)
	if err != nil {
		t.Fatalf("replace first invitation: %v", err)
	}
	if firstCredential.Token == replacement.Token || firstCredential.Invitation.ID == replacement.Invitation.ID {
		t.Fatal("replacement invitation reused its credential")
	}
	if _, err := service.AcceptInvitation(ctx, firstID, firstCredential.Token); !errors.Is(
		err, workspace.ErrInvitationUnavailable,
	) {
		t.Fatalf("replaced token error = %v, want ErrInvitationUnavailable", err)
	}
	accepted, err := service.AcceptInvitation(ctx, firstID, replacement.Token)
	if err != nil {
		t.Fatalf("accept email-independent invitation: %v", err)
	}
	if accepted.Workspace.ID != workspaceID || accepted.Member.UserID != firstID ||
		accepted.Member.Role != workspace.RoleViewer {
		t.Fatalf("accepted membership = %#v", accepted)
	}
	if replay, err := service.AcceptInvitation(ctx, firstID, replacement.Token); err != nil ||
		replay.Member.UserID != firstID {
		t.Fatalf("same-user acceptance replay = %#v, %v", replay, err)
	}

	firstMember, err := service.UpdateMemberRole(
		ctx, workspaceID, ownerID, firstID, workspace.RoleAdmin,
	)
	if err != nil || firstMember.Role != workspace.RoleAdmin {
		t.Fatalf("promote first member to admin = %#v, %v", firstMember, err)
	}
	if _, err := service.CreateInvitation(
		ctx, workspaceID, firstID,
		workspace.InvitationInput{Email: "admin@example.com", Role: workspace.RoleAdmin},
	); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("admin-to-admin invitation error = %v, want ErrForbidden", err)
	}
	secondCredential, err := service.CreateInvitation(
		ctx, workspaceID, firstID,
		workspace.InvitationInput{Email: "delivered-elsewhere@example.com", Role: workspace.RoleViewer},
	)
	if err != nil {
		t.Fatalf("admin create viewer invitation: %v", err)
	}
	if _, err := service.AcceptInvitation(ctx, firstID, secondCredential.Token); !errors.Is(
		err, workspace.ErrAlreadyMember,
	) {
		t.Fatalf("active-member acceptance error = %v, want ErrAlreadyMember", err)
	}
	if _, err := service.AcceptInvitation(ctx, secondID, secondCredential.Token); err != nil {
		t.Fatalf("invitation consumed after existing-member conflict: %v", err)
	}
	if _, err := service.UpdateMemberRole(
		ctx, workspaceID, firstID, secondID, workspace.RoleMember,
	); err != nil {
		t.Fatalf("admin change viewer to member: %v", err)
	}

	// Authored history must be a complete aggregate: ADR 0003 requires at least one entry,
	// and the reconciliation trigger is deferred, so the account, transaction, and entry are
	// written in one statement that commits them together.
	if _, err := store.Pool().Exec(ctx, `
		WITH new_account AS (
			INSERT INTO accounts (workspace_id, name, type, currency)
			VALUES ($1, 'Audit trail', 'bank', 'TRY')
			RETURNING id
		), new_transaction AS (
			INSERT INTO transactions (
				workspace_id, kind, status, transaction_date, source, created_by, updated_by
			)
			VALUES ($1, 'adjustment', 'posted', CURRENT_DATE, 'manual', $2, $2)
			RETURNING id
		)
		INSERT INTO transaction_entries (
			workspace_id, transaction_id, account_id, amount_minor, base_amount_minor
		)
		SELECT $1, new_transaction.id, new_account.id, 500000, 500000
		FROM new_transaction, new_account
	`, workspaceID, secondID); err != nil {
		t.Fatalf("insert member-authored history: %v", err)
	}
	if err := service.RemoveMember(ctx, workspaceID, firstID, secondID); err != nil {
		t.Fatalf("admin remove member with history: %v", err)
	}
	if err := access.RequireRead(ctx, workspaceID, secondID); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("removed member read error = %v, want ErrForbidden", err)
	}
	var retained, removed bool
	if err := store.Pool().QueryRow(ctx, `
		SELECT true, removed_at IS NOT NULL
		FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, secondID).Scan(&retained, &removed); err != nil || !retained || !removed {
		t.Fatalf("soft-removed membership retained/removed = %v/%v, %v", retained, removed, err)
	}

	reactivation, err := service.CreateInvitation(
		ctx, workspaceID, firstID,
		workspace.InvitationInput{Email: "second@example.com", Role: workspace.RoleMember},
	)
	if err != nil {
		t.Fatalf("reinvite removed member: %v", err)
	}
	if _, err := service.AcceptInvitation(ctx, secondID, reactivation.Token); err != nil {
		t.Fatalf("reactivate removed member: %v", err)
	}
	if err := access.RequireRead(ctx, workspaceID, secondID); err != nil {
		t.Fatalf("reactivated member read access: %v", err)
	}

	if err := service.RemoveMember(ctx, workspaceID, ownerID, ownerID); !errors.Is(
		err, workspace.ErrLastOwner,
	) {
		t.Fatalf("last owner departure error = %v, want ErrLastOwner", err)
	}
	if _, err := service.UpdateMemberRole(
		ctx, workspaceID, ownerID, firstID, workspace.RoleOwner,
	); err != nil {
		t.Fatalf("appoint second owner: %v", err)
	}
	if err := service.RemoveMember(ctx, workspaceID, ownerID, ownerID); err != nil {
		t.Fatalf("owner leave after appointing successor: %v", err)
	}
	ownerSession, err := authentication.Session(ctx, owner.Principal)
	if err != nil {
		t.Fatalf("load departed owner session: %v", err)
	}
	for _, value := range ownerSession.Workspaces {
		if value.ID == workspaceID {
			t.Fatal("soft-removed owner remained in session workspaces")
		}
	}

	if _, err := service.ListMembers(ctx, second.Workspaces[0].ID, firstID); !errors.Is(
		err, workspace.ErrForbidden,
	) {
		t.Fatalf("cross-workspace member listing error = %v, want ErrForbidden", err)
	}
}

func registerCollaborationUser(
	t *testing.T,
	ctx context.Context,
	service *auth.Service,
	email, displayName string,
) auth.AuthResult {
	t.Helper()
	result, err := service.Register(ctx, auth.RegisterInput{
		Email: email, Password: "a sufficiently long password", DisplayName: displayName,
		WorkspaceName: displayName + " Personal", BaseCurrency: "TRY",
		Timezone: "Europe/Istanbul", Transport: auth.TransportBearer,
	})
	if err != nil {
		t.Fatalf("register %s: %v", email, err)
	}
	return result
}
