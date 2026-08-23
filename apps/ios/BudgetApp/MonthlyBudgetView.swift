import Foundation
import SwiftUI

struct MonthlyBudgetView: View {
    let workspace: BudgetWorkspace
    @ObservedObject var model: AppModel

    @State private var selectedMonth: Date
    @State private var editorPresented = false

    private var canManage: Bool { workspace.canManage }
    private var month: String { workspaceMonthKey(selectedMonth, timezone: workspace.timezone) }

    init(workspace: BudgetWorkspace, model: AppModel) {
        self.workspace = workspace
        self.model = model
        _selectedMonth = State(initialValue: workspaceMonthStart(timezone: workspace.timezone))
    }

    var body: some View {
        List {
            Section {
                HStack {
                    Button("Previous month", systemImage: "chevron.left") {
                        shiftMonth(by: -1)
                    }
                    .labelStyle(.iconOnly)
                    .frame(minWidth: 44, minHeight: 44)
                    Spacer()
                    Text(budgetMonthLabel(month))
                        .font(.headline)
                        .accessibilityAddTraits(.isHeader)
                    Spacer()
                    Button("Next month", systemImage: "chevron.right") {
                        shiftMonth(by: 1)
                    }
                    .labelStyle(.iconOnly)
                    .frame(minWidth: 44, minHeight: 44)
                }
            } footer: {
                Text("Calendar month in \(workspace.timezone). Posted allocations determine usage; pending transactions are excluded.")
            }

            if model.isLoadingBudget {
                Section {
                    HStack {
                        Spacer()
                        ProgressView("Loading monthly budget…")
                        Spacer()
                    }
                }
            } else if let message = model.budgetErrorMessage {
                Section {
                    ContentUnavailableView(
                        "Budget unavailable",
                        systemImage: "chart.pie.fill",
                        description: Text(message)
                    )
                    Button("Try again") { reload() }
                }
            } else if let plan = model.monthlyBudget, plan.month == month {
                MonthlyBudgetUsageSections(plan: plan)
                if !canManage {
                    viewerSection
                }
            } else {
                Section {
                    ContentUnavailableView(
                        "No monthly plan",
                        systemImage: "chart.pie",
                        description: Text("Create a spending plan for this calendar month.")
                    )
                    if canManage {
                        Button("Create monthly plan", systemImage: "plus") {
                            editorPresented = true
                        }
                    }
                }
                if !canManage { viewerSection }
            }
        }
        .navigationTitle("Budget")
        .toolbar {
            if canManage && !model.isLoadingBudget && model.budgetErrorMessage == nil {
                ToolbarItem(placement: .primaryAction) {
                    Button(
                        displayedPlan == nil ? "Create monthly plan" : "Edit monthly plan",
                        systemImage: displayedPlan == nil ? "plus" : "pencil"
                    ) {
                        editorPresented = true
                    }
                }
            }
        }
        .task(id: month) {
            await model.loadMonthlyBudget(workspaceID: workspace.id, month: month)
        }
        .refreshable {
            await model.loadMonthlyBudget(workspaceID: workspace.id, month: month)
        }
        .sheet(isPresented: $editorPresented, onDismiss: {
            model.budgetErrorMessage = nil
        }) {
            MonthlyBudgetEditorView(
                workspace: workspace,
                month: month,
                plan: model.monthlyBudget?.month == month ? model.monthlyBudget : nil,
                categories: model.categories,
                model: model
            )
        }
    }

    private var viewerSection: some View {
        Section {
            Label("Viewer access can review usage but cannot change the plan.", systemImage: "eye")
                .foregroundStyle(.secondary)
        }
    }

    private var displayedPlan: MonthlyBudgetPlan? {
        guard let plan = model.monthlyBudget, plan.month == month else { return nil }
        return plan
    }

    private func reload() {
        Task { await model.loadMonthlyBudget(workspaceID: workspace.id, month: month) }
    }

    private func shiftMonth(by value: Int) {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: workspace.timezone) ?? .current
        selectedMonth = calendar.date(byAdding: .month, value: value, to: selectedMonth) ?? selectedMonth
    }

}

private struct MonthlyBudgetUsageSections: View {
    let plan: MonthlyBudgetPlan

    var body: some View {
        Section {
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .firstTextBaseline) {
                    Text(plan.name)
                        .font(.title3.weight(.semibold))
                    Spacer(minLength: 12)
                    BudgetUsageStateLabel(state: usage.state)
                }
                ProgressView(value: usage.progress)
                    .tint(usageColor)
                    .accessibilityLabel("Monthly budget usage")
                    .accessibilityValue(
                        "\(plan.baseCurrency.formatted(minorUnits: plan.usedBaseMinor)) used of "
                        + plan.baseCurrency.formatted(minorUnits: plan.plannedBaseMinor)
                    )
                LazyVGrid(columns: [GridItem(.adaptive(minimum: 100), spacing: 12)], spacing: 8) {
                    BudgetSummaryValue(title: "Planned", amount: plan.plannedBaseMinor, currency: plan.baseCurrency)
                    BudgetSummaryValue(title: "Used", amount: plan.usedBaseMinor, currency: plan.baseCurrency)
                    BudgetSummaryValue(title: "Remaining", amount: plan.remainingBaseMinor, currency: plan.baseCurrency)
                }
            }
            .padding(18)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 20))
            .accessibilityElement(children: .contain)
            .listRowInsets(EdgeInsets())
            .listRowBackground(Color.clear)
        } header: {
            Text("Posted usage")
        } footer: {
            Text("Refunds reduce usage. Remaining values can be negative when a category is over plan.")
        }

        Section("Category progress") {
            ForEach(plan.items) { item in
                let itemUsage = BudgetUsagePresentation(
                    planned: item.plannedBaseMinor,
                    used: item.usedBaseMinor,
                    remaining: item.remainingBaseMinor
                )
                VStack(alignment: .leading, spacing: 7) {
                    BudgetItemHeader(plan: plan, item: item)
                    ProgressView(value: progress(item))
                        .tint(color(itemUsage.state))
                        .accessibilityLabel("\(item.categoryName) budget usage")
                        .accessibilityValue(
                            "\(plan.baseCurrency.formatted(minorUnits: item.usedBaseMinor)) of "
                            + plan.baseCurrency.formatted(minorUnits: item.plannedBaseMinor)
                        )
                    Text("\(plan.baseCurrency.formatted(minorUnits: item.remainingBaseMinor)) remaining")
                        .font(.caption.monospacedDigit())
                        .foregroundStyle(
                            itemUsage.state == .overspent ? Color.red : Color.secondary
                        )
                    if itemUsage.state == .refundCredit {
                        Text("Refund credit reduces posted usage")
                            .font(.caption)
                            .foregroundStyle(.green)
                    }
                }
                .padding(.vertical, 4)
                .accessibilityElement(children: .contain)
            }
        }
    }

    private var usage: BudgetUsagePresentation {
        BudgetUsagePresentation(
            planned: plan.plannedBaseMinor,
            used: plan.usedBaseMinor,
            remaining: plan.remainingBaseMinor
        )
    }

    private var usageColor: Color { color(usage.state) }

    private func progress(_ item: MonthlyBudgetItem) -> Double {
        BudgetUsagePresentation(
            planned: item.plannedBaseMinor,
            used: item.usedBaseMinor,
            remaining: item.remainingBaseMinor
        ).progress
    }

    private func color(_ state: BudgetUsageState) -> Color {
        switch state {
        case .noTarget: .secondary
        case .onTrack: .green
        case .overspent: .red
        case .refundCredit: .mint
        }
    }
}

private struct BudgetItemHeader: View {
    let plan: MonthlyBudgetPlan
    let item: MonthlyBudgetItem

    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .firstTextBaseline) {
                category
                Spacer(minLength: 12)
                amounts
            }
            VStack(alignment: .leading, spacing: 6) {
                category
                amounts
            }
        }
    }

    private var category: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text((item.categoryIcon.map { "\($0) " } ?? "") + item.categoryName)
                .font(.headline)
            Text((item.categoryArchivedAt == nil ? "" : "Archived · ") + "Includes subcategories")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }

    private var amounts: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(plan.baseCurrency.formatted(minorUnits: item.usedBaseMinor))
                .font(.subheadline.monospacedDigit())
            Text("of \(plan.baseCurrency.formatted(minorUnits: item.plannedBaseMinor))")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }
}

private struct BudgetUsageStateLabel: View {
    let state: BudgetUsageState

    var body: some View {
        Label(title, systemImage: systemImage)
            .font(.caption.weight(.semibold))
            .foregroundStyle(color)
            .labelStyle(.titleAndIcon)
    }

    private var title: String {
        switch state {
        case .noTarget: "No target"
        case .onTrack: "On track"
        case .overspent: "Over plan"
        case .refundCredit: "Refund credit"
        }
    }

    private var systemImage: String {
        switch state {
        case .noTarget: "minus.circle"
        case .onTrack: "checkmark.circle.fill"
        case .overspent: "exclamationmark.circle.fill"
        case .refundCredit: "arrow.uturn.backward.circle.fill"
        }
    }

    private var color: Color {
        switch state {
        case .noTarget: .secondary
        case .onTrack: .green
        case .overspent: .red
        case .refundCredit: .mint
        }
    }
}

private struct BudgetSummaryValue: View {
    let title: String
    let amount: Int64
    let currency: BudgetCurrency

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(title)
                .font(.caption)
                .foregroundStyle(.secondary)
            Text(currency.formatted(minorUnits: amount))
                .font(.subheadline.monospacedDigit().weight(.semibold))
                .minimumScaleFactor(0.7)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }
}

private struct MonthlyBudgetEditorView: View {
    let workspace: BudgetWorkspace
    let month: String
    let plan: MonthlyBudgetPlan?
    let categories: [BudgetCategory]
    @ObservedObject var model: AppModel
    @Environment(\.dismiss) private var dismiss

    @State private var name: String
    @State private var items: [MonthlyBudgetDraft]
    @State private var validationMessage: String?

    init(
        workspace: BudgetWorkspace,
        month: String,
        plan: MonthlyBudgetPlan?,
        categories: [BudgetCategory],
        model: AppModel
    ) {
        self.workspace = workspace
        self.month = month
        self.plan = plan
        self.categories = categories
        self.model = model
        _name = State(initialValue: plan?.name ?? "\(budgetMonthLabel(month)) plan")
        _items = State(initialValue: plan?.items.map {
            MonthlyBudgetDraft(
                categoryID: $0.categoryID,
                amount: Self.formatAmount($0.plannedBaseMinor)
            )
        } ?? [])
    }

    private var activeExpenseCategories: [BudgetCategory] {
        categories
            .filter { $0.kind == .expense && $0.archivedAt == nil }
            .sorted { $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending }
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("Plan") {
                    TextField("Plan name", text: $name)
                    LabeledContent("Month", value: budgetMonthLabel(month))
                    LabeledContent("Currency", value: workspace.baseCurrency.rawValue)
                }

                Section {
                    ForEach($items) { $item in
                        VStack(alignment: .leading, spacing: 10) {
                            Picker("Expense category", selection: $item.categoryID) {
                                Text("Choose a category").tag("")
                                ForEach(options(for: item)) { option in
                                    Text(option.title).tag(option.id)
                                }
                            }
                            TextField("Planned amount", text: $item.amount)
                                .keyboardType(.decimalPad)
                        }
                    }
                    .onDelete { offsets in items.remove(atOffsets: offsets) }
                    Button("Add expense category", systemImage: "plus") {
                        let selected = Set(items.map(\.categoryID))
                        let categoryID = activeExpenseCategories.first { !selected.contains($0.id) }?.id ?? ""
                        items.append(MonthlyBudgetDraft(categoryID: categoryID))
                    }
                    .disabled(items.count >= 200)
                } header: {
                    Text("Category plans")
                } footer: {
                    Text("Choose a parent category or its subcategories, not both. Existing archived items may be retained.")
                }

                if let validationMessage {
                    Section {
                        Label(validationMessage, systemImage: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                    }
                }
                if let message = model.budgetErrorMessage {
                    Section {
                        Label(message, systemImage: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                    }
                }
            }
            .scrollDismissesKeyboard(.interactively)
            .navigationTitle(plan == nil ? "Create monthly plan" : "Edit monthly plan")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(model.isSavingResource)
                }
            }
        }
    }

    private func options(for draft: MonthlyBudgetDraft) -> [MonthlyBudgetCategoryOption] {
        var result = activeExpenseCategories.map {
            MonthlyBudgetCategoryOption(
                id: $0.id,
                title: ($0.icon.map { "\($0) " } ?? "") + $0.name
            )
        }
        if let retained = plan?.items.first(where: { $0.categoryID == draft.categoryID }),
           !result.contains(where: { $0.id == retained.categoryID }) {
            result.append(MonthlyBudgetCategoryOption(
                id: retained.categoryID,
                title: (retained.categoryIcon.map { "\($0) " } ?? "")
                    + retained.categoryName + " (archived)"
            ))
        }
        return result.sorted { $0.title.localizedCaseInsensitiveCompare($1.title) == .orderedAscending }
    }

    private func save() {
        validationMessage = nil
        let normalizedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalizedName.isEmpty, normalizedName.count <= 100, !items.isEmpty else {
            validationMessage = "Give the plan a name and add at least one expense category."
            return
        }
        var selected = Set<String>()
        var inputs: [MonthlyBudgetItemInput] = []
        var total: Int64 = 0
        for item in items {
            guard !item.categoryID.isEmpty,
                  let amount = Self.parseAmount(item.amount), amount > 0 else {
                validationMessage = "Every item needs an expense category and a positive amount with at most two decimals."
                return
            }
            guard selected.insert(item.categoryID).inserted else {
                validationMessage = "Each category can appear only once."
                return
            }
            let addition = total.addingReportingOverflow(amount)
            guard !addition.overflow else {
                validationMessage = "The planned total is too large."
                return
            }
            total = addition.partialValue
            inputs.append(MonthlyBudgetItemInput(
                categoryID: item.categoryID,
                amountBaseMinor: amount
            ))
        }
        guard !hasBranchOverlap(selected) else {
            validationMessage = "Choose a category or its subcategories, not both."
            return
        }
        Task {
            let saved = await model.saveMonthlyBudget(
                workspaceID: workspace.id,
                month: month,
                input: MonthlyBudgetInput(name: normalizedName, items: inputs)
            )
            if saved { dismiss() }
        }
    }

    private func hasBranchOverlap(_ selected: Set<String>) -> Bool {
        let byID = Dictionary(uniqueKeysWithValues: categories.map { ($0.id, $0) })
        for categoryID in selected {
            var visited = Set([categoryID])
            var parentID = byID[categoryID]?.parentID
            while let current = parentID, visited.insert(current).inserted {
                if selected.contains(current) { return true }
                parentID = byID[current]?.parentID
            }
        }
        return false
    }

    private static func parseAmount(_ input: String) -> Int64? {
        let value = input.trimmingCharacters(in: .whitespacesAndNewlines)
            .replacingOccurrences(of: ",", with: ".")
        guard value.range(of: #"^(?:\d+(?:\.\d{0,2})?|\.\d{1,2})$"#, options: .regularExpression) != nil,
              let decimal = Decimal(string: value, locale: Locale(identifier: "en_US_POSIX")) else {
            return nil
        }
        let scaled = decimal * 100
        guard scaled >= Decimal(Int64.min), scaled <= Decimal(Int64.max) else { return nil }
        return NSDecimalNumber(decimal: scaled).int64Value
    }

    private static func formatAmount(_ minorUnits: Int64) -> String {
        NSDecimalNumber(decimal: Decimal(minorUnits) / 100).stringValue
    }

}

private struct MonthlyBudgetDraft: Identifiable {
    let id = UUID()
    var categoryID: String
    var amount: String

    init(categoryID: String, amount: String = "") {
        self.categoryID = categoryID
        self.amount = amount
    }
}

private struct MonthlyBudgetCategoryOption: Identifiable {
    let id: String
    let title: String
}
