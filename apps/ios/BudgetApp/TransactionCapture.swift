import Foundation

/// The everyday capture form collects what a person knows — what it was, how much, which
/// category, when — and derives the transaction aggregate documented in docs/domain-model.md
/// from it. The detailed editor still writes entries and allocations directly.
///
/// Two rules make the derivation safe. The amount is typed once without a sign, so an entry and
/// its allocation can never disagree and reconciliation cannot fail from this form. And a
/// transaction the draft cannot express — a split, a balance adjustment, more than one account
/// entry, a mixed-currency transfer, or a refund allocated against an expense category — has no
/// draft at all, which is what sends the detailed editor to the screen instead of flattening it.
///
/// This mirrors `apps/web/src/lib/transactionCapture.ts`; the two clients must derive the same
/// aggregate from the same answers.
enum CaptureType: String, CaseIterable, Identifiable, Sendable {
    case expense
    case income
    case transfer

    var id: Self { self }

    var title: String {
        switch self {
        case .expense: return L10n.text("Expense")
        case .income: return L10n.text("Income")
        case .transfer: return L10n.text("Transfer")
        }
    }
}

struct CaptureDraft: Equatable, Sendable {
    var type: CaptureType = .expense
    /// Unsigned major-unit amount in the chosen account's currency.
    var amount = ""
    /// Unsigned major-unit amount in the workspace base currency, for a foreign-currency
    /// account. Optional: left empty, the server books the transaction date's rate and derives
    /// the allocation from it.
    var baseAmount = ""
    /// Empty means the server applies the protected uncategorized category.
    var categoryID = ""
    var accountID = ""
    /// Destination account for a transfer.
    var toAccountID = ""
    var transactionDate = Date()
    var payee = ""
    var summary = ""
    var notes = ""
    var pending = false
}

struct CaptureContext {
    let accounts: [BudgetAccount]
    let categories: [BudgetCategory]
    let baseCurrency: BudgetCurrency
}

enum TransactionCapture {
    enum DraftError: Error, Equatable {
        case amount
        case account
        case transferAccounts
        case mixedCurrencyTransfer
        case baseAmount(BudgetCurrency)

        var message: String {
            switch self {
            case .amount:
                return L10n.text("Enter an amount above zero with at most two decimals.")
            case .account:
                return L10n.text("Choose an account.")
            case .transferAccounts:
                return L10n.text("Choose two different accounts for a transfer.")
            case .mixedCurrencyTransfer:
                return L10n.text("A transfer between two currencies needs the detailed editor.")
            case let .baseAmount(currency):
                return String(
                    format: L10n.text("Enter the %@ value of this amount as well."),
                    currency.rawValue
                )
            }
        }
    }

    /// The account of the most recent transaction, which is the one a person is most likely to use again.
    static func suggestedAccountID(
        transactions: [BudgetTransaction],
        accounts: [BudgetAccount]
    ) -> String {
        let usable = Set(accounts.map(\.id))
        let recent = transactions
            .sorted { $0.transactionDate > $1.transactionDate }
            .flatMap { $0.entries.map(\.accountID) }
            .first { usable.contains($0) }
        return recent ?? accounts.first?.id ?? ""
    }

    static func input(
        from draft: CaptureDraft,
        context: CaptureContext,
        dateFormatter: DateFormatter
    ) -> Result<TransactionInput, DraftError> {
        guard let amount = parseAmount(draft.amount), amount != 0 else { return .failure(.amount) }
        let magnitude = abs(amount)
        guard let account = context.accounts.first(where: { $0.id == draft.accountID }) else {
            return .failure(.account)
        }
        let date = dateFormatter.string(from: draft.transactionDate)
        let status: BudgetTransactionStatus = draft.pending ? .pending : .posted

        if draft.type == .transfer {
            guard let destination = context.accounts.first(where: { $0.id == draft.toAccountID }),
                  destination.id != account.id else {
                return .failure(.transferAccounts)
            }
            guard destination.currency == account.currency else {
                return .failure(.mixedCurrencyTransfer)
            }
            return .success(TransactionInput(
                kind: .transfer,
                status: status,
                transactionDate: date,
                payee: draft.payee.nilIfBlank,
                description: draft.summary.nilIfBlank,
                notes: draft.notes.nilIfBlank,
                entries: [
                    TransactionEntryInput(accountID: account.id, amountMinor: -magnitude, baseAmountMinor: nil),
                    TransactionEntryInput(accountID: destination.id, amountMinor: magnitude, baseAmountMinor: nil),
                ],
                allocations: []
            ))
        }

        let direction: Int64 = draft.type == .expense ? -1 : 1
        let signed = direction * magnitude
        // A foreign account may state its base-currency value, and must when the deployment has
        // no rate feed. Left empty, both the entry and its allocation are booked from the
        // transaction date's rate on the server, which is the only place that rate is known.
        var baseMinor: Int64?
        if account.currency == context.baseCurrency {
            baseMinor = signed
        } else if !draft.baseAmount.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            guard let base = parseAmount(draft.baseAmount), base != 0 else {
                return .failure(.baseAmount(context.baseCurrency))
            }
            baseMinor = direction * abs(base)
        }
        return .success(TransactionInput(
            kind: .standard,
            status: status,
            transactionDate: date,
            payee: draft.payee.nilIfBlank,
            description: draft.summary.nilIfBlank,
            notes: draft.notes.nilIfBlank,
            entries: [TransactionEntryInput(
                accountID: account.id,
                amountMinor: signed,
                baseAmountMinor: baseMinor == signed ? nil : baseMinor
            )],
            allocations: draft.categoryID.isEmpty
                ? []
                : [TransactionAllocationInput(categoryID: draft.categoryID, amountBaseMinor: baseMinor)]
        ))
    }

    /// The draft that reproduces this transaction, or nil when only the detailed editor can.
    static func draft(
        from transaction: BudgetTransaction,
        context: CaptureContext,
        dateFormatter: DateFormatter
    ) -> CaptureDraft? {
        guard transaction.kind != .adjustment else { return nil }
        var draft = CaptureDraft()
        draft.transactionDate = dateFormatter.date(from: transaction.transactionDate) ?? Date()
        draft.payee = transaction.payee ?? ""
        draft.summary = transaction.description ?? ""
        draft.notes = transaction.notes ?? ""
        draft.pending = transaction.status == .pending

        if transaction.kind == .transfer {
            guard transaction.entries.count == 2, transaction.allocations.isEmpty,
                  let source = transaction.entries.first(where: { $0.amountMinor < 0 }),
                  let destination = transaction.entries.first(where: { $0.amountMinor > 0 }),
                  source.amountMinor == -destination.amountMinor,
                  let from = context.accounts.first(where: { $0.id == source.accountID }),
                  let to = context.accounts.first(where: { $0.id == destination.accountID }),
                  from.currency == to.currency else { return nil }
            draft.type = .transfer
            draft.amount = formatAmount(abs(source.amountMinor))
            draft.accountID = from.id
            draft.toAccountID = to.id
            return draft
        }

        guard transaction.entries.count == 1, transaction.allocations.count <= 1,
              let entry = transaction.entries.first,
              entry.amountMinor != 0,
              (entry.amountMinor < 0) == (entry.baseAmountMinor < 0),
              let account = context.accounts.first(where: { $0.id == entry.accountID })
        else { return nil }
        draft.type = entry.baseAmountMinor < 0 ? .expense : .income

        if let allocation = transaction.allocations.first {
            guard allocation.amountBaseMinor == entry.baseAmountMinor,
                  let category = context.categories.first(where: { $0.id == allocation.categoryID }),
                  // A refund allocated to an expense category, or a reversal against an income
                  // one, is a valid ledger entry the simple form has no way to offer.
                  category.kind.rawValue == draft.type.rawValue
            else { return nil }
            draft.categoryID = category.systemKey == nil ? category.id : ""
        }
        draft.amount = formatAmount(abs(entry.amountMinor))
        draft.baseAmount = account.currency == context.baseCurrency
            ? ""
            : formatAmount(abs(entry.baseAmountMinor))
        draft.accountID = account.id
        return draft
    }

    static func parseAmount(_ input: String) -> Int64? {
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

    static func formatAmount(_ minorUnits: Int64) -> String {
        NSDecimalNumber(decimal: Decimal(minorUnits) / 100).stringValue
    }
}
