import Foundation
import Testing
@testable import Budget

@Suite("Workspace month")
struct WorkspaceMonthTests {
    private func date(_ iso: String) -> Date {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.date(from: iso)!
    }

    /// Reporting months follow the workspace timezone, not UTC. At 22:30 UTC on 31 August a
    /// workspace in Istanbul is already in September, and using UTC would file the period
    /// under the wrong month.
    @Test("Month key follows the workspace timezone across a boundary")
    func monthKeyUsesWorkspaceTimezone() {
        let instant = date("2026-08-31T22:30:00Z")
        #expect(workspaceMonthKey(instant, timezone: "Europe/Istanbul") == "2026-09")
        #expect(workspaceMonthKey(instant, timezone: "UTC") == "2026-08")
    }

    @Test("Month start is the first instant of the workspace month")
    func monthStartIsFirstOfMonth() {
        let instant = date("2026-08-17T09:15:00Z")
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "Europe/Istanbul")!
        let start = workspaceMonthStart(instant, timezone: "Europe/Istanbul")
        let parts = calendar.dateComponents([.year, .month, .day, .hour, .minute], from: start)
        #expect(parts.year == 2026)
        #expect(parts.month == 8)
        #expect(parts.day == 1)
        #expect(parts.hour == 0)
        #expect(parts.minute == 0)
    }

    /// An unknown identifier must not crash; falling back to the device timezone keeps the
    /// screen usable.
    @Test("An unknown timezone falls back instead of failing")
    func unknownTimezoneFallsBack() {
        #expect(workspaceMonthKey(date("2026-08-17T09:15:00Z"), timezone: "Not/AZone").count == 7)
    }

    @Test("Month labels are derived from the key and survive bad input")
    func monthLabelHandlesInput() {
        #expect(budgetMonthLabel("2026-08").contains("2026"))
        #expect(budgetMonthLabel("nonsense") == "nonsense")
    }
}
