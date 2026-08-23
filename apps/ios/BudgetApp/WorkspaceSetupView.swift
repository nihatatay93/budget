import SwiftUI

struct WorkspaceSetupView: View {
    let workspace: BudgetWorkspace
    let session: UserSession
    @ObservedObject var model: AppModel
    let onSelectWorkspace: (String) -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @SceneStorage("selectedWorkspaceTab") private var selectedTabRawValue = WorkspaceTab.overview.rawValue
    @State private var hasLoadedWorkspace = false
    @State private var transactionSearchText = ""
    @State private var transactionStatusScope = TransactionStatusScope.all
    @State private var transactionKindScope = TransactionKindScope.all
    @State private var accountEditorPresented = false
    @State private var categoryEditorPresented = false
    @State private var transactionEditorPresented = false
    @State private var invitationAcceptancePresented = false
    @State private var editingAccount: BudgetAccount?
    @State private var editingCategory: BudgetCategory?
    @State private var editingTransaction: BudgetTransaction?
    @State private var archiveTarget: ArchiveTarget?
    @State private var deleteTransactionTarget: BudgetTransaction?

    private var canManage: Bool { workspace.canManage }

    var body: some View {
        TabView(selection: selectedTabSelection) {
            NavigationStack {
                workspaceContent {
                    WorkspaceOverviewView(
                        workspace: workspace,
                        model: model,
                        onAddTransaction: {
                            editingTransaction = nil
                            transactionEditorPresented = true
                        },
                        onOpenTransactions: { selectTab(.transactions) },
                        onOpenBudget: { selectTab(.budget) }
                    )
                }
            }
            .tabItem { tabLabel(.overview) }
            .tag(WorkspaceTab.overview)

            NavigationStack {
                workspaceContent { transactionsView }
            }
            .tabItem { tabLabel(.transactions) }
            .tag(WorkspaceTab.transactions)

            NavigationStack {
                workspaceContent {
                    MonthlyBudgetView(workspace: workspace, model: model)
                }
            }
            .tabItem { tabLabel(.budget) }
            .tag(WorkspaceTab.budget)

            NavigationStack {
                workspaceContent { accountsView }
            }
            .tabItem { tabLabel(.accounts) }
            .tag(WorkspaceTab.accounts)

            NavigationStack {
                workspaceContent { moreView }
            }
            .tabItem { tabLabel(.more) }
            .tag(WorkspaceTab.more)
        }
        .transaction { transaction in
            if reduceMotion { transaction.animation = nil }
        }
        .task(id: workspace.id) {
            hasLoadedWorkspace = false
            await model.loadResources(workspaceID: workspace.id)
            guard !Task.isCancelled else { return }
            hasLoadedWorkspace = true
        }
        .sheet(isPresented: $accountEditorPresented) {
            AccountEditorView(workspace: workspace, account: editingAccount, model: model)
        }
        .sheet(isPresented: $categoryEditorPresented) {
            CategoryEditorView(
                workspace: workspace,
                category: editingCategory,
                availableCategories: model.categories,
                model: model
            )
        }
        .sheet(isPresented: $transactionEditorPresented) {
            TransactionEditorView(
                workspace: workspace,
                transaction: editingTransaction,
                accounts: model.accounts,
                categories: model.categories,
                model: model
            )
        }
        .sheet(isPresented: $invitationAcceptancePresented) {
            InvitationAcceptanceView(model: model)
        }
        .confirmationDialog(
            archiveTarget?.confirmationTitle ?? "Archive item?",
            isPresented: Binding(
                get: { archiveTarget != nil },
                set: { if !$0 { archiveTarget = nil } }
            ),
            titleVisibility: .visible
        ) {
            Button("Archive", role: .destructive) {
                guard let archiveTarget else { return }
                Task {
                    switch archiveTarget {
                    case let .account(account):
                        await model.archiveAccount(workspaceID: workspace.id, accountID: account.id)
                    case let .category(category):
                        await model.archiveCategory(workspaceID: workspace.id, categoryID: category.id)
                    }
                    self.archiveTarget = nil
                }
            }
            Button("Cancel", role: .cancel) { archiveTarget = nil }
        } message: {
            Text(
                archiveTarget?.confirmationMessage
                    ?? "Historical financial records will remain intact."
            )
        }
        .confirmationDialog(
            "Delete this transaction?",
            isPresented: Binding(
                get: { deleteTransactionTarget != nil },
                set: { if !$0 { deleteTransactionTarget = nil } }
            ),
            titleVisibility: .visible
        ) {
            Button("Delete transaction", role: .destructive) {
                guard let transaction = deleteTransactionTarget else { return }
                Task {
                    await model.deleteTransaction(
                        workspaceID: workspace.id,
                        transactionID: transaction.id
                    )
                    deleteTransactionTarget = nil
                }
            }
            Button("Cancel", role: .cancel) { deleteTransactionTarget = nil }
        } message: {
            Text("This is a soft deletion. The transaction remains recoverable in storage, but stops affecting balances, budgets, and reports.")
        }
        .overlay(alignment: .bottom) {
            if model.isSavingResource {
                ProgressView()
                    .padding()
                    .background(.regularMaterial, in: Capsule())
                    .padding()
            }
        }
        .alert(
            "Workspace error",
            isPresented: Binding(
                get: {
                    model.resourceErrorMessage != nil
                        && !accountEditorPresented
                        && !categoryEditorPresented
                        && !transactionEditorPresented
                },
                set: { if !$0 { model.resourceErrorMessage = nil } }
            )
        ) {
            Button("OK") { model.resourceErrorMessage = nil }
        } message: {
            Text(model.resourceErrorMessage ?? "The request could not be completed.")
        }
    }

    @ViewBuilder
    private func workspaceContent<Content: View>(
        @ViewBuilder content: () -> Content
    ) -> some View {
        if hasLoadedWorkspace {
            content()
        } else {
            ProgressView("Loading \(workspace.name)…")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
    }

    private func tabLabel(_ tab: WorkspaceTab) -> some View {
        Label(
            tab.title,
            systemImage: selectedTab == tab ? tab.selectedSystemImage : tab.systemImage
        )
    }

    private var selectedTab: WorkspaceTab {
        WorkspaceTab.restored(from: selectedTabRawValue)
    }

    private var selectedTabSelection: Binding<WorkspaceTab> {
        Binding(
            get: { selectedTab },
            set: { selectedTabRawValue = $0.rawValue }
        )
    }

    private func selectTab(_ tab: WorkspaceTab) {
        selectedTabRawValue = tab.rawValue
    }

    private var transactionsView: some View {
        List {
            if model.transactions.isEmpty {
                ContentUnavailableView(
                    "No transactions",
                    systemImage: "list.bullet.rectangle",
                    description: Text("Record an expense, income, transfer, or adjustment.")
                )
            } else if filteredTransactions.isEmpty {
                ContentUnavailableView(
                    "No matching transactions",
                    systemImage: "line.3.horizontal.decrease.circle",
                    description: Text("Try a different search or clear the current filters.")
                )
                Button("Clear search and filters") { clearTransactionFilters() }
            }
            ForEach(filteredTransactions) { transaction in
                transactionListRow(transaction)
            }
            viewerNotice
        }
        .navigationTitle("Transactions")
        .searchable(
            text: $transactionSearchText,
            placement: .navigationBarDrawer(displayMode: .always),
            prompt: "Payee, account, category, or note"
        )
        .toolbar {
            ToolbarItemGroup(placement: .primaryAction) {
                Menu {
                    Picker("Status", selection: $transactionStatusScope) {
                        ForEach(TransactionStatusScope.allCases) { scope in
                            Text(scope.title).tag(scope)
                        }
                    }
                    Picker("Kind", selection: $transactionKindScope) {
                        ForEach(TransactionKindScope.allCases) { scope in
                            Text(scope.title).tag(scope)
                        }
                    }
                    if transactionFilter.isActive {
                        Divider()
                        Button("Clear filters", systemImage: "xmark.circle") {
                            clearTransactionFilters()
                        }
                    }
                } label: {
                    Label(
                        "Filter transactions",
                        systemImage: transactionFilter.isActive
                            ? "line.3.horizontal.decrease.circle.fill"
                            : "line.3.horizontal.decrease.circle"
                    )
                }
                if canManage {
                    Button("Add transaction", systemImage: "plus") {
                        editingTransaction = nil
                        transactionEditorPresented = true
                    }
                }
            }
        }
        .refreshable { await model.loadResources(workspaceID: workspace.id) }
    }

    private var transactionFilter: TransactionListFilter {
        TransactionListFilter(
            searchText: transactionSearchText,
            status: transactionStatusScope,
            kind: transactionKindScope
        )
    }

    private var filteredTransactions: [BudgetTransaction] {
        let accountNames = Dictionary(uniqueKeysWithValues: model.accounts.map { ($0.id, $0.name) })
        let categoryNames = Dictionary(uniqueKeysWithValues: model.categories.map { ($0.id, $0.name) })
        return model.transactions.filter {
            transactionFilter.matches(
                $0,
                accountNames: accountNames,
                categoryNames: categoryNames
            )
        }
    }

    @ViewBuilder
    private func transactionListRow(_ transaction: BudgetTransaction) -> some View {
        if canManage {
            Button {
                editingTransaction = transaction
                transactionEditorPresented = true
            } label: {
                TransactionRow(
                    transaction: transaction,
                    workspace: workspace,
                    accounts: model.accounts
                )
            }
            .buttonStyle(.plain)
            .accessibilityHint("Opens the transaction editor")
            .swipeActions {
                Button("Delete", role: .destructive) {
                    deleteTransactionTarget = transaction
                }
            }
        } else {
            TransactionRow(
                transaction: transaction,
                workspace: workspace,
                accounts: model.accounts
            )
        }
    }

    private func clearTransactionFilters() {
        transactionSearchText = ""
        transactionStatusScope = .all
        transactionKindScope = .all
    }

    private var accountsView: some View {
        List {
            if model.accounts.isEmpty {
                ContentUnavailableView(
                    "No accounts",
                    systemImage: "building.columns",
                    description: Text("Create an account to begin tracking money.")
                )
            }
            Section("Base-currency position") {
                BaseCurrencyTotalView(
                    workspace: workspace,
                    accounts: model.accounts,
                    rates: model.exchangeRates
                )
            }
            if !currencySummaries.isEmpty {
                Section("Native-currency totals") {
                    ForEach(currencySummaries) { summary in
                        HStack {
                            VStack(alignment: .leading, spacing: 3) {
                                Text(summary.currency.title)
                                    .font(.headline)
                                Text("\(summary.accountCount) active account\(summary.accountCount == 1 ? "" : "s")")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                            Spacer(minLength: 12)
                            Text(
                                summary.balanceMinor.map {
                                    summary.currency.formatted(minorUnits: $0)
                                } ?? "Total unavailable"
                            )
                            .font(.subheadline.monospacedDigit().weight(.semibold))
                        }
                        .accessibilityElement(children: .combine)
                    }
                }
            }
            if !activeAccounts.isEmpty {
                Section {
                    ForEach(activeAccounts) { account in
                        accountListRow(account)
                    }
                } header: {
                    Text("Active accounts")
                } footer: {
                    Text("Account currency locks after financial history exists. Archive keeps every historical entry and balance intact.")
                }
            }
            if !archivedAccounts.isEmpty {
                Section("Archived accounts") {
                    ForEach(archivedAccounts) { account in
                        AccountRow(account: account)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            viewerNotice
        }
        .navigationTitle("Accounts")
        .toolbar {
            if canManage {
                ToolbarItem(placement: .primaryAction) {
                    Button("Add account", systemImage: "plus") {
                        editingAccount = nil
                        accountEditorPresented = true
                    }
                }
            }
        }
        .refreshable { await model.loadResources(workspaceID: workspace.id) }
    }

    private var activeAccounts: [BudgetAccount] {
        model.accounts.filter { $0.archivedAt == nil }
    }

    private var archivedAccounts: [BudgetAccount] {
        model.accounts.filter { $0.archivedAt != nil }
    }

    private var currencySummaries: [AccountCurrencySummary] {
        accountCurrencySummaries(model.accounts)
    }

    @ViewBuilder
    private func accountListRow(_ account: BudgetAccount) -> some View {
        if canManage {
            Button {
                editingAccount = account
                accountEditorPresented = true
            } label: {
                AccountRow(account: account)
            }
            .buttonStyle(.plain)
            .accessibilityHint("Opens the account editor")
            .swipeActions {
                Button("Archive", role: .destructive) {
                    archiveTarget = .account(account)
                }
            }
            .contextMenu {
                Button("Archive account", systemImage: "archivebox", role: .destructive) {
                    archiveTarget = .account(account)
                }
            }
        } else {
            AccountRow(account: account)
        }
    }

    private var moreView: some View {
        List {
            Section("Workspace") {
                Menu {
                    ForEach(session.workspaces) { candidate in
                        Button {
                            onSelectWorkspace(candidate.id)
                        } label: {
                            if candidate.id == workspace.id {
                                Label(candidate.name, systemImage: "checkmark")
                            } else {
                                Text(candidate.name)
                            }
                        }
                    }
                } label: {
                    LabeledContent("Current workspace", value: workspace.name)
                }
                LabeledContent("Base currency", value: workspace.baseCurrency.rawValue)
                LabeledContent("Timezone", value: workspace.timezone)
                LabeledContent("Your role", value: workspace.role.capitalized)
            }

            Section("Explore and organize") {
                NavigationLink {
                    FinancialReportsView(workspace: workspace, model: model)
                } label: {
                    Label("Reports", systemImage: "chart.bar.xaxis")
                }
                NavigationLink {
                    categoriesView
                } label: {
                    LabeledContent {
                        Text("\(model.categories.count)")
                            .foregroundStyle(.secondary)
                    } label: {
                        Label("Categories", systemImage: "tag")
                    }
                }
                NavigationLink {
                    WorkspaceCollaborationView(
                        workspace: workspace,
                        currentUserID: session.user.id,
                        model: model
                    )
                } label: {
                    Label("Members and invitations", systemImage: "person.2")
                }
            }

            Section("Join another workspace") {
                Button("Accept invitation", systemImage: "envelope.open") {
                    model.resourceErrorMessage = nil
                    invitationAcceptancePresented = true
                }
            }

            Section("Account") {
                LabeledContent("Signed in as", value: session.user.displayName)
                Text(session.user.email)
                    .foregroundStyle(.secondary)
                LabeledContent("Server", value: model.serverAddress)
                Button("Sign out", systemImage: "rectangle.portrait.and.arrow.right", role: .destructive) {
                    Task { await model.logout() }
                }
                .disabled(model.isSubmitting)
            }
            viewerNotice
        }
        .navigationTitle("More")
        .toolbar {
            if canManage {
                ToolbarItem(placement: .primaryAction) {
                    Button("Add category", systemImage: "plus") {
                        editingCategory = nil
                        categoryEditorPresented = true
                    }
                }
            }
        }
        .refreshable { await model.loadResources(workspaceID: workspace.id) }
    }

    private var categoriesView: some View {
        List {
            if model.categories.isEmpty {
                ContentUnavailableView(
                    "No categories",
                    systemImage: "tag",
                    description: Text("Create categories to organize reporting and monthly plans.")
                )
            }
            ForEach(BudgetCategoryKind.allCases) { kind in
                let rows = categoryTree(model.categories).filter { $0.category.kind == kind }
                if !rows.isEmpty {
                    Section {
                        ForEach(rows) { row in
                            categoryListRow(row)
                        }
                    } header: {
                        Text(kind.title)
                    } footer: {
                        if kind == .expense {
                            Text("Refunds may appear as positive allocations in expense categories. Protected Uncategorized categories cannot be archived.")
                        }
                    }
                }
            }
            viewerNotice
        }
        .navigationTitle("Categories")
        .toolbar {
            if canManage {
                ToolbarItem(placement: .primaryAction) {
                    Button("Add category", systemImage: "plus") {
                        editingCategory = nil
                        categoryEditorPresented = true
                    }
                }
            }
        }
        .refreshable { await model.loadResources(workspaceID: workspace.id) }
    }

    @ViewBuilder
    private func categoryListRow(_ row: CategoryTreeRow) -> some View {
        let category = row.category
        if canManage && !category.isSystem {
            Button {
                editingCategory = category
                categoryEditorPresented = true
            } label: {
                CategoryRow(category: category, depth: row.depth)
            }
            .buttonStyle(.plain)
            .accessibilityHint("Opens the category editor")
            .swipeActions {
                Button("Archive", role: .destructive) {
                    archiveTarget = .category(category)
                }
            }
            .contextMenu {
                Button("Archive category", systemImage: "archivebox", role: .destructive) {
                    archiveTarget = .category(category)
                }
            }
        } else {
            CategoryRow(category: category, depth: row.depth)
        }
    }

    @ViewBuilder
    private var viewerNotice: some View {
        if !canManage {
            Section {
                Label("Viewer access is read-only.", systemImage: "eye")
                    .foregroundStyle(.secondary)
            }
        }
    }
}

struct TransactionRow: View {
    let transaction: BudgetTransaction
    let workspace: BudgetWorkspace
    let accounts: [BudgetAccount]

    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .center, spacing: 12) {
                identity
                Spacer(minLength: 12)
                amount
            }
            VStack(alignment: .leading, spacing: 10) {
                identity
                amount
            }
        }
        .padding(.vertical, 3)
        .contentShape(Rectangle())
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilitySummary)
    }

    private var identity: some View {
        HStack(alignment: .center, spacing: 12) {
            Image(systemName: kindIcon)
                .font(.headline)
                .foregroundStyle(kindColor)
                .frame(width: 36, height: 36)
                .background(kindColor.opacity(0.12), in: Circle())
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(transaction.payee ?? transaction.description ?? transaction.kind.title)
                    .font(.headline)
                HStack(spacing: 6) {
                    Text(transaction.transactionDate)
                    Text("·")
                    Text(transaction.kind.title)
                    if transaction.status == .pending {
                        Text("Pending")
                            .font(.caption2.weight(.semibold))
                            .foregroundStyle(.orange)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(.orange.opacity(0.12), in: Capsule())
                    }
                }
                .font(.caption)
                .foregroundStyle(.secondary)
                Text(accountNames)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }

    @ViewBuilder
    private var amount: some View {
        Group {
            if transaction.kind == .transfer {
                Text("Transfer")
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(.secondary)
            } else if let total {
                Text(workspace.baseCurrency.formatted(minorUnits: total))
                    .font(.subheadline.monospacedDigit().weight(.semibold))
                    .foregroundStyle(total > 0 ? Color.green : Color.primary)
                    .minimumScaleFactor(0.75)
                    .lineLimit(1)
            }
        }
    }

    private var kindIcon: String {
        switch transaction.kind {
        case .standard: total ?? 0 >= 0 ? "arrow.down.left" : "arrow.up.right"
        case .transfer: "arrow.left.arrow.right"
        case .adjustment: "plus.forwardslash.minus"
        }
    }

    private var kindColor: Color {
        switch transaction.kind {
        case .standard: total ?? 0 >= 0 ? .green : .orange
        case .transfer: .blue
        case .adjustment: .purple
        }
    }

    private var accessibilitySummary: String {
        let title = transaction.payee ?? transaction.description ?? transaction.kind.title
        let amount = transaction.kind == .transfer
            ? "transfer"
            : total.map { workspace.baseCurrency.formatted(minorUnits: $0) } ?? "amount unavailable"
        return "\(title), \(transaction.transactionDate), \(transaction.kind.title), "
            + "\(transaction.status.title), \(amount), \(accountNames)"
    }

    private var accountNames: String {
        let names = transaction.entries.map { entry in
            accounts.first { $0.id == entry.accountID }?.name ?? "Unavailable account"
        }
        return names.joined(separator: " → ")
    }

    private var total: Int64? {
        var result: Int64 = 0
        for entry in transaction.entries {
            let sum = result.addingReportingOverflow(entry.baseAmountMinor)
            if sum.overflow { return nil }
            result = sum.partialValue
        }
        return result
    }
}

private struct BaseCurrencyTotalView: View {
    let workspace: BudgetWorkspace
    let accounts: [BudgetAccount]
    let rates: [BudgetExchangeRate]
    @State private var displayCurrency: BudgetCurrency

    init(
        workspace: BudgetWorkspace,
        accounts: [BudgetAccount],
        rates: [BudgetExchangeRate]
    ) {
        self.workspace = workspace
        self.accounts = accounts
        self.rates = rates
        _displayCurrency = State(initialValue: workspace.baseCurrency)
    }

    var body: some View {
        if let total {
            VStack(alignment: .leading, spacing: 8) {
                Text("Posted across active \(workspace.baseCurrency.rawValue) accounts")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text(workspace.baseCurrency.formatted(minorUnits: total))
                    .font(.title2.monospacedDigit().weight(.semibold))
                    .minimumScaleFactor(0.7)
                    .lineLimit(1)
                if let selectedRate,
                   let converted = selectedRate.convert(minorUnits: total) {
                    Text(
                        "≈ \(displayCurrency.formatted(minorUnits: converted)) at the rate published "
                        + selectedRate.rateDate
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                }
                if excludedAccountCount > 0 {
                    Text("Accounts in another currency are not included in this total.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                if !rates.isEmpty {
                    Picker("Show in", selection: $displayCurrency) {
                        Text(workspace.baseCurrency.rawValue).tag(workspace.baseCurrency)
                        ForEach(rates) { rate in
                            Text(rate.quoteCurrency.rawValue).tag(rate.quoteCurrency)
                        }
                    }
                }
            }
            .padding(.vertical, 5)
            .accessibilityElement(children: .contain)
        } else {
            Label(
                "No active accounts use \(workspace.baseCurrency.rawValue)",
                systemImage: "building.columns"
            )
            .foregroundStyle(.secondary)
        }
    }

    private var total: Int64? {
        let amounts = accounts
            .filter { $0.currency == workspace.baseCurrency && $0.archivedAt == nil }
            .map(\.balanceMinor)
        guard !amounts.isEmpty else { return nil }
        var result: Int64 = 0
        for amount in amounts {
            let sum = result.addingReportingOverflow(amount)
            if sum.overflow { return nil }
            result = sum.partialValue
        }
        return result
    }

    private var excludedAccountCount: Int {
        accounts.count { $0.currency != workspace.baseCurrency && $0.archivedAt == nil }
    }

    private var selectedRate: BudgetExchangeRate? {
        rates.first { $0.quoteCurrency == displayCurrency }
    }
}

private struct AccountRow: View {
    let account: BudgetAccount

    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: 12) {
                identity
                Spacer(minLength: 12)
                balance
            }
            VStack(alignment: .leading, spacing: 10) {
                identity
                balance
            }
        }
        .padding(.vertical, 3)
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
    }

    private var identity: some View {
        HStack(spacing: 12) {
            Image(systemName: systemImage)
                .font(.headline)
                .foregroundStyle(.blue)
                .frame(width: 36, height: 36)
                .background(.blue.opacity(0.1), in: Circle())
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(account.name)
                    .font(.headline)
                Text([account.type.title, account.currency.rawValue, account.institutionName]
                    .compactMap { $0 }
                    .joined(separator: " · "))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                if account.archivedAt != nil {
                    Label("Archived", systemImage: "archivebox.fill")
                        .font(.caption2.weight(.semibold))
                    .foregroundStyle(.secondary)
                }
            }
        }
    }

    private var balance: some View {
        Text(account.currency.formatted(minorUnits: account.balanceMinor))
            .font(.subheadline.monospacedDigit().weight(.semibold))
            .minimumScaleFactor(0.75)
            .lineLimit(1)
    }

    private var systemImage: String {
        switch account.type {
        case .bank: "building.columns"
        case .cash: "banknote"
        case .creditCard: "creditcard"
        case .savings: "building.columns.fill"
        case .investment: "chart.line.uptrend.xyaxis"
        case .other: "wallet.bifold"
        }
    }
}

private struct CategoryRow: View {
    let category: BudgetCategory
    let depth: Int

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text([category.icon, category.name].compactMap { $0 }.joined(separator: " "))
                    .font(.headline)
                Text(category.isSystem ? "\(category.kind.title) · Protected" : category.kind.title)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if category.isSystem {
                Image(systemName: "lock.fill")
                    .foregroundStyle(.secondary)
                    .accessibilityLabel("Protected category")
            }
        }
        .padding(.leading, CGFloat(depth) * 16)
        .contentShape(Rectangle())
    }
}

private struct AccountEditorView: View {
    let workspace: BudgetWorkspace
    let account: BudgetAccount?
    @ObservedObject var model: AppModel
    @Environment(\.dismiss) private var dismiss

    @State private var name: String
    @State private var type: BudgetAccountType
    @State private var currency: BudgetCurrency
    @State private var institutionName: String

    init(workspace: BudgetWorkspace, account: BudgetAccount?, model: AppModel) {
        self.workspace = workspace
        self.account = account
        self.model = model
        _name = State(initialValue: account?.name ?? "")
        _type = State(initialValue: account?.type ?? .bank)
        _currency = State(initialValue: account?.currency ?? workspace.baseCurrency)
        _institutionName = State(initialValue: account?.institutionName ?? "")
    }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Name", text: $name)
                    Picker("Type", selection: $type) {
                        ForEach(BudgetAccountType.allCases) { type in
                            Text(type.title).tag(type)
                        }
                    }
                    Picker("Currency", selection: $currency) {
                        ForEach(BudgetCurrency.allCases) { currency in
                            Text(currency.title).tag(currency)
                        }
                    }
                    TextField("Institution (optional)", text: $institutionName)
                } header: {
                    Text("Account")
                } footer: {
                    Text("Currency cannot change after the account has financial history. Archiving preserves every entry and derived balance.")
                }
                ResourceErrorSection(message: model.resourceErrorMessage)
            }
            .scrollDismissesKeyboard(.interactively)
            .navigationTitle(account == nil ? "Add account" : "Edit account")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") {
                        Task {
                            let saved = await model.saveAccount(
                                workspaceID: workspace.id,
                                accountID: account?.id,
                                input: AccountInput(
                                    name: name.trimmingCharacters(in: .whitespacesAndNewlines),
                                    type: type,
                                    currency: currency,
                                    institutionName: institutionName.nilIfBlank
                                )
                            )
                            if saved { dismiss() }
                        }
                    }
                    .disabled(!isValid || model.isSavingResource)
                }
            }
        }
    }

    private var isValid: Bool {
        !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}

private struct CategoryEditorView: View {
    let workspace: BudgetWorkspace
    let category: BudgetCategory?
    let availableCategories: [BudgetCategory]
    @ObservedObject var model: AppModel
    @Environment(\.dismiss) private var dismiss

    @State private var name: String
    @State private var kind: BudgetCategoryKind
    @State private var parentID: String
    @State private var icon: String

    init(
        workspace: BudgetWorkspace,
        category: BudgetCategory?,
        availableCategories: [BudgetCategory],
        model: AppModel
    ) {
        self.workspace = workspace
        self.category = category
        self.availableCategories = availableCategories
        self.model = model
        _name = State(initialValue: category?.name ?? "")
        _kind = State(initialValue: category?.kind ?? .expense)
        _parentID = State(initialValue: category?.parentID ?? "")
        _icon = State(initialValue: category?.icon ?? "")
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("Category") {
                    TextField("Name", text: $name)
                    Picker("Kind", selection: $kind) {
                        ForEach(BudgetCategoryKind.allCases) { kind in
                            Text(kind.title).tag(kind)
                        }
                    }
                    .onChange(of: kind) { _, _ in parentID = "" }
                    Picker("Parent", selection: $parentID) {
                        Text("Top level").tag("")
                        ForEach(parentCandidates) { candidate in
                            Text(candidate.name).tag(candidate.id)
                        }
                    }
                    TextField("Icon (optional)", text: $icon)
                }
                ResourceErrorSection(message: model.resourceErrorMessage)
            }
            .navigationTitle(category == nil ? "Add category" : "Edit category")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") {
                        Task {
                            let saved = await model.saveCategory(
                                workspaceID: workspace.id,
                                categoryID: category?.id,
                                input: CategoryInput(
                                    name: name.trimmingCharacters(in: .whitespacesAndNewlines),
                                    kind: kind,
                                    parentID: parentID.nilIfBlank,
                                    icon: icon.nilIfBlank
                                )
                            )
                            if saved { dismiss() }
                        }
                    }
                    .disabled(name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || model.isSavingResource)
                }
            }
        }
    }

    private var parentCandidates: [BudgetCategory] {
        let excluded = categoryDescendantIDs(availableCategories, rootID: category?.id)
        return availableCategories.filter {
            $0.kind == kind && !excluded.contains($0.id) && $0.archivedAt == nil
        }
    }
}

/// Shared by the workspace setup and collaboration screens.
struct ResourceErrorSection: View {
    let message: String?

    var body: some View {
        if let message {
            Section {
                Label(message, systemImage: "exclamationmark.triangle.fill")
                    .foregroundStyle(.red)
            }
        }
    }
}

private enum ArchiveTarget: Identifiable {
    case account(BudgetAccount)
    case category(BudgetCategory)

    var id: String {
        switch self {
        case let .account(account): "account-\(account.id)"
        case let .category(category): "category-\(category.id)"
        }
    }

    var confirmationTitle: String {
        switch self {
        case let .account(account): "Archive \(account.name)?"
        case let .category(category): "Archive \(category.name)?"
        }
    }

    var confirmationMessage: String {
        switch self {
        case .account:
            "The account leaves active setup but its entries and derived balance remain in financial history."
        case .category:
            "The category leaves active organization but historical allocations remain in reports. Categories with active children must be reorganized first."
        }
    }
}

private struct CategoryTreeRow: Identifiable {
    let category: BudgetCategory
    let depth: Int
    var id: String { category.id }
}

private func categoryTree(_ categories: [BudgetCategory]) -> [CategoryTreeRow] {
    let children = Dictionary(grouping: categories) { $0.parentID ?? "" }
        .mapValues { values in
            values.sorted {
                if $0.kind != $1.kind { return $0.kind.rawValue < $1.kind.rawValue }
                return $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending
            }
        }
    var rows: [CategoryTreeRow] = []
    var visited = Set<String>()
    func append(parentID: String, depth: Int) {
        for category in children[parentID] ?? [] where !visited.contains(category.id) {
            visited.insert(category.id)
            rows.append(CategoryTreeRow(category: category, depth: depth))
            append(parentID: category.id, depth: depth + 1)
        }
    }
    append(parentID: "", depth: 0)
    for category in categories where !visited.contains(category.id) {
        rows.append(CategoryTreeRow(category: category, depth: 0))
    }
    return rows
}

// Excluding the entire subtree keeps the native editor aligned with the domain and database
// cycle guards instead of offering a parent choice that can only fail with a conflict.
private func categoryDescendantIDs(
    _ categories: [BudgetCategory],
    rootID: String?
) -> Set<String> {
    guard let rootID else { return [] }
    var excluded: Set<String> = [rootID]
    var changed = true
    while changed {
        changed = false
        for category in categories
            where category.parentID.map(excluded.contains) == true && !excluded.contains(category.id)
        {
            excluded.insert(category.id)
            changed = true
        }
    }
    return excluded
}
