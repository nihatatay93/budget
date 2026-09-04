import Foundation

/// Category hierarchy shared by everything that presents categories: the Categories destination,
/// the capture form's picker, and the transaction draft that has to name a branch by its root.
///
/// This mirrors `apps/web/src/lib/categoryTree.ts`. The seeded defaults arrive in groups — a
/// group is an ordinary category its members hang under — but nothing here assumes that. A
/// workspace that flattens or rearranges its own categories gets sections describing whatever it
/// actually built.
enum CategoryHierarchy {
    struct Member: Identifiable, Equatable {
        let category: BudgetCategory
        /// 0 for the section's own root, deeper for everything under it.
        let depth: Int

        var id: String { category.id }

        static func == (left: Member, right: Member) -> Bool {
            left.category.id == right.category.id && left.depth == right.depth
        }
    }

    struct Section: Identifiable, Equatable {
        let root: BudgetCategory
        let members: [Member]

        var id: String { root.id }

        static func == (left: Section, right: Section) -> Bool {
            left.root.id == right.root.id && left.members == right.members
        }
    }

    /// The top of a category's branch, which is what a picker offers first.
    static func root(of categoryID: String, in categories: [BudgetCategory]) -> BudgetCategory? {
        var current = categories.first { $0.id == categoryID }
        // Ancestry is server-validated as acyclic; the visit set keeps a corrupt response from hanging.
        var visited: Set<String> = []
        while let candidate = current, let parentID = candidate.parentID, !visited.contains(candidate.id) {
            visited.insert(candidate.id)
            guard let parent = categories.first(where: { $0.id == parentID }) else { break }
            current = parent
        }
        return current
    }

    /// Every category under a root, depth-first, so a branch reads as a branch.
    static func branch(of rootID: String, in categories: [BudgetCategory]) -> [Member] {
        var rows: [Member] = []
        var visited: Set<String> = []
        func append(parentID: String, depth: Int) {
            for category in categories where category.parentID == parentID {
                guard !visited.contains(category.id) else { continue }
                visited.insert(category.id)
                rows.append(Member(category: category, depth: depth))
                append(parentID: category.id, depth: depth + 1)
            }
        }
        append(parentID: rootID, depth: 0)
        return rows
    }

    /// One section per root, each holding the root and its descendants. The root is a member of
    /// its own section because a group is spendable: a workspace can allocate to Entertainment
    /// itself rather than to one of the things inside it.
    static func sections(
        of categories: [BudgetCategory],
        by name: (BudgetCategory) -> String
    ) -> [Section] {
        let present = Set(categories.map(\.id))
        // A category whose parent was filtered out of this set reads as a root here, so an
        // archived or wrong-kind parent never hides its children.
        let roots = categories
            .filter { category in
                guard let parentID = category.parentID else { return true }
                return !present.contains(parentID)
            }
            .sorted { name($0).localizedCaseInsensitiveCompare(name($1)) == .orderedAscending }
        return roots.map { root in
            Section(
                root: root,
                members: [Member(category: root, depth: 0)]
                    + branch(of: root.id, in: categories).map {
                        Member(category: $0.category, depth: $0.depth + 1)
                    }
            )
        }
    }

    /// The categories used most often, derived from recent activity rather than stored, so the
    /// list follows how a workspace actually spends without anything to maintain.
    static func frequentCategoryIDs(in transactions: [BudgetTransaction], limit: Int = 8) -> [String] {
        var counts: [String: Int] = [:]
        for transaction in transactions.sorted(by: { $0.transactionDate > $1.transactionDate }).prefix(80) {
            for allocation in transaction.allocations {
                counts[allocation.categoryID, default: 0] += 1
            }
        }
        return counts
            .sorted { left, right in
                left.value == right.value ? left.key < right.key : left.value > right.value
            }
            .prefix(limit)
            .map(\.key)
    }
}
