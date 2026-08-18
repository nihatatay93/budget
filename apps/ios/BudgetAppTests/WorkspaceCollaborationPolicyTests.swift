import Testing
@testable import Budget

/// These mirror the tables in internal/workspace/collaboration_test.go. If the server policy
/// changes and this does not, the app starts offering actions the server rejects.
@Suite("Collaboration policy")
struct WorkspaceCollaborationPolicyTests {
    @Test("Only owners and admins may see pending invitations")
    func invitationVisibility() {
        #expect(WorkspaceCollaborationPolicy.canListInvitations(actorRole: .owner))
        #expect(WorkspaceCollaborationPolicy.canListInvitations(actorRole: .admin))
        #expect(WorkspaceCollaborationPolicy.canListInvitations(actorRole: .member) == false)
        #expect(WorkspaceCollaborationPolicy.canListInvitations(actorRole: .viewer) == false)
    }

    @Test("An owner may invite any role except another owner")
    func ownerInvitations() {
        #expect(WorkspaceCollaborationPolicy.invitableRoles(actorRole: .owner) == [.admin, .member, .viewer])
        #expect(WorkspaceCollaborationPolicy.canInvite(actorRole: .owner, invitationRole: .owner) == false)
    }

    @Test("An admin may not invite a peer or a superior")
    func adminInvitations() {
        #expect(WorkspaceCollaborationPolicy.invitableRoles(actorRole: .admin) == [.member, .viewer])
        #expect(WorkspaceCollaborationPolicy.canInvite(actorRole: .admin, invitationRole: .admin) == false)
        #expect(WorkspaceCollaborationPolicy.canInvite(actorRole: .admin, invitationRole: .owner) == false)
    }

    @Test("Members and viewers may not invite at all", arguments: [
        BudgetWorkspaceRoleValue.member, BudgetWorkspaceRoleValue.viewer,
    ])
    func nonManagersCannotInvite(role: BudgetWorkspaceRoleValue) {
        #expect(WorkspaceCollaborationPolicy.invitableRoles(actorRole: role).isEmpty)
    }

    @Test("An owner may set any role, including transferring ownership")
    func ownerRoleChanges() {
        #expect(WorkspaceCollaborationPolicy.canChangeRole(actorRole: .owner, targetRole: .member, newRole: .owner))
        #expect(WorkspaceCollaborationPolicy.canChangeRole(actorRole: .owner, targetRole: .owner, newRole: .member))
        #expect(
            WorkspaceCollaborationPolicy.assignableRoles(actorRole: .owner, targetRole: .member)
                == [.owner, .admin, .member, .viewer]
        )
    }

    @Test("An admin may only move members and viewers between those roles")
    func adminRoleChanges() {
        #expect(WorkspaceCollaborationPolicy.canChangeRole(actorRole: .admin, targetRole: .member, newRole: .viewer))
        #expect(WorkspaceCollaborationPolicy.canChangeRole(actorRole: .admin, targetRole: .member, newRole: .admin) == false)
        #expect(WorkspaceCollaborationPolicy.canChangeRole(actorRole: .admin, targetRole: .admin, newRole: .member) == false)
        #expect(WorkspaceCollaborationPolicy.canChangeRole(actorRole: .admin, targetRole: .owner, newRole: .member) == false)
        #expect(
            WorkspaceCollaborationPolicy.assignableRoles(actorRole: .admin, targetRole: .member)
                == [.member, .viewer]
        )
    }

    @Test("Members and viewers may change nobody's role")
    func nonManagersCannotChangeRoles() {
        for actor in [BudgetWorkspaceRoleValue.member, .viewer] {
            #expect(
                WorkspaceCollaborationPolicy.assignableRoles(actorRole: actor, targetRole: .viewer).isEmpty
            )
        }
    }

    @Test("Anyone may leave, whatever their role", arguments: BudgetWorkspaceRoleValue.allCases)
    func anyoneMayLeave(role: BudgetWorkspaceRoleValue) {
        #expect(
            WorkspaceCollaborationPolicy.canRemoveMember(
                actorID: "me", targetID: "me", actorRole: role, targetRole: role
            )
        )
    }

    @Test("Removal rights follow the documented policy")
    func removalPolicy() {
        let policy = WorkspaceCollaborationPolicy.self
        #expect(policy.canRemoveMember(actorID: "a", targetID: "b", actorRole: .owner, targetRole: .admin))
        #expect(policy.canRemoveMember(actorID: "a", targetID: "b", actorRole: .admin, targetRole: .member))
        #expect(policy.canRemoveMember(actorID: "a", targetID: "b", actorRole: .admin, targetRole: .viewer))
        #expect(policy.canRemoveMember(actorID: "a", targetID: "b", actorRole: .admin, targetRole: .admin) == false)
        #expect(policy.canRemoveMember(actorID: "a", targetID: "b", actorRole: .admin, targetRole: .owner) == false)
        #expect(policy.canRemoveMember(actorID: "a", targetID: "b", actorRole: .member, targetRole: .viewer) == false)
        #expect(policy.canRemoveMember(actorID: "a", targetID: "b", actorRole: .viewer, targetRole: .member) == false)
    }

    /// A role this build does not recognise yields no rights rather than being treated as a
    /// managing role.
    @Test("An unrecognised workspace role grants nothing")
    func unknownRoleGrantsNothing() {
        let workspace = BudgetWorkspace(
            id: "w", name: "Atay Family", baseCurrency: .turkishLira,
            timezone: "Europe/Istanbul", role: "auditor"
        )
        #expect(workspace.membershipRole == nil)
        #expect(workspace.canManage == false)
    }
}
