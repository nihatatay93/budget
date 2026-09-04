import SwiftUI
import UIKit

/// The visual foundation for Budget's native experience.
///
/// Everything visual resolves through these tokens: three layered surfaces, one radius scale,
/// one elevation recipe, and a restrained brand ramp. Financial data carries the only saturated
/// color on screen, so status and category appearance stay legible rather than decorative noise.
enum BudgetTheme {

    // MARK: - Brand

    /// The primary accent. Desaturated relative to the web brand because a dark canvas pushes
    /// saturated greens toward neon.
    static let forest = dynamic(dark: 0x3FA372, light: 0x2A7A52)
    static let deepForest = dynamic(dark: 0x0E2A1D, light: 0x123D28)
    static let sage = dynamic(dark: 0x5FC08D, light: 0x2F8355)

    // MARK: - Surfaces

    /// The page behind everything.
    static let canvas = dynamic(dark: 0x0A0C0B, light: 0xF7F6F2)
    /// Resting cards and list rows.
    static let surface = dynamic(dark: 0x121614, light: 0xFFFFFF)
    /// Controls and nested fills that must separate from a card they sit on.
    static let elevated = dynamic(dark: 0x1A201D, light: 0xF1F0EB)

    /// Card outlines. Deliberately near-invisible: the fill separates the card, the stroke only
    /// keeps its edge from dissolving on OLED black.
    static let border = Color(uiColor: UIColor { traits in
        traits.userInterfaceStyle == .dark
            ? UIColor.white.withAlphaComponent(0.08)
            : UIColor.black.withAlphaComponent(0.07)
    })

    /// Row separators inside a card, one step fainter than `border`.
    static let separator = Color(uiColor: UIColor { traits in
        traits.userInterfaceStyle == .dark
            ? UIColor.white.withAlphaComponent(0.06)
            : UIColor.black.withAlphaComponent(0.05)
    })

    // MARK: - Text

    static let primaryText = dynamic(dark: 0xF2F4F2, light: 0x111814)
    static let secondaryText = Color(uiColor: UIColor { traits in
        traits.userInterfaceStyle == .dark
            ? UIColor.white.withAlphaComponent(0.58)
            : UIColor.black.withAlphaComponent(0.55)
    })
    static let tertiaryText = Color(uiColor: UIColor { traits in
        traits.userInterfaceStyle == .dark
            ? UIColor.white.withAlphaComponent(0.38)
            : UIColor.black.withAlphaComponent(0.38)
    })

    // MARK: - Money semantics

    /// Money arriving. The only place a bright green is earned.
    static let positive = sage
    /// Money leaving. Muted amber rather than system orange, which reads as an alert.
    static let spend = dynamic(dark: 0xD9915C, light: 0xA6602C)
    /// Over plan. Muted rather than system red for the same reason.
    static let over = dynamic(dark: 0xDE7068, light: 0xAE4139)
    /// Not yet posted.
    static let pending = dynamic(dark: 0xC9A75C, light: 0x8A6A1E)
    /// Movement between accounts, which is neither income nor spending.
    static let transfer = dynamic(dark: 0x6FA3C4, light: 0x39708F)
    /// Balance corrections.
    static let adjustment = dynamic(dark: 0x9C8CC4, light: 0x64548C)

    // MARK: - Metrics

    enum Radius {
        /// Chips, badges, inline fills.
        static let small: CGFloat = 12
        /// Cards and list-row groups.
        static let medium: CGFloat = 18
        /// The hero balance card.
        static let large: CGFloat = 28
    }

    enum Space {
        /// The horizontal inset every screen shares. One number, so nothing misaligns.
        static let screen: CGFloat = 20
        /// Padding inside a card.
        static let card: CGFloat = 18
        /// Gap between stacked cards.
        static let section: CGFloat = 24
        /// Gap between rows inside a card.
        static let row: CGFloat = 14
    }

    /// The single elevation recipe. Applying it more than once to the same card is the bug that
    /// made the old surfaces look accidentally different from each other.
    static func shadow<V: View>(_ view: V, strength: Double = 1) -> some View {
        view.shadow(color: .black.opacity(0.28 * strength), radius: 18 * strength, y: 8 * strength)
    }

    /// The color a signed amount should be drawn in.
    static func money(_ amount: Int64) -> Color {
        amount > 0 ? positive : primaryText
    }

    private static func dynamic(dark: UInt32, light: UInt32) -> Color {
        Color(uiColor: UIColor { traits in
            UIColor(rgb: traits.userInterfaceStyle == .dark ? dark : light)
        })
    }
}

private extension UIColor {
    convenience init(rgb: UInt32) {
        self.init(
            red: CGFloat((rgb >> 16) & 0xFF) / 255,
            green: CGFloat((rgb >> 8) & 0xFF) / 255,
            blue: CGFloat(rgb & 0xFF) / 255,
            alpha: 1
        )
    }
}

// MARK: - Typography

extension Font {
    /// The hero balance. Tabular figures so a changing amount does not shift the layout.
    static let budgetHero = Font.system(size: 40, weight: .semibold).monospacedDigit()
    /// A card's headline amount.
    static let budgetAmountLarge = Font.system(size: 24, weight: .semibold).monospacedDigit()
    /// The amount column in a ledger row.
    static let budgetAmount = Font.system(size: 17, weight: .semibold).monospacedDigit()
    /// A supporting figure under a primary amount.
    static let budgetAmountSmall = Font.system(size: 13, weight: .medium).monospacedDigit()
    /// Small all-caps label above a value.
    static let budgetEyebrow = Font.system(size: 11, weight: .semibold)
    /// A section's title.
    static let budgetSectionTitle = Font.system(size: 15, weight: .semibold)
}

/// The all-caps label that sits above a value. Tracking is what keeps it from reading as
/// shouting at this size.
struct BudgetEyebrow: View {
    let text: LocalizedStringKey
    var color: Color = BudgetTheme.tertiaryText

    init(_ text: LocalizedStringKey, color: Color = BudgetTheme.tertiaryText) {
        self.text = text
        self.color = color
    }

    var body: some View {
        // `textCase` rather than `uppercased()`: the transform has to happen after the string is
        // localized, and it has to follow the locale's own casing rules.
        Text(text)
            .textCase(.uppercase)
            .font(.budgetEyebrow)
            .tracking(0.9)
            .foregroundStyle(color)
    }
}

// MARK: - Dynamic Type

/// Keeps Budget's dense financial layouts usable at larger Dynamic Type sizes while offering
/// people an explicit path to use the full system accessibility range.
enum BudgetTextSize: String, CaseIterable, Identifiable {
    case balanced
    case large
    case system

    var id: Self { self }

    var title: String { L10n.text("appearance.textSize.\(rawValue)") }

    var supportedRange: ClosedRange<DynamicTypeSize> {
        switch self {
        case .balanced: .xSmall ... .large
        case .large: .xSmall ... .accessibility2
        case .system: .xSmall ... .accessibility5
        }
    }
}

// MARK: - Surfaces

/// A resting card. One fill, one hairline, one shadow — every card on every screen.
struct BudgetCard<Content: View>: View {
    var padding: CGFloat = BudgetTheme.Space.card
    var radius: CGFloat = BudgetTheme.Radius.medium
    @ViewBuilder let content: Content

    var body: some View {
        content
            .padding(padding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .budgetSurface(radius: radius)
    }
}

private struct BudgetSurfaceModifier: ViewModifier {
    let radius: CGFloat
    let fill: Color
    let showsShadow: Bool

    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius: radius, style: .continuous)
        let surface = content
            .background(fill, in: shape)
            .overlay { shape.stroke(BudgetTheme.border, lineWidth: 1) }
        if showsShadow {
            BudgetTheme.shadow(surface)
        } else {
            surface
        }
    }
}

extension View {
    /// Applies the shared card treatment. Never apply twice to the same view — that double
    /// stroke is what made nested cards look muddy.
    func budgetSurface(
        radius: CGFloat = BudgetTheme.Radius.medium,
        fill: Color = BudgetTheme.surface,
        showsShadow: Bool = true
    ) -> some View {
        modifier(BudgetSurfaceModifier(radius: radius, fill: fill, showsShadow: showsShadow))
    }
}

// MARK: - Screen chrome

private struct BudgetScreenModifier: ViewModifier {
    func body(content: Content) -> some View {
        content
            .scrollContentBackground(.hidden)
            .background(BudgetTheme.canvas.ignoresSafeArea())
    }
}

extension View {
    func budgetScreen() -> some View {
        modifier(BudgetScreenModifier())
    }

    /// Lets a card own its full width inside a `List` instead of inheriting the row's inset,
    /// which is what made the hero card and the card beneath it disagree on where the margin is.
    ///
    /// `horizontal` is 0 because an inset-grouped `List` already applies its section margin;
    /// adding the screen inset on top of it is what pushed the card in twice as far.
    func budgetPlainRow(
        horizontal: CGFloat = 0,
        top: CGFloat = 0,
        bottom: CGFloat = BudgetTheme.Space.section
    ) -> some View {
        listRowInsets(EdgeInsets(top: top, leading: horizontal, bottom: bottom, trailing: horizontal))
            .listRowBackground(Color.clear)
            .listRowSeparator(.hidden)
    }

    /// A row that should read as part of a Budget card rather than a system list row. Setting
    /// the fill explicitly is what stops an inset-grouped `List` from painting its own gray,
    /// which is why two "cards" on one screen used to be different colors.
    func budgetCardRow() -> some View {
        listRowBackground(BudgetTheme.surface)
            .listRowSeparatorTint(BudgetTheme.separator)
            .listRowInsets(EdgeInsets(top: 12, leading: 16, bottom: 12, trailing: 16))
    }
}

/// A section title inside a `List`, styled to match `BudgetSection` on the scroll-based screens.
struct BudgetListHeader: View {
    let title: LocalizedStringKey

    init(_ title: LocalizedStringKey) {
        self.title = title
    }

    var body: some View {
        Text(title)
            .font(.budgetSectionTitle)
            .foregroundStyle(BudgetTheme.secondaryText)
            .textCase(nil)
            .padding(.bottom, 2)
    }
}

/// The explanatory line under a section. Quieter than the system footer, which rendered these
/// at body size and turned every screen into a manual.
struct BudgetListFooter: View {
    let text: LocalizedStringKey

    init(_ text: LocalizedStringKey) {
        self.text = text
    }

    var body: some View {
        Text(text)
            .font(.caption)
            .foregroundStyle(BudgetTheme.tertiaryText)
    }
}

/// A screen section: a quiet title, an optional trailing action, and its content.
struct BudgetSection<Content: View>: View {
    let title: LocalizedStringKey?
    var action: (title: LocalizedStringKey, handler: () -> Void)?
    var caption: LocalizedStringKey?
    @ViewBuilder let content: Content

    init(
        _ title: LocalizedStringKey? = nil,
        action: (title: LocalizedStringKey, handler: () -> Void)? = nil,
        caption: LocalizedStringKey? = nil,
        @ViewBuilder content: () -> Content
    ) {
        self.title = title
        self.action = action
        self.caption = caption
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            if title != nil || action != nil {
                HStack(alignment: .firstTextBaseline) {
                    if let title {
                        Text(title)
                            .font(.budgetSectionTitle)
                            .foregroundStyle(BudgetTheme.secondaryText)
                            .accessibilityAddTraits(.isHeader)
                    }
                    Spacer(minLength: 12)
                    if let action {
                        Button(action.title, action: action.handler)
                            .font(.footnote.weight(.semibold))
                            .foregroundStyle(BudgetTheme.forest)
                    }
                }
                .padding(.horizontal, 2)
            }
            content
            if let caption {
                Text(caption)
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .padding(.horizontal, 2)
            }
        }
    }
}

// MARK: - Components

/// The circular glyph that identifies a row. The tint is carried by the glyph and a soft wash,
/// never by a fully saturated fill.
struct BudgetIconBadge: View {
    let systemImage: String
    let color: Color
    var size: CGFloat = 38

    var body: some View {
        Image(systemName: systemImage)
            .font(.system(size: size * 0.42, weight: .semibold))
            .foregroundStyle(color)
            .frame(width: size, height: size)
            .background(color.opacity(0.15), in: Circle())
            .overlay { Circle().stroke(color.opacity(0.18), lineWidth: 1) }
            .accessibilityHidden(true)
    }
}

/// A progress meter. Replaces `ProgressView(value:)` so the track, the cap shape, and the
/// height are the same on every screen.
struct BudgetMeter: View {
    let progress: Double
    var tint: Color = BudgetTheme.forest
    var height: CGFloat = 6

    var body: some View {
        GeometryReader { proxy in
            ZStack(alignment: .leading) {
                Capsule()
                    .fill(BudgetTheme.elevated)
                Capsule()
                    .fill(
                        LinearGradient(
                            colors: [tint.opacity(0.75), tint],
                            startPoint: .leading,
                            endPoint: .trailing
                        )
                    )
                    .frame(width: max(progress <= 0 ? 0 : height, proxy.size.width * clamped))
            }
        }
        .frame(height: height)
        .accessibilityHidden(true)
    }

    private var clamped: Double {
        progress.isFinite ? min(1, max(0, progress)) : 0
    }
}

/// A labelled figure. Used wherever several amounts sit side by side.
struct BudgetStat: View {
    let title: LocalizedStringKey
    let value: String
    var valueColor: Color = BudgetTheme.primaryText
    var alignment: HorizontalAlignment = .leading

    var body: some View {
        VStack(alignment: alignment, spacing: 4) {
            BudgetEyebrow(title)
            Text(value)
                .font(.budgetAmountSmall)
                .foregroundStyle(valueColor)
                .minimumScaleFactor(0.7)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: alignment == .leading ? .leading : .trailing)
        .accessibilityElement(children: .combine)
    }
}

/// A small status pill.
struct BudgetChip: View {
    let text: LocalizedStringKey
    var systemImage: String?
    var color: Color = BudgetTheme.secondaryText

    var body: some View {
        HStack(spacing: 4) {
            if let systemImage {
                Image(systemName: systemImage)
                    .font(.system(size: 10, weight: .bold))
            }
            Text(text)
                .font(.system(size: 11, weight: .semibold))
        }
        .foregroundStyle(color)
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(color.opacity(0.14), in: Capsule())
        .accessibilityElement(children: .combine)
    }
}

/// The hairline between rows inside a card. Inset so it never touches the card's edge.
struct BudgetHairline: View {
    var leading: CGFloat = 0

    var body: some View {
        Rectangle()
            .fill(BudgetTheme.separator)
            .frame(height: 1)
            .padding(.leading, leading)
            .accessibilityHidden(true)
    }
}

/// Stacks rows into one card with hairlines between them, replacing an inset-grouped `List`
/// section without inheriting its Settings-app feel.
struct BudgetRowGroup<Item: Identifiable, Row: View>: View {
    let items: [Item]
    var hairlineInset: CGFloat = 0
    @ViewBuilder let row: (Item) -> Row

    var body: some View {
        VStack(spacing: 0) {
            ForEach(Array(items.enumerated()), id: \.element.id) { index, item in
                row(item)
                    .padding(.horizontal, BudgetTheme.Space.card)
                    .padding(.vertical, 13)
                if index < items.count - 1 {
                    BudgetHairline(leading: hairlineInset + BudgetTheme.Space.card)
                }
            }
        }
        .budgetSurface()
    }
}

// MARK: - Formatting

/// Renders an API date string (`yyyy-MM-dd`) the way a person reads dates, falling back to the
/// raw value only when it cannot be parsed.
func budgetDisplayDate(_ value: String) -> String {
    let parts = value.split(separator: "-").compactMap { Int($0) }
    guard parts.count == 3,
          let date = Calendar(identifier: .gregorian).date(
            from: DateComponents(year: parts[0], month: parts[1], day: parts[2])
          ) else { return value }
    return date.formatted(.dateTime.year().month(.abbreviated).day())
}

/// A signed amount, with an explicit `+` so a positive delta is unambiguous.
func budgetSignedMoney(_ amount: Int64, currency: BudgetCurrency) -> String {
    let formatted = currency.formatted(minorUnits: amount)
    return amount > 0 ? "+\(formatted)" : formatted
}
