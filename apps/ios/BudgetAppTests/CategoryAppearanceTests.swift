import Foundation
import Testing
import BudgetAPI
@testable import Budget

@Suite("Category appearance")
struct CategoryAppearanceTests {
    @Test("Semantic Budget icon keys map to supported SF Symbols")
    func semanticIconsMapToSymbols() {
        #expect(CategoryPresentation.symbol(for: "home") == "house")
        #expect(CategoryPresentation.symbol(for: "shopping-cart") == "cart")
        #expect(CategoryPresentation.symbol(for: "graduation-cap") == "graduationcap")
        #expect(CategoryPresentation.symbol(for: "trending-up") == "chart.line.uptrend.xyaxis")
        #expect(CategoryPresentation.symbol(for: "wallet-more") == "wallet.pass")
    }

    @Test("Unknown semantic icon and color keys use safe fallbacks")
    func unknownKeysFallBack() {
        #expect(CategoryPresentation.symbol(for: "not-a-budget-icon") == "questionmark.circle")
        #expect(CategoryPresentation.color(for: "not-a-budget-color")
            == CategoryPresentation.color(for: BudgetCategoryColorKey.slate.rawValue))
        #expect(CategoryPresentation.iconType(iconType: "unexpected", iconValue: "home") == .system)
    }

    @Test("Emoji validation uses one Unicode grapheme cluster")
    func emojiValidationSupportsComposedEmoji() {
        #expect(CategoryPresentation.isSingleEmoji("👩🏽‍💻"))
        #expect(CategoryPresentation.isSingleEmoji("🇹🇷"))
        #expect(CategoryPresentation.isSingleEmoji("👨‍👩‍👧‍👦"))
        #expect(!CategoryPresentation.isSingleEmoji("🍀🎁"))
        #expect(!CategoryPresentation.isSingleEmoji("not an emoji"))
    }

    @Test("Predefined category names localize while custom names remain user text")
    func categoryNamesRespectTheirSource() {
        #expect(L10n.categoryName(name: "User supplied name") == "User supplied name")
        #expect(
            L10n.categoryName(name: "Server default", kind: .expense, predefinedKey: "housing")
                != "Server default"
        )
    }

    @Test("Category API payload persists semantic appearance keys, not SF Symbols")
    func categoryRequestUsesSemanticAppearance() throws {
        let request = URLSessionAPIClient.categoryRequest(CategoryInput(
            name: "Work",
            kind: .income,
            parentID: nil,
            iconType: .emoji,
            iconValue: "👩🏽‍💻",
            colorKey: .purple
        ))

        let data = try JSONEncoder().encode(request)
        let payload = try #require(JSONSerialization.jsonObject(with: data) as? [String: Any])

        #expect(payload["icon"] == nil)
        #expect(request.icon_type?.rawValue == "emoji")
        #expect(request.icon_value == "👩🏽‍💻")
        #expect(request.color_key?.rawValue == "purple")
    }
}
