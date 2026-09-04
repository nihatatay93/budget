import Foundation

enum TransactionStatusScope: String, CaseIterable, Identifiable {
    case all
    case posted
    case pending

    var id: Self { self }
    var title: String { L10n.text("transaction.scope.status.\(rawValue)") }

    func includes(_ status: BudgetTransactionStatus) -> Bool {
        switch self {
        case .all: true
        case .posted: status == .posted
        case .pending: status == .pending
        }
    }
}

enum TransactionKindScope: String, CaseIterable, Identifiable {
    case all
    case standard
    case transfer
    case adjustment

    var id: Self { self }

    var title: String { L10n.text("transaction.scope.kind.\(rawValue)") }

    func includes(_ kind: BudgetTransactionKind) -> Bool {
        switch self {
        case .all: true
        case .standard: kind == .standard
        case .transfer: kind == .transfer
        case .adjustment: kind == .adjustment
        }
    }
}

struct TransactionListFilter {
    var searchText = ""
    var status = TransactionStatusScope.all
    var kind = TransactionKindScope.all

    var isActive: Bool {
        !searchText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            || status != .all
            || kind != .all
    }

    func matches(
        _ transaction: BudgetTransaction,
        accountNames: [String: String],
        categoryNames: [String: String]
    ) -> Bool {
        guard status.includes(transaction.status), kind.includes(transaction.kind) else {
            return false
        }
        let query = searchText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !query.isEmpty else { return true }

        let searchableValues = [
            transaction.payee,
            transaction.description,
            transaction.notes,
            transaction.transactionDate,
            transaction.kind.title,
            transaction.status.title,
        ].compactMap { $0 }
            + transaction.entries.compactMap { accountNames[$0.accountID] }
            + transaction.allocations.compactMap { categoryNames[$0.categoryID] }

        // `localizedCaseInsensitiveContains` folds case with the current locale. Under Turkish
        // that maps "I" to dotless "ı", so searching "GROCERIES" stops matching "groceries" —
        // and searching "istanbul" stops matching "İstanbul". A search box wants the opposite:
        // passing `locale: nil` folds case invariantly, and folding diacritics too lets someone
        // type without Turkish characters and still find the row.
        return searchableValues.contains {
            $0.range(
                of: query,
                options: [.caseInsensitive, .diacriticInsensitive],
                range: nil,
                locale: nil
            ) != nil
        }
    }
}
