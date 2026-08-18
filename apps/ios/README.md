# Budget for iOS

The iOS client is a native SwiftUI application. Open `Budget.xcodeproj` in Xcode 16 or
newer. The checked-in project uses synchronized folder groups so files added under
`BudgetApp/` are discovered without manually editing the project file.

The local `BudgetAPI` Swift package runs Apple's OpenAPI build plugin. `make generate-api`
synchronizes the root API contract into that package; generated client code remains build
output behind the app's `APIClient` boundary.

Bearer session credentials belong in Keychain; `UserDefaults` contains only the selected
self-hosted server address and must not contain tokens.
