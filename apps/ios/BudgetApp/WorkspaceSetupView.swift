import SwiftUI

struct WorkspaceSetupView: View {
    let workspace: BudgetWorkspace
    let currentUserID: String
    @ObservedObject var model: AppModel

    @State private var accountEditorPresented = false
    @State private var categoryEditorPresented = false
    @State private var transactionEditorPresented = false
    @State private var editingAccount: BudgetAccount?
    @State private var editingCategory: BudgetCategory?
    @State private var editingTransaction: BudgetTransaction?
    @State private var archiveTarget: ArchiveTarget?
    @State private var deleteTransactionTarget: BudgetTransaction?

    private var canManage: Bool { workspace.canManage }

    var body: some View {
        List {
            if model.isLoadingResources {
                Section {
                    HStack {
                        Spacer()
                        ProgressView("Loading workspace…")
                        Spacer()
                    }
                }
            }

            Section("Overview") {
                NavigationLink {
                    FinancialDashboardView(workspace: workspace, model: model)
                } label: {
                    ProjectionOverviewRow(
                        projection: model.financialProjection,
                        isLoading: model.isLoadingResources
                    )
                }
                NavigationLink {
                    MonthlyBudgetView(workspace: workspace, model: model)
                } label: {
                    Label("Monthly budget", systemImage: "chart.pie.fill")
                }
            }

            Section("Accounts") {
                BaseCurrencyTotalView(
                    workspace: workspace,
                    accounts: model.accounts,
                    rates: model.exchangeRates
                )
                if !model.isLoadingResources && model.accounts.isEmpty {
                    ContentUnavailableView(
                        "No accounts",
                        systemImage: "building.columns",
                        description: Text("Create an account to begin tracking money.")
                    )
                }
                ForEach(model.accounts) { account in
                    Button {
                        guard canManage else { return }
                        editingAccount = account
                        accountEditorPresented = true
                    } label: {
                        AccountRow(account: account)
                    }
                    .buttonStyle(.plain)
                    .swipeActions {
                        if canManage {
                            Button("Archive", role: .destructive) {
                                archiveTarget = .account(account)
                            }
                        }
                    }
                }
            }

            Section("Categories") {
                ForEach(categoryTree(model.categories)) { row in
                    let category = row.category
                    Button {
                        guard canManage, !category.isSystem else { return }
                        editingCategory = category
                        categoryEditorPresented = true
                    } label: {
                        CategoryRow(
                            category: category,
                            depth: row.depth
                        )
                    }
                    .buttonStyle(.plain)
                    .swipeActions {
                        if canManage && !category.isSystem {
                            Button("Archive", role: .destructive) {
                                archiveTarget = .category(category)
                            }
                        }
                    }
                }
            }

            Section("Transactions") {
                if !model.isLoadingResources && model.transactions.isEmpty {
                    ContentUnavailableView(
                        "No transactions",
                        systemImage: "list.bullet.rectangle",
                        description: Text("Record an expense, income, transfer, or adjustment.")
                    )
                }
                ForEach(model.transactions) { transaction in
                    Button {
                        guard canManage else { return }
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
                    .swipeActions {
                        if canManage {
                            Button("Delete", role: .destructive) {
                                deleteTransactionTarget = transaction
                            }
                        }
                    }
                }
            }

            Section("People") {
                NavigationLink {
                    WorkspaceCollaborationView(
                        workspace: workspace, currentUserID: currentUserID, model: model
                    )
                } label: {
                    Label("Members and invitations", systemImage: "person.2")
                }
            }

            if !canManage {
                Section {
                    Label("Viewer access is read-only.", systemImage: "eye")
                        .foregroundStyle(.secondary)
                }
            }
        }
        .navigationTitle(workspace.name)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            if canManage {
                ToolbarItem(placement: .topBarTrailing) {
                    Menu("Add", systemImage: "plus") {
                        Button("Account", systemImage: "building.columns") {
                            editingAccount = nil
                            accountEditorPresented = true
                        }
                        Button("Category", systemImage: "tag") {
                            editingCategory = nil
                            categoryEditorPresented = true
                        }
                        Button("Transaction", systemImage: "list.bullet.rectangle") {
                            editingTransaction = nil
                            transactionEditorPresented = true
                        }
                    }
                }
            }
        }
        .task(id: workspace.id) {
            await model.loadResources(workspaceID: workspace.id)
        }
        .refreshable {
            await model.loadResources(workspaceID: workspace.id)
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
        }
        .confirmationDialog(
            "Delete this transaction?",
            isPresented: Binding(
                get: { deleteTransactionTarget != nil },
                set: { if !$0 { deleteTransactionTarget = nil } }
            ),
            titleVisibility: .visible
        ) {
            Button("Delete", role: .destructive) {
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
            Text("Its entries will stop affecting account balances.")
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
}

private struct TransactionRow: View {
    let transaction: BudgetTransaction
    let workspace: BudgetWorkspace
    let accounts: [BudgetAccount]

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text(transaction.payee ?? transaction.description ?? transaction.kind.title)
                    .font(.headline)
                Text(
                    "\(transaction.transactionDate) · \(transaction.kind.title) · \(transaction.status.title)"
                )
                .font(.caption)
                .foregroundStyle(.secondary)
                Text(accountNames)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if transaction.kind == .transfer {
                Text("Transfer")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            } else if let total {
                Text(workspace.baseCurrency.formatted(minorUnits: total))
                    .font(.subheadline.monospacedDigit())
            }
        }
        .contentShape(Rectangle())
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
            VStack(alignment: .leading, spacing: 5) {
                Text(workspace.baseCurrency.formatted(minorUnits: total))
                    .font(.headline.monospacedDigit())
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
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text(account.name)
                    .font(.headline)
                Text([account.type.title, account.currency.rawValue, account.institutionName]
                    .compactMap { $0 }
                    .joined(separator: " · "))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            Text(account.currency.formatted(minorUnits: account.balanceMinor))
                .font(.subheadline.monospacedDigit())
        }
        .contentShape(Rectangle())
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
                Section("Account") {
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
                }
                ResourceErrorSection(message: model.resourceErrorMessage)
            }
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
