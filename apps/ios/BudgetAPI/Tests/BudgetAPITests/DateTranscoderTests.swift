import Foundation
import XCTest
@testable import BudgetAPI

final class DateTranscoderTests: XCTestCase {
    func testDecodesPostgreSQLMicrosecondsAndWholeSeconds() throws {
        let transcoder = FlexibleISO8601DateTranscoder()
        let fractional = try transcoder.decode("2026-08-18T07:28:45.372043Z")
        let wholeSeconds = try transcoder.decode("2026-08-18T07:28:45Z")

        XCTAssertEqual(
            fractional.timeIntervalSince(wholeSeconds),
            0.372,
            accuracy: 0.001
        )
        XCTAssertThrowsError(try transcoder.decode("not-a-timestamp")) { error in
            XCTAssertTrue(error is DecodingError)
        }
    }
}
