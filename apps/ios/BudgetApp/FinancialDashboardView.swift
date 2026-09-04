import SwiftUI

struct WorkspaceOverviewView: View {
    let workspace: BudgetWorkspace
    @ObservedObject var model: AppModel
    let onAddTransaction: () -> Void
    let onOpenTransactions: () -> Void
    let onOpenBudget: () -> Void

    private var currentBudgetMonth: String {
        workspaceMonthKey(timezone: workspace.timezone)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: BudgetTheme.Space.section) {
                if let projection = model.financialProjection {
                    OverviewBalanceCard(projection: projection)

                    BudgetSection("This period") {
                        OverviewActivityCard(projection: projection)
                    }

                    BudgetSection("Monthly plan", action: ("Open Budget", onOpenBudget)) {
                        Button(action: onOpenBudget) {
                            monthlyPlanCard
                        }
                        .buttonStyle(.plain)
                        .accessibilityHint("Opens the Budget tab")
                    }

                    BudgetSection(
                        "Recent activity",
                        action: recentTransactions.isEmpty ? nil : ("See all", onOpenTransactions)
                    ) {
                        recentActivityCard
                    }
                } else if let message = model.resourceErrorMessage {
                    BudgetCard {
                        BudgetMessage(
                            title: "Overview unavailable",
                            systemImage: "exclamationmark.triangle",
                            message: .resolved(message),
                            action: ("Try again", reload)
                        )
                    }
                } else {
                    BudgetCard {
                        BudgetLoading("Loading overview…")
                    }
                }
            }
            .padding(.horizontal, BudgetTheme.Space.screen)
            .padding(.top, 4)
            .padding(.bottom, 32)
        }
        .budgetScreen()
        .navigationTitle("Overview")
        .toolbar {
            if workspace.canManage {
                ToolbarItem(placement: .primaryAction) {
                    Button("Add transaction", systemImage: "plus") {
                        onAddTransaction()
                    }
                }
            }
        }
        .task(id: workspace.id) {
            await model.loadMonthlyBudget(
                workspaceID: workspace.id,
                month: currentBudgetMonth
            )
        }
        .refreshable { await refresh() }
    }

    @ViewBuilder
    private var monthlyPlanCard: some View {
        if let budget = model.monthlyBudget, budget.month == currentBudgetMonth {
            BudgetCard {
                BudgetPlanSummary(plan: budget)
            }
        } else if model.isLoadingBudget {
            BudgetCard {
                BudgetLoading("Loading current budget…")
            }
        } else {
            BudgetCard {
                BudgetMessage(
                    title: "No plan for this month",
                    systemImage: "chart.pie",
                    message: "Set a spending plan to track how the month is going."
                )
            }
        }
    }

    @ViewBuilder
    private var recentActivityCard: some View {
        if recentTransactions.isEmpty {
            BudgetCard {
                BudgetMessage(
                    title: "No transactions yet",
                    systemImage: "list.bullet.rectangle",
                    message: "Add the first entry to begin building your ledger."
                )
            }
        } else {
            BudgetRowGroup(items: recentTransactions, hairlineInset: 50) { transaction in
                TransactionRow(
                    transaction: transaction,
                    workspace: workspace,
                    accounts: model.accounts,
                    categories: model.categories
                )
            }
        }
    }

    private var recentTransactions: [BudgetTransaction] {
        Array(model.transactions.prefix(5))
    }

    private func reload() {
        Task { await refresh() }
    }

    private func refresh() async {
        await model.loadResources(workspaceID: workspace.id)
        await model.loadMonthlyBudget(
            workspaceID: workspace.id,
            month: currentBudgetMonth
        )
    }
}

/// The one place on the screen that carries the brand gradient. Everything else stays quiet so
/// this reads as the headline figure.
private struct OverviewBalanceCard: View {
    let projection: BudgetFinancialProjection

    private var currency: BudgetCurrency { projection.period.baseCurrency }
    private var amounts: BudgetProjectionAmounts { projection.summary.balanceBaseMinor }

    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 6) {
                    BudgetEyebrow("Posted balance", color: .white.opacity(0.55))
                    Text(currency.formatted(minorUnits: amounts.posted))
                        .font(.budgetHero)
                        .foregroundStyle(.white)
                        .minimumScaleFactor(0.5)
                        .lineLimit(1)
                }
                Spacer(minLength: 12)
                Text(currency.rawValue)
                    .font(.system(size: 11, weight: .bold))
                    .tracking(0.6)
                    .foregroundStyle(.white.opacity(0.85))
                    .padding(.horizontal, 9)
                    .padding(.vertical, 5)
                    .background(.white.opacity(0.13), in: Capsule())
                    .overlay { Capsule().stroke(.white.opacity(0.14), lineWidth: 1) }
            }

            Text(
                "\(budgetDisplayDate(projection.period.fromDate)) – "
                + budgetDisplayDate(projection.period.toDate)
            )
            .font(.caption)
            .foregroundStyle(.white.opacity(0.6))

            Rectangle()
                .fill(.white.opacity(0.12))
                .frame(height: 1)

            HStack(spacing: 12) {
                heroStat(
                    "Pending delta",
                    budgetSignedMoney(amounts.pending, currency: currency)
                )
                Rectangle()
                    .fill(.white.opacity(0.12))
                    .frame(width: 1, height: 30)
                heroStat(
                    "Projected",
                    currency.formatted(minorUnits: amounts.projected)
                )
            }
        }
        .padding(22)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(
                colors: [
                    BudgetTheme.forest.opacity(0.85),
                    BudgetTheme.deepForest
                ],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            ),
            in: RoundedRectangle(cornerRadius: BudgetTheme.Radius.large, style: .continuous)
        )
        .overlay {
            RoundedRectangle(cornerRadius: BudgetTheme.Radius.large, style: .continuous)
                .stroke(.white.opacity(0.1), lineWidth: 1)
        }
        .shadow(color: BudgetTheme.deepForest.opacity(0.5), radius: 22, y: 12)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(
            "Posted balance \(currency.formatted(minorUnits: amounts.posted)), "
            + "pending delta \(currency.formatted(minorUnits: amounts.pending)), "
            + "projected \(currency.formatted(minorUnits: amounts.projected))"
        )
    }

    private func heroStat(_ title: LocalizedStringKey, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            BudgetEyebrow(title, color: .white.opacity(0.5))
            Text(value)
                .font(.budgetAmountSmall)
                .foregroundStyle(.white.opacity(0.92))
                .minimumScaleFactor(0.7)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

private struct OverviewActivityCard: View {
    let projection: BudgetFinancialProjection

    private var comparison: FinancialActivityComparison {
        FinancialActivityComparison(
            income: projection.summary.incomeBaseMinor.posted,
            spending: projection.summary.spendingBaseMinor.posted
        )
    }

    var body: some View {
        BudgetCard {
            VStack(spacing: 18) {
                OverviewActivityLine(
                    title: "Income",
                    systemImage: "arrow.down.left",
                    color: BudgetTheme.positive,
                    amount: projection.summary.incomeBaseMinor.posted,
                    pending: projection.summary.incomeBaseMinor.pending,
                    progress: comparison.incomeProgress,
                    currency: projection.period.baseCurrency
                )
                BudgetHairline()
                OverviewActivityLine(
                    title: "Spending",
                    systemImage: "arrow.up.right",
                    color: BudgetTheme.spend,
                    amount: projection.summary.spendingBaseMinor.posted,
                    pending: projection.summary.spendingBaseMinor.pending,
                    progress: comparison.spendingProgress,
                    currency: projection.period.baseCurrency
                )
            }
        }
    }
}

private struct OverviewActivityLine: View {
    let title: LocalizedStringKey
    let systemImage: String
    let color: Color
    let amount: Int64
    let pending: Int64
    let progress: Double
    let currency: BudgetCurrency

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 12) {
                BudgetIconBadge(systemImage: systemImage, color: color, size: 34)
                Text(title)
                    .font(.subheadline.weight(.medium))
                    .foregroundStyle(BudgetTheme.primaryText)
                Spacer(minLength: 12)
                Text(currency.formatted(minorUnits: amount))
                    .font(.budgetAmount)
                    .foregroundStyle(BudgetTheme.primaryText)
                    .minimumScaleFactor(0.7)
                    .lineLimit(1)
            }
            BudgetMeter(progress: progress, tint: color)
            if pending != 0 {
                Text("\(budgetSignedMoney(pending, currency: currency)) pending")
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.pending)
            }
        }
        .accessibilityElement(children: .combine)
    }
}

struct FinancialActivityComparison: Equatable {
    let incomeProgress: Double
    let spendingProgress: Double

    init(income: Int64, spending: Int64) {
        let maximum = max(income.magnitude, spending.magnitude)
        guard maximum > 0 else {
            incomeProgress = 0
            spendingProgress = 0
            return
        }
        incomeProgress = Double(income.magnitude) / Double(maximum)
        spendingProgress = Double(spending.magnitude) / Double(maximum)
    }
}

// MARK: - Reports

struct FinancialReportsView: View {
    let workspace: BudgetWorkspace
    @ObservedObject var model: AppModel

    @State private var usesCustomRange = false
    @State private var fromDate = Calendar.current.date(
        from: Calendar.current.dateComponents([.year, .month], from: Date())
    ) ?? Date()
    @State private var toDate = Date()
    @State private var rangeError: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: BudgetTheme.Space.section) {
                rangeCard

                if let projection = model.financialProjection {
                    BudgetSection(
                        "Posted overview",
                        caption: "Posted figures are authoritative. Projected totals add pending activity; they are not a forecast."
                    ) {
                        VStack(spacing: 10) {
                            ProjectionSummaryCard(
                                title: "Balance",
                                pendingTitle: "Pending delta",
                                amounts: projection.summary.balanceBaseMinor,
                                accent: BudgetTheme.forest,
                                currency: projection.period.baseCurrency
                            )
                            ProjectionSummaryCard(
                                title: "Income",
                                pendingTitle: "Pending income",
                                amounts: projection.summary.incomeBaseMinor,
                                accent: BudgetTheme.positive,
                                currency: projection.period.baseCurrency
                            )
                            ProjectionSummaryCard(
                                title: "Spending",
                                pendingTitle: "Pending spending",
                                amounts: projection.summary.spendingBaseMinor,
                                accent: BudgetTheme.spend,
                                currency: projection.period.baseCurrency
                            )
                        }
                    }

                    BudgetSection("Current monthly budget") {
                        currentBudgetCard
                    }

                    BudgetSection("Account balances") {
                        if projection.accounts.isEmpty {
                            BudgetCard {
                                BudgetMessage(
                                    title: "No accounts",
                                    systemImage: "building.columns",
                                    message: "Create an account to begin tracking money."
                                )
                            }
                        } else {
                            BudgetRowGroup(items: projection.accounts) { account in
                                ProjectionAccountRow(
                                    account: account,
                                    baseCurrency: projection.period.baseCurrency
                                )
                            }
                        }
                    }

                    ProjectionCategorySection(
                        title: "Spending by category",
                        kind: .expense,
                        categories: projection.categories,
                        currency: projection.period.baseCurrency
                    )
                    ProjectionCategorySection(
                        title: "Income by category",
                        kind: .income,
                        categories: projection.categories,
                        currency: projection.period.baseCurrency
                    )
                } else if !model.isLoadingResources && !model.isLoadingProjection {
                    BudgetCard {
                        BudgetMessage(
                            title: "Reports unavailable",
                            systemImage: "chart.bar.xaxis",
                            message: .resolved(model.resourceErrorMessage ?? L10n.text("Pull to refresh and try again."))
                        )
                    }
                }
            }
            .padding(.horizontal, BudgetTheme.Space.screen)
            .padding(.top, 8)
            .padding(.bottom, 32)
        }
        .budgetScreen()
        .navigationTitle("Reports")
        .navigationBarTitleDisplayMode(.inline)
        .task(id: workspace.id) {
            usesCustomRange = false
            rangeError = nil
            async let projection: Void = model.loadFinancialProjection(
                workspaceID: workspace.id,
                range: nil
            )
            async let budget: Void = model.loadMonthlyBudget(
                workspaceID: workspace.id,
                month: currentBudgetMonth
            )
            _ = await (projection, budget)
        }
        .overlay {
            if model.isLoadingProjection {
                BudgetLoading("Updating reports…")
                    .padding(20)
                    .budgetSurface(radius: BudgetTheme.Radius.small)
            }
        }
        .onChange(of: usesCustomRange) { _, custom in
            guard !custom else { return }
            rangeError = nil
            Task {
                await model.loadFinancialProjection(workspaceID: workspace.id, range: nil)
            }
        }
        .refreshable {
            async let projection: Void = model.loadFinancialProjection(
                workspaceID: workspace.id,
                range: selectedRange
            )
            async let budget: Void = model.loadMonthlyBudget(
                workspaceID: workspace.id,
                month: currentBudgetMonth
            )
            _ = await (projection, budget)
        }
    }

    private var rangeCard: some View {
        BudgetCard {
            VStack(alignment: .leading, spacing: 14) {
                Toggle("Custom date range", isOn: $usesCustomRange)
                    .font(.subheadline.weight(.medium))
                    .tint(BudgetTheme.forest)
                if usesCustomRange {
                    BudgetHairline()
                    DatePicker("From", selection: $fromDate, displayedComponents: .date)
                        .font(.subheadline)
                    DatePicker("To", selection: $toDate, displayedComponents: .date)
                        .font(.subheadline)
                    Button("Apply range", action: applyRange)
                        .buttonStyle(BudgetPrimaryButtonStyle())
                }
                if let rangeError {
                    Label(rangeError, systemImage: "exclamationmark.triangle.fill")
                        .font(.footnote)
                        .foregroundStyle(BudgetTheme.over)
                }
                if let projection = model.financialProjection {
                    Text(
                        "\(budgetDisplayDate(projection.period.fromDate)) – "
                        + "\(budgetDisplayDate(projection.period.toDate)) · \(projection.period.timezone)"
                    )
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .accessibilityLabel(
                        "Reporting period from \(projection.period.fromDate) through "
                        + "\(projection.period.toDate), \(projection.period.timezone)"
                    )
                }
            }
        }
    }

    @ViewBuilder
    private var currentBudgetCard: some View {
        NavigationLink {
            MonthlyBudgetView(workspace: workspace, model: model)
        } label: {
            BudgetCard {
                if let budget = model.monthlyBudget, budget.month == currentBudgetMonth {
                    BudgetPlanSummary(plan: budget)
                } else if model.isLoadingBudget {
                    BudgetLoading("Loading current budget…")
                } else if model.budgetErrorMessage != nil {
                    BudgetMessage(
                        title: "Monthly budget unavailable",
                        systemImage: "chart.pie",
                        message: "Open the Budget tab to retry."
                    )
                } else {
                    BudgetMessage(
                        title: "No plan yet",
                        systemImage: "chart.pie",
                        message: "Create or review this month's plan."
                    )
                }
            }
        }
        .buttonStyle(.plain)
    }

    private var selectedRange: BudgetProjectionRange? {
        guard usesCustomRange else { return nil }
        return BudgetProjectionRange(fromDate: dateOnly(fromDate), toDate: dateOnly(toDate))
    }

    private var currentBudgetMonth: String {
        workspaceMonthKey(timezone: workspace.timezone)
    }

    private func applyRange() {
        guard fromDate <= toDate else {
            rangeError = L10n.text("The start date must be on or before the end date.")
            return
        }
        rangeError = nil
        Task {
            await model.loadFinancialProjection(workspaceID: workspace.id, range: selectedRange)
        }
    }
}

/// The plan headline shared by Overview, Reports, and the Budget tab, so one plan never renders
/// three different ways.
struct BudgetPlanSummary: View {
    let plan: MonthlyBudgetPlan

    private var usage: BudgetUsagePresentation {
        BudgetUsagePresentation(
            planned: plan.plannedBaseMinor,
            used: plan.usedBaseMinor,
            remaining: plan.remainingBaseMinor
        )
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .firstTextBaseline) {
                Text(plan.name)
                    .font(.headline)
                    .foregroundStyle(BudgetTheme.primaryText)
                Spacer(minLength: 12)
                BudgetUsageStateLabel(state: usage.state)
            }
            BudgetMeter(progress: usage.progress, tint: budgetUsageColor(usage.state))
            HStack(spacing: 12) {
                BudgetStat(
                    title: "Planned",
                    value: plan.baseCurrency.formatted(minorUnits: plan.plannedBaseMinor)
                )
                BudgetStat(
                    title: "Used",
                    value: plan.baseCurrency.formatted(minorUnits: plan.usedBaseMinor)
                )
                BudgetStat(
                    title: "Remaining",
                    value: plan.baseCurrency.formatted(minorUnits: plan.remainingBaseMinor),
                    valueColor: plan.remainingBaseMinor < 0
                        ? BudgetTheme.over
                        : BudgetTheme.primaryText
                )
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(
            "\(plan.name), \(plan.baseCurrency.formatted(minorUnits: plan.usedBaseMinor)) used, "
            + "\(plan.baseCurrency.formatted(minorUnits: plan.remainingBaseMinor)) remaining, "
            + "of \(plan.baseCurrency.formatted(minorUnits: plan.plannedBaseMinor)) planned"
        )
    }
}

private struct ProjectionSummaryCard: View {
    let title: LocalizedStringKey
    let pendingTitle: LocalizedStringKey
    let amounts: BudgetProjectionAmounts
    let accent: Color
    let currency: BudgetCurrency

    var body: some View {
        BudgetCard {
            VStack(alignment: .leading, spacing: 12) {
                HStack(alignment: .firstTextBaseline) {
                    BudgetEyebrow(title, color: accent)
                    Spacer(minLength: 12)
                    Text(currency.formatted(minorUnits: amounts.posted))
                        .font(.budgetAmountLarge)
                        .foregroundStyle(BudgetTheme.primaryText)
                        .minimumScaleFactor(0.6)
                        .lineLimit(1)
                }
                BudgetHairline()
                HStack(spacing: 12) {
                    BudgetStat(
                        title: pendingTitle,
                        value: budgetSignedMoney(amounts.pending, currency: currency)
                    )
                    BudgetStat(
                        title: "Projected",
                        value: currency.formatted(minorUnits: amounts.projected),
                        alignment: .trailing
                    )
                }
            }
        }
        .accessibilityElement(children: .combine)
    }
}

private struct ProjectionAccountRow: View {
    let account: BudgetProjectionAccount
    let baseCurrency: BudgetCurrency

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            VStack(alignment: .leading, spacing: 3) {
                Text(account.name)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(BudgetTheme.primaryText)
                Text(account.type.title + (account.archivedAt == nil ? "" : " · Archived"))
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
            }
            Spacer(minLength: 12)
            VStack(alignment: .trailing, spacing: 3) {
                Text(account.currency.formatted(minorUnits: account.nativeBalanceMinor.posted))
                    .font(.budgetAmount)
                    .foregroundStyle(BudgetTheme.primaryText)
                    .lineLimit(1)
                    .minimumScaleFactor(0.7)
                if account.nativeBalanceMinor.pending != 0 {
                    Text(
                        "\(budgetSignedMoney(account.nativeBalanceMinor.pending, currency: account.currency)) pending"
                    )
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.pending)
                }
                if account.currency != baseCurrency {
                    Text("\(baseCurrency.formatted(minorUnits: account.baseBalanceMinor.posted)) base")
                        .font(.caption)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                }
            }
        }
        .accessibilityElement(children: .combine)
    }
}

private struct ProjectionCategorySection: View {
    let title: LocalizedStringKey
    let kind: BudgetCategoryKind
    let categories: [BudgetProjectionCategory]
    let currency: BudgetCurrency

    var body: some View {
        BudgetSection(title) {
            if visibleCategories.isEmpty {
                BudgetCard {
                    Text("No net activity in this period.")
                        .font(.subheadline)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                }
            } else {
                BudgetRowGroup(items: visibleCategories, hairlineInset: 40) { category in
                    row(category)
                }
            }
        }
    }

    private func row(_ category: BudgetProjectionCategory) -> some View {
        HStack(alignment: .center, spacing: 12) {
            VStack(alignment: .leading, spacing: 3) {
                CategoryNameLabel(
                    name: category.name,
                    kind: category.kind,
                    predefinedKey: category.predefinedKey,
                    systemKey: nil,
                    iconType: category.iconType,
                    iconValue: category.iconValue,
                    colorKey: category.colorKey
                )
                .font(.subheadline.weight(.semibold))
                Text(category.archivedAt == nil ? rollupLabel(category) : "\(rollupLabel(category)) · Archived")
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .padding(.leading, 36)
            }
            .padding(.leading, CGFloat(categoryDepth(category, in: categories)) * 12)
            Spacer(minLength: 12)
            VStack(alignment: .trailing, spacing: 3) {
                Text(currency.formatted(minorUnits: category.rolledUpBaseMinor.posted))
                    .font(.budgetAmount)
                    .foregroundStyle(BudgetTheme.primaryText)
                    .lineLimit(1)
                    .minimumScaleFactor(0.7)
                if category.rolledUpBaseMinor.pending != 0 {
                    Text("\(budgetSignedMoney(category.rolledUpBaseMinor.pending, currency: currency)) pending")
                        .font(.caption)
                        .foregroundStyle(BudgetTheme.pending)
                }
            }
        }
        .accessibilityElement(children: .combine)
    }

    private var visibleCategories: [BudgetProjectionCategory] {
        categories.filter {
            $0.kind == kind && ($0.rolledUpBaseMinor.posted != 0 || $0.rolledUpBaseMinor.pending != 0)
        }
    }

    private func rollupLabel(_ category: BudgetProjectionCategory) -> String {
        categories.contains { $0.parentID == category.id } ? L10n.text("Includes subcategories") : L10n.text("Category total")
    }
}

// MARK: - Shared states

/// Replaces `ContentUnavailableView` inside cards, which brought its own centered layout and
/// spacing and never matched the surrounding rhythm.
struct BudgetMessage: View {
    let title: LocalizedStringKey
    let systemImage: String
    var message: LocalizedStringKey?
    var action: (title: LocalizedStringKey, handler: () -> Void)?

    var body: some View {
        HStack(alignment: .top, spacing: 14) {
            BudgetIconBadge(systemImage: systemImage, color: BudgetTheme.secondaryText, size: 36)
            VStack(alignment: .leading, spacing: 5) {
                Text(title)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(BudgetTheme.primaryText)
                if let message {
                    Text(message)
                        .font(.caption)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                        .fixedSize(horizontal: false, vertical: true)
                }
                if let action {
                    Button(action.title, action: action.handler)
                        .font(.footnote.weight(.semibold))
                        .foregroundStyle(BudgetTheme.forest)
                        .padding(.top, 4)
                }
            }
            Spacer(minLength: 0)
        }
        .accessibilityElement(children: .contain)
    }
}

struct BudgetLoading: View {
    let title: LocalizedStringKey

    init(_ title: LocalizedStringKey) {
        self.title = title
    }

    var body: some View {
        HStack(spacing: 12) {
            ProgressView()
                .tint(BudgetTheme.forest)
            Text(title)
                .font(.subheadline)
                .foregroundStyle(BudgetTheme.secondaryText)
            Spacer(minLength: 0)
        }
    }
}

struct BudgetUsageStateLabel: View {
    let state: BudgetUsageState

    var body: some View {
        BudgetChip(text: title, systemImage: systemImage, color: budgetUsageColor(state))
    }

    private var title: LocalizedStringKey {
        switch state {
        case .noTarget: "No target"
        case .onTrack: "On track"
        case .overspent: "Over plan"
        case .refundCredit: "Refund credit"
        }
    }

    private var systemImage: String {
        switch state {
        case .noTarget: "minus"
        case .onTrack: "checkmark"
        case .overspent: "exclamationmark"
        case .refundCredit: "arrow.uturn.backward"
        }
    }
}

func budgetUsageColor(_ state: BudgetUsageState) -> Color {
    switch state {
    case .noTarget: BudgetTheme.secondaryText
    case .onTrack: BudgetTheme.positive
    case .overspent: BudgetTheme.over
    case .refundCredit: BudgetTheme.sage
    }
}

/// The one filled button in the app.
struct BudgetPrimaryButtonStyle: ButtonStyle {
    var isEnabled = true

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.subheadline.weight(.semibold))
            .foregroundStyle(.white)
            .frame(maxWidth: .infinity)
            .padding(.vertical, 14)
            .background(
                isEnabled ? BudgetTheme.forest : BudgetTheme.forest.opacity(0.3),
                in: RoundedRectangle(cornerRadius: BudgetTheme.Radius.small, style: .continuous)
            )
            .opacity(configuration.isPressed ? 0.82 : 1)
    }
}

private func dateOnly(_ date: Date) -> String {
    let components = Calendar.current.dateComponents([.year, .month, .day], from: date)
    return String(
        format: "%04d-%02d-%02d",
        components.year ?? 0,
        components.month ?? 0,
        components.day ?? 0
    )
}

private func categoryDepth(
    _ category: BudgetProjectionCategory,
    in categories: [BudgetProjectionCategory]
) -> Int {
    let byID = Dictionary(uniqueKeysWithValues: categories.map { ($0.id, $0) })
    var visited = Set([category.id])
    var parentID = category.parentID
    var depth = 0
    while let currentParentID = parentID,
          !visited.contains(currentParentID),
          depth < categories.count {
        visited.insert(currentParentID)
        depth += 1
        parentID = byID[currentParentID]?.parentID
    }
    return depth
}
