import Testing
@testable import Budget

struct WorkspaceTabTests {
    @Test func exposesTheStableWorkspaceDestinationsInPriorityOrder() {
        #expect(WorkspaceTab.allCases == [
            .overview,
            .transactions,
            .budget,
            .accounts,
            .more,
        ])
        #expect(WorkspaceTab.allCases.map(\.title) == [
            "Overview",
            "Transactions",
            "Budget",
            "Accounts",
            "More",
        ])
    }

    @Test func givesEveryDestinationDistinctSelectedAndUnselectedSymbols() {
        let symbols = Set(WorkspaceTab.allCases.map(\.systemImage))
        let selectedSymbols = Set(WorkspaceTab.allCases.map(\.selectedSystemImage))

        #expect(symbols.count == WorkspaceTab.allCases.count)
        #expect(selectedSymbols.count == WorkspaceTab.allCases.count)
        for tab in WorkspaceTab.allCases {
            #expect(tab.systemImage != tab.selectedSystemImage)
        }
    }

    @Test func restoresKnownDestinationsAndFallsBackSafely() {
        #expect(WorkspaceTab.restored(from: "transactions") == .transactions)
        #expect(WorkspaceTab.restored(from: "future-destination") == .overview)
    }
}
