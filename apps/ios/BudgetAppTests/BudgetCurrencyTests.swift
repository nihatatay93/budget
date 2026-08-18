import Foundation
import Testing
@testable import Budget

/// A minor-unit amount paired with the major-unit value it must render as.
struct FormattingCase: Sendable {
    let currency: BudgetCurrency
    let minorUnits: Int64
    let expected: Decimal

    static let all: [FormattingCase] = [
        FormattingCase(currency: .usDollar, minorUnits: 5_000_000, expected: Decimal(50_000)),
        FormattingCase(currency: .turkishLira, minorUnits: 125_050, expected: Decimal(string: "1250.50")!),
        FormattingCase(currency: .euro, minorUnits: 0, expected: Decimal(0)),
        FormattingCase(currency: .usDollar, minorUnits: -35_000, expected: Decimal(-350)),
    ]
}

@Suite("Budget currency")
struct BudgetCurrencyTests {
    @Test("Wire values match the OpenAPI Currency enum")
    func rawValuesMatchContract() {
        #expect(BudgetCurrency.turkishLira.rawValue == "TRY")
        #expect(BudgetCurrency.usDollar.rawValue == "USD")
        #expect(BudgetCurrency.euro.rawValue == "EUR")
        #expect(BudgetCurrency.allCases.count == 3)
    }

    @Test("An unsupported code does not decode")
    func unsupportedCodesRejected() {
        for code in ["GBP", "XYZ", "try", ""] {
            #expect(BudgetCurrency(rawValue: code) == nil)
        }
    }

    /// Minor units are the stored representation, so formatting must divide by exactly 100.
    /// Comparing against the same formatter applied to the expected major-unit value keeps
    /// the assertion about the conversion rather than about the device locale, which decides
    /// the grouping and decimal separators.
    @Test("Minor units render as major-unit amounts", arguments: FormattingCase.all)
    func formattingConvertsMinorUnits(testCase: FormattingCase) {
        #expect(
            testCase.currency.formatted(minorUnits: testCase.minorUnits)
                == testCase.expected.formatted(.currency(code: testCase.currency.rawValue))
        )
    }

    /// A negative balance must not render identically to its positive counterpart.
    @Test("Sign survives formatting")
    func signSurvivesFormatting() {
        let debit = BudgetCurrency.usDollar.formatted(minorUnits: -35_000)
        let credit = BudgetCurrency.usDollar.formatted(minorUnits: 35_000)
        #expect(debit != credit)
    }

    @Test("Every currency has a distinct human label")
    func labelsAreDistinct() {
        let titles = Set(BudgetCurrency.allCases.map(\.title))
        #expect(titles.count == BudgetCurrency.allCases.count)
    }
}
