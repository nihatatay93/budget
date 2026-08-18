import SwiftUI

struct FinancialDashboardView: View {
    let workspace: BudgetWorkspace
    @ObservedObject var model: AppModel

    @State private var usesCustomRange = false
    @State private var fromDate = Calendar.current.date(
        from: Calendar.current.dateComponents([.year, .month], from: Date())
    ) ?? Date()
    @State private var toDate = Date()
    @State private var rangeError: String?

    var body: some View {
        List {
            Section {
                Toggle("Custom date range", isOn: $usesCustomRange)
                if usesCustomRange {
                    DatePicker("From", selection: $fromDate, displayedComponents: .date)
                    DatePicker("To", selection: $toDate, displayedComponents: .date)
                    Button("Apply range") { applyRange() }
                }
                if let rangeError {
                    Label(rangeError, systemImage: "exclamationmark.triangle.fill")
                        .foregroundStyle(.red)
                }
            } footer: {
                Text("Dates are inclusive and interpreted in \(workspace.timezone).")
            }

            if let projection = model.financialProjection {
                Section {
                    Text(
                        "\(displayDate(projection.period.fromDate))–"
                        + "\(displayDate(projection.period.toDate)) · \(projection.period.timezone)"
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .accessibilityLabel(
                        "Reporting period from \(projection.period.fromDate) through "
                        + "\(projection.period.toDate), \(projection.period.timezone)"
                    )

                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 145), spacing: 10)], spacing: 10) {
                        ProjectionSummaryCard(
                            title: "Balance",
                            pendingTitle: "Pending delta",
                            amounts: projection.summary.balanceBaseMinor,
                            currency: projection.period.baseCurrency
                        )
                        ProjectionSummaryCard(
                            title: "Income",
                            pendingTitle: "Pending income",
                            amounts: projection.summary.incomeBaseMinor,
                            currency: projection.period.baseCurrency
                        )
                        ProjectionSummaryCard(
                            title: "Spending",
                            pendingTitle: "Pending spending",
                            amounts: projection.summary.spendingBaseMinor,
                            currency: projection.period.baseCurrency
                        )
                    }
                    .padding(.vertical, 4)
                } header: {
                    Text("Posted overview")
                } footer: {
                    Text("Posted figures are authoritative. Projected totals add pending activity; they are not a forecast.")
                }

                Section("Current monthly budget") {
                    if let budget = model.monthlyBudget, budget.month == currentBudgetMonth {
                        NavigationLink {
                            MonthlyBudgetView(workspace: workspace, model: model)
                        } label: {
                            DashboardBudgetProgress(plan: budget)
                        }
                    } else if model.isLoadingBudget {
                        ProgressView("Loading current budget…")
                    } else if model.budgetErrorMessage != nil {
                        NavigationLink {
                            MonthlyBudgetView(workspace: workspace, model: model)
                        } label: {
                            Label("Monthly budget unavailable", systemImage: "chart.pie")
                                .foregroundStyle(.secondary)
                        }
                    } else {
                        NavigationLink {
                            MonthlyBudgetView(workspace: workspace, model: model)
                        } label: {
                            Label("No plan yet · Create or review", systemImage: "chart.pie")
                                .foregroundStyle(.secondary)
                        }
                    }
                }

                Section("Account balances") {
                    if projection.accounts.isEmpty {
                        ContentUnavailableView("No accounts", systemImage: "building.columns")
                    }
                    ForEach(projection.accounts) { account in
                        ProjectionAccountRow(
                            account: account,
                            baseCurrency: projection.period.baseCurrency
                        )
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
                ContentUnavailableView(
                    "Overview unavailable",
                    systemImage: "chart.bar.xaxis",
                    description: Text("Pull to refresh and try again.")
                )
            }
        }
        .navigationTitle("Financial overview")
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
                ProgressView("Updating overview…")
                    .padding()
                    .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 14))
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

    private var selectedRange: BudgetProjectionRange? {
        guard usesCustomRange else { return nil }
        return BudgetProjectionRange(fromDate: dateOnly(fromDate), toDate: dateOnly(toDate))
    }

    private var currentBudgetMonth: String {
        workspaceMonthKey(timezone: workspace.timezone)
    }

    private func applyRange() {
        guard fromDate <= toDate else {
            rangeError = "The start date must be on or before the end date."
            return
        }
        rangeError = nil
        Task {
            await model.loadFinancialProjection(workspaceID: workspace.id, range: selectedRange)
        }
    }
}

private struct DashboardBudgetProgress: View {
    let plan: MonthlyBudgetPlan

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: 3) {
                    Text(plan.name)
                        .font(.headline)
                    Text("Posted category allocations")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer(minLength: 12)
                VStack(alignment: .trailing, spacing: 3) {
                    Text("\(plan.baseCurrency.formatted(minorUnits: plan.usedBaseMinor)) used")
                        .font(.subheadline.monospacedDigit())
                    Text("\(plan.baseCurrency.formatted(minorUnits: plan.remainingBaseMinor)) remaining")
                        .font(.caption.monospacedDigit())
                        .foregroundStyle(plan.remainingBaseMinor < 0 ? .red : .secondary)
                }
            }
            ProgressView(value: progress)
                .tint(plan.remainingBaseMinor < 0 ? .red : .green)
            Text("of \(plan.baseCurrency.formatted(minorUnits: plan.plannedBaseMinor)) planned")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding(.vertical, 4)
        .accessibilityElement(children: .combine)
        .accessibilityLabel(
            "\(plan.name), \(plan.baseCurrency.formatted(minorUnits: plan.usedBaseMinor)) used, "
            + "\(plan.baseCurrency.formatted(minorUnits: plan.remainingBaseMinor)) remaining, "
            + "of \(plan.baseCurrency.formatted(minorUnits: plan.plannedBaseMinor)) planned"
        )
    }

    private var progress: Double {
        guard plan.plannedBaseMinor > 0 else { return 0 }
        return min(1, max(0, Double(plan.usedBaseMinor) / Double(plan.plannedBaseMinor)))
    }
}

struct ProjectionOverviewRow: View {
    let projection: BudgetFinancialProjection?
    let isLoading: Bool

    var body: some View {
        if let projection {
            HStack(spacing: 12) {
                Image(systemName: "chart.bar.fill")
                    .font(.title2)
                    .foregroundStyle(.green)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 4) {
                    Text("Financial overview")
                        .font(.headline)
                    Text(projection.period.baseCurrency.formatted(
                        minorUnits: projection.summary.balanceBaseMinor.posted
                    ))
                    .font(.title3.monospacedDigit().weight(.semibold))
                    Text(
                        "Period activity · "
                        + "\(projection.period.baseCurrency.formatted(minorUnits: projection.summary.incomeBaseMinor.posted)) income · "
                        + "\(projection.period.baseCurrency.formatted(minorUnits: projection.summary.spendingBaseMinor.posted)) spent"
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
                }
            }
            .padding(.vertical, 4)
            .accessibilityElement(children: .combine)
        } else if isLoading {
            ProgressView("Loading financial overview…")
        } else {
            Label("Financial overview unavailable", systemImage: "chart.bar.xaxis")
                .foregroundStyle(.secondary)
        }
    }
}

private struct ProjectionSummaryCard: View {
    let title: String
    let pendingTitle: String
    let amounts: BudgetProjectionAmounts
    let currency: BudgetCurrency

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title)
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
            Text(currency.formatted(minorUnits: amounts.posted))
                .font(.title3.monospacedDigit().weight(.semibold))
                .minimumScaleFactor(0.75)
            Text("Posted")
                .font(.caption2)
                .foregroundStyle(.secondary)
            Divider()
            ProjectionAmountLine(
                title: pendingTitle,
                amount: signedMoney(amounts.pending, currency: currency)
            )
            ProjectionAmountLine(
                title: "Projected",
                amount: currency.formatted(minorUnits: amounts.projected)
            )
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(.green.opacity(0.1), in: RoundedRectangle(cornerRadius: 14))
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(
            "\(title), posted \(currency.formatted(minorUnits: amounts.posted)), "
            + "\(pendingTitle) \(currency.formatted(minorUnits: amounts.pending)), "
            + "projected \(currency.formatted(minorUnits: amounts.projected))"
        )
    }
}

private struct ProjectionAmountLine: View {
    let title: String
    let amount: String

    var body: some View {
        HStack(alignment: .firstTextBaseline) {
            Text(title)
                .foregroundStyle(.secondary)
            Spacer(minLength: 4)
            Text(amount)
                .monospacedDigit()
        }
        .font(.caption2)
    }
}

private struct ProjectionAccountRow: View {
    let account: BudgetProjectionAccount
    let baseCurrency: BudgetCurrency

    var body: some View {
        HStack(alignment: .firstTextBaseline) {
            VStack(alignment: .leading, spacing: 4) {
                Text(account.name)
                    .font(.headline)
                Text(account.type.title + (account.archivedAt == nil ? "" : " · Archived"))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer(minLength: 12)
            VStack(alignment: .trailing, spacing: 4) {
                Text(account.currency.formatted(minorUnits: account.nativeBalanceMinor.posted))
                    .font(.subheadline.monospacedDigit())
                if account.nativeBalanceMinor.pending != 0 {
                    Text(
                        "\(signedMoney(account.nativeBalanceMinor.pending, currency: account.currency)) pending"
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                }
                if account.currency != baseCurrency {
                    Text("\(baseCurrency.formatted(minorUnits: account.baseBalanceMinor.posted)) base")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .accessibilityElement(children: .combine)
    }
}

private struct ProjectionCategorySection: View {
    let title: String
    let kind: BudgetCategoryKind
    let categories: [BudgetProjectionCategory]
    let currency: BudgetCurrency

    var body: some View {
        Section(title) {
            if visibleCategories.isEmpty {
                Text("No net activity in this period.")
                    .foregroundStyle(.secondary)
            }
            ForEach(visibleCategories) { category in
                HStack(alignment: .firstTextBaseline) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text((category.icon.map { "\($0) " } ?? "") + category.name)
                            .font(.headline)
                        Text(category.archivedAt == nil ? rollupLabel(category) : "\(rollupLabel(category)) · Archived")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    .padding(.leading, CGFloat(categoryDepth(category, in: categories)) * 12)
                    Spacer(minLength: 12)
                    VStack(alignment: .trailing, spacing: 4) {
                        Text(currency.formatted(minorUnits: category.rolledUpBaseMinor.posted))
                            .font(.subheadline.monospacedDigit())
                        if category.rolledUpBaseMinor.pending != 0 {
                            Text("\(signedMoney(category.rolledUpBaseMinor.pending, currency: currency)) pending")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
                .accessibilityElement(children: .combine)
            }
        }
    }

    private var visibleCategories: [BudgetProjectionCategory] {
        categories.filter {
            $0.kind == kind && ($0.rolledUpBaseMinor.posted != 0 || $0.rolledUpBaseMinor.pending != 0)
        }
    }

    private func rollupLabel(_ category: BudgetProjectionCategory) -> String {
        categories.contains { $0.parentID == category.id } ? "Includes subcategories" : "Category total"
    }
}

private func signedMoney(_ amount: Int64, currency: BudgetCurrency) -> String {
    let formatted = currency.formatted(minorUnits: amount)
    return amount > 0 ? "+\(formatted)" : formatted
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

private func displayDate(_ value: String) -> String {
    let parts = value.split(separator: "-").compactMap { Int($0) }
    guard parts.count == 3,
          let date = Calendar(identifier: .gregorian).date(
            from: DateComponents(year: parts[0], month: parts[1], day: parts[2])
          ) else { return value }
    return date.formatted(.dateTime.year().month(.abbreviated).day())
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
