import Foundation

@MainActor
final class AppModel: ObservableObject {
    @Published private(set) var session: UserSession?
    @Published private(set) var isLoading = true
    @Published private(set) var isSubmitting = false
    @Published private(set) var accounts: [BudgetAccount] = []
    @Published private(set) var categories: [BudgetCategory] = []
    @Published private(set) var exchangeRates: [BudgetExchangeRate] = []
    @Published private(set) var transactions: [BudgetTransaction] = []
    @Published private(set) var financialProjection: BudgetFinancialProjection?
    @Published private(set) var monthlyBudget: MonthlyBudgetPlan?
    @Published private(set) var isLoadingResources = false
    @Published private(set) var isLoadingProjection = false
    @Published private(set) var isLoadingBudget = false
    @Published private(set) var isSavingResource = false
    @Published var resourceErrorMessage: String?
    @Published private(set) var members: [BudgetWorkspaceMember] = []
    @Published private(set) var invitations: [BudgetWorkspaceInvitation] = []
    /// Disclosed once when an invitation is created; held only long enough to show it.
    @Published var issuedInvitation: BudgetIssuedInvitation?
    @Published var budgetErrorMessage: String?
    @Published var errorMessage: String?
    @Published var serverAddress: String

    private let apiClient: any APIClient
    private let sessionStore: any SessionStore
    private var restored = false
    private var token: String?
    private var activeResourceWorkspaceID: String?
    private var activeProjectionRange: BudgetProjectionRange?
    private var activeBudgetMonth: String?
    private var resourceLoadSequence = 0
    private var projectionLoadSequence = 0
    private var budgetLoadSequence = 0

    init(environment: AppEnvironment) {
        apiClient = environment.apiClient
        sessionStore = environment.sessionStore
        serverAddress = UserDefaults.standard.string(forKey: "serverAddress") ?? "http://localhost:8080"
    }

    func restore() async {
        guard !restored else { return }
        restored = true
        defer { isLoading = false }
        do {
            guard let storedToken = try sessionStore.loadToken(),
                  let serverURL = validatedServerURL() else { return }
            token = storedToken
            session = try await apiClient.session(serverURL: serverURL, token: storedToken)
        } catch APIClientError.unauthorized {
            token = nil
            try? sessionStore.deleteToken()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func login(email: String, password: String) async {
        guard let serverURL = validatedServerURL() else { return }
        await authenticate {
            try await apiClient.login(serverURL: serverURL, email: email, password: password)
        }
    }

    func register(
        email: String,
        password: String,
        displayName: String,
        workspaceName: String,
        baseCurrency: BudgetCurrency
    ) async {
        guard let serverURL = validatedServerURL() else { return }
        await authenticate {
            try await apiClient.register(
                serverURL: serverURL,
                input: RegistrationInput(
                    email: email,
                    password: password,
                    displayName: displayName,
                    workspaceName: workspaceName,
                    baseCurrency: baseCurrency,
                    timezone: TimeZone.current.identifier
                )
            )
        }
    }

    func logout() async {
        guard let token else { return }
        isSubmitting = true
        if let serverURL = validatedServerURL(showError: false) {
            try? await apiClient.logout(serverURL: serverURL, token: token)
        }
        try? sessionStore.deleteToken()
        self.token = nil
        session = nil
        accounts = []
        categories = []
        exchangeRates = []
        transactions = []
        financialProjection = nil
        monthlyBudget = nil
        activeResourceWorkspaceID = nil
        activeProjectionRange = nil
        activeBudgetMonth = nil
        resourceLoadSequence += 1
        projectionLoadSequence += 1
        budgetLoadSequence += 1
        isLoadingBudget = false
        errorMessage = nil
        resourceErrorMessage = nil
        budgetErrorMessage = nil
        isSubmitting = false
    }

    func loadResources(workspaceID: String) async {
        guard let context = resourceContext() else { return }
        if activeResourceWorkspaceID != workspaceID {
            accounts = []
            categories = []
            exchangeRates = []
            transactions = []
            financialProjection = nil
            monthlyBudget = nil
            activeProjectionRange = nil
            activeBudgetMonth = nil
            budgetErrorMessage = nil
            budgetLoadSequence += 1
            isLoadingBudget = false
        }
        activeResourceWorkspaceID = workspaceID
        resourceLoadSequence += 1
        let loadSequence = resourceLoadSequence
        projectionLoadSequence += 1
        let projectionSequence = projectionLoadSequence
        let projectionRange = activeProjectionRange
        isLoadingResources = true
        isLoadingProjection = true
        resourceErrorMessage = nil
        defer {
            if resourceLoadSequence == loadSequence {
                isLoadingResources = false
            }
            if projectionLoadSequence == projectionSequence {
                isLoadingProjection = false
            }
        }
        do {
            async let loadedAccounts = apiClient.listAccounts(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID
            )
            async let loadedCategories = apiClient.listCategories(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID
            )
            async let loadedExchangeRates = apiClient.listExchangeRates(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID
            )
            async let loadedTransactions = apiClient.listTransactions(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID
            )
            async let loadedProjection = apiClient.financialProjection(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID,
                range: projectionRange
            )
            let result = try await (
                loadedAccounts,
                loadedCategories,
                loadedExchangeRates,
                loadedTransactions,
                loadedProjection
            )
            guard activeResourceWorkspaceID == workspaceID,
                  resourceLoadSequence == loadSequence else { return }
            (accounts, categories, exchangeRates, transactions) = (
                result.0, result.1, result.2, result.3
            )
            if projectionLoadSequence == projectionSequence,
               activeProjectionRange == projectionRange {
                financialProjection = result.4
            }
        } catch {
            guard activeResourceWorkspaceID == workspaceID, resourceLoadSequence == loadSequence else { return }
            handleResourceError(error)
        }
    }

    func loadFinancialProjection(
        workspaceID: String,
        range: BudgetProjectionRange?
    ) async {
        guard let context = resourceContext(), activeResourceWorkspaceID == workspaceID else { return }
        activeProjectionRange = range
        projectionLoadSequence += 1
        let loadSequence = projectionLoadSequence
        isLoadingProjection = true
        resourceErrorMessage = nil
        defer {
            if projectionLoadSequence == loadSequence {
                isLoadingProjection = false
            }
        }
        do {
            let projection = try await apiClient.financialProjection(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID,
                range: range
            )
            guard activeResourceWorkspaceID == workspaceID,
                  projectionLoadSequence == loadSequence,
                  activeProjectionRange == range else { return }
            financialProjection = projection
        } catch {
            guard activeResourceWorkspaceID == workspaceID, projectionLoadSequence == loadSequence else { return }
            handleResourceError(error)
        }
    }

    func loadMonthlyBudget(workspaceID: String, month: String) async {
        guard let context = resourceContext(), activeResourceWorkspaceID == workspaceID else { return }
        if activeBudgetMonth != month {
            monthlyBudget = nil
        }
        activeBudgetMonth = month
        budgetLoadSequence += 1
        let loadSequence = budgetLoadSequence
        isLoadingBudget = true
        budgetErrorMessage = nil
        defer {
            if budgetLoadSequence == loadSequence {
                isLoadingBudget = false
            }
        }
        do {
            let plan = try await apiClient.monthlyBudget(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID,
                month: month
            )
            guard activeResourceWorkspaceID == workspaceID,
                  activeBudgetMonth == month,
                  budgetLoadSequence == loadSequence else { return }
            monthlyBudget = plan
        } catch {
            guard activeResourceWorkspaceID == workspaceID,
                  activeBudgetMonth == month,
                  budgetLoadSequence == loadSequence else { return }
            handleBudgetError(error)
        }
    }

    func saveMonthlyBudget(
        workspaceID: String,
        month: String,
        input: MonthlyBudgetInput
    ) async -> Bool {
        guard let context = resourceContext() else { return false }
        isSavingResource = true
        budgetErrorMessage = nil
        defer { isSavingResource = false }
        do {
            let plan = try await apiClient.replaceMonthlyBudget(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID,
                month: month,
                input: input
            )
            guard activeResourceWorkspaceID == workspaceID, activeBudgetMonth == month else {
                return true
            }
            monthlyBudget = plan
            return true
        } catch {
            handleBudgetError(error)
            return false
        }
    }

    func saveAccount(workspaceID: String, accountID: String?, input: AccountInput) async -> Bool {
        guard let context = resourceContext() else { return false }
        return await performResourceMutation {
            let account: BudgetAccount
            if let accountID {
                account = try await apiClient.updateAccount(
                    serverURL: context.serverURL,
                    token: context.token,
                    workspaceID: workspaceID,
                    accountID: accountID,
                    input: input
                )
            } else {
                account = try await apiClient.createAccount(
                    serverURL: context.serverURL,
                    token: context.token,
                    workspaceID: workspaceID,
                    input: input
                )
            }
            guard activeResourceWorkspaceID == workspaceID else { return }
            accounts.removeAll { $0.id == account.id }
            accounts.append(account)
            accounts.sort { $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending }
            try await refreshFinancialProjection(context: context, workspaceID: workspaceID)
        }
    }

    func archiveAccount(workspaceID: String, accountID: String) async {
        guard let context = resourceContext() else { return }
        _ = await performResourceMutation {
            try await apiClient.archiveAccount(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID,
                accountID: accountID
            )
            guard activeResourceWorkspaceID == workspaceID else { return }
            accounts.removeAll { $0.id == accountID }
            try await refreshFinancialProjection(context: context, workspaceID: workspaceID)
        }
    }

    func saveCategory(workspaceID: String, categoryID: String?, input: CategoryInput) async -> Bool {
        guard let context = resourceContext() else { return false }
        return await performResourceMutation {
            let category: BudgetCategory
            if let categoryID {
                category = try await apiClient.updateCategory(
                    serverURL: context.serverURL,
                    token: context.token,
                    workspaceID: workspaceID,
                    categoryID: categoryID,
                    input: input
                )
            } else {
                category = try await apiClient.createCategory(
                    serverURL: context.serverURL,
                    token: context.token,
                    workspaceID: workspaceID,
                    input: input
                )
            }
            guard activeResourceWorkspaceID == workspaceID else { return }
            categories.removeAll { $0.id == category.id }
            categories.append(category)
            categories.sort { $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending }
            try await refreshFinancialProjection(context: context, workspaceID: workspaceID)
        }
    }

    func archiveCategory(workspaceID: String, categoryID: String) async {
        guard let context = resourceContext() else { return }
        _ = await performResourceMutation {
            try await apiClient.archiveCategory(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID,
                categoryID: categoryID
            )
            guard activeResourceWorkspaceID == workspaceID else { return }
            categories.removeAll { $0.id == categoryID }
            try await refreshFinancialProjection(context: context, workspaceID: workspaceID)
        }
    }

    func saveTransaction(
        workspaceID: String,
        transactionID: String?,
        input: TransactionInput
    ) async -> Bool {
        guard let context = resourceContext() else { return false }
        return await performResourceMutation {
            let transaction: BudgetTransaction
            if let transactionID {
                transaction = try await apiClient.updateTransaction(
                    serverURL: context.serverURL,
                    token: context.token,
                    workspaceID: workspaceID,
                    transactionID: transactionID,
                    input: input
                )
            } else {
                transaction = try await apiClient.createTransaction(
                    serverURL: context.serverURL,
                    token: context.token,
                    workspaceID: workspaceID,
                    input: input
                )
            }
            guard activeResourceWorkspaceID == workspaceID else { return }
            transactions.removeAll { $0.id == transaction.id }
            transactions.append(transaction)
            sortTransactions()
            // A posted transaction can change derived balances immediately.
            accounts = try await apiClient.listAccounts(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID
            )
            try await refreshFinancialProjection(context: context, workspaceID: workspaceID)
        }
    }

    func deleteTransaction(workspaceID: String, transactionID: String) async {
        guard let context = resourceContext() else { return }
        _ = await performResourceMutation {
            try await apiClient.deleteTransaction(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID,
                transactionID: transactionID
            )
            guard activeResourceWorkspaceID == workspaceID else { return }
            transactions.removeAll { $0.id == transactionID }
            accounts = try await apiClient.listAccounts(
                serverURL: context.serverURL,
                token: context.token,
                workspaceID: workspaceID
            )
            try await refreshFinancialProjection(context: context, workspaceID: workspaceID)
        }
    }

    private func authenticate(
        operation: () async throws -> AuthenticatedSession
    ) async {
        isSubmitting = true
        errorMessage = nil
        defer { isSubmitting = false }
        do {
            let authenticated = try await operation()
            try sessionStore.saveToken(authenticated.token)
            token = authenticated.token
            session = authenticated.session
            UserDefaults.standard.set(serverAddress, forKey: "serverAddress")
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    // MARK: - Collaboration

    func loadCollaboration(workspaceID: String, canListInvitations: Bool) async {
        guard let context = resourceContext() else { return }
        _ = await performResourceMutation {
            let loadedMembers = try await apiClient.listWorkspaceMembers(
                serverURL: context.serverURL, token: context.token, workspaceID: workspaceID
            )
            // Invitations are requested only for a role the server will serve them to, so a
            // member never triggers a forbidden response just by opening the screen.
            let loadedInvitations = canListInvitations
                ? try await apiClient.listWorkspaceInvitations(
                    serverURL: context.serverURL, token: context.token, workspaceID: workspaceID
                )
                : []
            members = loadedMembers
            invitations = loadedInvitations
        }
    }

    func changeMemberRole(
        workspaceID: String, userID: String, role: BudgetWorkspaceRoleValue
    ) async {
        guard let context = resourceContext() else { return }
        _ = await performResourceMutation {
            let updated = try await apiClient.updateWorkspaceMemberRole(
                serverURL: context.serverURL, token: context.token,
                workspaceID: workspaceID, userID: userID, role: role
            )
            if let index = members.firstIndex(where: { $0.userID == updated.userID }) {
                members[index] = updated
            }
            // The actor's own rights can change, so the session is reloaded.
            try await reloadSession(context: context)
        }
    }

    func removeMember(workspaceID: String, userID: String) async {
        guard let context = resourceContext() else { return }
        _ = await performResourceMutation {
            try await apiClient.removeWorkspaceMember(
                serverURL: context.serverURL, token: context.token,
                workspaceID: workspaceID, userID: userID
            )
            members.removeAll { $0.userID == userID }
            try await reloadSession(context: context)
        }
    }

    func createInvitation(workspaceID: String, input: BudgetInvitationInput) async -> Bool {
        guard let context = resourceContext() else { return false }
        return await performResourceMutation {
            let issued = try await apiClient.createWorkspaceInvitation(
                serverURL: context.serverURL, token: context.token,
                workspaceID: workspaceID, input: input
            )
            issuedInvitation = issued
            invitations.append(issued.invitation)
        }
    }

    func revokeInvitation(workspaceID: String, invitationID: String) async {
        guard let context = resourceContext() else { return }
        _ = await performResourceMutation {
            try await apiClient.revokeWorkspaceInvitation(
                serverURL: context.serverURL, token: context.token,
                workspaceID: workspaceID, invitationID: invitationID
            )
            invitations.removeAll { $0.id == invitationID }
            if issuedInvitation?.invitation.id == invitationID { issuedInvitation = nil }
        }
    }

    func acceptInvitation(acceptanceToken: String) async -> Bool {
        guard let context = resourceContext() else { return false }
        return await performResourceMutation {
            _ = try await apiClient.acceptWorkspaceInvitation(
                serverURL: context.serverURL, token: context.token,
                acceptanceToken: acceptanceToken
            )
            // Joining adds a workspace, which the switcher reads from the session.
            try await reloadSession(context: context)
        }
    }

    private func reloadSession(context: (serverURL: URL, token: String)) async throws {
        session = try await apiClient.session(
            serverURL: context.serverURL, token: context.token
        )
    }

    private func resourceContext() -> (serverURL: URL, token: String)? {
        guard let token, let serverURL = validatedServerURL(showError: false) else {
            resourceErrorMessage = "Sign in again to continue."
            return nil
        }
        return (serverURL, token)
    }

    private func performResourceMutation(operation: () async throws -> Void) async -> Bool {
        isSavingResource = true
        resourceErrorMessage = nil
        defer { isSavingResource = false }
        do {
            try await operation()
            return true
        } catch {
            handleResourceError(error)
            return false
        }
    }

    private func refreshFinancialProjection(
        context: (serverURL: URL, token: String),
        workspaceID: String
    ) async throws {
        let range = activeProjectionRange
        let projection = try await apiClient.financialProjection(
            serverURL: context.serverURL,
            token: context.token,
            workspaceID: workspaceID,
            range: range
        )
        guard activeResourceWorkspaceID == workspaceID, activeProjectionRange == range else { return }
        financialProjection = projection
    }

    private func handleResourceError(_ error: Error) {
        if case APIClientError.unauthorized = error {
            token = nil
            session = nil
            accounts = []
            categories = []
            exchangeRates = []
            transactions = []
            financialProjection = nil
            monthlyBudget = nil
            activeResourceWorkspaceID = nil
            activeProjectionRange = nil
            activeBudgetMonth = nil
            resourceLoadSequence += 1
            projectionLoadSequence += 1
            budgetLoadSequence += 1
            isLoadingBudget = false
            try? sessionStore.deleteToken()
        }
        resourceErrorMessage = error.localizedDescription
    }

    private func handleBudgetError(_ error: Error) {
        if case APIClientError.unauthorized = error {
            handleResourceError(error)
        }
        budgetErrorMessage = error.localizedDescription
    }

    private func validatedServerURL(showError: Bool = true) -> URL? {
        let normalized = serverAddress.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let components = URLComponents(string: normalized),
              let scheme = components.scheme?.lowercased(),
              let host = components.host,
              !host.isEmpty,
              components.path.isEmpty || components.path == "/",
              components.query == nil,
              components.fragment == nil,
              scheme == "https" || (scheme == "http" && ["localhost", "127.0.0.1"].contains(host)),
              let url = components.url else {
            if showError {
                errorMessage = "Use an HTTPS server URL, or HTTP for localhost development."
            }
            return nil
        }
        serverAddress = url.absoluteString.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        return URL(string: serverAddress)
    }

    private func sortTransactions() {
        transactions.sort {
            if $0.transactionDate != $1.transactionDate {
                return $0.transactionDate > $1.transactionDate
            }
            return $0.id > $1.id
        }
    }
}
