import Foundation

enum WorkspaceInvitationCredential {
    /// Mirrors the exact min/max length in the OpenAPI invitation schemas.
    static let requiredLength = 43

    static func isValid(_ value: String) -> Bool {
        value.trimmingCharacters(in: .whitespacesAndNewlines).count == requiredLength
    }
}

/*
 * These predicates mirror internal/workspace/collaboration.go. The server stays the
 * authority; they exist so the interface offers only actions it will accept, rather than
 * showing controls that every request then rejects.
 */
enum WorkspaceCollaborationPolicy {
    /// Pending invitations expose the email addresses of people who are not yet members.
    static func canListInvitations(actorRole: BudgetWorkspaceRoleValue) -> Bool {
        actorRole == .owner || actorRole == .admin
    }

    /// An admin may not invite a peer or a superior.
    static func canInvite(
        actorRole: BudgetWorkspaceRoleValue,
        invitationRole: BudgetWorkspaceRoleValue
    ) -> Bool {
        switch actorRole {
        case .owner: invitationRole != .owner
        case .admin: invitationRole == .member || invitationRole == .viewer
        case .member, .viewer: false
        }
    }

    static func invitableRoles(
        actorRole: BudgetWorkspaceRoleValue
    ) -> [BudgetWorkspaceRoleValue] {
        BudgetWorkspaceRoleValue.allCases.filter {
            canInvite(actorRole: actorRole, invitationRole: $0)
        }
    }

    /// An admin may only move members and viewers between those two roles.
    static func canChangeRole(
        actorRole: BudgetWorkspaceRoleValue,
        targetRole: BudgetWorkspaceRoleValue,
        newRole: BudgetWorkspaceRoleValue
    ) -> Bool {
        if actorRole == .owner { return true }
        return actorRole == .admin
            && (targetRole == .member || targetRole == .viewer)
            && (newRole == .member || newRole == .viewer)
    }

    static func assignableRoles(
        actorRole: BudgetWorkspaceRoleValue,
        targetRole: BudgetWorkspaceRoleValue
    ) -> [BudgetWorkspaceRoleValue] {
        BudgetWorkspaceRoleValue.allCases.filter {
            canChangeRole(actorRole: actorRole, targetRole: targetRole, newRole: $0)
        }
    }

    /// Anyone may leave a workspace, whatever their role.
    static func canRemoveMember(
        actorID: String,
        targetID: String,
        actorRole: BudgetWorkspaceRoleValue,
        targetRole: BudgetWorkspaceRoleValue
    ) -> Bool {
        if actorID == targetID { return true }
        if actorRole == .owner { return true }
        return actorRole == .admin && (targetRole == .member || targetRole == .viewer)
    }
}

extension BudgetWorkspace {
    /// The workspace role as a known value, or nil when the server sent something this build
    /// does not recognise. Callers treat nil as "no rights" rather than assuming any.
    var membershipRole: BudgetWorkspaceRoleValue? {
        BudgetWorkspaceRoleValue(rawValue: role)
    }
}
