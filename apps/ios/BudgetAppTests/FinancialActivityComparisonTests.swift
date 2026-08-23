import Testing
@testable import Budget

struct FinancialActivityComparisonTests {
    @Test func scalesIncomeAndSpendingAgainstTheLargerExactAmount() {
        let comparison = FinancialActivityComparison(income: 2_000, spending: 500)

        #expect(comparison.incomeProgress == 1)
        #expect(comparison.spendingProgress == 0.25)
    }

    @Test func keepsAnEmptyPeriodFiniteAndEmpty() {
        let comparison = FinancialActivityComparison(income: 0, spending: 0)

        #expect(comparison.incomeProgress == 0)
        #expect(comparison.spendingProgress == 0)
    }

    @Test func safelyScalesTheFullMinorUnitRange() {
        let comparison = FinancialActivityComparison(income: .min, spending: .max)

        #expect(comparison.incomeProgress == 1)
        #expect(comparison.spendingProgress > 0.99)
        #expect(comparison.spendingProgress <= 1)
    }
}
