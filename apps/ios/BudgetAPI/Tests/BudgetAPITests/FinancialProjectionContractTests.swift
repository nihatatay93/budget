import Foundation
import XCTest
@testable import BudgetAPI

final class FinancialProjectionContractTests: XCTestCase {
    func testDecodesExplicitPostedPendingAndProjectedAmounts() throws {
        let data = Data(#"""
        {
          "period": {
            "from_date": "2026-08-01",
            "to_date": "2026-08-18",
            "timezone": "Europe/Istanbul",
            "base_currency": "TRY"
          },
          "summary": {
            "balance_base_minor": {"posted": 8000, "pending": -300, "projected": 7700},
            "income_base_minor": {"posted": 5000, "pending": 0, "projected": 5000},
            "spending_base_minor": {"posted": 1300, "pending": -100, "projected": 1200}
          },
          "accounts": [{
            "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97d",
            "name": "Checking",
            "type": "bank",
            "currency": "TRY",
            "native_balance_minor": {"posted": 8000, "pending": -300, "projected": 7700},
            "base_balance_minor": {"posted": 8000, "pending": -300, "projected": 7700}
          }],
          "categories": [{
            "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97e",
            "name": "Food",
            "kind": "expense",
            "icon_type": "system",
            "icon_value": "ellipsis",
            "color_key": "slate",
            "direct_base_minor": {"posted": 1300, "pending": -100, "projected": 1200},
            "rolled_up_base_minor": {"posted": 1300, "pending": -100, "projected": 1200}
          }]
        }
        """#.utf8)

        let projection = try JSONDecoder().decode(
            Components.Schemas.FinancialProjection.self,
            from: data
        )

        XCTAssertEqual(projection.period.from_date, "2026-08-01")
        XCTAssertEqual(projection.summary.balance_base_minor.pending, -300)
        XCTAssertEqual(projection.summary.spending_base_minor.projected, 1200)
        XCTAssertEqual(projection.accounts.first?.native_balance_minor.posted, 8000)
        XCTAssertEqual(projection.categories.first?.rolled_up_base_minor.pending, -100)
    }

    func testProjectionOperationKeepsDateParametersPaired() {
        let input = Operations.getFinancialProjection.Input(
            path: .init(workspaceId: "0198bdc8-d73e-7b28-bd8e-4f29d4f4b97b"),
            query: .init(from_date: "2026-07-01", to_date: "2026-07-31")
        )

        XCTAssertEqual(input.query.from_date, "2026-07-01")
        XCTAssertEqual(input.query.to_date, "2026-07-31")
    }
}
