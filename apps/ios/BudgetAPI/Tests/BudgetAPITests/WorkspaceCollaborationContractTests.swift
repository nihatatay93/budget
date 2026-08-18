import Foundation
import XCTest
@testable import BudgetAPI

final class WorkspaceCollaborationContractTests: XCTestCase {
    func testDecodesAcceptedMembershipAndWorkspaceRole() throws {
        let data = Data(#"""
        {
          "workspace": {
            "id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b971",
            "name": "Family",
            "base_currency": "TRY",
            "timezone": "Europe/Istanbul",
            "role": "member"
          },
          "member": {
            "user_id": "0198bdc8-d73e-7b28-bd8e-4f29d4f4b972",
            "email": "person@example.com",
            "display_name": "Person",
            "role": "member",
            "joined_at": "2026-08-18T12:00:00Z"
          }
        }
        """#.utf8)

        let decoder = JSONDecoder()
        let dateTranscoder = FlexibleISO8601DateTranscoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let value = try decoder.singleValueContainer().decode(String.self)
            return try dateTranscoder.decode(value)
        }
        let acceptance = try decoder.decode(
            Components.Schemas.WorkspaceMembershipAcceptance.self,
            from: data
        )

        XCTAssertEqual(acceptance.workspace.name, "Family")
        XCTAssertEqual(acceptance.workspace.role, .member)
        XCTAssertEqual(acceptance.member.email, "person@example.com")
        XCTAssertEqual(acceptance.member.role, .member)
    }

    func testInvitationTokenStaysInAcceptanceBody() {
        let token = String(repeating: "a", count: 43)
        let input = Operations.acceptWorkspaceInvitation.Input(
            body: .json(.init(token: token))
        )

        guard case let .json(body) = input.body else {
            return XCTFail("Expected a JSON acceptance body")
        }
        XCTAssertEqual(body.token, token)
    }
}
