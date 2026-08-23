# Budget for iOS

The iOS client is a native SwiftUI application for the same self-hosted Budget workspace used
by the web client. Its five stable tabs are Overview, Transactions, Budget, Accounts, and More.
Reports, categories, people, workspace switching, server details, and sign-out remain available
under More without crowding the primary tab bar.

Open `Budget.xcodeproj` in Xcode 16 or newer. Xcode 26.x is the validated release toolchain.
The checked-in project uses synchronized folder groups so files added under `BudgetApp/` are
discovered without manually editing the project file.

The local `BudgetAPI` Swift package runs Apple's OpenAPI build plugin. `make generate-api`
synchronizes the root API contract into that package; generated client code remains build
output behind the app's `APIClient` boundary.

Bearer session credentials belong in Keychain; `UserDefaults` contains only the selected
self-hosted server address and must not contain tokens.

Run the repository-level native validation from the project root:

```bash
make ios-check
```

The validation covers the OpenAPI Swift package and the Budget application test build. The UI
supports compact iPhone layouts, regular-width iPad presentation, Dynamic Type, VoiceOver
labels, state restoration, and native pull-to-refresh behavior.
