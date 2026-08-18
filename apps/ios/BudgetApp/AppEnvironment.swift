import Foundation

struct AppEnvironment {
    let apiClient: any APIClient
    let sessionStore: any SessionStore

    static let live = AppEnvironment(
        apiClient: URLSessionAPIClient(),
        sessionStore: KeychainSessionStore()
    )
}
