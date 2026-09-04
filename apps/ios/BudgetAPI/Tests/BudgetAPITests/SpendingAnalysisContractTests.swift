import Foundation
import XCTest
@testable import BudgetAPI

final class SpendingAnalysisContractTests: XCTestCase {
    /// The client decodes timestamps through FlexibleISO8601DateTranscoder, so a test using a
    /// bare JSONDecoder would reject payloads the app accepts. This mirrors the real path.
    private func analysisDecoder() -> JSONDecoder {
        let transcoder = FlexibleISO8601DateTranscoder()
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            try transcoder.decode(decoder.singleValueContainer().decode(String.self))
        }
        return decoder
    }

    /// Analysis is posted-only, so a decoded payload must carry plain amounts rather than the
    /// posted/pending/projected triples the projection uses. It must also carry the comparison
    /// window, which is what makes period-over-period movement meaningful.
    func testDecodesPostedOnlyAmountsAndTheComparisonWindow() throws {
        let data = Data(#"""
        {
          "period": {
            "from_date": "2026-08-01",
            "to_date": "2026-08-31",
            "comparison_from_date": "2026-07-01",
            "comparison_to_date": "2026-07-31",
            "granularity": "week",
            "timezone": "Europe/Istanbul",
            "base_currency": "TRY"
          },
          "totals": {
            "income_base_minor": 500000,
            "spending_base_minor": 320000,
            "net_base_minor": 180000,
            "comparison_income_base_minor": 480000,
            "comparison_spending_base_minor": 350000,
            "comparison_net_base_minor": 130000,
            "transaction_count": 42,
            "spending_transaction_count": 33,
            "largest_spending_base_minor": 90000,
            "spending_day_count": 18,
            "day_count": 31
          },
          "series": [{
            "start_date": "2026-08-01",
            "end_date": "2026-08-02",
            "income_base_minor": 500000,
            "spending_base_minor": 120000,
            "net_base_minor": 380000,
            "transaction_count": 12
          }],
          "categories": [{
            "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97e",
            "parent_id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97f",
            "name": "Restaurants",
            "kind": "expense",
            "system_key": "uncategorized_expense",
            "predefined_key": "dining",
            "icon_type": "system",
            "icon_value": "fork.knife",
            "color_key": "orange",
            "archived_at": "2026-07-01T00:00:00Z",
            "direct_base_minor": 200000,
            "rolled_up_base_minor": 260000,
            "comparison_direct_base_minor": 150000,
            "comparison_rolled_up_base_minor": 190000,
            "transaction_count": 20,
            "rolled_up_transaction_count": 26,
            "largest_base_minor": 45000,
            "first_date": "2026-08-03",
            "last_date": "2026-08-29"
          }],
          "category_series": [{
            "category_id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97e",
            "start_date": "2026-08-01",
            "base_minor": 200000
          }],
          "weekdays": [{
            "weekday": 6,
            "income_base_minor": 0,
            "spending_base_minor": 140000,
            "transaction_count": 9
          }],
          "days": [{
            "date": "2026-08-01",
            "income_base_minor": 500000,
            "spending_base_minor": 4000,
            "transaction_count": 2
          }],
          "payees": [{
            "payee": "Migros",
            "spending_base_minor": 88000,
            "income_base_minor": 0,
            "transaction_count": 7,
            "first_date": "2026-08-01",
            "last_date": "2026-08-31"
          }],
          "accounts": [{
            "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97d",
            "name": "Checking",
            "type": "bank",
            "currency": "TRY",
            "outflow_base_minor": 320000,
            "inflow_base_minor": 500000,
            "transaction_count": 42
          }]
        }
        """#.utf8)

        let analysis = try analysisDecoder().decode(
            Components.Schemas.SpendingAnalysis.self,
            from: data
        )

        XCTAssertEqual(analysis.period.comparison_from_date, "2026-07-01")
        XCTAssertEqual(analysis.period.comparison_to_date, "2026-07-31")
        XCTAssertEqual(analysis.period.granularity, .week)
        XCTAssertEqual(analysis.totals.comparison_spending_base_minor, 350000)
        XCTAssertEqual(analysis.totals.day_count, 31)
        XCTAssertEqual(analysis.series.first?.end_date, "2026-08-02")
        XCTAssertEqual(analysis.categories.first?.rolled_up_base_minor, 260000)
        XCTAssertEqual(analysis.categories.first?.parent_id, "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97f")
        XCTAssertEqual(analysis.categories.first?.system_key?.value1, .uncategorized_expense)
        XCTAssertEqual(analysis.categories.first?.first_date, "2026-08-03")
        XCTAssertNotNil(analysis.categories.first?.archived_at)
        XCTAssertEqual(analysis.category_series.first?.base_minor, 200000)
        XCTAssertEqual(analysis.weekdays.first?.weekday, 6)
        XCTAssertEqual(analysis.days.first?.date, "2026-08-01")
        XCTAssertEqual(analysis.payees.first?.payee, "Migros")
        XCTAssertEqual(analysis.accounts.first?.outflow_base_minor, 320000)
    }

    /// The optional fields are genuinely optional: a top-level category has no parent, a
    /// category with no activity has no first or last date, and an active account is not
    /// archived. A payload omitting them must still decode.
    func testDecodesACategoryWithoutOptionalFields() throws {
        let data = Data(#"""
        {
          "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97e",
          "name": "Housing",
          "kind": "expense",
          "icon_type": "system",
          "icon_value": "house",
          "color_key": "slate",
          "direct_base_minor": 0,
          "rolled_up_base_minor": 0,
          "comparison_direct_base_minor": 0,
          "comparison_rolled_up_base_minor": 0,
          "transaction_count": 0,
          "rolled_up_transaction_count": 0,
          "largest_base_minor": 0
        }
        """#.utf8)

        let category = try analysisDecoder().decode(
            Components.Schemas.SpendingAnalysisCategory.self,
            from: data
        )

        XCTAssertNil(category.parent_id)
        XCTAssertNil(category.first_date)
        XCTAssertNil(category.last_date)
        XCTAssertNil(category.archived_at)
        XCTAssertNil(category.system_key)
    }

    /// Omitting the query entirely is how the trailing-year default is requested, and the
    /// granularity is optional independently of the dates.
    func testAnalysisOperationLeavesEveryQueryParameterOptional() {
        let defaulted = Operations.getSpendingAnalysis.Input(
            path: .init(workspaceId: "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97b")
        )
        XCTAssertNil(defaulted.query.from_date)
        XCTAssertNil(defaulted.query.to_date)
        XCTAssertNil(defaulted.query.granularity)

        let explicit = Operations.getSpendingAnalysis.Input(
            path: .init(workspaceId: "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97b"),
            query: .init(from_date: "2026-08-01", to_date: "2026-08-31", granularity: .month)
        )
        XCTAssertEqual(explicit.query.from_date, "2026-08-01")
        XCTAssertEqual(explicit.query.granularity, .month)
    }
}
