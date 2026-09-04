import Foundation
import Testing
@testable import Budget

/// The Turkish table is the one that has to be complete: English interface copy uses its own
/// source string as the key, so an absent English entry still renders correctly, while an
/// absent Turkish entry silently leaves that line in English.
///
/// Two failures matter here and neither shows up in a build. A semantic key present in one
/// table and missing from the other renders a raw key like `analysis.bucket.week` on screen.
/// A format string whose translation drops or renumbers a specifier renders the wrong number,
/// or traps at runtime — which is why the specifier check walks every entry rather than a
/// curated list.
@Suite("Localization coverage")
struct LocalizationCoverageTests {
    private func strings(_ language: String) throws -> [String: String] {
        let path = try #require(
            Bundle.main.path(forResource: language, ofType: "lproj"),
            "the \(language) localization is missing from the app bundle"
        )
        let bundle = try #require(Bundle(path: path))
        let url = try #require(bundle.url(forResource: "Localizable", withExtension: "strings"))
        return try #require(NSDictionary(contentsOf: url) as? [String: String])
    }

    /// Positional specifiers in the order the format string uses them. Turkish word order
    /// differs from English often enough that translations reorder arguments deliberately, so
    /// the comparison is on the set, not the sequence.
    private func specifiers(_ value: String) -> Set<String> {
        let pattern = #"%(\d+\$)?(?:lld|ld|d|@|f)"#
        guard let expression = try? NSRegularExpression(pattern: pattern) else { return [] }
        let range = NSRange(value.startIndex..., in: value)
        return Set(expression.matches(in: value, range: range).compactMap { match in
            Range(match.range, in: value).map { String(value[$0]) }
        })
    }

    @Test("Every English key is also translated into Turkish")
    func semanticKeysAreTranslated() throws {
        let english = try strings("en")
        let turkish = try strings("tr")

        let missing = english.keys.filter { turkish[$0] == nil }.sorted()
        #expect(missing.isEmpty, "missing Turkish translations: \(missing.joined(separator: ", "))")
    }

    @Test("Every spending-analysis key carries a Turkish translation")
    func analysisKeysAreTranslated() throws {
        let turkish = try strings("tr")

        for granularity in BudgetAnalysisGranularity.allCases {
            #expect(turkish["analysis.granularity.\(granularity.rawValue)"] != nil)
            #expect(turkish["analysis.bucket.\(granularity.rawValue)"] != nil)
        }
        for preset in AnalysisRangePreset.allCases {
            #expect(turkish["analysis.range.\(preset.rawValue)"] != nil)
        }
        // The count phrases are chosen at runtime, so a missing plural form is invisible in
        // English and only surfaces in Turkish.
        #expect(turkish["%lld transaction"] != nil)
        #expect(turkish["%lld transactions"] != nil)
    }

    @Test("No Turkish translation drops or invents a format specifier")
    func formatSpecifiersSurviveTranslation() throws {
        let turkish = try strings("tr")

        var mismatched: [String] = []
        for (key, value) in turkish {
            let expected = specifiers(key)
            guard !expected.isEmpty else { continue }
            if specifiers(value) != expected {
                mismatched.append("\(key) -> \(value)")
            }
        }
        #expect(
            mismatched.isEmpty,
            "format specifier mismatch:\n\(mismatched.sorted().joined(separator: "\n"))"
        )
    }
}
