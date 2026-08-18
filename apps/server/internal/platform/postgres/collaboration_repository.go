package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
	"github.com/nihatatay93/budget/internal/workspace"
)

type CollaborationRepository struct {
	pool *pgxpool.Pool
}

func NewCollaborationRepository(pool *pgxpool.Pool) *CollaborationRepository {
	return &CollaborationRepository{pool: pool}
}

func (r *CollaborationRepository) ListMembers(
	ctx context.Context, workspaceID string,
) ([]workspace.Member, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return nil, workspace.ErrForbidden
	}
	rows, err := sqlc.New(r.pool).ListActiveWorkspaceMembers(ctx, workspaceUUID)
	if err != nil {
		return nil, fmt.Errorf("list active workspace members: %w", err)
	}
	result := make([]workspace.Member, 0, len(rows))
	for _, row := range rows {
		member, err := collaborationMember(
			row.UserID, row.Email, row.DisplayName, row.Role, row.JoinedAt,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, member)
	}
	return result, nil
}

func (r *CollaborationRepository) Member(
	ctx context.Context, workspaceID, userID string,
) (workspace.Member, error) {
	workspaceUUID, userUUID, err := resourceUUIDs(workspaceID, userID)
	if err != nil {
		return workspace.Member{}, workspace.ErrNotFound
	}
	return getCollaborationMember(ctx, sqlc.New(r.pool), workspaceUUID, userUUID)
}

func (r *CollaborationRepository) ActiveOwnerCount(
	ctx context.Context, workspaceID string,
) (int64, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return 0, workspace.ErrForbidden
	}
	count, err := sqlc.New(r.pool).CountActiveWorkspaceOwners(ctx, workspaceUUID)
	if err != nil {
		return 0, fmt.Errorf("count active workspace owners: %w", err)
	}
	return count, nil
}

func (r *CollaborationRepository) ListInvitations(
	ctx context.Context, workspaceID string, now time.Time,
) ([]workspace.Invitation, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return nil, workspace.ErrForbidden
	}
	rows, err := sqlc.New(r.pool).ListPendingWorkspaceInvitations(
		ctx,
		sqlc.ListPendingWorkspaceInvitationsParams{
			WorkspaceID: workspaceUUID, ExpiresAt: postgresTime(now),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list pending workspace invitations: %w", err)
	}
	result := make([]workspace.Invitation, 0, len(rows))
	for _, row := range rows {
		invitation, err := collaborationInvitation(
			row.ID, row.WorkspaceID, row.Email, row.Role, row.InvitedBy,
			row.InviterDisplayName, row.ExpiresAt, row.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, invitation)
	}
	return result, nil
}

func (r *CollaborationRepository) Invitation(
	ctx context.Context, workspaceID, invitationID string, now time.Time,
) (workspace.Invitation, error) {
	workspaceUUID, invitationUUID, err := resourceUUIDs(workspaceID, invitationID)
	if err != nil {
		return workspace.Invitation{}, workspace.ErrNotFound
	}
	row, err := sqlc.New(r.pool).GetPendingWorkspaceInvitation(
		ctx,
		sqlc.GetPendingWorkspaceInvitationParams{
			WorkspaceID: workspaceUUID, ID: invitationUUID, ExpiresAt: postgresTime(now),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return workspace.Invitation{}, workspace.ErrNotFound
	}
	if err != nil {
		return workspace.Invitation{}, fmt.Errorf("get pending workspace invitation: %w", err)
	}
	return collaborationInvitation(
		row.ID, row.WorkspaceID, row.Email, row.Role, row.InvitedBy,
		row.InviterDisplayName, row.ExpiresAt, row.CreatedAt,
	)
}

func (r *CollaborationRepository) CreateInvitation(
	ctx context.Context, command workspace.CreateInvitationCommand,
) (workspace.Invitation, error) {
	workspaceUUID, invitationUUID, err := resourceUUIDs(command.WorkspaceID, command.ID)
	if err != nil {
		return workspace.Invitation{}, workspace.ErrInvalidInput
	}
	actorUUID, err := postgresUUID(command.ActorID)
	if err != nil {
		return workspace.Invitation{}, workspace.ErrForbidden
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return workspace.Invitation{}, fmt.Errorf("begin invitation create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := sqlc.New(tx)
	if err := lockCollaborationWorkspace(ctx, queries, workspaceUUID); err != nil {
		return workspace.Invitation{}, err
	}
	actor, err := getCollaborationMember(ctx, queries, workspaceUUID, actorUUID)
	if errors.Is(err, workspace.ErrNotFound) {
		return workspace.Invitation{}, workspace.ErrForbidden
	}
	if err != nil {
		return workspace.Invitation{}, err
	}
	if !workspace.CanInvite(actor.Role, command.Role) {
		return workspace.Invitation{}, workspace.ErrForbidden
	}
	_, err = queries.GetActiveWorkspaceMemberByEmail(
		ctx,
		sqlc.GetActiveWorkspaceMemberByEmailParams{
			WorkspaceID: workspaceUUID, Lower: command.Email,
		},
	)
	if err == nil {
		return workspace.Invitation{}, workspace.ErrAlreadyMember
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return workspace.Invitation{}, fmt.Errorf("check invitation member email: %w", err)
	}
	if err := queries.RevokeOpenWorkspaceInvitationsForEmail(
		ctx,
		sqlc.RevokeOpenWorkspaceInvitationsForEmailParams{
			WorkspaceID: workspaceUUID, Lower: command.Email, RevokedAt: postgresTime(command.Now),
		},
	); err != nil {
		return workspace.Invitation{}, fmt.Errorf("replace open workspace invitation: %w", err)
	}
	row, err := queries.CreateWorkspaceInvitation(
		ctx,
		sqlc.CreateWorkspaceInvitationParams{
			ID: invitationUUID, WorkspaceID: workspaceUUID, Email: command.Email,
			Role: string(command.Role), TokenHash: command.TokenHash, InvitedBy: actorUUID,
			ExpiresAt: postgresTime(command.ExpiresAt),
		},
	)
	if err != nil {
		return workspace.Invitation{}, fmt.Errorf("create workspace invitation: %w", err)
	}
	invitation, err := collaborationInvitation(
		row.ID, row.WorkspaceID, row.Email, row.Role, row.InvitedBy,
		actor.DisplayName, row.ExpiresAt, row.CreatedAt,
	)
	if err != nil {
		return workspace.Invitation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workspace.Invitation{}, mapCollaborationWriteError("commit invitation create", err)
	}
	return invitation, nil
}

func (r *CollaborationRepository) RevokeInvitation(
	ctx context.Context, workspaceID, actorID, invitationID string, now time.Time,
) error {
	workspaceUUID, invitationUUID, err := resourceUUIDs(workspaceID, invitationID)
	if err != nil {
		return workspace.ErrNotFound
	}
	actorUUID, err := postgresUUID(actorID)
	if err != nil {
		return workspace.ErrForbidden
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin invitation revoke: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := sqlc.New(tx)
	if err := lockCollaborationWorkspace(ctx, queries, workspaceUUID); err != nil {
		return err
	}
	actor, err := getCollaborationMember(ctx, queries, workspaceUUID, actorUUID)
	if errors.Is(err, workspace.ErrNotFound) {
		return workspace.ErrForbidden
	}
	if err != nil {
		return err
	}
	row, err := queries.GetPendingWorkspaceInvitation(
		ctx,
		sqlc.GetPendingWorkspaceInvitationParams{
			WorkspaceID: workspaceUUID, ID: invitationUUID, ExpiresAt: postgresTime(now),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return workspace.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get invitation for revoke: %w", err)
	}
	if !workspace.CanInvite(actor.Role, workspace.Role(row.Role)) {
		return workspace.ErrForbidden
	}
	rows, err := queries.RevokeWorkspaceInvitation(
		ctx,
		sqlc.RevokeWorkspaceInvitationParams{
			WorkspaceID: workspaceUUID, ID: invitationUUID, RevokedAt: postgresTime(now),
		},
	)
	if err != nil {
		return fmt.Errorf("revoke workspace invitation: %w", err)
	}
	if rows != 1 {
		return workspace.ErrInvitationUnavailable
	}
	if err := tx.Commit(ctx); err != nil {
		return mapCollaborationWriteError("commit invitation revoke", err)
	}
	return nil
}

func (r *CollaborationRepository) UpdateMemberRole(
	ctx context.Context,
	workspaceID, actorID, targetID string,
	newRole workspace.Role,
) (workspace.Member, error) {
	workspaceUUID, actorUUID, targetUUID, err := collaborationUUIDs(workspaceID, actorID, targetID)
	if err != nil {
		return workspace.Member{}, workspace.ErrNotFound
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return workspace.Member{}, fmt.Errorf("begin membership role update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := sqlc.New(tx)
	if err := lockCollaborationWorkspace(ctx, queries, workspaceUUID); err != nil {
		return workspace.Member{}, err
	}
	actor, err := getCollaborationMember(ctx, queries, workspaceUUID, actorUUID)
	if errors.Is(err, workspace.ErrNotFound) {
		return workspace.Member{}, workspace.ErrForbidden
	}
	if err != nil {
		return workspace.Member{}, err
	}
	target, err := getCollaborationMember(ctx, queries, workspaceUUID, targetUUID)
	if err != nil {
		return workspace.Member{}, err
	}
	if !workspace.CanChangeRole(actor.Role, target.Role, newRole) {
		return workspace.Member{}, workspace.ErrForbidden
	}
	if target.Role == workspace.RoleOwner && newRole != workspace.RoleOwner {
		if err := requireAdditionalDatabaseOwner(ctx, queries, workspaceUUID); err != nil {
			return workspace.Member{}, err
		}
	}
	rows, err := queries.UpdateWorkspaceMembershipRole(
		ctx,
		sqlc.UpdateWorkspaceMembershipRoleParams{
			WorkspaceID: workspaceUUID, UserID: targetUUID, Role: string(newRole),
		},
	)
	if err != nil {
		return workspace.Member{}, mapCollaborationWriteError("update workspace member role", err)
	}
	if rows != 1 {
		return workspace.Member{}, workspace.ErrNotFound
	}
	updated, err := getCollaborationMember(ctx, queries, workspaceUUID, targetUUID)
	if err != nil {
		return workspace.Member{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workspace.Member{}, mapCollaborationWriteError("commit workspace member role", err)
	}
	return updated, nil
}

func (r *CollaborationRepository) RemoveMember(
	ctx context.Context,
	workspaceID, actorID, targetID string,
	now time.Time,
) error {
	workspaceUUID, actorUUID, targetUUID, err := collaborationUUIDs(workspaceID, actorID, targetID)
	if err != nil {
		return workspace.ErrNotFound
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin membership removal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := sqlc.New(tx)
	if err := lockCollaborationWorkspace(ctx, queries, workspaceUUID); err != nil {
		return err
	}
	actor, err := getCollaborationMember(ctx, queries, workspaceUUID, actorUUID)
	if errors.Is(err, workspace.ErrNotFound) {
		return workspace.ErrForbidden
	}
	if err != nil {
		return err
	}
	target, err := getCollaborationMember(ctx, queries, workspaceUUID, targetUUID)
	if err != nil {
		return err
	}
	if !workspace.CanRemoveMember(actorID, targetID, actor.Role, target.Role) {
		return workspace.ErrForbidden
	}
	if target.Role == workspace.RoleOwner {
		if err := requireAdditionalDatabaseOwner(ctx, queries, workspaceUUID); err != nil {
			return err
		}
	}
	rows, err := queries.RemoveWorkspaceMembership(
		ctx,
		sqlc.RemoveWorkspaceMembershipParams{
			WorkspaceID: workspaceUUID, UserID: targetUUID, RemovedAt: postgresTime(now),
		},
	)
	if err != nil {
		return mapCollaborationWriteError("remove workspace member", err)
	}
	if rows != 1 {
		return workspace.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return mapCollaborationWriteError("commit workspace member removal", err)
	}
	return nil
}

func (r *CollaborationRepository) AcceptInvitation(
	ctx context.Context, tokenHash []byte, userID string, now time.Time,
) (workspace.Acceptance, error) {
	userUUID, err := postgresUUID(userID)
	if err != nil {
		return workspace.Acceptance{}, workspace.ErrInvalidInput
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return workspace.Acceptance{}, fmt.Errorf("begin invitation acceptance: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := sqlc.New(tx)
	lookup, err := queries.GetWorkspaceInvitationByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return workspace.Acceptance{}, workspace.ErrNotFound
	}
	if err != nil {
		return workspace.Acceptance{}, fmt.Errorf("find workspace invitation token: %w", err)
	}
	if err := lockCollaborationWorkspace(ctx, queries, lookup.WorkspaceID); err != nil {
		if errors.Is(err, workspace.ErrForbidden) {
			return workspace.Acceptance{}, workspace.ErrInvitationUnavailable
		}
		return workspace.Acceptance{}, err
	}
	invitation, err := queries.GetWorkspaceInvitationByTokenHashForUpdate(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return workspace.Acceptance{}, workspace.ErrNotFound
	}
	if err != nil {
		return workspace.Acceptance{}, fmt.Errorf("lock workspace invitation token: %w", err)
	}
	if invitation.AcceptedAt.Valid {
		if invitation.AcceptedBy.Valid && invitation.AcceptedBy.Bytes == userUUID.Bytes {
			acceptance, err := loadAcceptance(ctx, queries, invitation.WorkspaceID, userUUID)
			if err == nil {
				if commitErr := tx.Commit(ctx); commitErr != nil {
					return workspace.Acceptance{}, mapCollaborationWriteError("commit invitation replay", commitErr)
				}
				return acceptance, nil
			}
		}
		return workspace.Acceptance{}, workspace.ErrInvitationUnavailable
	}
	if invitation.RevokedAt.Valid || !invitation.ExpiresAt.Valid || !invitation.ExpiresAt.Time.After(now) {
		return workspace.Acceptance{}, workspace.ErrInvitationUnavailable
	}
	existing, err := queries.GetWorkspaceMembershipForUpdate(
		ctx,
		sqlc.GetWorkspaceMembershipForUpdateParams{
			WorkspaceID: invitation.WorkspaceID, UserID: userUUID,
		},
	)
	if err == nil && !existing.RemovedAt.Valid {
		return workspace.Acceptance{}, workspace.ErrAlreadyMember
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return workspace.Acceptance{}, fmt.Errorf("check accepting workspace member: %w", err)
	}
	if _, err := queries.ActivateWorkspaceMembership(
		ctx,
		sqlc.ActivateWorkspaceMembershipParams{
			WorkspaceID: invitation.WorkspaceID, UserID: userUUID,
			Role: invitation.Role, JoinedAt: postgresTime(now),
		},
	); err != nil {
		return workspace.Acceptance{}, mapCollaborationWriteError("activate workspace membership", err)
	}
	rows, err := queries.AcceptWorkspaceInvitation(
		ctx,
		sqlc.AcceptWorkspaceInvitationParams{
			ID: invitation.ID, WorkspaceID: invitation.WorkspaceID,
			AcceptedAt: postgresTime(now), AcceptedBy: userUUID,
		},
	)
	if err != nil {
		return workspace.Acceptance{}, mapCollaborationWriteError("accept workspace invitation", err)
	}
	if rows != 1 {
		return workspace.Acceptance{}, workspace.ErrInvitationUnavailable
	}
	acceptance, err := loadAcceptance(ctx, queries, invitation.WorkspaceID, userUUID)
	if err != nil {
		return workspace.Acceptance{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workspace.Acceptance{}, mapCollaborationWriteError("commit invitation acceptance", err)
	}
	return acceptance, nil
}

func lockCollaborationWorkspace(
	ctx context.Context, queries *sqlc.Queries, workspaceID pgtype.UUID,
) error {
	if _, err := queries.LockWorkspaceForCollaboration(ctx, workspaceID); errors.Is(err, pgx.ErrNoRows) {
		return workspace.ErrForbidden
	} else if err != nil {
		return fmt.Errorf("lock collaboration workspace: %w", err)
	}
	return nil
}

func getCollaborationMember(
	ctx context.Context,
	queries *sqlc.Queries,
	workspaceID, userID pgtype.UUID,
) (workspace.Member, error) {
	row, err := queries.GetActiveWorkspaceMember(
		ctx,
		sqlc.GetActiveWorkspaceMemberParams{WorkspaceID: workspaceID, UserID: userID},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return workspace.Member{}, workspace.ErrNotFound
	}
	if err != nil {
		return workspace.Member{}, fmt.Errorf("get active workspace member: %w", err)
	}
	return collaborationMember(row.UserID, row.Email, row.DisplayName, row.Role, row.JoinedAt)
}

func collaborationMember(
	userID pgtype.UUID,
	email, displayName, role string,
	joinedAt pgtype.Timestamptz,
) (workspace.Member, error) {
	id, err := stringUUID(userID)
	if err != nil || !joinedAt.Valid || !workspace.Role(role).Valid() {
		return workspace.Member{}, errors.New("database returned invalid workspace member")
	}
	return workspace.Member{
		UserID: id, Email: email, DisplayName: displayName,
		Role: workspace.Role(role), JoinedAt: joinedAt.Time,
	}, nil
}

func collaborationInvitation(
	id, workspaceID pgtype.UUID,
	email, role string,
	invitedBy pgtype.UUID,
	inviterDisplayName string,
	expiresAt, createdAt pgtype.Timestamptz,
) (workspace.Invitation, error) {
	idString, err := stringUUID(id)
	if err != nil {
		return workspace.Invitation{}, err
	}
	workspaceIDString, err := stringUUID(workspaceID)
	if err != nil {
		return workspace.Invitation{}, err
	}
	invitedByString, err := stringUUID(invitedBy)
	if err != nil || !expiresAt.Valid || !createdAt.Valid || !workspace.Role(role).Valid() {
		return workspace.Invitation{}, errors.New("database returned invalid workspace invitation")
	}
	return workspace.Invitation{
		ID: idString, WorkspaceID: workspaceIDString, Email: email, Role: workspace.Role(role),
		InvitedBy: invitedByString, InviterDisplayName: inviterDisplayName,
		ExpiresAt: expiresAt.Time, CreatedAt: createdAt.Time,
	}, nil
}

func loadAcceptance(
	ctx context.Context,
	queries *sqlc.Queries,
	workspaceID, userID pgtype.UUID,
) (workspace.Acceptance, error) {
	member, err := getCollaborationMember(ctx, queries, workspaceID, userID)
	if err != nil {
		return workspace.Acceptance{}, err
	}
	row, err := queries.GetAcceptedWorkspaceSummary(
		ctx,
		sqlc.GetAcceptedWorkspaceSummaryParams{ID: workspaceID, UserID: userID},
	)
	if err != nil {
		return workspace.Acceptance{}, fmt.Errorf("get accepted workspace summary: %w", err)
	}
	id, err := stringUUID(row.ID)
	if err != nil || !workspace.Role(row.Role).Valid() {
		return workspace.Acceptance{}, errors.New("database returned invalid accepted workspace")
	}
	return workspace.Acceptance{
		Workspace: workspace.WorkspaceSummary{
			ID: id, Name: row.Name, BaseCurrency: row.BaseCurrency,
			Timezone: row.Timezone, Role: workspace.Role(row.Role),
		},
		Member: member,
	}, nil
}

func requireAdditionalDatabaseOwner(
	ctx context.Context, queries *sqlc.Queries, workspaceID pgtype.UUID,
) error {
	count, err := queries.CountActiveWorkspaceOwners(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count owners before collaboration write: %w", err)
	}
	if count <= 1 {
		return workspace.ErrLastOwner
	}
	return nil
}

func collaborationUUIDs(
	workspaceID, actorID, targetID string,
) (pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	actorUUID, err := postgresUUID(actorID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	targetUUID, err := postgresUUID(targetID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	return workspaceUUID, actorUUID, targetUUID, nil
}

func postgresTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func mapCollaborationWriteError(action string, err error) error {
	if constraintViolation(err, "workspace_members_active_owner_required") {
		return workspace.ErrLastOwner
	}
	return fmt.Errorf("%s: %w", action, err)
}
