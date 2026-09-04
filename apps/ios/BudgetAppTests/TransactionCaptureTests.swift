import Foundation
import Testing
@testable import Budget

/// The capture rules are the same contract the web client implements in
/// `apps/web/src/lib/transactionCapture.ts`: one unsigned amount becomes a signed entry and a
/// matching allocation, and anything the simple form cannot reproduce has no draft at all.
@Suite("Transaction capture")
struct TransactionCaptureTests {
    private let context = CaptureContext(
        accounts: [
            TransactionCaptureTests.account(id: "everyday", currency: .turkishLira),
            TransactionCaptureTests.account(id: "savings", currency: .turkishLira),
            TransactionCaptureTests.account(id: "travel", currency: .euro),
        ],
        categories: [
            TransactionCaptureTests.category(id: "food", kind: .expense),
            TransactionCaptureTests.category(id: "groceries", kind: .expense, parentID: "food"),
            TransactionCaptureTests.category(id: "salary", kind: .income),
            TransactionCaptureTests.category(id: "uncategorized", kind: .expense, systemKey: "uncategorized_expense"),
        ],
        baseCurrency: .turkishLira
    )

    private var draft: CaptureDraft {
        var value = CaptureDraft()
        value.accountID = "everyday"
        value.transactionDate = TransactionCaptureTests.date("2026-08-26")
        return value
    }

    @Test("An expense is signed once and mirrored into its allocation")
    func expenseDerivesEntryAndAllocation() throws {
        var value = draft
        value.amount = "185.00"
        value.categoryID = "groceries"

        let input = try #require(try capture(value).get())
        #expect(input.kind == .standard)
        #expect(input.status == .posted)
        #expect(input.transactionDate == "2026-08-26")
        #expect(input.entries.count == 1)
        #expect(input.entries[0].accountID == "everyday")
        #expect(input.entries[0].amountMinor == -18_500)
        #expect(input.entries[0].baseAmountMinor == nil)
        #expect(input.allocations.count == 1)
        #expect(input.allocations[0].categoryID == "groceries")
        #expect(input.allocations[0].amountBaseMinor == -18_500)
    }

    @Test("Income is positive and a pending draft stays pending")
    func incomeKeepsItsSignAndStatus() throws {
        var value = draft
        value.type = .income
        value.amount = "22000"
        value.categoryID = "salary"
        value.pending = true
        value.payee = "  Employer  "

        let input = try #require(try capture(value).get())
        #expect(input.status == .pending)
        #expect(input.payee == "Employer")
        #expect(input.entries[0].amountMinor == 2_200_000)
        #expect(input.allocations[0].amountBaseMinor == 2_200_000)
    }

    @Test("No category leaves allocations to the server's uncategorized rule")
    func missingCategoryLeavesAllocationsEmpty() throws {
        var value = draft
        value.amount = "12.5"

        let input = try #require(try capture(value).get())
        #expect(input.allocations.isEmpty)
        #expect(input.entries[0].amountMinor == -1_250)
    }

    @Test("A foreign amount is left to the transaction date's rate unless it is stated")
    func foreignAccountMayDeferItsBaseAmount() throws {
        var value = draft
        value.accountID = "travel"
        value.amount = "40"
        value.categoryID = "food"

        // Neither the entry nor its allocation carries a base amount, so the server books both
        // at the rate for that date — the only place that rate is known.
        let deferred = try #require(try capture(value).get())
        #expect(deferred.entries[0].amountMinor == -4_000)
        #expect(deferred.entries[0].baseAmountMinor == nil)
        #expect(deferred.allocations[0].amountBaseMinor == nil)

        value.baseAmount = "1500"
        let input = try #require(try capture(value).get())
        #expect(input.entries[0].amountMinor == -4_000)
        #expect(input.entries[0].baseAmountMinor == -150_000)
        #expect(input.allocations[0].amountBaseMinor == -150_000)

        value.baseAmount = "nonsense"
        #expect(refusal(value) == .baseAmount(.turkishLira))
    }

    @Test("A transfer balances two accounts and allocates nothing")
    func transferBalancesTwoAccounts() throws {
        var value = draft
        value.type = .transfer
        value.amount = "1000"
        value.toAccountID = "savings"

        let input = try #require(try capture(value).get())
        #expect(input.kind == .transfer)
        #expect(input.entries.map(\.amountMinor) == [-100_000, 100_000])
        #expect(input.allocations.isEmpty)
    }

    @Test("Every draft the form cannot send is refused with a reason")
    func refusesIncompleteDrafts() {
        var zero = draft
        zero.amount = "0"
        var imprecise = draft
        imprecise.amount = "1.005"
        var accountless = draft
        accountless.amount = "10"
        accountless.accountID = ""
        var sameAccount = draft
        sameAccount.type = .transfer
        sameAccount.amount = "10"
        sameAccount.toAccountID = "everyday"
        var mixedCurrency = draft
        mixedCurrency.type = .transfer
        mixedCurrency.amount = "10"
        mixedCurrency.toAccountID = "travel"

        #expect(refusal(zero) == .amount)
        #expect(refusal(imprecise) == .amount)
        #expect(refusal(accountless) == .account)
        #expect(refusal(sameAccount) == .transferAccounts)
        #expect(refusal(mixedCurrency) == .mixedCurrencyTransfer)
    }

    @Test("A categorized expense round-trips through the simple form")
    func expenseRoundTrips() throws {
        let saved = transaction(
            entries: [BudgetTransactionEntry(accountID: "everyday", amountMinor: -18_500, baseAmountMinor: -18_500)],
            allocations: [BudgetTransactionAllocation(categoryID: "groceries", amountBaseMinor: -18_500)]
        )

        let value = try #require(draft(from: saved))
        #expect(value.type == .expense)
        #expect(value.amount == "185")
        #expect(value.categoryID == "groceries")
        #expect(value.accountID == "everyday")
        #expect(try capture(value).get().entries[0].amountMinor == -18_500)
    }

    @Test("A protected uncategorized allocation reads as no category")
    func uncategorizedAllocationClearsTheCategory() throws {
        let saved = transaction(
            entries: [BudgetTransactionEntry(accountID: "everyday", amountMinor: -8_500, baseAmountMinor: -8_500)],
            allocations: [BudgetTransactionAllocation(categoryID: "uncategorized", amountBaseMinor: -8_500)]
        )

        #expect(try #require(draft(from: saved)).categoryID == "")
    }

    @Test("A balanced same-currency transfer keeps both of its accounts")
    func transferRoundTrips() throws {
        let saved = transaction(
            kind: .transfer,
            entries: [
                BudgetTransactionEntry(accountID: "everyday", amountMinor: -100_000, baseAmountMinor: -100_000),
                BudgetTransactionEntry(accountID: "savings", amountMinor: 100_000, baseAmountMinor: 100_000),
            ]
        )

        let value = try #require(draft(from: saved))
        #expect(value.type == .transfer)
        #expect(value.accountID == "everyday")
        #expect(value.toAccountID == "savings")
        #expect(value.amount == "1000")
    }

    @Test("Anything the detailed editor owns has no simple draft")
    func detailedTransactionsHaveNoDraft() {
        let split = transaction(
            entries: [BudgetTransactionEntry(accountID: "everyday", amountMinor: -10_000, baseAmountMinor: -10_000)],
            allocations: [
                BudgetTransactionAllocation(categoryID: "food", amountBaseMinor: -6_000),
                BudgetTransactionAllocation(categoryID: "groceries", amountBaseMinor: -4_000),
            ]
        )
        let refund = transaction(
            entries: [BudgetTransactionEntry(accountID: "everyday", amountMinor: 5_000, baseAmountMinor: 5_000)],
            allocations: [BudgetTransactionAllocation(categoryID: "food", amountBaseMinor: 5_000)]
        )
        let twoEntries = transaction(
            entries: [
                BudgetTransactionEntry(accountID: "everyday", amountMinor: -6_000, baseAmountMinor: -6_000),
                BudgetTransactionEntry(accountID: "savings", amountMinor: -4_000, baseAmountMinor: -4_000),
            ]
        )
        let mixedTransfer = transaction(
            kind: .transfer,
            entries: [
                BudgetTransactionEntry(accountID: "everyday", amountMinor: -100_000, baseAmountMinor: -100_000),
                BudgetTransactionEntry(accountID: "travel", amountMinor: 2_600, baseAmountMinor: 100_000),
            ]
        )

        #expect(draft(from: transaction(kind: .adjustment)) == nil)
        #expect(draft(from: split) == nil)
        #expect(draft(from: refund) == nil)
        #expect(draft(from: twoEntries) == nil)
        #expect(draft(from: mixedTransfer) == nil)
    }

    @Test("A branch is offered by its root and read depth-first")
    func categoryBranchesFollowTheHierarchy() {
        #expect(CategoryHierarchy.root(of: "groceries", in: context.categories)?.id == "food")
        #expect(CategoryHierarchy.root(of: "food", in: context.categories)?.id == "food")
        #expect(CategoryHierarchy.root(of: "missing", in: context.categories) == nil)

        let nested = context.categories + [Self.category(id: "markets", kind: .expense, parentID: "groceries")]
        let rows = CategoryHierarchy.branch(of: "food", in: nested)
        #expect(rows.map(\.category.id) == ["groceries", "markets"])
        #expect(rows.map(\.depth) == [0, 1])
    }

    @Test("Each root heads a section that opens with the root itself")
    func sectionsOpenWithTheirRoot() {
        let named: (BudgetCategory) -> String = { $0.name }
        let expense = context.categories.filter { $0.kind == .expense && $0.systemKey == nil }

        let sections = CategoryHierarchy.sections(of: expense, by: named)
        #expect(sections.map(\.root.id) == ["food"])
        #expect(sections[0].members.map(\.category.id) == ["food", "groceries"])
        #expect(sections[0].members.map(\.depth) == [0, 1])

        // A category whose parent is filtered out of the set reads as a root of its own, so an
        // archived or wrong-kind parent never hides its children.
        let orphaned = expense.filter { $0.id != "food" }
        #expect(CategoryHierarchy.sections(of: orphaned, by: named).map(\.root.id) == ["groceries"])
    }

    @Test("The most-used list ranks categories by how often they are allocated to")
    func frequentCategoriesFollowActivity() {
        let used = [
            transaction(date: "2026-08-26", allocations: [BudgetTransactionAllocation(categoryID: "food", amountBaseMinor: -100)]),
            transaction(date: "2026-08-25", allocations: [BudgetTransactionAllocation(categoryID: "food", amountBaseMinor: -100)]),
            transaction(date: "2026-08-24", allocations: [BudgetTransactionAllocation(categoryID: "salary", amountBaseMinor: 100)]),
        ]

        #expect(CategoryHierarchy.frequentCategoryIDs(in: used) == ["food", "salary"])
        #expect(CategoryHierarchy.frequentCategoryIDs(in: used, limit: 1) == ["food"])
        #expect(CategoryHierarchy.frequentCategoryIDs(in: []).isEmpty)
    }

    @Test("The suggested account is the one most recently used")
    func suggestsTheMostRecentAccount() {
        let older = transaction(
            date: "2026-08-01",
            entries: [BudgetTransactionEntry(accountID: "savings", amountMinor: -100, baseAmountMinor: -100)]
        )
        let newer = transaction(
            date: "2026-08-20",
            entries: [BudgetTransactionEntry(accountID: "travel", amountMinor: -100, baseAmountMinor: -100)]
        )

        #expect(TransactionCapture.suggestedAccountID(
            transactions: [older, newer],
            accounts: context.accounts
        ) == "travel")
        #expect(TransactionCapture.suggestedAccountID(
            transactions: [],
            accounts: context.accounts
        ) == "everyday")
        #expect(TransactionCapture.suggestedAccountID(transactions: [], accounts: []) == "")
    }

    // MARK: - Fixtures

    private func capture(_ value: CaptureDraft) -> Result<TransactionInput, TransactionCapture.DraftError> {
        TransactionCapture.input(from: value, context: context, dateFormatter: Self.formatter)
    }

    /// `Result` equality would need an equatable `TransactionInput`, so refusals are read alone.
    private func refusal(_ value: CaptureDraft) -> TransactionCapture.DraftError? {
        guard case let .failure(error) = capture(value) else { return nil }
        return error
    }

    private func draft(from transaction: BudgetTransaction) -> CaptureDraft? {
        TransactionCapture.draft(from: transaction, context: context, dateFormatter: Self.formatter)
    }

    private func transaction(
        kind: BudgetTransactionKind = .standard,
        date: String = "2026-08-26",
        entries: [BudgetTransactionEntry] = [
            BudgetTransactionEntry(accountID: "everyday", amountMinor: -100, baseAmountMinor: -100),
        ],
        allocations: [BudgetTransactionAllocation] = []
    ) -> BudgetTransaction {
        BudgetTransaction(
            id: "transaction",
            workspaceID: "workspace",
            kind: kind,
            status: .posted,
            transactionDate: date,
            payee: nil,
            description: nil,
            notes: nil,
            source: "manual",
            entries: entries,
            allocations: allocations
        )
    }

    private static func account(id: String, currency: BudgetCurrency) -> BudgetAccount {
        BudgetAccount(
            id: id,
            workspaceID: "workspace",
            name: id,
            type: .bank,
            currency: currency,
            institutionName: nil,
            archivedAt: nil,
            balanceMinor: 0
        )
    }

    private static func category(
        id: String,
        kind: BudgetCategoryKind,
        parentID: String? = nil,
        systemKey: String? = nil
    ) -> BudgetCategory {
        BudgetCategory(
            id: id,
            workspaceID: "workspace",
            parentID: parentID,
            name: id,
            kind: kind,
            isSystem: systemKey != nil,
            systemKey: systemKey,
            predefinedKey: nil,
            iconType: "system",
            iconValue: "ellipsis",
            colorKey: "slate",
            archivedAt: nil
        )
    }

    private static func date(_ value: String) -> Date {
        formatter.date(from: value) ?? Date()
    }

    private static let formatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = .current
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter
    }()
}
