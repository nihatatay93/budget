import SwiftUI

@main
struct BudgetApp: App {
    private let environment = AppEnvironment.live
    @AppStorage("budget.textSizePreference") private var textSizePreference = BudgetTextSize.balanced.rawValue

    private var textSize: BudgetTextSize {
        BudgetTextSize(rawValue: textSizePreference) ?? .balanced
    }

    var body: some Scene {
        WindowGroup {
            AppView(environment: environment)
                .tint(BudgetTheme.forest)
                .preferredColorScheme(.dark)
                .dynamicTypeSize(textSize.supportedRange)
        }
    }
}
