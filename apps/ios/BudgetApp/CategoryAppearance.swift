import SwiftUI
import UIKit

/// Values persisted by the API are semantic keys. This layer is the only place that turns
/// those keys into Apple presentation details, so SF Symbol names never leave the client.
enum BudgetCategoryIconType: String, CaseIterable, Identifiable, Sendable {
    case system
    case emoji

    var id: Self { self }
    var title: String { L10n.text("category.appearance.iconType.\(rawValue)") }
}

enum BudgetCategoryColorKey: String, CaseIterable, Identifiable, Sendable {
    case green, mint, blue, cyan, purple, pink, red, orange, amber, slate

    var id: Self { self }
    var title: String { L10n.text("category.color.\(rawValue)") }
}

/// A category's color, held as one hue resolved per interface style.
///
/// The previous palette stored fixed light-theme pastels, so on Budget's dark canvas every
/// badge rendered as a near-white disc. Storing the hue per style and deriving the wash from it
/// keeps a badge legible in either scheme without a second set of hand-picked values.
struct CategoryPaletteColor: Equatable, Sendable {
    let darkAccentHex: String
    let lightAccentHex: String

    /// The glyph color.
    var accent: Color {
        Color(uiColor: UIColor { traits in
            UIColor(categoryHex: traits.userInterfaceStyle == .dark ? darkAccentHex : lightAccentHex)
        })
    }

    /// The wash the glyph sits on. Derived from the accent so it always belongs to the canvas
    /// it is drawn against.
    var soft: Color {
        Color(uiColor: UIColor { traits in
            let dark = traits.userInterfaceStyle == .dark
            return UIColor(categoryHex: dark ? darkAccentHex : lightAccentHex)
                .withAlphaComponent(dark ? 0.18 : 0.14)
        })
    }

    /// The glyph colour for a solid badge, chosen from the accent's own brightness rather than
    /// fixed to white. The palette flips to a lighter hue on the dark canvas, where a white
    /// glyph would wash out, so the contrasting side has to be decided per interface style.
    var onAccent: Color {
        Color(uiColor: UIColor { traits in
            let dark = traits.userInterfaceStyle == .dark
            return UIColor(categoryHex: dark ? darkAccentHex : lightAccentHex).categoryContrastingGlyph
        })
    }

    /// The stronger edge used for selection strokes and checkmarks.
    var ink: Color {
        Color(uiColor: UIColor { traits in
            let dark = traits.userInterfaceStyle == .dark
            return UIColor(categoryHex: dark ? darkAccentHex : lightAccentHex)
                .withAlphaComponent(dark ? 0.9 : 1)
        })
    }
}

enum CategoryPresentation {
    static let fallbackSystemSymbol = "questionmark.circle"
    static let fallbackColorKey = BudgetCategoryColorKey.slate
    static let fallbackSystemIconKey = "ellipsis"

    /// This is deliberately the curated Budget registry, not the device's full SF Symbol set.
    static let systemIconKeys = [
        "home", "shopping-cart", "utensils", "car", "receipt", "shopping-bag", "heart",
        "gamepad", "repeat", "plane", "graduation-cap", "sparkles", "gift", "ellipsis",
        "wallet", "laptop", "trending-up", "building", "refund", "wallet-more"
    ]

    private static let sfSymbols: [String: String] = [
        "home": "house",
        "shopping-cart": "cart",
        "utensils": "fork.knife",
        "car": "car",
        "receipt": "receipt",
        "shopping-bag": "bag",
        "heart": "heart",
        "gamepad": "gamecontroller",
        "repeat": "repeat",
        "plane": "airplane",
        "graduation-cap": "graduationcap",
        "sparkles": "sparkles",
        "gift": "gift",
        "ellipsis": "ellipsis",
        "wallet": "wallet.pass",
        "laptop": "laptopcomputer",
        "trending-up": "chart.line.uptrend.xyaxis",
        "building": "building.2",
        "refund": "arrow.uturn.backward",
        "wallet-more": "wallet.pass"
    ]

    /// Hues are muted in both directions: bright enough to separate on near-black, dark enough
    /// to stay readable on paper white. Nothing here is a system color.
    private static let palette: [BudgetCategoryColorKey: CategoryPaletteColor] = [
        .green: .init(darkAccentHex: "4FB07C", lightAccentHex: "287A54"),
        .mint: .init(darkAccentHex: "4FB5A0", lightAccentHex: "418A79"),
        .blue: .init(darkAccentHex: "5C9FC7", lightAccentHex: "39759A"),
        .cyan: .init(darkAccentHex: "4FAFBC", lightAccentHex: "3C8592"),
        .purple: .init(darkAccentHex: "9A88C4", lightAccentHex: "70618F"),
        .pink: .init(darkAccentHex: "C98BA0", lightAccentHex: "A16478"),
        .red: .init(darkAccentHex: "D1706A", lightAccentHex: "A34E48"),
        .orange: .init(darkAccentHex: "D8905B", lightAccentHex: "B86F3F"),
        .amber: .init(darkAccentHex: "C7A048", lightAccentHex: "A77B28"),
        .slate: .init(darkAccentHex: "8B9C90", lightAccentHex: "66776C")
    ]

    static func symbol(for semanticKey: String?) -> String {
        sfSymbols[semanticKey ?? ""] ?? fallbackSystemSymbol
    }

    static func isSupportedSystemIcon(_ key: String) -> Bool {
        sfSymbols[key] != nil
    }

    static func iconType(iconType: String?, iconValue: String?) -> BudgetCategoryIconType {
        if iconType == BudgetCategoryIconType.emoji.rawValue, isSingleEmoji(iconValue ?? "") {
            return .emoji
        }
        if iconType == nil, isSingleEmoji(iconValue ?? "") {
            return .emoji
        }
        return .system
    }

    static func color(for key: String?) -> CategoryPaletteColor {
        guard let key, let categoryKey = BudgetCategoryColorKey(rawValue: key) else {
            return palette[fallbackColorKey]!
        }
        return palette[categoryKey]!
    }

    static func systemIconLabel(_ key: String) -> String {
        let localizationKey = "category.icon.\(key)"
        let localized = L10n.text(localizationKey)
        return localized == localizationKey ? L10n.text("category.icon.unknown") : localized
    }

    /// `Character` follows Unicode extended grapheme cluster boundaries, retaining joined
    /// emoji, flags, and skin-tone variants as one displayed character.
    static func isSingleEmoji(_ value: String) -> Bool {
        let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines)
        guard normalized.count == 1, let character = normalized.first else { return false }
        return character.unicodeScalars.contains {
            $0.properties.isEmojiPresentation || $0.properties.isEmoji
        }
    }

    static func normalizedEmoji(_ value: String) -> String? {
        let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return isSingleEmoji(normalized) ? normalized : nil
    }
}

struct CategoryAppearanceBadge: View {
    let iconType: String?
    let iconValue: String?
    let colorKey: String?
    var size: CGFloat = 34

    private var palette: CategoryPaletteColor { CategoryPresentation.color(for: colorKey) }
    private var resolvedType: BudgetCategoryIconType {
        CategoryPresentation.iconType(iconType: iconType, iconValue: iconValue)
    }

    var body: some View {
        Group {
            if resolvedType == .emoji, let emoji = CategoryPresentation.normalizedEmoji(iconValue ?? "") {
                Text(emoji)
                    .font(.system(size: size * 0.52))
                    .minimumScaleFactor(0.6)
            } else {
                Image(systemName: CategoryPresentation.symbol(for: iconValue))
                    .font(.system(size: size * 0.46, weight: .semibold))
                    .foregroundStyle(palette.onAccent)
            }
        }
        .frame(width: size, height: size)
        // Solid rather than a wash: the colour is what identifies a category at a glance in a
        // grid of them, and a tint that pale reads as disabled at tile size.
        .background(palette.accent, in: Circle())
        .accessibilityHidden(true)
    }
}

struct CategoryNameLabel: View {
    let name: String
    let kind: BudgetCategoryKind?
    let predefinedKey: String?
    let systemKey: String?
    let iconType: String?
    let iconValue: String?
    let colorKey: String?
    var iconSize: CGFloat = 28

    var body: some View {
        HStack(spacing: 8) {
            CategoryAppearanceBadge(
                iconType: iconType,
                iconValue: iconValue,
                colorKey: colorKey,
                size: iconSize
            )
            Text(L10n.categoryName(
                name: name,
                kind: kind,
                predefinedKey: predefinedKey,
                systemKey: systemKey
            ))
        }
        .accessibilityElement(children: .combine)
    }
}

struct CategorySystemIconPicker: View {
    @Binding var selectedKey: String
    let colorKey: BudgetCategoryColorKey

    private let columns = [GridItem(.adaptive(minimum: 44), spacing: 10)]

    var body: some View {
        LazyVGrid(columns: columns, spacing: 10) {
            ForEach(CategoryPresentation.systemIconKeys, id: \.self) { key in
                Button {
                    selectedKey = key
                } label: {
                    ZStack {
                        CategoryAppearanceBadge(
                            iconType: BudgetCategoryIconType.system.rawValue,
                            iconValue: key,
                            colorKey: colorKey.rawValue,
                            size: 40
                        )
                        if selectedKey == key {
                            Image(systemName: "checkmark.circle.fill")
                                .symbolRenderingMode(.palette)
                                .foregroundStyle(.white, CategoryPresentation.color(for: colorKey.rawValue).accent)
                                .font(.caption)
                                .offset(x: 14, y: 14)
                        }
                    }
                    .frame(minWidth: 44, minHeight: 44)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(CategoryPresentation.systemIconLabel(key))
                .accessibilityValue(selectedKey == key ? L10n.text("category.appearance.selected") : "")
                .accessibilityHint(L10n.text("category.appearance.selectIcon"))
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(L10n.text("category.appearance.chooseIcon"))
    }
}

struct CategoryEmojiPicker: View {
    @Binding var emoji: String

    private let suggestions = ["🍀", "🍲", "☕", "🛒", "🎁", "💼", "🏠", "✈️", "🎮", "💊", "🐶", "👩🏽‍💻"]
    private let columns = [GridItem(.adaptive(minimum: 44), spacing: 10)]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            TextField(L10n.text("category.appearance.emoji"), text: $emoji)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .accessibilityHint(L10n.text("category.appearance.emojiHint"))
            Text(L10n.text("category.appearance.emojiHint"))
                .font(.footnote)
                .foregroundStyle(BudgetTheme.tertiaryText)
            LazyVGrid(columns: columns, spacing: 10) {
                ForEach(suggestions, id: \.self) { suggestion in
                    Button {
                        emoji = suggestion
                    } label: {
                        Text(suggestion)
                            .font(.title2)
                            .frame(minWidth: 44, minHeight: 44)
                            .background(
                                emoji == suggestion
                                    ? BudgetTheme.forest.opacity(0.16)
                                    : BudgetTheme.elevated,
                                in: Circle()
                            )
                            .overlay {
                                Circle().stroke(
                                    emoji == suggestion ? BudgetTheme.forest : BudgetTheme.border,
                                    lineWidth: emoji == suggestion ? 2 : 1
                                )
                            }
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel(L10n.text("category.appearance.useEmoji") + " \(suggestion)")
                    .accessibilityValue(emoji == suggestion ? L10n.text("category.appearance.selected") : "")
                }
            }
            if !CategoryPresentation.isSingleEmoji(emoji) {
                Label(L10n.text("category.appearance.emojiValidation"), systemImage: "exclamationmark.triangle.fill")
                    .font(.footnote)
                    .foregroundStyle(BudgetTheme.over)
                    .accessibilityAddTraits(.isStaticText)
            }
        }
    }
}

struct CategoryColorPicker: View {
    @Binding var selectedKey: BudgetCategoryColorKey

    private let columns = [GridItem(.adaptive(minimum: 44), spacing: 10)]

    var body: some View {
        LazyVGrid(columns: columns, spacing: 10) {
            ForEach(BudgetCategoryColorKey.allCases) { key in
                let palette = CategoryPresentation.color(for: key.rawValue)
                Button {
                    selectedKey = key
                } label: {
                    ZStack {
                        Circle()
                            .fill(palette.accent)
                            .frame(width: 32, height: 32)
                        if selectedKey == key {
                            Image(systemName: "checkmark")
                                .font(.caption.weight(.bold))
                                .foregroundStyle(.white)
                        }
                    }
                    .frame(minWidth: 44, minHeight: 44)
                    .overlay {
                        if selectedKey == key {
                            Circle().stroke(palette.ink, lineWidth: 2)
                        }
                    }
                }
                .buttonStyle(.plain)
                .accessibilityLabel(key.title)
                .accessibilityValue(selectedKey == key ? L10n.text("category.appearance.selected") : "")
                .accessibilityHint(L10n.text("category.appearance.selectColor"))
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(L10n.text("category.appearance.chooseColor"))
    }
}

private extension UIColor {
    /// White or near-black, whichever carries further against this colour. The threshold is the
    /// usual relative-luminance one, so a palette change can never quietly produce a glyph that
    /// disappears into its own badge.
    var categoryContrastingGlyph: UIColor {
        var red: CGFloat = 0, green: CGFloat = 0, blue: CGFloat = 0, alpha: CGFloat = 0
        guard getRed(&red, green: &green, blue: &blue, alpha: &alpha) else { return .white }
        func channel(_ value: CGFloat) -> CGFloat {
            value <= 0.03928 ? value / 12.92 : pow((value + 0.055) / 1.055, 2.4)
        }
        let luminance = 0.2126 * channel(red) + 0.7152 * channel(green) + 0.0722 * channel(blue)
        return luminance > 0.45 ? UIColor(white: 0.08, alpha: 1) : .white
    }

    convenience init(categoryHex: String) {
        let value = UInt64(categoryHex, radix: 16) ?? 0
        self.init(
            red: CGFloat((value >> 16) & 0xFF) / 255,
            green: CGFloat((value >> 8) & 0xFF) / 255,
            blue: CGFloat(value & 0xFF) / 255,
            alpha: 1
        )
    }
}
