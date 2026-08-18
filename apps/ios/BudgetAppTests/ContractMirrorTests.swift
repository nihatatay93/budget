import BudgetAPI
import Testing
@testable import Budget

/// The app keeps hand-written enums for values the generated contract also names, so views
/// can enumerate them. A copy that drifts offers a control the server rejects, or omits one
/// it accepts. Comparing raw values against the generated types makes regeneration the thing
/// that fails this, rather than a user.
@Suite("Contract mirrors")
struct ContractMirrorTests {
    @Test("Every app currency is a contract currency, and none is missing")
    func currenciesMatch() {
        let app = Set(BudgetCurrency.allCases.map(\.rawValue))
        let contract = Set(Components.Schemas.Currency.allCases.map(\.rawValue))
        #expect(app == contract)
        for currency in Components.Schemas.Currency.allCases {
            #expect(BudgetCurrency(rawValue: currency.rawValue) != nil)
        }
    }

    @Test("Every app workspace role is a contract role, and none is missing")
    func workspaceRolesMatch() {
        let app = Set(BudgetWorkspaceRoleValue.allCases.map(\.rawValue))
        let contract = Set(Components.Schemas.WorkspaceRole.allCases.map(\.rawValue))
        #expect(app == contract)
    }

    /// An invitation never confers ownership, so the contract's invitation roles are a strict
    /// subset and the app's policy must agree.
    @Test("Invitation roles exclude owner on both sides")
    func invitationRolesExcludeOwner() {
        let contract = Set(Components.Schemas.WorkspaceInvitationRole.allCases.map(\.rawValue))
        #expect(contract.contains(BudgetWorkspaceRoleValue.owner.rawValue) == false)

        let offered = Set(
            WorkspaceCollaborationPolicy.invitableRoles(actorRole: .owner).map(\.rawValue)
        )
        #expect(offered == contract)
    }

    @Test("Every app account type is a contract account type")
    func accountTypesMatch() {
        let app = Set(BudgetAccountType.allCases.map(\.rawValue))
        let contract = Set(Components.Schemas.AccountType.allCases.map(\.rawValue))
        #expect(app == contract)
    }

    @Test("Every app category kind is a contract category kind")
    func categoryKindsMatch() {
        let app = Set(BudgetCategoryKind.allCases.map(\.rawValue))
        let contract = Set(Components.Schemas.CategoryKind.allCases.map(\.rawValue))
        #expect(app == contract)
    }

    @Test("Transaction kinds and statuses match the contract")
    func transactionVocabularyMatches() {
        #expect(
            Set(BudgetTransactionKind.allCases.map(\.rawValue))
                == Set(Components.Schemas.TransactionKind.allCases.map(\.rawValue))
        )
        #expect(
            Set(BudgetTransactionStatus.allCases.map(\.rawValue))
                == Set(Components.Schemas.TransactionStatus.allCases.map(\.rawValue))
        )
    }
}
