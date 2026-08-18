import SwiftUI

/// Members, invitations, and invitation acceptance for one workspace. Every control is gated
/// by WorkspaceCollaborationPolicy so the screen offers only what the server will accept.
struct WorkspaceCollaborationView: View {
    let workspace: BudgetWorkspace
    let currentUserID: String
    @ObservedObject var model: AppModel

    @State private var invitationEmail = ""
    @State private var invitationRole: BudgetWorkspaceRoleValue = .member
    @State private var acceptanceToken = ""

    private var actorRole: BudgetWorkspaceRoleValue? { workspace.membershipRole }

    private var invitableRoles: [BudgetWorkspaceRoleValue] {
        guard let actorRole else { return [] }
        return WorkspaceCollaborationPolicy.invitableRoles(actorRole: actorRole)
    }

    private var canListInvitations: Bool {
        guard let actorRole else { return false }
        return WorkspaceCollaborationPolicy.canListInvitations(actorRole: actorRole)
    }

    var body: some View {
        Form {
            membersSection
            if canListInvitations {
                invitationsSection
                inviteSection
            }
            acceptSection
            ResourceErrorSection(message: model.resourceErrorMessage)
        }
        .navigationTitle("Collaboration")
        .navigationBarTitleDisplayMode(.inline)
        .task(id: workspace.id) {
            await model.loadCollaboration(
                workspaceID: workspace.id, canListInvitations: canListInvitations
            )
        }
        .onAppear {
            if let first = invitableRoles.first { invitationRole = first }
        }
    }

    private var membersSection: some View {
        Section("Members") {
            ForEach(model.members) { member in
                MemberRow(
                    member: member,
                    actorRole: actorRole,
                    currentUserID: currentUserID,
                    workspaceName: workspace.name,
                    isBusy: model.isSavingResource,
                    onChangeRole: { role in
                        Task {
                            await model.changeMemberRole(
                                workspaceID: workspace.id, userID: member.userID, role: role
                            )
                        }
                    },
                    onRemove: {
                        Task {
                            await model.removeMember(
                                workspaceID: workspace.id, userID: member.userID
                            )
                        }
                    }
                )
            }
            if model.members.isEmpty {
                Text("No members loaded yet.").foregroundStyle(.secondary)
            }
        }
    }

    private var invitationsSection: some View {
        Section("Pending invitations") {
            ForEach(model.invitations) { invitation in
                VStack(alignment: .leading, spacing: 4) {
                    Text(invitation.email).font(.headline)
                    Text("\(invitation.role.title) · invited by \(invitation.inviterDisplayName)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Text("Expires \(invitation.expiresAt.formatted(date: .abbreviated, time: .omitted))")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
                .swipeActions {
                    Button("Revoke", role: .destructive) {
                        Task {
                            await model.revokeInvitation(
                                workspaceID: workspace.id, invitationID: invitation.id
                            )
                        }
                    }
                }
            }
            if model.invitations.isEmpty {
                Text("No invitations are waiting to be accepted.").foregroundStyle(.secondary)
            }
        }
    }

    private var inviteSection: some View {
        Section("Invite someone") {
            TextField("Email", text: $invitationEmail)
                .textContentType(.emailAddress)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
            Picker("Role", selection: $invitationRole) {
                ForEach(invitableRoles) { role in
                    Text(role.title).tag(role)
                }
            }
            Button("Create invitation") {
                Task {
                    let created = await model.createInvitation(
                        workspaceID: workspace.id,
                        input: BudgetInvitationInput(
                            email: invitationEmail.trimmingCharacters(in: .whitespacesAndNewlines),
                            role: invitationRole
                        )
                    )
                    if created { invitationEmail = "" }
                }
            }
            .disabled(model.isSavingResource || invitationEmail.nilIfBlank == nil)

            if let issued = model.issuedInvitation {
                IssuedInvitationRow(issued: issued)
            }
        }
    }

    private var acceptSection: some View {
        Section("Accept an invitation") {
            TextField("Invitation code", text: $acceptanceToken)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
            Button("Join workspace") {
                Task {
                    let joined = await model.acceptInvitation(
                        acceptanceToken: acceptanceToken.trimmingCharacters(in: .whitespacesAndNewlines)
                    )
                    if joined { acceptanceToken = "" }
                }
            }
            .disabled(model.isSavingResource || acceptanceToken.nilIfBlank == nil)
        }
    }
}

private struct MemberRow: View {
    let member: BudgetWorkspaceMember
    let actorRole: BudgetWorkspaceRoleValue?
    let currentUserID: String
    let workspaceName: String
    let isBusy: Bool
    let onChangeRole: (BudgetWorkspaceRoleValue) -> Void
    let onRemove: () -> Void

    private var isSelf: Bool { member.userID == currentUserID }

    private var assignableRoles: [BudgetWorkspaceRoleValue] {
        guard let actorRole else { return [] }
        return WorkspaceCollaborationPolicy.assignableRoles(
            actorRole: actorRole, targetRole: member.role
        )
    }

    private var canRemove: Bool {
        guard let actorRole else { return false }
        return WorkspaceCollaborationPolicy.canRemoveMember(
            actorID: currentUserID,
            targetID: member.userID,
            actorRole: actorRole,
            targetRole: member.role
        )
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(isSelf ? "\(member.displayName) (you)" : member.displayName).font(.headline)
            Text(member.email).font(.caption).foregroundStyle(.secondary)
            if assignableRoles.isEmpty {
                Text(member.role.title).font(.caption).foregroundStyle(.secondary)
            } else {
                Picker("Role", selection: Binding(
                    get: { member.role },
                    set: { onChangeRole($0) }
                )) {
                    ForEach(assignableRoles) { role in
                        Text(role.title).tag(role)
                    }
                }
                .pickerStyle(.segmented)
                .disabled(isBusy)
            }
        }
        .swipeActions {
            if canRemove {
                Button(isSelf ? "Leave" : "Remove", role: .destructive, action: onRemove)
            }
        }
    }
}

/// The acceptance token is disclosed once, at creation. There is no email delivery yet, so
/// the inviter passes it on; it is shown here and never stored.
private struct IssuedInvitationRow: View {
    let issued: BudgetIssuedInvitation

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Share this one-time code with \(issued.invitation.email).")
                .font(.caption)
                .foregroundStyle(.secondary)
            Text(issued.acceptanceToken)
                .font(.footnote.monospaced())
                .textSelection(.enabled)
        }
    }
}
