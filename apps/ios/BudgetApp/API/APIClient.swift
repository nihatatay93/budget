import Foundation
import BudgetAPI

struct AuthenticatedSession: Sendable {
    let token: String
    let session: UserSession
}

struct UserSession: Sendable {
    let user: BudgetUser
    let workspaces: [BudgetWorkspace]
}

struct BudgetUser: Identifiable, Sendable {
    let id: String
    let email: String
    let displayName: String
}

struct BudgetWorkspace: Identifiable, Sendable {
    let id: String
    let name: String
    let baseCurrency: BudgetCurrency
    let timezone: String
    let role: String
}

struct RegistrationInput: Sendable {
    let email: String
    let password: String
    let displayName: String
    let workspaceName: String
    let baseCurrency: BudgetCurrency
    let timezone: String
}

/// Supported currencies. Every member uses two minor-unit decimal places, which is what lets
/// `formatted(minorUnits:)` divide by a constant.
/// See docs/decisions/0005-supported-currencies-and-display-conversion.md.
enum BudgetCurrency: String, CaseIterable, Identifiable, Sendable {
    case turkishLira = "TRY"
    case usDollar = "USD"
    case euro = "EUR"

    var id: Self { self }

    /// The ISO code stays verbatim; only the currency's name is translated, so the code a
    /// person recognizes reads the same in every language.
    var title: String {
        "\(rawValue) · \(L10n.text("currency.\(rawValue.lowercased())"))"
    }

    func formatted(minorUnits: Int64) -> String {
        (Decimal(minorUnits) / 100).formatted(.currency(code: rawValue))
    }
}

enum BudgetAccountType: String, CaseIterable, Identifiable, Sendable {
    case bank
    case cash
    case creditCard = "credit_card"
    case savings
    case investment
    case other

    var id: Self { self }

    var title: String { L10n.text("account.type.\(rawValue)") }
}

struct BudgetAccount: Identifiable, Sendable {
    let id: String
    let workspaceID: String
    let name: String
    let type: BudgetAccountType
    let currency: BudgetCurrency
    let institutionName: String?
    let archivedAt: Date?
    let balanceMinor: Int64
}

struct BudgetExchangeRate: Identifiable, Sendable {
    let baseCurrency: BudgetCurrency
    let quoteCurrency: BudgetCurrency
    let rate: Decimal
    let rateDate: String

    var id: BudgetCurrency { quoteCurrency }

    func convert(minorUnits: Int64) -> Int64? {
        var product = Decimal(minorUnits) * rate
        var rounded = Decimal()
        NSDecimalRound(&rounded, &product, 0, .plain)
        guard rounded >= Decimal(Int64.min), rounded <= Decimal(Int64.max) else { return nil }
        return NSDecimalNumber(decimal: rounded).int64Value
    }
}

struct AccountInput: Sendable {
    let name: String
    let type: BudgetAccountType
    let currency: BudgetCurrency
    let institutionName: String?
}

enum BudgetCategoryKind: String, CaseIterable, Identifiable, Sendable {
    case expense
    case income

    var id: Self { self }
    var title: String { L10n.text("category.kind.\(rawValue)") }
}

struct BudgetCategory: Identifiable, Sendable {
    let id: String
    let workspaceID: String
    let parentID: String?
    let name: String
    let kind: BudgetCategoryKind
    let isSystem: Bool
    let systemKey: String?
    let predefinedKey: String?
    let iconType: String
    let iconValue: String
    let colorKey: String
    let archivedAt: Date?

    var icon: String? { iconValue }
}

struct CategoryInput: Sendable {
    let name: String
    let kind: BudgetCategoryKind
    let parentID: String?
    let iconType: BudgetCategoryIconType
    let iconValue: String?
    let colorKey: BudgetCategoryColorKey
}

enum BudgetTransactionKind: String, CaseIterable, Identifiable, Sendable {
    case standard
    case transfer
    case adjustment

    var id: Self { self }
    var title: String { L10n.text("transaction.kind.\(rawValue)") }
}

enum BudgetTransactionStatus: String, CaseIterable, Identifiable, Sendable {
    case pending
    case posted

    var id: Self { self }
    var title: String { L10n.text("transaction.status.\(rawValue)") }
}

struct BudgetTransactionEntry: Sendable {
    let accountID: String
    let amountMinor: Int64
    let baseAmountMinor: Int64
}

struct BudgetTransactionAllocation: Sendable {
    let categoryID: String
    let amountBaseMinor: Int64
}

struct BudgetTransaction: Identifiable, Sendable {
    let id: String
    let workspaceID: String
    let kind: BudgetTransactionKind
    let status: BudgetTransactionStatus
    let transactionDate: String
    let payee: String?
    let description: String?
    let notes: String?
    let source: String
    let entries: [BudgetTransactionEntry]
    let allocations: [BudgetTransactionAllocation]
}

struct TransactionEntryInput: Sendable {
    let accountID: String
    let amountMinor: Int64
    let baseAmountMinor: Int64?
}

struct TransactionAllocationInput: Sendable {
    let categoryID: String
    /// Nil on a lone allocation leaves the amount to the server, which takes the transaction's
    /// entry total — for a foreign-currency account, the value booked at that date's rate.
    let amountBaseMinor: Int64?
}

struct TransactionInput: Sendable {
    let kind: BudgetTransactionKind
    let status: BudgetTransactionStatus
    let transactionDate: String
    let payee: String?
    let description: String?
    let notes: String?
    let entries: [TransactionEntryInput]
    let allocations: [TransactionAllocationInput]
}

struct BudgetProjectionAmounts: Sendable {
    let posted: Int64
    let pending: Int64
    let projected: Int64
}

struct BudgetProjectionPeriod: Sendable {
    let fromDate: String
    let toDate: String
    let timezone: String
    let baseCurrency: BudgetCurrency
}

struct BudgetProjectionSummary: Sendable {
    let balanceBaseMinor: BudgetProjectionAmounts
    let incomeBaseMinor: BudgetProjectionAmounts
    let spendingBaseMinor: BudgetProjectionAmounts
}

struct BudgetProjectionAccount: Identifiable, Sendable {
    let id: String
    let name: String
    let type: BudgetAccountType
    let currency: BudgetCurrency
    let archivedAt: Date?
    let nativeBalanceMinor: BudgetProjectionAmounts
    let baseBalanceMinor: BudgetProjectionAmounts
}

struct BudgetProjectionCategory: Identifiable, Sendable {
    let id: String
    let parentID: String?
    let name: String
    let kind: BudgetCategoryKind
    let predefinedKey: String?
    let iconType: String
    let iconValue: String
    let colorKey: String
    let archivedAt: Date?
    let directBaseMinor: BudgetProjectionAmounts
    let rolledUpBaseMinor: BudgetProjectionAmounts

    var icon: String? { iconValue }
}

struct BudgetFinancialProjection: Sendable {
    let period: BudgetProjectionPeriod
    let summary: BudgetProjectionSummary
    let accounts: [BudgetProjectionAccount]
    let categories: [BudgetProjectionCategory]
}

struct BudgetProjectionRange: Equatable, Sendable {
    let fromDate: String
    let toDate: String
}

/// Spending analysis is posted-only by contract, so nothing here carries a pending figure.
/// Spending counts up when money left the workspace, matching the projection's orientation.
/// See docs/decisions/0011-analyze-posted-spending-over-time.md.
enum BudgetAnalysisGranularity: String, CaseIterable, Identifiable, Sendable {
    case day
    case week
    case month

    var id: Self { self }

    /// The adjective a control shows: "Weekly".
    var title: String { L10n.text("analysis.granularity.\(rawValue)") }

    /// The noun a sentence needs: "one week of posted activity".
    var noun: String { L10n.text("analysis.bucket.\(rawValue)") }
}

struct BudgetAnalysisPeriod: Sendable {
    let fromDate: String
    let toDate: String
    let comparisonFromDate: String
    let comparisonToDate: String
    let granularity: BudgetAnalysisGranularity
    let timezone: String
    let baseCurrency: BudgetCurrency
}

struct BudgetAnalysisTotals: Sendable {
    let incomeBaseMinor: Int64
    let spendingBaseMinor: Int64
    let netBaseMinor: Int64
    let comparisonIncomeBaseMinor: Int64
    let comparisonSpendingBaseMinor: Int64
    let comparisonNetBaseMinor: Int64
    let transactionCount: Int64
    let spendingTransactionCount: Int64
    let largestSpendingBaseMinor: Int64
    let spendingDayCount: Int64
    let dayCount: Int64
}

struct BudgetAnalysisBucket: Identifiable, Sendable {
    let startDate: String
    let endDate: String
    let incomeBaseMinor: Int64
    let spendingBaseMinor: Int64
    let netBaseMinor: Int64
    let transactionCount: Int64

    var id: String { startDate }
}

struct BudgetAnalysisCategory: Identifiable, Sendable {
    let id: String
    let parentID: String?
    let name: String
    let kind: BudgetCategoryKind
    let systemKey: String?
    let predefinedKey: String?
    let iconType: String
    let iconValue: String
    let colorKey: String
    let archivedAt: Date?
    let directBaseMinor: Int64
    let rolledUpBaseMinor: Int64
    let comparisonDirectBaseMinor: Int64
    let comparisonRolledUpBaseMinor: Int64
    let transactionCount: Int64
    let rolledUpTransactionCount: Int64
    let largestBaseMinor: Int64
    let firstDate: String?
    let lastDate: String?
}

/// One category's activity inside one bucket. Points exist only where activity does, so a
/// long window of sparse categories stays small on the wire.
struct BudgetAnalysisCategoryPoint: Sendable {
    let categoryID: String
    let startDate: String
    let baseMinor: Int64
}

struct BudgetAnalysisWeekday: Identifiable, Sendable {
    /// ISO weekday, where 1 is Monday and 7 is Sunday.
    let weekday: Int
    let incomeBaseMinor: Int64
    let spendingBaseMinor: Int64
    let transactionCount: Int64

    var id: Int { weekday }
}

struct BudgetAnalysisDay: Identifiable, Sendable {
    let date: String
    let incomeBaseMinor: Int64
    let spendingBaseMinor: Int64
    let transactionCount: Int64

    var id: String { date }
}

struct BudgetAnalysisPayee: Identifiable, Sendable {
    let payee: String
    let spendingBaseMinor: Int64
    let incomeBaseMinor: Int64
    let transactionCount: Int64
    let firstDate: String
    let lastDate: String

    var id: String { payee }
}

struct BudgetAnalysisAccount: Identifiable, Sendable {
    let id: String
    let name: String
    let type: BudgetAccountType
    let currency: BudgetCurrency
    let archivedAt: Date?
    let outflowBaseMinor: Int64
    let inflowBaseMinor: Int64
    let transactionCount: Int64
}

struct BudgetSpendingAnalysis: Sendable {
    let period: BudgetAnalysisPeriod
    let totals: BudgetAnalysisTotals
    let series: [BudgetAnalysisBucket]
    let categories: [BudgetAnalysisCategory]
    let categorySeries: [BudgetAnalysisCategoryPoint]
    let weekdays: [BudgetAnalysisWeekday]
    let days: [BudgetAnalysisDay]
    let payees: [BudgetAnalysisPayee]
    let accounts: [BudgetAnalysisAccount]
}

/// A nil granularity asks the server to choose a bucket width that suits the window length.
struct BudgetAnalysisRange: Equatable, Sendable {
    let fromDate: String
    let toDate: String
    let granularity: BudgetAnalysisGranularity?
}

struct MonthlyBudgetItem: Identifiable, Sendable {
    let id: String
    let categoryID: String
    let categoryName: String
    let categoryPredefinedKey: String?
    let categoryIconType: String
    let categoryIconValue: String
    let categoryColorKey: String
    let categoryArchivedAt: Date?
    let plannedBaseMinor: Int64
    let usedBaseMinor: Int64
    let remainingBaseMinor: Int64

    var categoryIcon: String? { categoryIconValue }
}

struct MonthlyBudgetPlan: Identifiable, Sendable {
    let id: String
    let workspaceID: String
    let name: String
    let month: String
    let timezone: String
    let baseCurrency: BudgetCurrency
    let plannedBaseMinor: Int64
    let usedBaseMinor: Int64
    let remainingBaseMinor: Int64
    let items: [MonthlyBudgetItem]
    let createdAt: Date
    let updatedAt: Date
}

struct MonthlyBudgetItemInput: Sendable {
    let categoryID: String
    let amountBaseMinor: Int64
}

struct MonthlyBudgetInput: Sendable {
    let name: String
    let items: [MonthlyBudgetItemInput]
}

/// Workspace membership roles, mirroring the server's WorkspaceRole contract.
enum BudgetWorkspaceRoleValue: String, CaseIterable, Identifiable, Sendable {
    case owner
    case admin
    case member
    case viewer

    var id: Self { self }
    var title: String { L10n.workspaceRole(rawValue) }
}

struct BudgetWorkspaceMember: Identifiable, Sendable {
    let userID: String
    let email: String
    let displayName: String
    let role: BudgetWorkspaceRoleValue
    let joinedAt: Date

    var id: String { userID }
}

struct BudgetWorkspaceInvitation: Identifiable, Sendable {
    let id: String
    let workspaceID: String
    let email: String
    let role: BudgetWorkspaceRoleValue
    let inviterDisplayName: String
    let expiresAt: Date
}

/// An invitation and the acceptance token disclosed once at creation. The token is a
/// credential: it is shown to the inviter to pass on and never persisted or logged.
struct BudgetIssuedInvitation: Sendable {
    let invitation: BudgetWorkspaceInvitation
    let acceptanceToken: String
}

struct BudgetInvitationInput: Sendable {
    let email: String
    let role: BudgetWorkspaceRoleValue
}

struct BudgetMembershipAcceptance: Sendable {
    let workspace: BudgetWorkspace
    let member: BudgetWorkspaceMember
}

enum APIClientError: LocalizedError {
    case invalidCredentials
    case invalidRequest
    case duplicateEmail
    case conflict
    case rateUnavailable
    case forbidden
    case notFound
    case unauthorized
    case server
    case unexpectedResponse

    var errorDescription: String? {
        switch self {
        case .invalidCredentials:
            L10n.text("The email or password is incorrect.")
        case .invalidRequest:
            L10n.text("Please check the information and try again.")
        case .duplicateEmail:
            L10n.text("That email address is already registered.")
        case .conflict:
            L10n.text("The change conflicts with existing financial data.")
        case .rateUnavailable:
            L10n.text("A historical exchange rate is unavailable. Enter the base-currency amount manually.")
        case .forbidden:
            L10n.text("You do not have permission to make this change.")
        case .notFound:
            L10n.text("The requested item could not be found.")
        case .unauthorized:
            L10n.text("Your session has expired. Please sign in again.")
        case .server:
            L10n.text("The server could not complete the request.")
        case .unexpectedResponse:
            L10n.text("The server returned an unexpected response.")
        }
    }
}

protocol APIClient: Sendable {
    func register(serverURL: URL, input: RegistrationInput) async throws -> AuthenticatedSession
    func login(serverURL: URL, email: String, password: String) async throws -> AuthenticatedSession
    func session(serverURL: URL, token: String) async throws -> UserSession
    func logout(serverURL: URL, token: String) async throws
    func listAccounts(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetAccount]
    func listExchangeRates(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetExchangeRate]
    func createAccount(serverURL: URL, token: String, workspaceID: String, input: AccountInput) async throws -> BudgetAccount
    func updateAccount(serverURL: URL, token: String, workspaceID: String, accountID: String, input: AccountInput) async throws -> BudgetAccount
    func archiveAccount(serverURL: URL, token: String, workspaceID: String, accountID: String) async throws
    func listCategories(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetCategory]
    func createCategory(serverURL: URL, token: String, workspaceID: String, input: CategoryInput) async throws -> BudgetCategory
    func updateCategory(serverURL: URL, token: String, workspaceID: String, categoryID: String, input: CategoryInput) async throws -> BudgetCategory
    func archiveCategory(serverURL: URL, token: String, workspaceID: String, categoryID: String) async throws
    func listTransactions(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetTransaction]
    func financialProjection(
        serverURL: URL,
        token: String,
        workspaceID: String,
        range: BudgetProjectionRange?
    ) async throws -> BudgetFinancialProjection
    func spendingAnalysis(
        serverURL: URL,
        token: String,
        workspaceID: String,
        range: BudgetAnalysisRange?
    ) async throws -> BudgetSpendingAnalysis
    func monthlyBudget(
        serverURL: URL,
        token: String,
        workspaceID: String,
        month: String
    ) async throws -> MonthlyBudgetPlan?
    func replaceMonthlyBudget(
        serverURL: URL,
        token: String,
        workspaceID: String,
        month: String,
        input: MonthlyBudgetInput
    ) async throws -> MonthlyBudgetPlan
    func createTransaction(serverURL: URL, token: String, workspaceID: String, input: TransactionInput) async throws -> BudgetTransaction
    func updateTransaction(serverURL: URL, token: String, workspaceID: String, transactionID: String, input: TransactionInput) async throws -> BudgetTransaction
    func deleteTransaction(serverURL: URL, token: String, workspaceID: String, transactionID: String) async throws
    func listWorkspaceMembers(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetWorkspaceMember]
    func updateWorkspaceMemberRole(
        serverURL: URL,
        token: String,
        workspaceID: String,
        userID: String,
        role: BudgetWorkspaceRoleValue
    ) async throws -> BudgetWorkspaceMember
    func removeWorkspaceMember(serverURL: URL, token: String, workspaceID: String, userID: String) async throws
    func listWorkspaceInvitations(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetWorkspaceInvitation]
    func createWorkspaceInvitation(
        serverURL: URL,
        token: String,
        workspaceID: String,
        input: BudgetInvitationInput
    ) async throws -> BudgetIssuedInvitation
    func revokeWorkspaceInvitation(serverURL: URL, token: String, workspaceID: String, invitationID: String) async throws
    func acceptWorkspaceInvitation(serverURL: URL, token: String, acceptanceToken: String) async throws -> BudgetMembershipAcceptance
}

struct URLSessionAPIClient: APIClient {
    func register(serverURL: URL, input: RegistrationInput) async throws -> AuthenticatedSession {
        let response = try await GeneratedAPI.client(serverURL: serverURL).register(.init(body: .json(.init(
            email: input.email,
            password: input.password,
            display_name: input.displayName,
            workspace_name: input.workspaceName,
            base_currency: .init(rawValue: input.baseCurrency.rawValue)!,
            timezone: input.timezone,
            transport: .bearer
        ))))
        switch response {
        case let .created(created):
            return try authenticatedSession(from: created.body.json)
        case .badRequest, .forbidden:
            throw APIClientError.invalidRequest
        case .conflict:
            throw APIClientError.duplicateEmail
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func login(serverURL: URL, email: String, password: String) async throws -> AuthenticatedSession {
        let response = try await GeneratedAPI.client(serverURL: serverURL).login(.init(body: .json(.init(
            email: email,
            password: password,
            transport: .bearer
        ))))
        switch response {
        case let .ok(ok):
            return try authenticatedSession(from: ok.body.json)
        case .badRequest, .forbidden:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.invalidCredentials
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func session(serverURL: URL, token: String) async throws -> UserSession {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .getSession(.init())
        switch response {
        case let .ok(ok):
            return try userSession(from: ok.body.json)
        case .unauthorized:
            throw APIClientError.unauthorized
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func logout(serverURL: URL, token: String) async throws {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .logout(.init())
        switch response {
        case .noContent:
            return
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.invalidRequest
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func listAccounts(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetAccount] {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .listAccounts(path: .init(workspaceId: workspaceID))
        switch response {
        case let .ok(ok):
            return try ok.body.json.accounts.map(budgetAccount)
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func listExchangeRates(
        serverURL: URL,
        token: String,
        workspaceID: String
    ) async throws -> [BudgetExchangeRate] {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .listExchangeRates(path: .init(workspaceId: workspaceID))
        switch response {
        case let .ok(ok):
            return try ok.body.json.rates.map(budgetExchangeRate)
        case .serviceUnavailable:
            // Conversion is optional and disabled by default. No rates is a normal state.
            return []
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func createAccount(
        serverURL: URL,
        token: String,
        workspaceID: String,
        input: AccountInput
    ) async throws -> BudgetAccount {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .createAccount(
                path: .init(workspaceId: workspaceID),
                body: .json(accountRequest(input))
            )
        switch response {
        case let .created(created):
            return try budgetAccount(from: created.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func updateAccount(
        serverURL: URL,
        token: String,
        workspaceID: String,
        accountID: String,
        input: AccountInput
    ) async throws -> BudgetAccount {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .updateAccount(
                path: .init(workspaceId: workspaceID, accountId: accountID),
                body: .json(accountRequest(input))
            )
        switch response {
        case let .ok(ok):
            return try budgetAccount(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func archiveAccount(serverURL: URL, token: String, workspaceID: String, accountID: String) async throws {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .archiveAccount(path: .init(workspaceId: workspaceID, accountId: accountID))
        switch response {
        case .noContent:
            return
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func listCategories(serverURL: URL, token: String, workspaceID: String) async throws -> [BudgetCategory] {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .listCategories(path: .init(workspaceId: workspaceID))
        switch response {
        case let .ok(ok):
            return try ok.body.json.categories.map(budgetCategory)
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func createCategory(
        serverURL: URL,
        token: String,
        workspaceID: String,
        input: CategoryInput
    ) async throws -> BudgetCategory {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .createCategory(
                path: .init(workspaceId: workspaceID),
                body: .json(Self.categoryRequest(input))
            )
        switch response {
        case let .created(created):
            return try budgetCategory(from: created.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func updateCategory(
        serverURL: URL,
        token: String,
        workspaceID: String,
        categoryID: String,
        input: CategoryInput
    ) async throws -> BudgetCategory {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .updateCategory(
                path: .init(workspaceId: workspaceID, categoryId: categoryID),
                body: .json(Self.categoryRequest(input))
            )
        switch response {
        case let .ok(ok):
            return try budgetCategory(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func archiveCategory(serverURL: URL, token: String, workspaceID: String, categoryID: String) async throws {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .archiveCategory(path: .init(workspaceId: workspaceID, categoryId: categoryID))
        switch response {
        case .noContent:
            return
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func listTransactions(
        serverURL: URL,
        token: String,
        workspaceID: String
    ) async throws -> [BudgetTransaction] {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .listTransactions(path: .init(workspaceId: workspaceID))
        switch response {
        case let .ok(ok):
            return try ok.body.json.transactions.map(budgetTransaction)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func financialProjection(
        serverURL: URL,
        token: String,
        workspaceID: String,
        range: BudgetProjectionRange?
    ) async throws -> BudgetFinancialProjection {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .getFinancialProjection(
                path: .init(workspaceId: workspaceID),
                query: .init(from_date: range?.fromDate, to_date: range?.toDate)
            )
        switch response {
        case let .ok(ok):
            return try budgetFinancialProjection(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func spendingAnalysis(
        serverURL: URL,
        token: String,
        workspaceID: String,
        range: BudgetAnalysisRange?
    ) async throws -> BudgetSpendingAnalysis {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .getSpendingAnalysis(
                path: .init(workspaceId: workspaceID),
                query: .init(
                    from_date: range?.fromDate,
                    to_date: range?.toDate,
                    granularity: range?.granularity.map { .init(rawValue: $0.rawValue) } ?? nil
                )
            )
        switch response {
        case let .ok(ok):
            return try budgetSpendingAnalysis(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func monthlyBudget(
        serverURL: URL,
        token: String,
        workspaceID: String,
        month: String
    ) async throws -> MonthlyBudgetPlan? {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .getMonthlyBudget(
                path: .init(workspaceId: workspaceID),
                query: .init(month: month)
            )
        switch response {
        case let .ok(ok):
            return try monthlyBudgetPlan(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            return nil
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func replaceMonthlyBudget(
        serverURL: URL,
        token: String,
        workspaceID: String,
        month: String,
        input: MonthlyBudgetInput
    ) async throws -> MonthlyBudgetPlan {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .replaceMonthlyBudget(
                path: .init(workspaceId: workspaceID, month: month),
                body: .json(.init(
                    name: input.name,
                    items: input.items.map {
                        .init(category_id: $0.categoryID, amount_base_minor: $0.amountBaseMinor)
                    }
                ))
            )
        switch response {
        case let .ok(ok):
            return try monthlyBudgetPlan(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func createTransaction(
        serverURL: URL,
        token: String,
        workspaceID: String,
        input: TransactionInput
    ) async throws -> BudgetTransaction {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .createTransaction(
                path: .init(workspaceId: workspaceID),
                body: .json(transactionRequest(input))
            )
        switch response {
        case let .created(created):
            return try budgetTransaction(from: created.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .conflict:
            throw APIClientError.conflict
        case .serviceUnavailable:
            throw APIClientError.rateUnavailable
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func updateTransaction(
        serverURL: URL,
        token: String,
        workspaceID: String,
        transactionID: String,
        input: TransactionInput
    ) async throws -> BudgetTransaction {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .updateTransaction(
                path: .init(workspaceId: workspaceID, transactionId: transactionID),
                body: .json(transactionRequest(input))
            )
        switch response {
        case let .ok(ok):
            return try budgetTransaction(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .serviceUnavailable:
            throw APIClientError.rateUnavailable
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func deleteTransaction(
        serverURL: URL,
        token: String,
        workspaceID: String,
        transactionID: String
    ) async throws {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .deleteTransaction(path: .init(workspaceId: workspaceID, transactionId: transactionID))
        switch response {
        case .noContent:
            return
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    private func authenticatedSession(
        from response: Components.Schemas.AuthResponse
    ) throws -> AuthenticatedSession {
        guard let token = response.bearer_token, !token.isEmpty else {
            throw APIClientError.unexpectedResponse
        }
        return AuthenticatedSession(
            token: token,
            session: UserSession(
                user: budgetUser(from: response.user),
                workspaces: try response.workspaces.map(budgetWorkspace)
            )
        )
    }

    private func userSession(from response: Components.Schemas.SessionResponse) throws -> UserSession {
        UserSession(
            user: budgetUser(from: response.user),
            workspaces: try response.workspaces.map(budgetWorkspace)
        )
    }

    private func budgetUser(from user: Components.Schemas.User) -> BudgetUser {
        BudgetUser(id: user.id, email: user.email, displayName: user.display_name)
    }

    func listWorkspaceMembers(
        serverURL: URL, token: String, workspaceID: String
    ) async throws -> [BudgetWorkspaceMember] {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .listWorkspaceMembers(path: .init(workspaceId: workspaceID))
        switch response {
        case let .ok(ok):
            return try ok.body.json.members.map(budgetWorkspaceMember)
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func updateWorkspaceMemberRole(
        serverURL: URL, token: String, workspaceID: String, userID: String,
        role: BudgetWorkspaceRoleValue
    ) async throws -> BudgetWorkspaceMember {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .updateWorkspaceMemberRole(
                path: .init(workspaceId: workspaceID, userId: userID),
                body: .json(.init(role: .init(rawValue: role.rawValue)!))
            )
        switch response {
        case let .ok(ok):
            return try budgetWorkspaceMember(from: ok.body.json)
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func removeWorkspaceMember(
        serverURL: URL, token: String, workspaceID: String, userID: String
    ) async throws {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .removeWorkspaceMember(path: .init(workspaceId: workspaceID, userId: userID))
        switch response {
        case .noContent:
            return
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func listWorkspaceInvitations(
        serverURL: URL, token: String, workspaceID: String
    ) async throws -> [BudgetWorkspaceInvitation] {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .listWorkspaceInvitations(path: .init(workspaceId: workspaceID))
        switch response {
        case let .ok(ok):
            return try ok.body.json.invitations.map(budgetWorkspaceInvitation)
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func createWorkspaceInvitation(
        serverURL: URL, token: String, workspaceID: String, input: BudgetInvitationInput
    ) async throws -> BudgetIssuedInvitation {
        guard let role = Components.Schemas.WorkspaceInvitationRole(rawValue: input.role.rawValue) else {
            throw APIClientError.invalidRequest
        }
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .createWorkspaceInvitation(
                path: .init(workspaceId: workspaceID),
                body: .json(.init(email: input.email, role: role))
            )
        switch response {
        case let .created(created):
            let issued = try created.body.json
            return BudgetIssuedInvitation(
                invitation: try budgetWorkspaceInvitation(from: issued.invitation),
                acceptanceToken: issued.acceptance_token
            )
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    func revokeWorkspaceInvitation(
        serverURL: URL, token: String, workspaceID: String, invitationID: String
    ) async throws {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .revokeWorkspaceInvitation(path: .init(workspaceId: workspaceID, invitationId: invitationID))
        switch response {
        case .noContent:
            return
        case .unauthorized:
            throw APIClientError.unauthorized
        case .forbidden:
            throw APIClientError.forbidden
        case .notFound:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    /// The acceptance token travels in the request body, never the URL, so it stays out of
    /// server logs and any intermediary's request history.
    func acceptWorkspaceInvitation(
        serverURL: URL, token: String, acceptanceToken: String
    ) async throws -> BudgetMembershipAcceptance {
        let response = try await GeneratedAPI.client(serverURL: serverURL, bearerToken: token)
            .acceptWorkspaceInvitation(body: .json(.init(token: acceptanceToken)))
        switch response {
        case let .ok(ok):
            let acceptance = try ok.body.json
            return BudgetMembershipAcceptance(
                workspace: try budgetWorkspace(from: acceptance.workspace),
                member: try budgetWorkspaceMember(from: acceptance.member)
            )
        case .badRequest:
            throw APIClientError.invalidRequest
        case .unauthorized:
            throw APIClientError.unauthorized
        case .notFound, .gone:
            throw APIClientError.notFound
        case .conflict:
            throw APIClientError.conflict
        case .internalServerError:
            throw APIClientError.server
        case .undocumented:
            throw APIClientError.unexpectedResponse
        }
    }

    private func budgetWorkspaceMember(
        from member: Components.Schemas.WorkspaceMember
    ) throws -> BudgetWorkspaceMember {
        guard let role = BudgetWorkspaceRoleValue(rawValue: member.role.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetWorkspaceMember(
            userID: member.user_id,
            email: member.email,
            displayName: member.display_name,
            role: role,
            joinedAt: member.joined_at
        )
    }

    private func budgetWorkspaceInvitation(
        from invitation: Components.Schemas.WorkspaceInvitation
    ) throws -> BudgetWorkspaceInvitation {
        guard let role = BudgetWorkspaceRoleValue(rawValue: invitation.role.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetWorkspaceInvitation(
            id: invitation.id,
            workspaceID: invitation.workspace_id,
            email: invitation.email,
            role: role,
            inviterDisplayName: invitation.inviter_display_name,
            expiresAt: invitation.expires_at
        )
    }

    private func budgetWorkspace(
        from workspace: Components.Schemas.WorkspaceSummary
    ) throws -> BudgetWorkspace {
        guard let baseCurrency = BudgetCurrency(rawValue: workspace.base_currency.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetWorkspace(
            id: workspace.id,
            name: workspace.name,
            baseCurrency: baseCurrency,
            timezone: workspace.timezone,
            role: workspace.role.rawValue
        )
    }

    private func accountRequest(_ input: AccountInput) -> Components.Schemas.AccountWriteRequest {
        .init(
            name: input.name,
            _type: .init(rawValue: input.type.rawValue)!,
            currency: .init(rawValue: input.currency.rawValue)!,
            institution_name: input.institutionName
        )
    }

    private func budgetAccount(from account: Components.Schemas.Account) throws -> BudgetAccount {
        guard let type = BudgetAccountType(rawValue: account._type.rawValue),
              let currency = BudgetCurrency(rawValue: account.currency.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetAccount(
            id: account.id,
            workspaceID: account.workspace_id,
            name: account.name,
            type: type,
            currency: currency,
            institutionName: account.institution_name,
            archivedAt: account.archived_at,
            balanceMinor: account.balance_minor
        )
    }

    private func budgetExchangeRate(
        from rate: Components.Schemas.ExchangeRate
    ) throws -> BudgetExchangeRate {
        let dateFormatter = ISO8601DateFormatter()
        dateFormatter.formatOptions = [.withFullDate]
        guard let base = BudgetCurrency(rawValue: rate.base_currency.rawValue),
              let quote = BudgetCurrency(rawValue: rate.quote_currency.rawValue),
              let value = Decimal(string: rate.rate, locale: Locale(identifier: "en_US_POSIX")),
              value > 0,
              dateFormatter.date(from: rate.rate_date) != nil else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetExchangeRate(
            baseCurrency: base,
            quoteCurrency: quote,
            rate: value,
            // Preserve a provider publication date as a date-only value. Converting it to an
            // instant can display the previous day in time zones west of UTC.
            rateDate: rate.rate_date
        )
    }

    static func categoryRequest(_ input: CategoryInput) -> Components.Schemas.CategoryWriteRequest {
        .init(
            name: input.name,
            kind: .init(rawValue: input.kind.rawValue)!,
            parent_id: input.parentID,
            icon: nil,
            icon_type: .init(rawValue: input.iconType.rawValue)!,
            icon_value: input.iconValue,
            color_key: .init(rawValue: input.colorKey.rawValue)!
        )
    }

    private func budgetCategory(from category: Components.Schemas.Category) throws -> BudgetCategory {
        guard let kind = BudgetCategoryKind(rawValue: category.kind.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetCategory(
            id: category.id,
            workspaceID: category.workspace_id,
            parentID: category.parent_id,
            name: category.name,
            kind: kind,
            isSystem: category.system_key != nil,
            systemKey: category.system_key?.value1.rawValue,
            predefinedKey: category.predefined_key?.value1.rawValue,
            iconType: category.icon_type.rawValue,
            iconValue: category.icon_value,
            colorKey: category.color_key.rawValue,
            archivedAt: category.archived_at
        )
    }

    private func transactionRequest(
        _ input: TransactionInput
    ) -> Components.Schemas.TransactionWriteRequest {
        .init(
            kind: .init(rawValue: input.kind.rawValue)!,
            status: .init(rawValue: input.status.rawValue)!,
            transaction_date: input.transactionDate,
            payee: input.payee,
            description: input.description,
            notes: input.notes,
            entries: input.entries.map {
                .init(
                    account_id: $0.accountID,
                    amount_minor: $0.amountMinor,
                    base_amount_minor: $0.baseAmountMinor
                )
            },
            allocations: input.allocations.map {
                .init(category_id: $0.categoryID, amount_base_minor: $0.amountBaseMinor)
            }
        )
    }

    private func budgetTransaction(
        from transaction: Components.Schemas.Transaction
    ) throws -> BudgetTransaction {
        guard let kind = BudgetTransactionKind(rawValue: transaction.kind.rawValue),
              let status = BudgetTransactionStatus(rawValue: transaction.status.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetTransaction(
            id: transaction.id,
            workspaceID: transaction.workspace_id,
            kind: kind,
            status: status,
            transactionDate: transaction.transaction_date,
            payee: transaction.payee,
            description: transaction.description,
            notes: transaction.notes,
            source: transaction.source.rawValue,
            entries: transaction.entries.map {
                BudgetTransactionEntry(
                    accountID: $0.account_id,
                    amountMinor: $0.amount_minor,
                    baseAmountMinor: $0.base_amount_minor
                )
            },
            allocations: transaction.allocations.map {
                BudgetTransactionAllocation(
                    categoryID: $0.category_id,
                    amountBaseMinor: $0.amount_base_minor
                )
            }
        )
    }

    private func budgetFinancialProjection(
        from projection: Components.Schemas.FinancialProjection
    ) throws -> BudgetFinancialProjection {
        guard let baseCurrency = BudgetCurrency(rawValue: projection.period.base_currency.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetFinancialProjection(
            period: BudgetProjectionPeriod(
                fromDate: projection.period.from_date,
                toDate: projection.period.to_date,
                timezone: projection.period.timezone,
                baseCurrency: baseCurrency
            ),
            summary: BudgetProjectionSummary(
                balanceBaseMinor: projectionAmounts(projection.summary.balance_base_minor),
                incomeBaseMinor: projectionAmounts(projection.summary.income_base_minor),
                spendingBaseMinor: projectionAmounts(projection.summary.spending_base_minor)
            ),
            accounts: try projection.accounts.map { account in
                guard let type = BudgetAccountType(rawValue: account._type.rawValue),
                      let currency = BudgetCurrency(rawValue: account.currency.rawValue) else {
                    throw APIClientError.unexpectedResponse
                }
                return BudgetProjectionAccount(
                    id: account.id,
                    name: account.name,
                    type: type,
                    currency: currency,
                    archivedAt: account.archived_at,
                    nativeBalanceMinor: projectionAmounts(account.native_balance_minor),
                    baseBalanceMinor: projectionAmounts(account.base_balance_minor)
                )
            },
            categories: try projection.categories.map { category in
                guard let kind = BudgetCategoryKind(rawValue: category.kind.rawValue) else {
                    throw APIClientError.unexpectedResponse
                }
                return BudgetProjectionCategory(
                    id: category.id,
                    parentID: category.parent_id,
                    name: category.name,
                    kind: kind,
                    predefinedKey: category.predefined_key?.value1.rawValue,
                    iconType: category.icon_type.rawValue,
                    iconValue: category.icon_value,
                    colorKey: category.color_key.rawValue,
                    archivedAt: category.archived_at,
                    directBaseMinor: projectionAmounts(category.direct_base_minor),
                    rolledUpBaseMinor: projectionAmounts(category.rolled_up_base_minor)
                )
            }
        )
    }

    private func budgetSpendingAnalysis(
        from analysis: Components.Schemas.SpendingAnalysis
    ) throws -> BudgetSpendingAnalysis {
        guard let baseCurrency = BudgetCurrency(rawValue: analysis.period.base_currency.rawValue),
              let granularity = BudgetAnalysisGranularity(
                rawValue: analysis.period.granularity.rawValue
              ) else {
            throw APIClientError.unexpectedResponse
        }
        return BudgetSpendingAnalysis(
            period: BudgetAnalysisPeriod(
                fromDate: analysis.period.from_date,
                toDate: analysis.period.to_date,
                comparisonFromDate: analysis.period.comparison_from_date,
                comparisonToDate: analysis.period.comparison_to_date,
                granularity: granularity,
                timezone: analysis.period.timezone,
                baseCurrency: baseCurrency
            ),
            totals: BudgetAnalysisTotals(
                incomeBaseMinor: analysis.totals.income_base_minor,
                spendingBaseMinor: analysis.totals.spending_base_minor,
                netBaseMinor: analysis.totals.net_base_minor,
                comparisonIncomeBaseMinor: analysis.totals.comparison_income_base_minor,
                comparisonSpendingBaseMinor: analysis.totals.comparison_spending_base_minor,
                comparisonNetBaseMinor: analysis.totals.comparison_net_base_minor,
                transactionCount: analysis.totals.transaction_count,
                spendingTransactionCount: analysis.totals.spending_transaction_count,
                largestSpendingBaseMinor: analysis.totals.largest_spending_base_minor,
                spendingDayCount: analysis.totals.spending_day_count,
                dayCount: analysis.totals.day_count
            ),
            series: analysis.series.map { bucket in
                BudgetAnalysisBucket(
                    startDate: bucket.start_date,
                    endDate: bucket.end_date,
                    incomeBaseMinor: bucket.income_base_minor,
                    spendingBaseMinor: bucket.spending_base_minor,
                    netBaseMinor: bucket.net_base_minor,
                    transactionCount: bucket.transaction_count
                )
            },
            categories: try analysis.categories.map { category in
                guard let kind = BudgetCategoryKind(rawValue: category.kind.rawValue) else {
                    throw APIClientError.unexpectedResponse
                }
                return BudgetAnalysisCategory(
                    id: category.id,
                    parentID: category.parent_id,
                    name: category.name,
                    kind: kind,
                    systemKey: category.system_key?.value1.rawValue,
                    predefinedKey: category.predefined_key?.value1.rawValue,
                    iconType: category.icon_type.rawValue,
                    iconValue: category.icon_value,
                    colorKey: category.color_key.rawValue,
                    archivedAt: category.archived_at,
                    directBaseMinor: category.direct_base_minor,
                    rolledUpBaseMinor: category.rolled_up_base_minor,
                    comparisonDirectBaseMinor: category.comparison_direct_base_minor,
                    comparisonRolledUpBaseMinor: category.comparison_rolled_up_base_minor,
                    transactionCount: category.transaction_count,
                    rolledUpTransactionCount: category.rolled_up_transaction_count,
                    largestBaseMinor: category.largest_base_minor,
                    firstDate: category.first_date,
                    lastDate: category.last_date
                )
            },
            categorySeries: analysis.category_series.map { point in
                BudgetAnalysisCategoryPoint(
                    categoryID: point.category_id,
                    startDate: point.start_date,
                    baseMinor: point.base_minor
                )
            },
            weekdays: analysis.weekdays.map { weekday in
                BudgetAnalysisWeekday(
                    weekday: weekday.weekday,
                    incomeBaseMinor: weekday.income_base_minor,
                    spendingBaseMinor: weekday.spending_base_minor,
                    transactionCount: weekday.transaction_count
                )
            },
            days: analysis.days.map { day in
                BudgetAnalysisDay(
                    date: day.date,
                    incomeBaseMinor: day.income_base_minor,
                    spendingBaseMinor: day.spending_base_minor,
                    transactionCount: day.transaction_count
                )
            },
            payees: analysis.payees.map { payee in
                BudgetAnalysisPayee(
                    payee: payee.payee,
                    spendingBaseMinor: payee.spending_base_minor,
                    incomeBaseMinor: payee.income_base_minor,
                    transactionCount: payee.transaction_count,
                    firstDate: payee.first_date,
                    lastDate: payee.last_date
                )
            },
            accounts: try analysis.accounts.map { account in
                guard let type = BudgetAccountType(rawValue: account._type.rawValue),
                      let currency = BudgetCurrency(rawValue: account.currency.rawValue) else {
                    throw APIClientError.unexpectedResponse
                }
                return BudgetAnalysisAccount(
                    id: account.id,
                    name: account.name,
                    type: type,
                    currency: currency,
                    archivedAt: account.archived_at,
                    outflowBaseMinor: account.outflow_base_minor,
                    inflowBaseMinor: account.inflow_base_minor,
                    transactionCount: account.transaction_count
                )
            }
        )
    }

    private func projectionAmounts(
        _ amounts: Components.Schemas.ProjectionAmounts
    ) -> BudgetProjectionAmounts {
        BudgetProjectionAmounts(
            posted: amounts.posted,
            pending: amounts.pending,
            projected: amounts.projected
        )
    }

    private func monthlyBudgetPlan(
        from budget: Components.Schemas.MonthlyBudget
    ) throws -> MonthlyBudgetPlan {
        guard let currency = BudgetCurrency(rawValue: budget.base_currency.rawValue) else {
            throw APIClientError.unexpectedResponse
        }
        return MonthlyBudgetPlan(
            id: budget.id,
            workspaceID: budget.workspace_id,
            name: budget.name,
            month: budget.month,
            timezone: budget.timezone,
            baseCurrency: currency,
            plannedBaseMinor: budget.planned_base_minor,
            usedBaseMinor: budget.used_base_minor,
            remainingBaseMinor: budget.remaining_base_minor,
            items: budget.items.map {
                MonthlyBudgetItem(
                    id: $0.id,
                    categoryID: $0.category_id,
                    categoryName: $0.category_name,
                    categoryPredefinedKey: $0.category_predefined_key?.value1.rawValue,
                    categoryIconType: $0.category_icon_type.rawValue,
                    categoryIconValue: $0.category_icon_value,
                    categoryColorKey: $0.category_color_key.rawValue,
                    categoryArchivedAt: $0.category_archived_at,
                    plannedBaseMinor: $0.planned_base_minor,
                    usedBaseMinor: $0.used_base_minor,
                    remainingBaseMinor: $0.remaining_base_minor
                )
            },
            createdAt: budget.created_at,
            updatedAt: budget.updated_at
        )
    }
}
