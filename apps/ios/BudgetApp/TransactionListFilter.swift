import Foundation

enum TransactionStatusScope: String, CaseIterable, Identifiable {
    case all
    case posted
    case pending

    var id: Self { self }
    var title: String { rawValue.capitalized }

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

    var title: String {
        switch self {
        case .all: "All kinds"
        case .standard: "Expense or income"
        case .transfer: "Transfers"
        case .adjustment: "Adjustments"
        }
    }

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

        return searchableValues.contains {
            $0.localizedCaseInsensitiveContains(query)
        }
    }
}
