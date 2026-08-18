import Testing
@testable import Budget

private func workspace(role: String) -> BudgetWorkspace {
    BudgetWorkspace(
        id: "00000000-0000-7000-8000-000000000010",
        name: "Atay Family",
        baseCurrency: .turkishLira,
        timezone: "Europe/Istanbul",
        role: role
    )
}

@Suite("Workspace permissions")
struct WorkspacePermissionsTests {
    @Test("Owner, admin and member may manage; viewer may not")
    func rolesGrantExpectedManagement() {
        #expect(workspace(role: "owner").canManage)
        #expect(workspace(role: "admin").canManage)
        #expect(workspace(role: "member").canManage)
        #expect(workspace(role: "viewer").canManage == false)
    }

    /// An unrecognised role must deny management. Failing open would render edit controls
    /// that every server request then rejects.
    @Test("An unrecognised role denies management", arguments: ["", "guest", "Owner", "admin "])
    func unknownRolesFailClosed(role: String) {
        #expect(workspace(role: role).canManage == false)
    }

    @Test("Every declared role maps from its wire value")
    func rolesRoundTrip() {
        for role in BudgetWorkspaceRole.allCases {
            #expect(BudgetWorkspaceRole(rawValue: role.rawValue) == role)
        }
    }
}
