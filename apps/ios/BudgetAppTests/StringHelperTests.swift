import Testing
@testable import Budget

@Suite("Optional text")
struct StringHelperTests {
    /// Optional fields are omitted rather than sent as blanks, so whitespace-only input must
    /// collapse to nil before it reaches the API.
    @Test("Blank and whitespace-only values collapse to nil")
    func blankValuesBecomeNil() {
        #expect("".nilIfBlank == nil)
        #expect("   ".nilIfBlank == nil)
        #expect("\n\t ".nilIfBlank == nil)
    }

    @Test("Real values are trimmed and preserved")
    func realValuesAreTrimmed() {
        #expect("  Garanti  ".nilIfBlank == "Garanti")
        #expect("Garanti".nilIfBlank == "Garanti")
    }
}
