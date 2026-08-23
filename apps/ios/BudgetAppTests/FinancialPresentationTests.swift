import Foundation
import Testing
@testable import Budget

struct FinancialPresentationTests {
    @Test func distinguishesNormalOverspentAndRefundUsage() {
        let onTrack = BudgetUsagePresentation(planned: 1_000, used: 400, remaining: 600)
        let overspent = BudgetUsagePresentation(planned: 1_000, used: 1_200, remaining: -200)
        let refund = BudgetUsagePresentation(planned: 1_000, used: -100, remaining: 1_100)
        let noTarget = BudgetUsagePresentation(planned: 0, used: 0, remaining: 0)

        #expect(onTrack.progress == 0.4)
        #expect(onTrack.state == .onTrack)
        #expect(overspent.progress == 1)
        #expect(overspent.state == .overspent)
        #expect(refund.progress == 0)
        #expect(refund.state == .refundCredit)
        #expect(noTarget.progress == 0)
        #expect(noTarget.state == .noTarget)
    }

    @Test func groupsOnlyActiveAccountsByNativeCurrency() {
        let summaries = accountCurrencySummaries([
            account(id: "try-1", currency: .turkishLira, balance: 125_000),
            account(id: "try-2", currency: .turkishLira, balance: -25_000),
            account(id: "eur-1", currency: .euro, balance: 50_000),
            account(id: "archived", currency: .euro, balance: 99_000, archived: true),
        ])

        #expect(summaries == [
            AccountCurrencySummary(currency: .euro, accountCount: 1, balanceMinor: 50_000),
            AccountCurrencySummary(currency: .turkishLira, accountCount: 2, balanceMinor: 100_000),
        ])
    }

    @Test func reportsAnUnavailableCurrencyTotalInsteadOfOverflowing() {
        let summaries = accountCurrencySummaries([
            account(id: "one", currency: .usDollar, balance: .max),
            account(id: "two", currency: .usDollar, balance: 1),
        ])

        #expect(summaries.count == 1)
        #expect(summaries.first?.balanceMinor == nil)
        #expect(summaries.first?.accountCount == 2)
    }

    private func account(
        id: String,
        currency: BudgetCurrency,
        balance: Int64,
        archived: Bool = false
    ) -> BudgetAccount {
        BudgetAccount(
            id: id,
            workspaceID: "workspace-1",
            name: id,
            type: .bank,
            currency: currency,
            institutionName: nil,
            archivedAt: archived ? Date(timeIntervalSince1970: 0) : nil,
            balanceMinor: balance
        )
    }
}
