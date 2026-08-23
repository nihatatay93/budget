import SwiftUI

/// Members and pending invitations for one workspace. Every control is gated by
/// WorkspaceCollaborationPolicy so the interface never assumes broader rights than the API.
struct WorkspaceCollaborationView: View {
    let workspace: BudgetWorkspace
    let currentUserID: String
    @ObservedObject var model: AppModel

    @State private var invitationCreationPresented = false
    @State private var memberRemovalTarget: BudgetWorkspaceMember?
    @State private var invitationRevokeTarget: BudgetWorkspaceInvitation?

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
        List {
            Section("Workspace access") {
                LabeledContent("Workspace", value: workspace.name)
                LabeledContent("Your role", value: actorRole?.title ?? workspace.role.capitalized)
                LabeledContent("Members", value: "\(model.members.count)")
            }

            Section("Members") {
                if model.members.isEmpty && !model.isSavingResource {
                    ContentUnavailableView(
                        "No members loaded",
                        systemImage: "person.2",
                        description: Text("Pull to refresh and try again.")
                    )
                }
                ForEach(model.members) { member in
                    MemberRow(
                        member: member,
                        actorRole: actorRole,
                        currentUserID: currentUserID,
                        isBusy: model.isSavingResource,
                        onChangeRole: { role in
                            Task {
                                await model.changeMemberRole(
                                    workspaceID: workspace.id,
                                    userID: member.userID,
                                    role: role
                                )
                            }
                        },
                        onRequestRemoval: { memberRemovalTarget = member }
                    )
                }
            }

            if canListInvitations {
                Section("Pending invitations") {
                    if model.invitations.isEmpty {
                        ContentUnavailableView(
                            "No pending invitations",
                            systemImage: "envelope.open",
                            description: Text("Invite someone when this workspace is ready to be shared.")
                        )
                    }
                    ForEach(model.invitations) { invitation in
                        InvitationRow(
                            invitation: invitation,
                            onRequestRevoke: { invitationRevokeTarget = invitation }
                        )
                    }
                }
            } else {
                Section {
                    Label(
                        "Only workspace owners and admins can view pending invitations.",
                        systemImage: "lock"
                    )
                    .foregroundStyle(.secondary)
                }
            }

            ResourceErrorSection(message: model.resourceErrorMessage)
        }
        .navigationTitle("People")
        .toolbar {
            if !invitableRoles.isEmpty {
                ToolbarItem(placement: .primaryAction) {
                    Button("Invite someone", systemImage: "person.badge.plus") {
                        model.resourceErrorMessage = nil
                        invitationCreationPresented = true
                    }
                }
            }
        }
        .task(id: workspace.id) { await reload() }
        .refreshable { await reload() }
        .sheet(isPresented: $invitationCreationPresented) {
            InvitationCreationView(
                workspace: workspace,
                invitableRoles: invitableRoles,
                model: model
            )
        }
        .confirmationDialog(
            memberRemovalTarget?.userID == currentUserID
                ? "Leave \(workspace.name)?"
                : "Remove this member?",
            isPresented: Binding(
                get: { memberRemovalTarget != nil },
                set: { if !$0 { memberRemovalTarget = nil } }
            ),
            titleVisibility: .visible
        ) {
            Button(
                memberRemovalTarget?.userID == currentUserID ? "Leave workspace" : "Remove member",
                role: .destructive
            ) {
                guard let member = memberRemovalTarget else { return }
                Task {
                    await model.removeMember(workspaceID: workspace.id, userID: member.userID)
                    memberRemovalTarget = nil
                }
            }
            Button("Cancel", role: .cancel) { memberRemovalTarget = nil }
        } message: {
            Text(
                memberRemovalTarget?.userID == currentUserID
                    ? "You will lose access to this workspace until invited again."
                    : "The person will immediately lose access to this workspace."
            )
        }
        .confirmationDialog(
            "Revoke this invitation?",
            isPresented: Binding(
                get: { invitationRevokeTarget != nil },
                set: { if !$0 { invitationRevokeTarget = nil } }
            ),
            titleVisibility: .visible
        ) {
            Button("Revoke invitation", role: .destructive) {
                guard let invitation = invitationRevokeTarget else { return }
                Task {
                    await model.revokeInvitation(
                        workspaceID: workspace.id,
                        invitationID: invitation.id
                    )
                    invitationRevokeTarget = nil
                }
            }
            Button("Cancel", role: .cancel) { invitationRevokeTarget = nil }
        } message: {
            Text("Its one-time acceptance code will stop working immediately.")
        }
        .overlay(alignment: .bottom) {
            if model.isSavingResource {
                ProgressView()
                    .padding()
                    .background(.regularMaterial, in: Capsule())
                    .padding()
            }
        }
    }

    private func reload() async {
        await model.loadCollaboration(
            workspaceID: workspace.id,
            canListInvitations: canListInvitations
        )
    }
}

private struct MemberRow: View {
    let member: BudgetWorkspaceMember
    let actorRole: BudgetWorkspaceRoleValue?
    let currentUserID: String
    let isBusy: Bool
    let onChangeRole: (BudgetWorkspaceRoleValue) -> Void
    let onRequestRemoval: () -> Void

    private var isSelf: Bool { member.userID == currentUserID }

    private var assignableRoles: [BudgetWorkspaceRoleValue] {
        guard let actorRole else { return [] }
        return WorkspaceCollaborationPolicy.assignableRoles(
            actorRole: actorRole,
            targetRole: member.role
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
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .top, spacing: 12) {
                identity
                Spacer(minLength: 10)
                roleControl
            }
            VStack(alignment: .leading, spacing: 10) {
                identity
                roleControl
            }
        }
        .padding(.vertical, 3)
        .swipeActions {
            if canRemove {
                Button(isSelf ? "Leave" : "Remove", role: .destructive) {
                    onRequestRemoval()
                }
            }
        }
        .contextMenu {
            if canRemove {
                Button(
                    isSelf ? "Leave workspace" : "Remove member",
                    systemImage: "person.crop.circle.badge.minus",
                    role: .destructive
                ) {
                    onRequestRemoval()
                }
            }
        }
    }

    private var identity: some View {
        HStack(alignment: .top, spacing: 12) {
            Text(initials)
                .font(.caption.weight(.bold))
                .foregroundStyle(.blue)
                .frame(width: 38, height: 38)
                .background(.blue.opacity(0.12), in: Circle())
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(isSelf ? "\(member.displayName) (you)" : member.displayName)
                    .font(.headline)
                Text(member.email)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Joined \(member.joinedAt.formatted(date: .abbreviated, time: .omitted))")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }

    @ViewBuilder
    private var roleControl: some View {
        Group {
            if assignableRoles.isEmpty {
                Text(member.role.title)
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
            } else {
                Menu(member.role.title) {
                    ForEach(assignableRoles) { role in
                        Button {
                            onChangeRole(role)
                        } label: {
                            if role == member.role {
                                Label(role.title, systemImage: "checkmark")
                            } else {
                                Text(role.title)
                            }
                        }
                    }
                }
                .disabled(isBusy)
                .accessibilityLabel("Role for \(member.displayName)")
            }
        }
    }

    private var initials: String {
        let parts = member.displayName.split(separator: " ")
        return parts.prefix(2).compactMap(\.first).map(String.init).joined().uppercased()
    }
}

private struct InvitationRow: View {
    let invitation: BudgetWorkspaceInvitation
    let onRequestRevoke: () -> Void

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: "envelope.fill")
                .foregroundStyle(.orange)
                .frame(width: 36, height: 36)
                .background(.orange.opacity(0.12), in: Circle())
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(invitation.email)
                    .font(.headline)
                Text("\(invitation.role.title) · invited by \(invitation.inviterDisplayName)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Expires \(invitation.expiresAt.formatted(date: .abbreviated, time: .omitted))")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 3)
        .accessibilityElement(children: .combine)
        .swipeActions {
            Button("Revoke", role: .destructive) { onRequestRevoke() }
        }
        .contextMenu {
            Button("Revoke invitation", systemImage: "xmark.circle", role: .destructive) {
                onRequestRevoke()
            }
        }
    }
}

private struct InvitationCreationView: View {
    let workspace: BudgetWorkspace
    let invitableRoles: [BudgetWorkspaceRoleValue]
    @ObservedObject var model: AppModel
    @Environment(\.dismiss) private var dismiss

    @State private var email = ""
    @State private var role: BudgetWorkspaceRoleValue

    init(
        workspace: BudgetWorkspace,
        invitableRoles: [BudgetWorkspaceRoleValue],
        model: AppModel
    ) {
        self.workspace = workspace
        self.invitableRoles = invitableRoles
        self.model = model
        _role = State(initialValue: invitableRoles.first ?? .member)
    }

    var body: some View {
        NavigationStack {
            Form {
                if let issued = model.issuedInvitation {
                    Section {
                        Label(
                            "Save this code now. Budget does not store or email it for later recovery.",
                            systemImage: "key.fill"
                        )
                        .foregroundStyle(.orange)
                        Text(issued.acceptanceToken)
                            .font(.body.monospaced())
                            .textSelection(.enabled)
                            .accessibilityLabel("Invitation code \(issued.acceptanceToken)")
                        ShareLink(
                            "Share invitation code",
                            item: issued.acceptanceToken,
                            subject: Text("Invitation to \(workspace.name)")
                        )
                    } header: {
                        Text("One-time invitation code")
                    } footer: {
                        Text("This credential grants the invited role until it expires or is revoked.")
                    }
                } else {
                    Section {
                        TextField("Email", text: $email)
                            .textContentType(.emailAddress)
                            .textInputAutocapitalization(.never)
                            .keyboardType(.emailAddress)
                            .autocorrectionDisabled()
                        Picker("Role", selection: $role) {
                            ForEach(invitableRoles) { role in
                                Text(role.title).tag(role)
                            }
                        }
                    } header: {
                        Text("Invite someone")
                    } footer: {
                        Text("The server returns an acceptance code once. Share it only with the intended person.")
                    }
                    ResourceErrorSection(message: model.resourceErrorMessage)
                }
            }
            .scrollDismissesKeyboard(.interactively)
            .navigationTitle(model.issuedInvitation == nil ? "New invitation" : "Save invitation")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                if model.issuedInvitation == nil {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { dismiss() }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Create") { create() }
                            .disabled(model.isSavingResource || email.nilIfBlank == nil)
                    }
                } else {
                    ToolbarItem(placement: .confirmationAction) {
                        Button("I saved the code") {
                            model.acknowledgeIssuedInvitation()
                            dismiss()
                        }
                    }
                }
            }
        }
        .interactiveDismissDisabled(model.issuedInvitation != nil)
    }

    private func create() {
        Task {
            let created = await model.createInvitation(
                workspaceID: workspace.id,
                input: BudgetInvitationInput(
                    email: email.trimmingCharacters(in: .whitespacesAndNewlines),
                    role: role
                )
            )
            if created { email = "" }
        }
    }
}

struct InvitationAcceptanceView: View {
    @ObservedObject var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var acceptanceToken = ""

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Invitation code", text: $acceptanceToken)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                } header: {
                    Text("Invitation")
                } footer: {
                    Text("Invitation codes contain \(WorkspaceInvitationCredential.requiredLength) characters. The code is sent in the secure request body and is never placed in a URL.")
                }
                ResourceErrorSection(message: model.resourceErrorMessage)
            }
            .scrollDismissesKeyboard(.interactively)
            .navigationTitle("Join workspace")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Join") { accept() }
                        .disabled(
                            model.isSavingResource
                                || !WorkspaceInvitationCredential.isValid(acceptanceToken)
                        )
                }
            }
        }
    }

    private func accept() {
        Task {
            let joined = await model.acceptInvitation(
                acceptanceToken: acceptanceToken.trimmingCharacters(in: .whitespacesAndNewlines)
            )
            if joined { dismiss() }
        }
    }
}
