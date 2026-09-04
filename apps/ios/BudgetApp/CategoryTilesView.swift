import SwiftUI

/// Categories as coloured sections of tiles rather than one long list, matching the web picker
/// so a section reads identically on either client.
///
/// Which sections a person collapses is a per-device convenience, so it lives in `AppStorage`
/// rather than on the workspace: hiding a section here is a shelf, not archiving, and must not
/// change what a workspace-mate sees or what any report contains.
struct CategoryTileSections: View {
    let categories: [BudgetCategory]
    /// Category identifiers to offer first, most used first.
    var frequent: [String] = []
    var selectedID: String?
    let workspaceID: String
    let onSelect: (BudgetCategory) -> Void

    @AppStorage("budget.categorySections.hidden") private var hiddenSections = ""
    @State private var search = ""

    private var hidden: Set<String> {
        Set(hiddenSections.split(separator: "\n").map(String.init))
    }

    private func key(_ sectionID: String) -> String { "\(workspaceID)/\(sectionID)" }

    private func toggle(_ sectionID: String) {
        var next = hidden
        let identifier = key(sectionID)
        if next.contains(identifier) {
            next.remove(identifier)
        } else {
            next.insert(identifier)
        }
        // Sections are stored newline-separated because AppStorage holds a plain string, and a
        // category identifier can never contain one.
        hiddenSections = next.sorted().joined(separator: "\n")
    }

    private var trimmedSearch: String {
        search.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private func matches(_ category: BudgetCategory) -> Bool {
        guard !trimmedSearch.isEmpty else { return true }
        return displayName(category).range(
            of: trimmedSearch,
            options: [.caseInsensitive, .diacriticInsensitive],
            range: nil,
            locale: nil
        ) != nil
    }

    private var sections: [CategoryHierarchy.Section] {
        // Searching is a flat question — "where is the taxi one" — so it looks through every
        // section at once and ignores which of them are collapsed.
        CategoryHierarchy.sections(of: categories, by: displayName)
            .map { section in
                CategoryHierarchy.Section(
                    root: section.root,
                    members: trimmedSearch.isEmpty ? section.members : section.members.filter { matches($0.category) }
                )
            }
            .filter { !$0.members.isEmpty }
    }

    private var frequentMembers: [CategoryHierarchy.Member] {
        frequent
            .compactMap { id in categories.first { $0.id == id } }
            .map { CategoryHierarchy.Member(category: $0, depth: 0) }
    }

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 22, pinnedViews: []) {
                if trimmedSearch.isEmpty, !frequentMembers.isEmpty {
                    section(
                        heading: L10n.text("Most used"),
                        identifier: "frequent",
                        members: frequentMembers
                    )
                }
                ForEach(sections) { value in
                    section(
                        heading: displayName(value.root),
                        identifier: value.root.id,
                        members: value.members
                    )
                }
                if sections.isEmpty {
                    Text("No matching categories")
                        .font(.footnote)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
        }
        .searchable(text: $search, prompt: Text("Search categories"))
    }

    @ViewBuilder
    private func section(
        heading: String,
        identifier: String,
        members: [CategoryHierarchy.Member]
    ) -> some View {
        let isHidden = trimmedSearch.isEmpty && hidden.contains(key(identifier))
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text(LocalizedStringKey.resolved(heading))
                    .font(.headline)
                    .foregroundStyle(BudgetTheme.primaryText)
                Spacer(minLength: 12)
                Button {
                    toggle(identifier)
                } label: {
                    Image(systemName: isHidden ? "eye.slash" : "eye")
                        .foregroundStyle(BudgetTheme.tertiaryText)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(Text(String(
                    format: L10n.text(isHidden ? "Show %@" : "Hide %@"),
                    heading
                )))
            }
            Divider()
            if !isHidden {
                LazyVGrid(
                    columns: [GridItem(.adaptive(minimum: 78), spacing: 8)],
                    alignment: .leading,
                    spacing: 14
                ) {
                    ForEach(members) { member in
                        CategoryTile(
                            category: member.category,
                            depth: member.depth,
                            selected: member.category.id == selectedID,
                            onSelect: onSelect
                        )
                    }
                }
            }
        }
    }

    private func displayName(_ category: BudgetCategory) -> String {
        L10n.categoryName(
            name: category.name,
            kind: category.kind,
            predefinedKey: category.predefinedKey,
            systemKey: category.systemKey
        )
    }
}

private struct CategoryTile: View {
    let category: BudgetCategory
    let depth: Int
    let selected: Bool
    let onSelect: (BudgetCategory) -> Void

    var body: some View {
        Button {
            onSelect(category)
        } label: {
            VStack(spacing: 6) {
                CategoryAppearanceBadge(
                    iconType: category.iconType,
                    iconValue: category.iconValue,
                    colorKey: category.colorKey,
                    // A nested category is one visual step in, and no more: a picker that
                    // indents four levels stops being scannable, which is the point of tiles.
                    size: depth > 0 ? 44 : 52
                )
                Text(LocalizedStringKey.resolved(L10n.categoryName(
                    name: category.name,
                    kind: category.kind,
                    predefinedKey: category.predefinedKey,
                    systemKey: category.systemKey
                )))
                .font(.caption2)
                .multilineTextAlignment(.center)
                .lineLimit(2)
                .foregroundStyle(BudgetTheme.primaryText)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 8)
            .background(
                RoundedRectangle(cornerRadius: 14)
                    .fill(selected ? BudgetTheme.forest.opacity(0.12) : .clear)
            )
            .overlay(
                RoundedRectangle(cornerRadius: 14)
                    .stroke(selected ? BudgetTheme.forest : .clear, lineWidth: 1)
            )
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(selected ? [.isButton, .isSelected] : .isButton)
    }
}
