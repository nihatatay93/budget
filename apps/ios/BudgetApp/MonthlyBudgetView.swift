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
        ScrollView {
            VStack(alignment: .leading, spacing: BudgetTheme.Space.section) {
                monthSwitcher

                if model.isLoadingBudget {
                    BudgetCard { BudgetLoading("Loading monthly budget…") }
                } else if let message = model.budgetErrorMessage {
                    BudgetCard {
                        BudgetMessage(
                            title: "Budget unavailable",
                            systemImage: "chart.pie.fill",
                            message: .resolved(message),
                            action: ("Try again", reload)
                        )
                    }
                } else if let plan = displayedPlan {
                    BudgetSection(
                        "Posted usage",
                        caption: "Refunds reduce usage. Remaining values can be negative when a category is over plan."
                    ) {
                        BudgetCard { BudgetPlanSummary(plan: plan) }
                    }

                    BudgetSection("Category progress") {
                        if plan.items.isEmpty {
                            BudgetCard {
                                BudgetMessage(
                                    title: "No categories planned",
                                    systemImage: "tag",
                                    message: "Add an expense category to this plan."
                                )
                            }
                        } else {
                            BudgetRowGroup(items: plan.items) { item in
                                MonthlyBudgetItemRow(plan: plan, item: item)
                            }
                        }
                    }
                } else {
                    BudgetCard {
                        BudgetMessage(
                            title: "No monthly plan",
                            systemImage: "chart.pie",
                            message: "Create a spending plan for this calendar month.",
                            action: canManage
                                ? ("Create monthly plan", { editorPresented = true })
                                : nil
                        )
                    }
                }

                if !canManage { viewerNotice }
            }
            .padding(.horizontal, BudgetTheme.Space.screen)
            .padding(.top, 4)
            .padding(.bottom, 32)
        }
        .budgetScreen()
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

    /// The month is the screen's primary control, so it gets a card of its own rather than a
    /// row buried in a list.
    private var monthSwitcher: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 0) {
                monthStepButton("Previous month", systemImage: "chevron.left", offset: -1)
                Spacer(minLength: 8)
                Text(budgetMonthLabel(month))
                    .font(.headline)
                    .foregroundStyle(BudgetTheme.primaryText)
                    .accessibilityAddTraits(.isHeader)
                Spacer(minLength: 8)
                monthStepButton("Next month", systemImage: "chevron.right", offset: 1)
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
            .budgetSurface(radius: BudgetTheme.Radius.medium)

            Text("Calendar month in \(workspace.timezone). Pending transactions are excluded from usage.")
                .font(.caption)
                .foregroundStyle(BudgetTheme.tertiaryText)
                .padding(.horizontal, 2)
        }
    }

    private func monthStepButton(_ title: String, systemImage: String, offset: Int) -> some View {
        Button(title, systemImage: systemImage) { shiftMonth(by: offset) }
            .labelStyle(.iconOnly)
            .font(.subheadline.weight(.semibold))
            .foregroundStyle(BudgetTheme.forest)
            .frame(minWidth: 44, minHeight: 44)
            .contentShape(Rectangle())
    }

    private var viewerNotice: some View {
        Label("Viewer access can review usage but cannot change the plan.", systemImage: "eye")
            .font(.caption)
            .foregroundStyle(BudgetTheme.tertiaryText)
            .padding(.horizontal, 2)
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

private struct MonthlyBudgetItemRow: View {
    let plan: MonthlyBudgetPlan
    let item: MonthlyBudgetItem

    private var usage: BudgetUsagePresentation {
        BudgetUsagePresentation(
            planned: item.plannedBaseMinor,
            used: item.usedBaseMinor,
            remaining: item.remainingBaseMinor
        )
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .center, spacing: 12) {
                CategoryNameLabel(
                    name: item.categoryName,
                    kind: nil,
                    predefinedKey: item.categoryPredefinedKey,
                    systemKey: nil,
                    iconType: item.categoryIconType,
                    iconValue: item.categoryIconValue,
                    colorKey: item.categoryColorKey,
                    iconSize: 32
                )
                .font(.subheadline.weight(.semibold))
                Spacer(minLength: 12)
                VStack(alignment: .trailing, spacing: 2) {
                    Text(plan.baseCurrency.formatted(minorUnits: item.usedBaseMinor))
                        .font(.budgetAmount)
                        .foregroundStyle(BudgetTheme.primaryText)
                        .lineLimit(1)
                        .minimumScaleFactor(0.7)
                    Text("of \(plan.baseCurrency.formatted(minorUnits: item.plannedBaseMinor))")
                        .font(.caption)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                        .lineLimit(1)
                }
            }
            BudgetMeter(progress: usage.progress, tint: budgetUsageColor(usage.state))
            HStack(spacing: 8) {
                Text("\(plan.baseCurrency.formatted(minorUnits: item.remainingBaseMinor)) remaining")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(
                        usage.state == .overspent ? BudgetTheme.over : BudgetTheme.tertiaryText
                    )
                if item.categoryArchivedAt != nil {
                    BudgetChip(text: "Archived", color: BudgetTheme.tertiaryText)
                }
                if usage.state == .refundCredit {
                    BudgetChip(text: "Refund credit", color: BudgetTheme.sage)
                }
                Spacer(minLength: 0)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(
            "\(L10n.categoryName(name: item.categoryName, predefinedKey: item.categoryPredefinedKey)), "
            + "\(plan.baseCurrency.formatted(minorUnits: item.usedBaseMinor)) of "
            + "\(plan.baseCurrency.formatted(minorUnits: item.plannedBaseMinor)) planned, "
            + "\(plan.baseCurrency.formatted(minorUnits: item.remainingBaseMinor)) remaining"
        )
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
                                    HStack(spacing: 6) {
                                        CategoryNameLabel(
                                            name: option.name,
                                            kind: .expense,
                                            predefinedKey: option.predefinedKey,
                                            systemKey: nil,
                                            iconType: option.iconType,
                                            iconValue: option.iconValue,
                                            colorKey: option.colorKey,
                                            iconSize: 22
                                        )
                                        if option.isArchived {
                                            Text(L10n.text("category.editor.archived"))
                                                .foregroundStyle(.secondary)
                                        }
                                    }
                                    .tag(option.id)
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
                name: L10n.categoryName(
                    name: $0.name,
                    kind: $0.kind,
                    predefinedKey: $0.predefinedKey,
                    systemKey: $0.systemKey
                ),
                predefinedKey: $0.predefinedKey,
                iconType: $0.iconType,
                iconValue: $0.iconValue,
                colorKey: $0.colorKey,
                isArchived: false
            )
        }
        if let retained = plan?.items.first(where: { $0.categoryID == draft.categoryID }),
           !result.contains(where: { $0.id == retained.categoryID }) {
            result.append(MonthlyBudgetCategoryOption(
                id: retained.categoryID,
                name: L10n.categoryName(name: retained.categoryName, predefinedKey: retained.categoryPredefinedKey),
                predefinedKey: retained.categoryPredefinedKey,
                iconType: retained.categoryIconType,
                iconValue: retained.categoryIconValue,
                colorKey: retained.categoryColorKey,
                isArchived: true
            ))
        }
        return result.sorted { $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending }
    }

    private func save() {
        validationMessage = nil
        let normalizedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalizedName.isEmpty, normalizedName.count <= 100, !items.isEmpty else {
            validationMessage = L10n.text("Give the plan a name and add at least one expense category.")
            return
        }
        var selected = Set<String>()
        var inputs: [MonthlyBudgetItemInput] = []
        var total: Int64 = 0
        for item in items {
            guard !item.categoryID.isEmpty,
                  let amount = Self.parseAmount(item.amount), amount > 0 else {
                validationMessage = L10n.text("Every item needs an expense category and a positive amount with at most two decimals.")
                return
            }
            guard selected.insert(item.categoryID).inserted else {
                validationMessage = L10n.text("Each category can appear only once.")
                return
            }
            let addition = total.addingReportingOverflow(amount)
            guard !addition.overflow else {
                validationMessage = L10n.text("The planned total is too large.")
                return
            }
            total = addition.partialValue
            inputs.append(MonthlyBudgetItemInput(
                categoryID: item.categoryID,
                amountBaseMinor: amount
            ))
        }
        guard !hasBranchOverlap(selected) else {
            validationMessage = L10n.text("Choose a category or its subcategories, not both.")
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
    let name: String
    let predefinedKey: String?
    let iconType: String
    let iconValue: String
    let colorKey: String
    let isArchived: Bool
}
