const MANAGING_ROLES = new Set(["owner", "admin", "member"]);

/**
 * Whether the signed-in member may modify this workspace.
 *
 * An unrecognised role denies management rather than granting it. The server is the real
 * authority, but a client that fails open renders controls that every request then rejects.
 */
export function canManageWorkspace(workspace: { role: string }): boolean {
  return MANAGING_ROLES.has(workspace.role);
}

export type WorkspaceRole = "owner" | "admin" | "member" | "viewer";
export type InvitationRole = Exclude<WorkspaceRole, "owner">;

export const INVITATION_ROLES: InvitationRole[] = ["admin", "member", "viewer"];
export const ASSIGNABLE_ROLES: WorkspaceRole[] = ["owner", "admin", "member", "viewer"];

function knownRole(value: string): WorkspaceRole | null {
  return (ASSIGNABLE_ROLES as string[]).includes(value) ? (value as WorkspaceRole) : null;
}

/*
 * The predicates below mirror internal/workspace/collaboration.go. The server stays the
 * authority; these exist so the interface offers only actions it will accept, rather than
 * rendering controls that every request then rejects.
 */

/** Pending invitations expose the email addresses of people not yet in the workspace. */
export function canListInvitations(actorRole: string): boolean {
  return actorRole === "owner" || actorRole === "admin";
}

/** An admin may not invite a peer or a superior. */
export function canInvite(actorRole: string, invitationRole: string): boolean {
  switch (knownRole(actorRole)) {
    case "owner":
      return invitationRole !== "owner" && knownRole(invitationRole) !== null;
    case "admin":
      return invitationRole === "member" || invitationRole === "viewer";
    default:
      return false;
  }
}

export function invitableRoles(actorRole: string): InvitationRole[] {
  return INVITATION_ROLES.filter((candidate) => canInvite(actorRole, candidate));
}

/** An admin may only move members and viewers between those two roles. */
export function canChangeRole(actorRole: string, targetRole: string, newRole: string): boolean {
  if (knownRole(newRole) === null) return false;
  if (actorRole === "owner") return true;
  return (
    actorRole === "admin" &&
    (targetRole === "member" || targetRole === "viewer") &&
    (newRole === "member" || newRole === "viewer")
  );
}

export function assignableRoles(actorRole: string, targetRole: string): WorkspaceRole[] {
  return ASSIGNABLE_ROLES.filter((candidate) => canChangeRole(actorRole, targetRole, candidate));
}

/** Anyone may leave a workspace, whatever their role. */
export function canRemoveMember(
  actorID: string,
  targetID: string,
  actorRole: string,
  targetRole: string,
): boolean {
  if (actorID === targetID) return true;
  if (actorRole === "owner") return true;
  return actorRole === "admin" && (targetRole === "member" || targetRole === "viewer");
}
