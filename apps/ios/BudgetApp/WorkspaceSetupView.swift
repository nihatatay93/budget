import SwiftUI

struct WorkspaceSetupView: View {
    let workspace: BudgetWorkspace
    let session: UserSession
    @ObservedObject var model: AppModel
    let onSelectWorkspace: (String) -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var selectedTab = WorkspaceTab.overview
    @State private var restoredTabSelection = false
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
    @AppStorage("budget.textSizePreference") private var textSizePreference = BudgetTextSize.balanced.rawValue

    private var canManage: Bool { workspace.canManage }

    var body: some View {
        TabView(selection: $selectedTab) {
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
        .tint(BudgetTheme.forest)
        .transaction { transaction in
            if reduceMotion { transaction.animation = nil }
        }
        .onAppear {
            guard !restoredTabSelection else { return }
            selectedTab = WorkspaceTab.restored(from: selectedTabRawValue)
            restoredTabSelection = true
        }
        .onChange(of: selectedTab) { _, tab in
            selectedTabRawValue = tab.rawValue
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
            archiveTarget?.confirmationTitle ?? L10n.text("Archive item?"),
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
                    ?? L10n.text("Historical financial records will remain intact.")
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
                HStack(spacing: 10) {
                    ProgressView()
                        .tint(BudgetTheme.forest)
                    Text("Saving…")
                        .font(.footnote.weight(.medium))
                        .foregroundStyle(BudgetTheme.secondaryText)
                }
                .padding(.horizontal, 18)
                .padding(.vertical, 12)
                .background(.regularMaterial, in: Capsule())
                .overlay { Capsule().stroke(BudgetTheme.border, lineWidth: 1) }
                .padding(.bottom, 8)
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
            Text(model.resourceErrorMessage ?? L10n.text("The request could not be completed."))
        }
    }

    @ViewBuilder
    private func workspaceContent<Content: View>(
        @ViewBuilder content: () -> Content
    ) -> some View {
        if hasLoadedWorkspace {
            content()
        } else {
            VStack(spacing: 14) {
                ProgressView()
                    .tint(BudgetTheme.forest)
                Text("Loading \(workspace.name)…")
                    .font(.subheadline)
                    .foregroundStyle(BudgetTheme.secondaryText)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .budgetScreen()
        }
    }

    /// The filled glyph on the selected tab is the only place the tab bar carries weight.
    private func tabLabel(_ tab: WorkspaceTab) -> some View {
        Label(
            tab.title,
            systemImage: selectedTab == tab ? tab.selectedSystemImage : tab.systemImage
        )
    }

    private func selectTab(_ tab: WorkspaceTab) {
        selectedTab = tab
    }

    // MARK: - Transactions

    private var transactionsView: some View {
        List {
            if model.transactions.isEmpty {
                BudgetCard {
                    BudgetMessage(
                        title: "No transactions",
                        systemImage: "list.bullet.rectangle",
                        message: "Record an expense, income, transfer, or adjustment."
                    )
                }
                .budgetPlainRow(top: 8)
            } else if filteredTransactions.isEmpty {
                BudgetCard {
                    BudgetMessage(
                        title: "No matching transactions",
                        systemImage: "line.3.horizontal.decrease.circle",
                        message: "Try a different search or clear the current filters.",
                        action: ("Clear search and filters", clearTransactionFilters)
                    )
                }
                .budgetPlainRow(top: 8)
            } else {
                ForEach(groupedTransactions) { group in
                    Section {
                        ForEach(group.transactions) { transaction in
                            transactionListRow(transaction)
                        }
                    } header: {
                        TransactionDayHeader(
                            title: group.title,
                            total: group.total,
                            currency: workspace.baseCurrency
                        )
                    }
                }
            }
            viewerNotice
        }
        .listStyle(.insetGrouped)
        .budgetScreen()
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
        let categoryNames = Dictionary(uniqueKeysWithValues: model.categories.map {
            ($0.id, L10n.categoryName(
                name: $0.name,
                kind: $0.kind,
                predefinedKey: $0.predefinedKey,
                systemKey: $0.systemKey
            ))
        })
        return model.transactions.filter {
            transactionFilter.matches(
                $0,
                accountNames: accountNames,
                categoryNames: categoryNames
            )
        }
    }

    /// A ledger reads by day. Grouping also gives the running day total somewhere to live
    /// without adding another line to every row.
    private var groupedTransactions: [TransactionDayGroup] {
        var order: [String] = []
        var byDate: [String: [BudgetTransaction]] = [:]
        for transaction in filteredTransactions {
            if byDate[transaction.transactionDate] == nil {
                order.append(transaction.transactionDate)
            }
            byDate[transaction.transactionDate, default: []].append(transaction)
        }
        return order.map { date in
            let transactions = byDate[date] ?? []
            return TransactionDayGroup(
                id: date,
                title: budgetDisplayDate(date),
                transactions: transactions,
                total: transactions.reduce(into: Int64(0)) { total, transaction in
                    guard transaction.kind != .transfer else { return }
                    let sum = total.addingReportingOverflow(transactionTotal(transaction) ?? 0)
                    total = sum.overflow ? total : sum.partialValue
                }
            )
        }
    }

    @ViewBuilder
    private func transactionListRow(_ transaction: BudgetTransaction) -> some View {
        let row = TransactionRow(
            transaction: transaction,
            workspace: workspace,
            accounts: model.accounts,
            categories: model.categories
        )
        if canManage {
            Button {
                editingTransaction = transaction
                transactionEditorPresented = true
            } label: {
                row
            }
            .buttonStyle(.plain)
            .accessibilityHint("Opens the transaction editor")
            .budgetCardRow()
            .swipeActions {
                Button("Delete", role: .destructive) {
                    deleteTransactionTarget = transaction
                }
            }
        } else {
            row.budgetCardRow()
        }
    }

    private func clearTransactionFilters() {
        transactionSearchText = ""
        transactionStatusScope = .all
        transactionKindScope = .all
    }

    // MARK: - Accounts

    private var accountsView: some View {
        List {
            Section {
                BaseCurrencyTotalView(
                    workspace: workspace,
                    accounts: model.accounts,
                    rates: model.exchangeRates
                )
                .budgetPlainRow(top: 4, bottom: 8)
            }

            if !currencySummaries.isEmpty {
                Section {
                    ForEach(currencySummaries) { summary in
                        CurrencySummaryRow(summary: summary)
                            .budgetCardRow()
                    }
                } header: {
                    BudgetListHeader("Native-currency totals")
                }
            }

            if model.accounts.isEmpty {
                Section {
                    BudgetCard {
                        BudgetMessage(
                            title: "No accounts",
                            systemImage: "building.columns",
                            message: "Create an account to begin tracking money.",
                            action: canManage
                                ? ("Add account", {
                                    editingAccount = nil
                                    accountEditorPresented = true
                                })
                                : nil
                        )
                    }
                    .budgetPlainRow(bottom: 8)
                }
            }

            if !activeAccounts.isEmpty {
                Section {
                    ForEach(activeAccounts) { account in
                        accountListRow(account)
                    }
                } header: {
                    BudgetListHeader("Active accounts")
                } footer: {
                    BudgetListFooter("Account currency locks after financial history exists. Archiving keeps every historical entry and balance intact.")
                }
            }

            if !archivedAccounts.isEmpty {
                Section {
                    ForEach(archivedAccounts) { account in
                        AccountRow(account: account)
                            .opacity(0.6)
                            .budgetCardRow()
                    }
                } header: {
                    BudgetListHeader("Archived accounts")
                }
            }
            viewerNotice
        }
        .listStyle(.insetGrouped)
        .budgetScreen()
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
            .budgetCardRow()
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
            AccountRow(account: account).budgetCardRow()
        }
    }

    // MARK: - More

    private var moreView: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: BudgetTheme.Space.section) {
                WorkspaceIdentityCard(workspace: workspace, session: session)

                BudgetSection("Workspace") {
                    VStack(spacing: 0) {
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
                            BudgetNavigationRow(
                                title: "Switch workspace",
                                systemImage: "rectangle.3.group",
                                value: workspace.name
                            )
                        }
                        BudgetHairline(leading: BudgetTheme.Space.card)
                        BudgetDetailRow(title: "Base currency", value: workspace.baseCurrency.rawValue)
                        BudgetHairline(leading: BudgetTheme.Space.card)
                        BudgetDetailRow(title: "Timezone", value: workspace.timezone)
                        BudgetHairline(leading: BudgetTheme.Space.card)
                        BudgetDetailRow(title: "Your role", value: L10n.workspaceRole(workspace.role))
                    }
                    .budgetSurface()
                }

                BudgetSection("Explore and organize") {
                    VStack(spacing: 0) {
                        NavigationLink {
                            SpendingAnalysisView(workspace: workspace, model: model)
                        } label: {
                            BudgetNavigationRow(
                                title: "Analysis",
                                systemImage: "chart.xyaxis.line",
                                value: L10n.text("Spending over time")
                            )
                        }
                        .buttonStyle(.plain)
                        BudgetHairline(leading: BudgetTheme.Space.card + 46)
                        NavigationLink {
                            FinancialReportsView(workspace: workspace, model: model)
                        } label: {
                            BudgetNavigationRow(title: "Reports", systemImage: "chart.bar.xaxis")
                        }
                        .buttonStyle(.plain)
                        BudgetHairline(leading: BudgetTheme.Space.card + 46)
                        NavigationLink {
                            categoriesView
                        } label: {
                            BudgetNavigationRow(
                                title: "Categories",
                                systemImage: "tag",
                                value: "\(model.categories.count)"
                            )
                        }
                        .buttonStyle(.plain)
                        BudgetHairline(leading: BudgetTheme.Space.card + 46)
                        NavigationLink {
                            WorkspaceCollaborationView(
                                workspace: workspace,
                                currentUserID: session.user.id,
                                model: model
                            )
                        } label: {
                            BudgetNavigationRow(
                                title: "Members and invitations",
                                systemImage: "person.2"
                            )
                        }
                        .buttonStyle(.plain)
                        BudgetHairline(leading: BudgetTheme.Space.card + 46)
                        Button {
                            model.resourceErrorMessage = nil
                            invitationAcceptancePresented = true
                        } label: {
                            BudgetNavigationRow(
                                title: "Accept invitation",
                                systemImage: "envelope.open"
                            )
                        }
                        .buttonStyle(.plain)
                    }
                    .budgetSurface()
                }

                BudgetSection(.resolved(L10n.text("appearance.title"))) {
                    BudgetCard {
                        Picker(L10n.text("appearance.textSize"), selection: $textSizePreference) {
                            ForEach(BudgetTextSize.allCases) { size in
                                Text(size.title).tag(size.rawValue)
                            }
                        }
                        .pickerStyle(.segmented)
                    }
                }

                BudgetSection("Account") {
                    VStack(spacing: 0) {
                        BudgetDetailRow(title: "Signed in as", value: session.user.displayName)
                        BudgetHairline(leading: BudgetTheme.Space.card)
                        BudgetDetailRow(title: "Email", value: session.user.email)
                        BudgetHairline(leading: BudgetTheme.Space.card)
                        BudgetDetailRow(title: "Server", value: model.serverAddress)
                    }
                    .budgetSurface()
                }

                Button {
                    Task { await model.logout() }
                } label: {
                    Label("Sign out", systemImage: "rectangle.portrait.and.arrow.right")
                        .font(.subheadline.weight(.semibold))
                        .foregroundStyle(BudgetTheme.over)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 15)
                        .budgetSurface(radius: BudgetTheme.Radius.medium, showsShadow: false)
                }
                .buttonStyle(.plain)
                .disabled(model.isSubmitting)

                if !canManage { viewerNoticeLabel }
            }
            .padding(.horizontal, BudgetTheme.Space.screen)
            .padding(.top, 4)
            .padding(.bottom, 32)
        }
        .budgetScreen()
        .navigationTitle("More")
        .refreshable { await model.loadResources(workspaceID: workspace.id) }
    }

    // MARK: - Categories

    private var categoriesView: some View {
        List {
            if model.categories.isEmpty {
                Section {
                    BudgetCard {
                        BudgetMessage(
                            title: "No categories",
                            systemImage: "tag",
                            message: "Create categories to organize reporting and monthly plans."
                        )
                    }
                    .budgetPlainRow(top: 4, bottom: 8)
                }
            }
            // One list section per category group rather than one per kind. Rows stay rows here
            // rather than becoming tiles: this is where a category is edited or archived, and
            // swipe actions and the context menu are how that is done on iOS. The tile grid is
            // the picker's job, where choosing is the only thing on offer.
            ForEach(categorySectionsForDisplay) { section in
                Section {
                    ForEach(section.members) { member in
                        categoryListRow(CategoryTreeRow(category: member.category, depth: member.depth))
                    }
                } header: {
                    BudgetListHeader(.resolved(L10n.categoryName(
                        name: section.root.name,
                        kind: section.root.kind,
                        predefinedKey: section.root.predefinedKey,
                        systemKey: section.root.systemKey
                    )))
                } footer: {
                    if section.id == categorySectionsForDisplay.last?.id {
                        BudgetListFooter("Refunds may appear as positive allocations in expense categories. Protected Uncategorized categories cannot be archived.")
                    }
                }
            }
            viewerNotice
        }
        .listStyle(.insetGrouped)
        .budgetScreen()
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

    /// Expense groups first, then income, so the two kinds still read apart even though the
    /// heading now names a group rather than a kind.
    private var categorySectionsForDisplay: [CategoryHierarchy.Section] {
        let named: (BudgetCategory) -> String = { category in
            L10n.categoryName(
                name: category.name,
                kind: category.kind,
                predefinedKey: category.predefinedKey,
                systemKey: category.systemKey
            )
        }
        return BudgetCategoryKind.allCases.flatMap { kind in
            CategoryHierarchy.sections(of: model.categories.filter { $0.kind == kind }, by: named)
        }
    }

    @ViewBuilder
    private func categoryListRow(_ row: CategoryTreeRow) -> some View {
        let category = row.category
        if canManage && !category.isSystem {
            Button {
                editingCategory = category
                categoryEditorPresented = true
            } label: {
                CategoryRow(category: category, depth: row.depth, showsEditAffordance: true)
            }
            .buttonStyle(.plain)
            .accessibilityHint(L10n.text(category.predefinedKey == nil
                ? "category.editor.editHint"
                : "category.editor.editAppearanceHint"))
            .budgetCardRow()
            .swipeActions(edge: .leading) {
                Button(L10n.text("category.editor.edit"), systemImage: "pencil") {
                    editingCategory = category
                    categoryEditorPresented = true
                }
                .tint(BudgetTheme.forest)
            }
            .swipeActions {
                if category.predefinedKey == nil {
                    Button("Archive", role: .destructive) {
                        archiveTarget = .category(category)
                    }
                }
            }
            .contextMenu {
                Button(L10n.text("category.editor.edit"), systemImage: "pencil") {
                    editingCategory = category
                    categoryEditorPresented = true
                }
                if category.predefinedKey == nil {
                    Button("Archive category", systemImage: "archivebox", role: .destructive) {
                        archiveTarget = .category(category)
                    }
                }
            }
        } else {
            CategoryRow(category: category, depth: row.depth).budgetCardRow()
        }
    }

    @ViewBuilder
    private var viewerNotice: some View {
        if !canManage {
            viewerNoticeLabel
                .budgetPlainRow(top: 8, bottom: 8)
        }
    }

    private var viewerNoticeLabel: some View {
        Label("Viewer access is read-only.", systemImage: "eye")
            .font(.caption)
            .foregroundStyle(BudgetTheme.tertiaryText)
            .padding(.horizontal, 2)
    }
}

// MARK: - Transaction rows

private struct TransactionDayGroup: Identifiable {
    let id: String
    let title: String
    let transactions: [BudgetTransaction]
    let total: Int64
}

private struct TransactionDayHeader: View {
    let title: String
    let total: Int64
    let currency: BudgetCurrency

    var body: some View {
        HStack(alignment: .firstTextBaseline) {
            Text(title)
                .font(.footnote.weight(.semibold))
                .foregroundStyle(BudgetTheme.secondaryText)
            Spacer(minLength: 12)
            if total != 0 {
                Text(budgetSignedMoney(total, currency: currency))
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(BudgetTheme.tertiaryText)
            }
        }
        .textCase(nil)
        .padding(.bottom, 2)
        .accessibilityElement(children: .combine)
    }
}

/// The ledger row.
///
/// The amount lives in its own trailing column with layout priority, so a long payee truncates
/// instead of pushing the figure onto a second line — the failure the previous `ViewThatFits`
/// layout produced on almost every real transaction.
struct TransactionRow: View {
    let transaction: BudgetTransaction
    let workspace: BudgetWorkspace
    let accounts: [BudgetAccount]
    let categories: [BudgetCategory]

    var body: some View {
        HStack(spacing: 12) {
            avatar
            VStack(alignment: .leading, spacing: 3) {
                Text(title)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(BudgetTheme.primaryText)
                    .lineLimit(1)
                HStack(spacing: 6) {
                    Text(subtitle)
                        .lineLimit(1)
                    if transaction.status == .pending {
                        BudgetChip(text: "Pending", color: BudgetTheme.pending)
                    }
                }
                .font(.caption)
                .foregroundStyle(BudgetTheme.tertiaryText)
            }
            Spacer(minLength: 8)
            amount
                .layoutPriority(1)
        }
        .contentShape(Rectangle())
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilitySummary)
    }

    /// The category's own badge identifies the row when there is one; the kind glyph is the
    /// fallback, so uncategorized and transfer rows still read at a glance.
    @ViewBuilder
    private var avatar: some View {
        if let category = allocationCategories.first {
            CategoryAppearanceBadge(
                iconType: category.iconType,
                iconValue: category.iconValue,
                colorKey: category.colorKey,
                size: 38
            )
        } else {
            BudgetIconBadge(systemImage: kindIcon, color: kindColor)
        }
    }

    @ViewBuilder
    private var amount: some View {
        if transaction.kind == .transfer {
            Text("Transfer")
                .font(.footnote.weight(.semibold))
                .foregroundStyle(BudgetTheme.transfer)
        } else if let total {
            Text(workspace.baseCurrency.formatted(minorUnits: total))
                .font(.budgetAmount)
                .foregroundStyle(BudgetTheme.money(total))
                .minimumScaleFactor(0.7)
                .lineLimit(1)
        }
    }

    private var title: String {
        transaction.payee ?? transaction.description ?? categoryTitle ?? transaction.kind.title
    }

    private var categoryTitle: String? {
        allocationCategories.first.map {
            L10n.categoryName(
                name: $0.name,
                kind: $0.kind,
                predefinedKey: $0.predefinedKey,
                systemKey: $0.systemKey
            )
        }
    }

    /// One supporting line rather than three. The date already heads the day's group, so the
    /// line carries the category — or the account when the category is already the title.
    private var subtitle: String {
        var parts: [String] = []
        if transaction.payee != nil || transaction.description != nil, let categoryTitle {
            parts.append(categoryTitle)
        } else if !accountNames.isEmpty {
            parts.append(accountNames)
        }
        if parts.isEmpty {
            parts.append(transaction.kind.title)
        }
        if allocationCategories.count > 1 {
            parts.append("+\(allocationCategories.count - 1)")
        }
        return parts.joined(separator: " · ")
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
        case .standard: total ?? 0 >= 0 ? BudgetTheme.positive : BudgetTheme.spend
        case .transfer: BudgetTheme.transfer
        case .adjustment: BudgetTheme.adjustment
        }
    }

    private var accessibilitySummary: String {
        let amount = transaction.kind == .transfer
            ? "transfer"
            : total.map { workspace.baseCurrency.formatted(minorUnits: $0) } ?? L10n.text("amount unavailable")
        return "\(title), \(budgetDisplayDate(transaction.transactionDate)), \(transaction.kind.title), "
            + "\(transaction.status.title), \(amount), \(accountNames)"
            + (categoryNames.isEmpty ? "" : ", \(categoryNames)")
    }

    private var accountNames: String {
        let names = transaction.entries.map { entry in
            accounts.first { $0.id == entry.accountID }?.name ?? L10n.text("Unavailable account")
        }
        return names.joined(separator: " → ")
    }

    private var allocationCategories: [BudgetCategory] {
        let byID = Dictionary(uniqueKeysWithValues: categories.map { ($0.id, $0) })
        return transaction.allocations.compactMap { byID[$0.categoryID] }
    }

    private var categoryNames: String {
        allocationCategories.map {
            L10n.categoryName(
                name: $0.name,
                kind: $0.kind,
                predefinedKey: $0.predefinedKey,
                systemKey: $0.systemKey
            )
        }.joined(separator: ", ")
    }

    private var total: Int64? {
        transactionTotal(transaction)
    }
}

/// Shared so the day-group total and the row agree on what a transaction is worth.
func transactionTotal(_ transaction: BudgetTransaction) -> Int64? {
    var result: Int64 = 0
    for entry in transaction.entries {
        let sum = result.addingReportingOverflow(entry.baseAmountMinor)
        if sum.overflow { return nil }
        result = sum.partialValue
    }
    return result
}

// MARK: - Account rows

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
        BudgetCard(padding: 22) {
            if let total {
                VStack(alignment: .leading, spacing: 14) {
                    BudgetEyebrow("Posted across active \(workspace.baseCurrency.rawValue) accounts")
                    Text(workspace.baseCurrency.formatted(minorUnits: total))
                        .font(.budgetHero)
                        .foregroundStyle(BudgetTheme.primaryText)
                        .minimumScaleFactor(0.5)
                        .lineLimit(1)
                    if let selectedRate,
                       let converted = selectedRate.convert(minorUnits: total) {
                        Text(
                            "≈ \(displayCurrency.formatted(minorUnits: converted)) at the rate published "
                            + selectedRate.rateDate
                        )
                        .font(.caption)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                    }
                    if excludedAccountCount > 0 {
                        Text("Accounts in another currency are not included in this total.")
                            .font(.caption)
                            .foregroundStyle(BudgetTheme.tertiaryText)
                    }
                    if !rates.isEmpty {
                        BudgetHairline()
                        Picker("Show in", selection: $displayCurrency) {
                            Text(workspace.baseCurrency.rawValue).tag(workspace.baseCurrency)
                            ForEach(rates) { rate in
                                Text(rate.quoteCurrency.rawValue).tag(rate.quoteCurrency)
                            }
                        }
                        .pickerStyle(.segmented)
                    }
                }
                .accessibilityElement(children: .contain)
            } else {
                BudgetMessage(
                    title: "No active accounts use \(workspace.baseCurrency.rawValue)",
                    systemImage: "building.columns",
                    message: "Add an account in the workspace's base currency to see a position."
                )
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

private struct CurrencySummaryRow: View {
    let summary: AccountCurrencySummary

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            VStack(alignment: .leading, spacing: 3) {
                Text(summary.currency.title)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(BudgetTheme.primaryText)
                Text(
                    summary.accountCount == 1
                        ? "\(summary.accountCount) active account"
                        : "\(summary.accountCount) active accounts"
                )
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
            }
            Spacer(minLength: 12)
            Text(
                summary.balanceMinor.map {
                    summary.currency.formatted(minorUnits: $0)
                } ?? L10n.text("Total unavailable")
            )
            .font(.budgetAmount)
            .foregroundStyle(BudgetTheme.primaryText)
            .lineLimit(1)
            .minimumScaleFactor(0.7)
        }
        .accessibilityElement(children: .combine)
    }
}

private struct AccountRow: View {
    let account: BudgetAccount

    var body: some View {
        HStack(spacing: 12) {
            BudgetIconBadge(systemImage: systemImage, color: BudgetTheme.forest)
            VStack(alignment: .leading, spacing: 3) {
                Text(account.name)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(BudgetTheme.primaryText)
                    .lineLimit(1)
                Text([account.type.title, account.currency.rawValue, account.institutionName]
                    .compactMap { $0 }
                    .joined(separator: " · "))
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .lineLimit(1)
            }
            Spacer(minLength: 8)
            VStack(alignment: .trailing, spacing: 3) {
                Text(account.currency.formatted(minorUnits: account.balanceMinor))
                    .font(.budgetAmount)
                    .foregroundStyle(BudgetTheme.primaryText)
                    .minimumScaleFactor(0.7)
                    .lineLimit(1)
                if account.archivedAt != nil {
                    BudgetChip(text: "Archived", color: BudgetTheme.tertiaryText)
                }
            }
            .layoutPriority(1)
        }
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
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
    var showsEditAffordance = false

    var body: some View {
        HStack(spacing: 10) {
            CategoryNameLabel(
                name: category.name,
                kind: category.kind,
                predefinedKey: category.predefinedKey,
                systemKey: category.systemKey,
                iconType: category.iconType,
                iconValue: category.iconValue,
                colorKey: category.colorKey,
                iconSize: 32
            )
            .font(.subheadline.weight(.semibold))
            Spacer(minLength: 8)
            if category.isSystem {
                Image(systemName: "lock.fill")
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .accessibilityLabel("Protected category")
            } else if showsEditAffordance {
                Image(systemName: "chevron.right")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .accessibilityHidden(true)
            }
        }
        .padding(.leading, CGFloat(depth) * 16)
        .contentShape(Rectangle())
    }
}

// MARK: - Settings rows

/// The workspace's identity, given a card so the More tab opens with something other than a
/// wall of label/value pairs.
private struct WorkspaceIdentityCard: View {
    let workspace: BudgetWorkspace
    let session: UserSession

    var body: some View {
        BudgetCard(padding: 20) {
            HStack(spacing: 14) {
                ZStack {
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .fill(
                            LinearGradient(
                                colors: [BudgetTheme.forest, BudgetTheme.deepForest],
                                startPoint: .topLeading,
                                endPoint: .bottomTrailing
                            )
                        )
                    Text(initials)
                        .font(.system(size: 18, weight: .semibold))
                        .foregroundStyle(.white)
                }
                .frame(width: 48, height: 48)
                .accessibilityHidden(true)

                VStack(alignment: .leading, spacing: 3) {
                    Text(workspace.name)
                        .font(.headline)
                        .foregroundStyle(BudgetTheme.primaryText)
                    Text(session.user.displayName)
                        .font(.caption)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                }
                Spacer(minLength: 8)
                BudgetChip(text: .resolved(L10n.workspaceRole(workspace.role)), color: BudgetTheme.forest)
            }
            .accessibilityElement(children: .combine)
        }
    }

    private var initials: String {
        let letters = workspace.name
            .split(separator: " ")
            .prefix(2)
            .compactMap { $0.first }
        return letters.isEmpty ? "B" : String(letters).uppercased()
    }
}

private struct BudgetNavigationRow: View {
    let title: LocalizedStringKey
    let systemImage: String
    var value: String?

    var body: some View {
        HStack(spacing: 14) {
            Image(systemName: systemImage)
                .font(.system(size: 15, weight: .medium))
                .foregroundStyle(BudgetTheme.forest)
                .frame(width: 32, height: 32)
                .background(BudgetTheme.forest.opacity(0.12), in: RoundedRectangle(cornerRadius: 9, style: .continuous))
                .accessibilityHidden(true)
            Text(title)
                .font(.subheadline.weight(.medium))
                .foregroundStyle(BudgetTheme.primaryText)
            Spacer(minLength: 8)
            if let value {
                Text(value)
                    .font(.subheadline)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .lineLimit(1)
            }
            Image(systemName: "chevron.right")
                .font(.caption.weight(.semibold))
                .foregroundStyle(BudgetTheme.tertiaryText)
                .accessibilityHidden(true)
        }
        .padding(.horizontal, BudgetTheme.Space.card)
        .padding(.vertical, 13)
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
    }
}

private struct BudgetDetailRow: View {
    let title: LocalizedStringKey
    let value: String

    var body: some View {
        HStack(spacing: 12) {
            Text(title)
                .font(.subheadline)
                .foregroundStyle(BudgetTheme.secondaryText)
            Spacer(minLength: 12)
            Text(value)
                .font(.subheadline.weight(.medium))
                .foregroundStyle(BudgetTheme.primaryText)
                .lineLimit(1)
                .truncationMode(.middle)
        }
        .padding(.horizontal, BudgetTheme.Space.card)
        .padding(.vertical, 13)
        .accessibilityElement(children: .combine)
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
    @State private var iconType: BudgetCategoryIconType
    @State private var iconValue: String
    @State private var colorKey: BudgetCategoryColorKey

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
        let savedIconType = CategoryPresentation.iconType(
            iconType: category?.iconType,
            iconValue: category?.iconValue
        )
        _iconType = State(initialValue: savedIconType)
        if savedIconType == .emoji {
            _iconValue = State(initialValue: CategoryPresentation.normalizedEmoji(category?.iconValue ?? "") ?? "🍀")
        } else {
            let savedKey = category?.iconValue ?? ""
            _iconValue = State(initialValue: CategoryPresentation.isSupportedSystemIcon(savedKey)
                ? savedKey
                : CategoryPresentation.fallbackSystemIconKey)
        }
        _colorKey = State(initialValue: BudgetCategoryColorKey(rawValue: category?.colorKey ?? "") ?? .slate)
    }

    var body: some View {
        NavigationStack {
            Form {
                Section(L10n.text("category.editor.category")) {
                    if isBuiltIn {
                        LabeledContent(L10n.text("category.editor.builtIn"), value: previewName)
                        Text(L10n.text("category.editor.builtInHint"))
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                    } else {
                        TextField(L10n.text("category.editor.name"), text: $name)
                        Picker(L10n.text("category.editor.kind"), selection: $kind) {
                            ForEach(BudgetCategoryKind.allCases) { kind in
                                Text(kind.title).tag(kind)
                            }
                        }
                        .onChange(of: kind) { _, _ in parentID = "" }
                        Picker(L10n.text("category.editor.parent"), selection: $parentID) {
                            Text(L10n.text("category.editor.topLevel")).tag("")
                            ForEach(parentCandidates) { candidate in
                                CategoryNameLabel(
                                    name: candidate.name,
                                    kind: candidate.kind,
                                    predefinedKey: candidate.predefinedKey,
                                    systemKey: candidate.systemKey,
                                    iconType: candidate.iconType,
                                    iconValue: candidate.iconValue,
                                    colorKey: candidate.colorKey,
                                    iconSize: 22
                                )
                                .tag(candidate.id)
                            }
                        }
                    }
                }

                Section(L10n.text("category.appearance.title")) {
                    HStack(spacing: 12) {
                        CategoryAppearanceBadge(
                            iconType: iconType.rawValue,
                            iconValue: iconValue,
                            colorKey: colorKey.rawValue,
                            size: 42
                        )
                        Text(previewName)
                            .font(.headline)
                    }
                    .padding(.vertical, 4)
                    .accessibilityElement(children: .combine)
                    .accessibilityLabel("\(L10n.text("category.appearance.preview")): \(previewName)")

                    Picker(L10n.text("category.appearance.icon"), selection: $iconType) {
                        ForEach(BudgetCategoryIconType.allCases) { type in
                            Text(type.title).tag(type)
                        }
                    }
                    .pickerStyle(.segmented)
                    .onChange(of: iconType) { _, type in
                        switch type {
                        case .system where !CategoryPresentation.isSupportedSystemIcon(iconValue):
                            iconValue = CategoryPresentation.fallbackSystemIconKey
                        case .emoji where !CategoryPresentation.isSingleEmoji(iconValue):
                            iconValue = "🍀"
                        default:
                            break
                        }
                    }

                    if iconType == .system {
                        CategorySystemIconPicker(selectedKey: $iconValue, colorKey: colorKey)
                    } else {
                        CategoryEmojiPicker(emoji: $iconValue)
                    }

                    Text(L10n.text("category.appearance.color"))
                        .font(.subheadline.weight(.semibold))
                    CategoryColorPicker(selectedKey: $colorKey)
                }
                ResourceErrorSection(message: model.resourceErrorMessage)
            }
            .scrollDismissesKeyboard(.interactively)
            .navigationTitle(category == nil
                ? L10n.text("category.editor.add")
                : L10n.text("category.editor.edit"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(L10n.text("category.editor.cancel")) { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button(L10n.text("category.editor.save")) {
                        Task {
                            let saved = await model.saveCategory(
                                workspaceID: workspace.id,
                                categoryID: category?.id,
                                input: categoryInput
                            )
                            if saved { dismiss() }
                        }
                    }
                    .disabled(!isValid || model.isSavingResource)
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

    private var isValid: Bool {
        (isBuiltIn || !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            && (iconType != .emoji || CategoryPresentation.isSingleEmoji(iconValue))
    }

    private var isBuiltIn: Bool { category?.predefinedKey != nil }

    private var categoryInput: CategoryInput {
        CategoryInput(
            name: isBuiltIn ? category?.name ?? name : name.trimmingCharacters(in: .whitespacesAndNewlines),
            kind: isBuiltIn ? category?.kind ?? kind : kind,
            parentID: isBuiltIn ? category?.parentID : parentID.nilIfBlank,
            iconType: iconType,
            iconValue: iconType == .emoji ? CategoryPresentation.normalizedEmoji(iconValue) : iconValue,
            colorKey: colorKey
        )
    }

    private var previewName: String {
        if let category, isBuiltIn {
            return L10n.categoryName(
                name: category.name,
                kind: category.kind,
                predefinedKey: category.predefinedKey,
                systemKey: category.systemKey
            )
        }
        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? L10n.text("category.appearance.namePreview") : trimmed
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
        case let .account(account): String(format: L10n.text("Archive %@?"), account.name)
        case let .category(category): String(format: L10n.text("Archive %@?"), category.name)
        }
    }

    var confirmationMessage: String {
        switch self {
        case .account:
            L10n.text("The account leaves active setup but its entries and derived balance remain in financial history.")
        case .category:
            L10n.text("The category leaves active organization but historical allocations remain in reports. Categories with active children must be reorganized first.")
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
