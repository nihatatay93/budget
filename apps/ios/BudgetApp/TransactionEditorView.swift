import Foundation
import SwiftUI

struct TransactionEditorView: View {
    let workspace: BudgetWorkspace
    let transaction: BudgetTransaction?
    let accounts: [BudgetAccount]
    let categories: [BudgetCategory]
    @ObservedObject var model: AppModel
    @Environment(\.dismiss) private var dismiss

    private enum EditorMode {
        case simple
        case detailed
    }

    @State private var mode: EditorMode
    @State private var draft: CaptureDraft
    /// What the detailed editor opened with, so an untouched visit can go straight back. An empty
    /// draft has no transaction to rebuild from, so without this the return trip would be refused
    /// for work nobody has done yet.
    @State private var detailedOrigin: DetailedSnapshot?
    @State private var kind: BudgetTransactionKind
    @State private var status: BudgetTransactionStatus
    @State private var transactionDate: Date
    @State private var payee: String
    @State private var description: String
    @State private var notes: String
    @State private var entries: [EntryDraft]
    @State private var allocations: [AllocationDraft]
    @State private var validationMessage: String?
    @State private var categoryPickerPresented = false

    init(
        workspace: BudgetWorkspace,
        transaction: BudgetTransaction?,
        accounts: [BudgetAccount],
        categories: [BudgetCategory],
        model: AppModel
    ) {
        self.workspace = workspace
        self.transaction = transaction
        self.accounts = accounts
        self.categories = categories
        self.model = model
        _kind = State(initialValue: transaction?.kind ?? .standard)
        _status = State(initialValue: transaction?.status ?? .posted)
        _transactionDate = State(initialValue:
            transaction.flatMap { Self.dateFormatter.date(from: $0.transactionDate) } ?? Date()
        )
        _payee = State(initialValue: transaction?.payee ?? "")
        _description = State(initialValue: transaction?.description ?? "")
        _notes = State(initialValue: transaction?.notes ?? "")
        let existingEntries = transaction?.entries.map { entry in
            EntryDraft(
                accountID: entry.accountID,
                amount: TransactionCapture.formatAmount(entry.amountMinor),
                baseAmount: accounts.first { $0.id == entry.accountID }?.currency == workspace.baseCurrency
                    ? ""
                    : TransactionCapture.formatAmount(entry.baseAmountMinor)
            )
        } ?? []
        _entries = State(initialValue: existingEntries.isEmpty
            ? [EntryDraft(accountID: accounts.first?.id ?? "")]
            : existingEntries
        )
        _allocations = State(initialValue: transaction?.allocations.map {
            AllocationDraft(
                categoryID: $0.categoryID,
                amount: TransactionCapture.formatAmount($0.amountBaseMinor)
            )
        } ?? [])

        // The simple form only opens on a transaction it can reproduce exactly; everything else —
        // a split, an adjustment, several account entries — goes straight to the detailed editor
        // rather than being flattened into a shape it never had.
        let context = CaptureContext(
            accounts: accounts,
            categories: categories,
            baseCurrency: workspace.baseCurrency
        )
        let captured = transaction.flatMap {
            TransactionCapture.draft(from: $0, context: context, dateFormatter: Self.dateFormatter)
        }
        if let captured {
            _draft = State(initialValue: captured)
            _mode = State(initialValue: .simple)
        } else if transaction == nil {
            var fresh = CaptureDraft()
            fresh.accountID = TransactionCapture.suggestedAccountID(
                transactions: model.transactions,
                accounts: accounts
            )
            _draft = State(initialValue: fresh)
            _mode = State(initialValue: .simple)
        } else {
            _draft = State(initialValue: CaptureDraft())
            _mode = State(initialValue: .detailed)
        }
    }

    var body: some View {
        NavigationStack {
            Form {
                if mode == .simple {
                    captureSections
                } else {
                    detailedSections
                }
                if let validationMessage {
                    Section {
                        Label(validationMessage, systemImage: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                    }
                }
                if let message = model.resourceErrorMessage {
                    Section {
                        Label(message, systemImage: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                    }
                }
            }
            .scrollDismissesKeyboard(.interactively)
            .sheet(isPresented: $categoryPickerPresented) {
                NavigationStack {
                    VStack(spacing: 0) {
                        Button("Uncategorized") {
                            draft.categoryID = ""
                            categoryPickerPresented = false
                        }
                        .buttonStyle(.bordered)
                        .padding(.horizontal, 16)
                        .padding(.top, 8)
                        CategoryTileSections(
                            categories: captureCategories,
                            frequent: CategoryHierarchy.frequentCategoryIDs(in: model.transactions),
                            selectedID: draft.categoryID,
                            workspaceID: workspace.id
                        ) { category in
                            draft.categoryID = category.id
                            categoryPickerPresented = false
                        }
                    }
                    .navigationTitle("Choose a category")
                    .navigationBarTitleDisplayMode(.inline)
                    .toolbar {
                        ToolbarItem(placement: .cancellationAction) {
                            Button("Cancel") { categoryPickerPresented = false }
                        }
                    }
                }
            }
            .navigationTitle(transaction == nil ? "Add transaction" : "Edit transaction")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(model.isSavingResource || accounts.isEmpty)
                }
            }
        }
    }

    // MARK: - Everyday capture

    @ViewBuilder private var captureSections: some View {
        Section {
            Picker("Type", selection: $draft.type) {
                ForEach(CaptureType.allCases) { value in
                    Text(value.title).tag(value)
                }
            }
            .pickerStyle(.segmented)
            .onChange(of: draft.type) { previous, next in
                guard previous != next else { return }
                // A category belongs to one kind, and a transfer has none at all.
                draft.categoryID = ""
                if next != .transfer { draft.toAccountID = "" }
            }

            HStack {
                Text(draftCurrency.rawValue)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(.secondary)
                TextField("Amount", text: $draft.amount)
                    .keyboardType(.decimalPad)
                    .multilineTextAlignment(.trailing)
                    .font(.title2.weight(.semibold))
            }
            if draftAccountIsForeign {
                TextField(
                    "Value in \(workspace.baseCurrency.rawValue) (optional)",
                    text: $draft.baseAmount,
                    prompt: Text("Rate for that date")
                )
                .keyboardType(.decimalPad)
            }
        }

        if draft.type == .transfer {
            Section("Accounts") {
                Picker("From account", selection: $draft.accountID) {
                    Text("Choose an account").tag("")
                    ForEach(accounts) { account in
                        Text("\(account.name) · \(account.currency.rawValue)").tag(account.id)
                    }
                }
                Picker("To account", selection: $draft.toAccountID) {
                    Text("Choose an account").tag("")
                    ForEach(accounts.filter { $0.id != draft.accountID }) { account in
                        Text("\(account.name) · \(account.currency.rawValue)").tag(account.id)
                    }
                }
            }
        } else {
            Section("Category") {
                Button {
                    categoryPickerPresented = true
                } label: {
                    HStack {
                        if let selected = captureCategories.first(where: { $0.id == draft.categoryID }) {
                            CategoryNameLabel(
                                name: selected.name,
                                kind: selected.kind,
                                predefinedKey: selected.predefinedKey,
                                systemKey: selected.systemKey,
                                iconType: selected.iconType,
                                iconValue: selected.iconValue,
                                colorKey: selected.colorKey,
                                iconSize: 22
                            )
                        } else {
                            Text("Uncategorized")
                        }
                        Spacer(minLength: 12)
                        Text("Change")
                            .font(.footnote.weight(.semibold))
                            .foregroundStyle(BudgetTheme.forest)
                    }
                }
                .buttonStyle(.plain)
                .accessibilityLabel(Text("Category"))
            }
        }

        Section {
            DatePicker("Date", selection: $draft.transactionDate, displayedComponents: .date)
            HStack(spacing: 18) {
                Button("Today") { draft.transactionDate = Date() }
                Button("Yesterday") {
                    draft.transactionDate = Calendar.current.date(byAdding: .day, value: -1, to: Date()) ?? Date()
                }
            }
            .buttonStyle(.borderless)
            .font(.subheadline)
            if draft.type != .transfer, accounts.count > 1 {
                Picker(
                    LocalizedStringKey(draft.type == .expense ? "Paid from" : "Received into"),
                    selection: $draft.accountID
                ) {
                    Text("Choose an account").tag("")
                    ForEach(accounts) { account in
                        Text("\(account.name) · \(account.currency.rawValue)").tag(account.id)
                    }
                }
            }
            if draft.type != .transfer {
                TextField(
                    LocalizedStringKey(draft.type == .expense ? "Paid to" : "Received from"),
                    text: $draft.payee,
                    prompt: Text("For example, Netflix")
                )
            }
        }

        Section {
            Toggle("Still pending, not cleared yet", isOn: $draft.pending)
            TextField("Optional notes", text: $draft.notes, axis: .vertical)
                .lineLimit(2...6)
            Button("Use the detailed editor") { openDetailedEditor() }
        } header: {
            Text("More options")
        } footer: {
            Text("The detailed editor writes account entries and category allocations directly, for splits, several accounts, and balance adjustments.")
        }
    }

    private var draftAccount: BudgetAccount? {
        accounts.first { $0.id == draft.accountID }
    }

    private var draftCurrency: BudgetCurrency {
        draftAccount?.currency ?? workspace.baseCurrency
    }

    private var draftAccountIsForeign: Bool {
        draftAccount != nil && draftCurrency != workspace.baseCurrency
    }

    /// An archived category stays selectable while it is the one this transaction already uses;
    /// it just cannot be chosen for anything new.
    private var captureCategories: [BudgetCategory] {
        let kind: BudgetCategoryKind = draft.type == .income ? .income : .expense
        return categories.filter { category in
            category.kind == kind
                && category.systemKey == nil
                && (category.archivedAt == nil || category.id == draft.categoryID)
        }
    }

    // MARK: - Detailed ledger editor

    @ViewBuilder private var detailedSections: some View {
        Section("Transaction") {
            Picker("Kind", selection: $kind) {
                ForEach(BudgetTransactionKind.allCases) { value in
                    Text(value.title).tag(value)
                }
            }
            .onChange(of: kind) { _, nextKind in
                if nextKind == .transfer {
                    allocations = []
                    while entries.count < 2 {
                        entries.append(EntryDraft(accountID: fallbackAccountID))
                    }
                }
            }
            Picker("Status", selection: $status) {
                ForEach(BudgetTransactionStatus.allCases) { value in
                    Text(value.title).tag(value)
                }
            }
            DatePicker("Date", selection: $transactionDate, displayedComponents: .date)
            TextField("Payee (optional)", text: $payee)
            TextField("Description (optional)", text: $description)
        }

        Section {
            ForEach($entries) { $entry in
                VStack(alignment: .leading, spacing: 10) {
                    Picker("Account", selection: $entry.accountID) {
                        Text("Choose an account").tag("")
                        ForEach(accounts) { account in
                            Text("\(account.name) · \(account.currency.rawValue)").tag(account.id)
                        }
                    }
                    TextField("Signed amount", text: $entry.amount)
                        .keyboardType(.numbersAndPunctuation)
                    if let currency = currency(for: entry.accountID),
                       currency != workspace.baseCurrency {
                        TextField(
                            "Base amount in \(workspace.baseCurrency.rawValue) (optional)",
                            text: $entry.baseAmount
                        )
                        .keyboardType(.numbersAndPunctuation)
                    }
                }
            }
            .onDelete { offsets in entries.remove(atOffsets: offsets) }
            Button("Add account entry", systemImage: "plus") {
                entries.append(EntryDraft(accountID: accounts.first?.id ?? ""))
            }
        } header: {
            Text("Account entries")
        } footer: {
            Text("Negative removes money; positive adds money. Transfers require at least two entries whose base amounts sum to zero.")
        }

        if kind != .transfer {
            Section {
                ForEach($allocations) { $allocation in
                    VStack(alignment: .leading, spacing: 10) {
                        Picker("Category", selection: $allocation.categoryID) {
                            Text("Choose a category").tag("")
                            ForEach(BudgetCategoryKind.allCases) { categoryKind in
                                Section(categoryKind.title) {
                                    ForEach(categories.filter { $0.kind == categoryKind }) { category in
                                        CategoryNameLabel(
                                            name: category.name,
                                            kind: category.kind,
                                            predefinedKey: category.predefinedKey,
                                            systemKey: category.systemKey,
                                            iconType: category.iconType,
                                            iconValue: category.iconValue,
                                            colorKey: category.colorKey,
                                            iconSize: 22
                                        )
                                        .tag(category.id)
                                    }
                                }
                            }
                        }
                        TextField(
                            "Amount in \(workspace.baseCurrency.rawValue)",
                            text: $allocation.amount
                        )
                        .keyboardType(.numbersAndPunctuation)
                    }
                }
                .onDelete { offsets in allocations.remove(atOffsets: offsets) }
                Button("Add category allocation", systemImage: "plus") {
                    allocations.append(AllocationDraft(categoryID: categories.first?.id ?? ""))
                }
            } header: {
                Text("Category allocations")
            } footer: {
                Text("Leave the whole section empty on an expense or income to use the protected Uncategorized category automatically, or leave a single allocation's amount empty to book it at the transaction date's rate.")
            }
        }

        Section("Notes") {
            TextField("Optional notes", text: $notes, axis: .vertical)
                .lineLimit(3...8)
        }

        Section {
            Button("Back to the simple form") { openSimpleEditor() }
        }
    }

    // MARK: - Moving between the two forms

    private var captureContext: CaptureContext {
        CaptureContext(accounts: accounts, categories: categories, baseCurrency: workspace.baseCurrency)
    }

    private func openDetailedEditor() {
        switch TransactionCapture.input(from: draft, context: captureContext, dateFormatter: Self.dateFormatter) {
        case let .success(input):
            kind = input.kind
            status = input.status
            transactionDate = draft.transactionDate
            payee = input.payee ?? ""
            description = input.description ?? ""
            notes = input.notes ?? ""
            entries = input.entries.map { entry in
                EntryDraft(
                    accountID: entry.accountID,
                    amount: TransactionCapture.formatAmount(entry.amountMinor),
                    baseAmount: entry.baseAmountMinor.map(TransactionCapture.formatAmount) ?? ""
                )
            }
            allocations = input.allocations.map {
                AllocationDraft(
                    categoryID: $0.categoryID,
                    amount: $0.amountBaseMinor.map(TransactionCapture.formatAmount) ?? ""
                )
            }
        case .failure:
            // An incomplete draft still carries the answers already given into the wider form.
            kind = draft.type == .transfer ? .transfer : .standard
            status = draft.pending ? .pending : .posted
            transactionDate = draft.transactionDate
            payee = draft.payee
            description = draft.summary
            notes = draft.notes
            entries = [EntryDraft(accountID: draft.accountID)]
            if draft.type == .transfer { entries.append(EntryDraft(accountID: draft.toAccountID)) }
            allocations = draft.categoryID.isEmpty ? [] : [AllocationDraft(categoryID: draft.categoryID)]
        }
        detailedOrigin = detailedSnapshot
        validationMessage = nil
        mode = .detailed
    }

    private var detailedSnapshot: DetailedSnapshot {
        DetailedSnapshot(
            kind: kind,
            status: status,
            transactionDate: transactionDate,
            payee: payee,
            summary: description,
            notes: notes,
            entries: entries,
            allocations: allocations
        )
    }

    private func openSimpleEditor() {
        if let detailedOrigin, detailedOrigin == detailedSnapshot {
            validationMessage = nil
            mode = .simple
            return
        }
        guard let candidate = draftFromDetailed() else {
            validationMessage = L10n.text("This transaction needs the detailed editor. The simple form cannot show splits, several account entries, or a balance adjustment.")
            return
        }
        draft = candidate
        validationMessage = nil
        mode = .simple
    }

    /// The detailed fields as a transaction, so the simple form can say whether it can show them.
    /// The identity fields are placeholders: only the ledger shape is inspected.
    private func draftFromDetailed() -> CaptureDraft? {
        var builtEntries: [BudgetTransactionEntry] = []
        for entry in entries {
            guard let amount = TransactionCapture.parseAmount(entry.amount), amount != 0,
                  let currency = currency(for: entry.accountID) else { return nil }
            let stated = entry.baseAmount.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            // An amount left to the transaction date's rate is unknown here, and the simple form
            // carries it as an empty field either way. Standing the entry amount in its place
            // keeps the shape checks reading what they would read if the rate were known; the
            // draft still takes its text from the fields themselves.
            let base = currency == workspace.baseCurrency || stated
                ? amount
                : TransactionCapture.parseAmount(entry.baseAmount)
            guard let base, base != 0 else { return nil }
            builtEntries.append(BudgetTransactionEntry(
                accountID: entry.accountID,
                amountMinor: amount,
                baseAmountMinor: base
            ))
        }
        var builtAllocations: [BudgetTransactionAllocation] = []
        for allocation in allocations {
            let derived = allocations.count == 1
                && allocation.amount.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            let amount = derived
                ? builtEntries.reduce(Int64(0)) { $0 + $1.baseAmountMinor }
                : TransactionCapture.parseAmount(allocation.amount)
            guard !allocation.categoryID.isEmpty, let amount else { return nil }
            builtAllocations.append(BudgetTransactionAllocation(
                categoryID: allocation.categoryID,
                amountBaseMinor: amount
            ))
        }
        let candidate = BudgetTransaction(
            id: transaction?.id ?? "",
            workspaceID: workspace.id,
            kind: kind,
            status: status,
            transactionDate: Self.dateFormatter.string(from: transactionDate),
            payee: payee.nilIfBlank,
            description: description.nilIfBlank,
            notes: notes.nilIfBlank,
            source: transaction?.source ?? "manual",
            entries: builtEntries,
            allocations: builtAllocations
        )
        guard var draft = TransactionCapture.draft(
            from: candidate,
            context: captureContext,
            dateFormatter: Self.dateFormatter
        ) else { return nil }
        draft.baseAmount = entries.first?.baseAmount.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return draft
    }

    private var fallbackAccountID: String {
        if entries.count < accounts.count { return accounts[entries.count].id }
        return accounts.first?.id ?? ""
    }

    private func currency(for accountID: String) -> BudgetCurrency? {
        accounts.first { $0.id == accountID }?.currency
    }

    // MARK: - Saving

    private func save() {
        validationMessage = nil
        if mode == .simple {
            switch TransactionCapture.input(
                from: draft,
                context: captureContext,
                dateFormatter: Self.dateFormatter
            ) {
            case let .success(input):
                submit(input)
            case let .failure(error):
                validationMessage = error.message
            }
            return
        }

        guard !entries.isEmpty, kind != .transfer || entries.count >= 2 else {
            validationMessage = kind == .transfer
                ? L10n.text("A transfer needs at least two account entries.")
                : L10n.text("Add an account entry.")
            return
        }
        var entryInputs: [TransactionEntryInput] = []
        for entry in entries {
            guard !entry.accountID.isEmpty,
                  let amount = TransactionCapture.parseAmount(entry.amount), amount != 0 else {
                validationMessage = L10n.text("Every entry needs an account and a non-zero amount with at most two decimals.")
                return
            }
            let baseAmount: Int64?
            if entry.baseAmount.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                baseAmount = nil
            } else if let parsed = TransactionCapture.parseAmount(entry.baseAmount) {
                guard parsed == 0 || (parsed < 0) == (amount < 0) else {
                    validationMessage = L10n.text("An entry and its base amount must have the same sign.")
                    return
                }
                baseAmount = parsed
            } else {
                validationMessage = L10n.text("A manual base amount must use at most two decimals.")
                return
            }
            entryInputs.append(TransactionEntryInput(
                accountID: entry.accountID,
                amountMinor: amount,
                baseAmountMinor: baseAmount
            ))
        }
        var allocationInputs: [TransactionAllocationInput] = []
        if kind != .transfer {
            for allocation in allocations {
                // A single allocation may leave its amount to the server, which takes the entry
                // total — the same allowance the entry's own base amount already has. A split
                // cannot: dividing a transaction between categories is this form's decision.
                let derived = allocations.count == 1
                    && allocation.amount.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
                var amount: Int64?
                if !derived {
                    guard let stated = TransactionCapture.parseAmount(allocation.amount), stated != 0 else {
                        validationMessage = L10n.text("Every allocation needs a category and a non-zero amount with at most two decimals.")
                        return
                    }
                    amount = stated
                }
                guard !allocation.categoryID.isEmpty else {
                    validationMessage = L10n.text("Every allocation needs a category and a non-zero amount with at most two decimals.")
                    return
                }
                allocationInputs.append(TransactionAllocationInput(
                    categoryID: allocation.categoryID,
                    amountBaseMinor: amount
                ))
            }
        }
        submit(TransactionInput(
            kind: kind,
            status: status,
            transactionDate: Self.dateFormatter.string(from: transactionDate),
            payee: payee.nilIfBlank,
            description: description.nilIfBlank,
            notes: notes.nilIfBlank,
            entries: entryInputs,
            allocations: allocationInputs
        ))
    }

    private func submit(_ input: TransactionInput) {
        Task {
            let saved = await model.saveTransaction(
                workspaceID: workspace.id,
                transactionID: transaction?.id,
                input: input
            )
            if saved { dismiss() }
        }
    }

    static let dateFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        // A transaction date is a workspace/business calendar date, not a UTC instant.
        formatter.timeZone = .current
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter
    }()
}

private struct EntryDraft: Identifiable, Equatable {
    let id = UUID()
    var accountID: String
    var amount = ""
    var baseAmount = ""
}

private struct AllocationDraft: Identifiable, Equatable {
    let id = UUID()
    var categoryID: String
    var amount = ""
}

/// The detailed fields as one comparable value. Row identity is part of it on purpose: adding or
/// removing a row is a change, whatever the remaining text says.
private struct DetailedSnapshot: Equatable {
    let kind: BudgetTransactionKind
    let status: BudgetTransactionStatus
    let transactionDate: Date
    let payee: String
    let summary: String
    let notes: String
    let entries: [EntryDraft]
    let allocations: [AllocationDraft]
}
