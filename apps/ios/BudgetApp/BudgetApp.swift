import SwiftUI

@main
struct BudgetApp: App {
    private let environment = AppEnvironment.live

    var body: some Scene {
        WindowGroup {
            AppView(environment: environment)
        }
    }
}
