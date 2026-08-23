import Foundation

enum WorkspaceTab: String, CaseIterable, Identifiable {
    case overview
    case transactions
    case budget
    case accounts
    case more

    var id: Self { self }

    var title: String {
        switch self {
        case .overview: "Overview"
        case .transactions: "Transactions"
        case .budget: "Budget"
        case .accounts: "Accounts"
        case .more: "More"
        }
    }

    var systemImage: String {
        switch self {
        case .overview: "house"
        case .transactions: "list.bullet.rectangle"
        case .budget: "chart.pie"
        case .accounts: "building.columns"
        case .more: "ellipsis.circle"
        }
    }

    var selectedSystemImage: String {
        switch self {
        case .overview: "house.fill"
        case .transactions: "list.bullet.rectangle.fill"
        case .budget: "chart.pie.fill"
        case .accounts: "building.columns.fill"
        case .more: "ellipsis.circle.fill"
        }
    }

    static func restored(from rawValue: String) -> WorkspaceTab {
        WorkspaceTab(rawValue: rawValue) ?? .overview
    }
}
