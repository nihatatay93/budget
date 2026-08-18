import Foundation
import XCTest
@testable import BudgetAPI

final class MonthlyBudgetContractTests: XCTestCase {
    func testDecodesPostedUsageAndArchivedCategoryMetadata() throws {
        let data = Data(#"""
        {
          "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b970",
          "workspace_id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b971",
          "name": "August plan",
          "month": "2026-08",
          "timezone": "Europe/Istanbul",
          "base_currency": "TRY",
          "planned_base_minor": 5000,
          "used_base_minor": 1300,
          "remaining_base_minor": 3700,
          "items": [{
            "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b972",
            "category_id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b973",
            "category_name": "Food",
            "category_archived_at": "2026-08-18T08:00:00Z",
            "planned_base_minor": 5000,
            "used_base_minor": 1300,
            "remaining_base_minor": 3700
          }],
          "created_at": "2026-08-01T08:00:00Z",
          "updated_at": "2026-08-18T08:00:00Z"
        }
        """#.utf8)

        let decoder = JSONDecoder()
        let dateTranscoder = FlexibleISO8601DateTranscoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let value = try decoder.singleValueContainer().decode(String.self)
            return try dateTranscoder.decode(value)
        }
        let budget = try decoder.decode(Components.Schemas.MonthlyBudget.self, from: data)

        XCTAssertEqual(budget.month, "2026-08")
        XCTAssertEqual(budget.used_base_minor, 1300)
        XCTAssertEqual(budget.items.first?.remaining_base_minor, 3700)
        XCTAssertNotNil(budget.items.first?.category_archived_at)
    }

    func testBudgetOperationsKeepMonthAndCompleteReplacement() {
        let workspaceID = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b971"
        let categoryID = "0198bdc8-d73e-7b28-bd8e-4f29d4f4b973"
        let get = Operations.getMonthlyBudget.Input(
            path: .init(workspaceId: workspaceID),
            query: .init(month: "2026-08")
        )
        let replace = Operations.replaceMonthlyBudget.Input(
            path: .init(workspaceId: workspaceID, month: "2026-08"),
            body: .json(.init(
                name: "August plan",
                items: [.init(category_id: categoryID, amount_base_minor: 5000)]
            ))
        )

        XCTAssertEqual(get.query.month, "2026-08")
        XCTAssertEqual(replace.path.month, "2026-08")
        guard case let .json(body) = replace.body else {
            return XCTFail("Expected a JSON replacement body")
        }
        XCTAssertEqual(body.items.first?.amount_base_minor, 5000)
    }
}
