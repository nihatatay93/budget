import Foundation

/// Workspace membership roles, mirroring the server's WorkspaceRole contract.
enum BudgetWorkspaceRole: String, CaseIterable, Sendable {
    case owner
    case admin
    case member
    case viewer

    /// Whether the role may create or change workspace resources.
    var canManage: Bool {
        switch self {
        case .owner, .admin, .member: true
        case .viewer: false
        }
    }
}

extension BudgetWorkspace {
    /// Whether the signed-in member may modify this workspace.
    ///
    /// An unrecognised role denies management rather than granting it. The server is the
    /// real authority, but a client that fails open would show edit controls that every
    /// request then rejects.
    var canManage: Bool {
        BudgetWorkspaceRole(rawValue: role)?.canManage ?? false
    }
}

extension String {
    /// The trimmed value, or nil when it holds only whitespace.
    var nilIfBlank: String? {
        let value = trimmingCharacters(in: .whitespacesAndNewlines)
        return value.isEmpty ? nil : value
    }
}
