import Testing
@testable import Budget

struct TransactionListFilterTests {
    private let pendingExpense = BudgetTransaction(
        id: "transaction-1",
        workspaceID: "workspace-1",
        kind: .standard,
        status: .pending,
        transactionDate: "2026-08-23",
        payee: "Neighbourhood Market",
        description: "Weekly groceries",
        notes: "Paid at checkout",
        source: "manual",
        entries: [
            BudgetTransactionEntry(
                accountID: "account-1",
                amountMinor: -18_500,
                baseAmountMinor: -18_500
            ),
        ],
        allocations: [
            BudgetTransactionAllocation(categoryID: "category-1", amountBaseMinor: -18_500),
        ]
    )

    @Test func filtersByStructuralStatusAndKind() {
        #expect(TransactionListFilter(status: .pending).matches(
            pendingExpense,
            accountNames: [:],
            categoryNames: [:]
        ))
        #expect(!TransactionListFilter(status: .posted).matches(
            pendingExpense,
            accountNames: [:],
            categoryNames: [:]
        ))
        #expect(!TransactionListFilter(kind: .transfer).matches(
            pendingExpense,
            accountNames: [:],
            categoryNames: [:]
        ))
    }

    @Test(arguments: ["market", "GROCERIES", "checkout", "everyday", "food", "2026-08"])
    func searchesLedgerAndRelatedResourceLabels(query: String) {
        let filter = TransactionListFilter(searchText: query)

        #expect(filter.matches(
            pendingExpense,
            accountNames: ["account-1": "Everyday account"],
            categoryNames: ["category-1": "Food"]
        ))
    }

    /// Turkish lowercases "I" to dotless "ı", so locale-sensitive folding breaks Latin search
    /// terms for exactly the people the Turkish translation is for.
    /// Dotless "ı" is a separate Turkish letter rather than a stripped "i", so it is correctly
    /// left out here: folding it into "i" would make "kısa" match "kisa".
    @Test(arguments: ["ISTANBUL", "istanbul", "İSTANBUL", "Istanbul"])
    func searchFoldsCaseWithoutApplyingTurkishDottedI(query: String) {
        let transaction = BudgetTransaction(
            id: "transaction-2",
            workspaceID: "workspace-1",
            kind: .standard,
            status: .posted,
            transactionDate: "2026-08-23",
            payee: "İstanbul Kitapçısı",
            description: nil,
            notes: nil,
            source: "manual",
            entries: [],
            allocations: []
        )

        #expect(TransactionListFilter(searchText: query).matches(
            transaction,
            accountNames: [:],
            categoryNames: [:]
        ))
    }

    @Test func reportsWhetherTheUserHasNarrowedTheRegister() {
        #expect(!TransactionListFilter(searchText: "   ").isActive)
        #expect(TransactionListFilter(status: .pending).isActive)
        #expect(TransactionListFilter(kind: .adjustment).isActive)
        #expect(TransactionListFilter(searchText: "salary").isActive)
    }
}
