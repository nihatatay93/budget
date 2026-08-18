import Foundation
import SwiftUI

struct TransactionEditorView: View {
    let workspace: BudgetWorkspace
    let transaction: BudgetTransaction?
    let accounts: [BudgetAccount]
    let categories: [BudgetCategory]
    @ObservedObject var model: AppModel
    @Environment(\.dismiss) private var dismiss

    @State private var kind: BudgetTransactionKind
    @State private var status: BudgetTransactionStatus
    @State private var transactionDate: Date
    @State private var payee: String
    @State private var description: String
    @State private var notes: String
    @State private var entries: [EntryDraft]
    @State private var allocations: [AllocationDraft]
    @State private var validationMessage: String?

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
                amount: Self.formatAmount(entry.amountMinor),
                baseAmount: accounts.first { $0.id == entry.accountID }?.currency == workspace.baseCurrency
                    ? ""
                    : Self.formatAmount(entry.baseAmountMinor)
            )
        } ?? []
        _entries = State(initialValue: existingEntries.isEmpty
            ? [EntryDraft(accountID: accounts.first?.id ?? "")]
            : existingEntries
        )
        _allocations = State(initialValue: transaction?.allocations.map {
            AllocationDraft(
                categoryID: $0.categoryID,
                amount: Self.formatAmount($0.amountBaseMinor)
            )
        } ?? [])
    }

    var body: some View {
        NavigationStack {
            Form {
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
                                    ForEach(categories) { category in
                                        Text("\(category.name) · \(category.kind.title)").tag(category.id)
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
                        Text("Leave empty on an expense or income to use the protected Uncategorized category automatically.")
                    }
                }

                Section("Notes") {
                    TextField("Optional notes", text: $notes, axis: .vertical)
                        .lineLimit(3...8)
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

    private var fallbackAccountID: String {
        if entries.count < accounts.count { return accounts[entries.count].id }
        return accounts.first?.id ?? ""
    }

    private func currency(for accountID: String) -> BudgetCurrency? {
        accounts.first { $0.id == accountID }?.currency
    }

    private func save() {
        validationMessage = nil
        guard !entries.isEmpty, kind != .transfer || entries.count >= 2 else {
            validationMessage = kind == .transfer
                ? "A transfer needs at least two account entries."
                : "Add an account entry."
            return
        }
        var entryInputs: [TransactionEntryInput] = []
        for entry in entries {
            guard !entry.accountID.isEmpty,
                  let amount = Self.parseAmount(entry.amount), amount != 0 else {
                validationMessage = "Every entry needs an account and a non-zero amount with at most two decimals."
                return
            }
            let baseAmount: Int64?
            if entry.baseAmount.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                baseAmount = nil
            } else if let parsed = Self.parseAmount(entry.baseAmount) {
                guard parsed == 0 || (parsed < 0) == (amount < 0) else {
                    validationMessage = "An entry and its base amount must have the same sign."
                    return
                }
                baseAmount = parsed
            } else {
                validationMessage = "A manual base amount must use at most two decimals."
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
                guard !allocation.categoryID.isEmpty,
                      let amount = Self.parseAmount(allocation.amount), amount != 0 else {
                    validationMessage = "Every allocation needs a category and a non-zero amount with at most two decimals."
                    return
                }
                allocationInputs.append(TransactionAllocationInput(
                    categoryID: allocation.categoryID,
                    amountBaseMinor: amount
                ))
            }
        }
        let input = TransactionInput(
            kind: kind,
            status: status,
            transactionDate: Self.dateFormatter.string(from: transactionDate),
            payee: payee.nilIfBlank,
            description: description.nilIfBlank,
            notes: notes.nilIfBlank,
            entries: entryInputs,
            allocations: allocationInputs
        )
        Task {
            let saved = await model.saveTransaction(
                workspaceID: workspace.id,
                transactionID: transaction?.id,
                input: input
            )
            if saved { dismiss() }
        }
    }

    private static let dateFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        // A transaction date is a workspace/business calendar date, not a UTC instant.
        formatter.timeZone = .current
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter
    }()

    private static func parseAmount(_ input: String) -> Int64? {
        let value = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard value.range(of: #"^[+-]?(?:\d+(?:\.\d{0,2})?|\.\d{1,2})$"#, options: .regularExpression) != nil,
              let decimal = Decimal(string: value, locale: Locale(identifier: "en_US_POSIX")) else {
            return nil
        }
        var scaled = decimal * 100
        var rounded = Decimal()
        NSDecimalRound(&rounded, &scaled, 0, .plain)
        guard rounded == scaled,
              rounded >= Decimal(Int64.min), rounded <= Decimal(Int64.max) else { return nil }
        return NSDecimalNumber(decimal: rounded).int64Value
    }

    private static func formatAmount(_ minorUnits: Int64) -> String {
        NSDecimalNumber(decimal: Decimal(minorUnits) / 100).stringValue
    }
}

private struct EntryDraft: Identifiable {
    let id = UUID()
    var accountID: String
    var amount = ""
    var baseAmount = ""
}

private struct AllocationDraft: Identifiable {
    let id = UUID()
    var categoryID: String
    var amount = ""
}
