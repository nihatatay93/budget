import Foundation
import SwiftUI

/// Budget resolves display text through `Localizable.strings` in the app bundle, which follows
/// the device's language preference — not its region or timezone. A phone set to English in
/// Istanbul is an English phone, and iOS is right to keep the app in English there.
///
/// Two kinds of key live in the table, matching the web client so a term never diverges between
/// the two apps:
///
/// - Semantic keys (`account.type.bank`, `transaction.kind.transfer`) for values that arrive
///   from the API as stable identifiers.
/// - The English source string itself for ordinary interface copy, which lets SwiftUI's
///   `LocalizedStringKey` resolve a literal with no call-site ceremony and fall back to readable
///   English whenever a translation is missing.
enum L10n {
    static func text(_ key: String) -> String {
        Bundle.main.localizedString(forKey: key, value: key, table: "Localizable")
    }

    /// Roles arrive as free-form strings on `BudgetWorkspace`, so this accepts any casing and
    /// falls back to the server's own value rather than showing a raw key.
    static func workspaceRole(_ role: String) -> String {
        let normalized = role.lowercased()
        let key = "workspace.role.\(normalized)"
        let value = text(key)
        return value == key ? role.capitalized : value
    }

    static func categoryName(
        name: String,
        kind: BudgetCategoryKind? = nil,
        predefinedKey: String? = nil,
        systemKey: String? = nil
    ) -> String {
        if let predefinedKey {
            let kinds = kind.map { [$0.rawValue] } ?? ["expense", "income"]
            for kind in kinds {
                let key = "category.\(kind).\(predefinedKey)"
                let value = text(key)
                if value != key { return value }
            }
        }
        if let systemKey {
            let key = "category.system.\(systemKey)"
            let value = text(key)
            if value != key { return value }
        }
        return name
    }
}

extension LocalizedStringKey {
    /// Wraps text that is already localized — an enum's translated `title`, or a value the API
    /// supplied — so it can be handed to a view that takes a `LocalizedStringKey`.
    ///
    /// A second lookup is harmless: a translated string is not itself a key, so the table misses
    /// and `LocalizedStringKey` renders the string unchanged.
    static func resolved(_ value: String) -> LocalizedStringKey {
        LocalizedStringKey(value)
    }
}
