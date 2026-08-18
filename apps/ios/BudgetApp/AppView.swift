import SwiftUI

struct AppView: View {
    @StateObject private var model: AppModel

    init(environment: AppEnvironment) {
        _model = StateObject(wrappedValue: AppModel(environment: environment))
    }

    var body: some View {
        Group {
            if model.isLoading {
                ProgressView("Opening Budget…")
            } else if let session = model.session {
                WorkspaceView(session: session, model: model)
            } else {
                AuthenticationView(model: model)
            }
        }
        .task { await model.restore() }
    }
}

private struct AuthenticationView: View {
    @ObservedObject var model: AppModel
    @State private var mode = Mode.login
    @State private var email = ""
    @State private var password = ""
    @State private var displayName = ""
    @State private var workspaceName = "Personal"
    @State private var baseCurrency = BudgetCurrency.usDollar

    enum Mode: String, CaseIterable, Identifiable {
        case login = "Sign in"
        case register = "Register"

        var id: Self { self }
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("Server") {
                    TextField("https://budget.example", text: $model.serverAddress)
                        .textInputAutocapitalization(.never)
                        .keyboardType(.URL)
                        .autocorrectionDisabled()
                }
                Section {
                    Picker("Authentication", selection: $mode) {
                        ForEach(Mode.allCases) { mode in
                            Text(mode.rawValue).tag(mode)
                        }
                    }
                    .pickerStyle(.segmented)
                }
                if mode == .register {
                    Section("Workspace") {
                        TextField("Your name", text: $displayName)
                            .textContentType(.name)
                        TextField("Workspace name", text: $workspaceName)
                        Picker("Base currency", selection: $baseCurrency) {
                            ForEach(BudgetCurrency.allCases) { currency in
                                Text(currency.title).tag(currency)
                            }
                        }
                    }
                }
                Section("Credentials") {
                    TextField("Email", text: $email)
                        .textContentType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .keyboardType(.emailAddress)
                        .autocorrectionDisabled()
                    SecureField("Password", text: $password)
                        .textContentType(mode == .register ? .newPassword : .password)
                    if mode == .register {
                        Text("Use at least 15 characters. Spaces are welcome.")
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                    }
                }
                if let errorMessage = model.errorMessage {
                    Section {
                        Text(errorMessage)
                            .foregroundStyle(.red)
                            .accessibilityLabel("Error: \(errorMessage)")
                    }
                }
                Section {
                    Button(mode.rawValue) {
                        Task {
                            if mode == .login {
                                await model.login(email: email, password: password)
                            } else {
                                await model.register(
                                    email: email,
                                    password: password,
                                    displayName: displayName,
                                    workspaceName: workspaceName,
                                    baseCurrency: baseCurrency
                                )
                            }
                        }
                    }
                    .disabled(model.isSubmitting || !formIsValid)
                }
            }
            .navigationTitle("Budget")
        }
    }

    private var formIsValid: Bool {
        guard !email.isEmpty, !password.isEmpty else { return false }
        if mode == .login { return true }
        return password.count >= 15
            && password.count <= 128
            && !displayName.isEmpty
            && !workspaceName.isEmpty
    }
}

private struct WorkspaceView: View {
    let session: UserSession
    @ObservedObject var model: AppModel

    var body: some View {
        NavigationStack {
            List(session.workspaces) { workspace in
                NavigationLink {
                    WorkspaceSetupView(
                        workspace: workspace,
                        currentUserID: session.user.id,
                        model: model
                    )
                } label: {
                    VStack(alignment: .leading, spacing: 5) {
                        Text(workspace.name)
                            .font(.headline)
                        Text("\(workspace.baseCurrency.rawValue) · \(workspace.timezone)")
                            .foregroundStyle(.secondary)
                        Text(workspace.role.uppercased())
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(.green)
                    }
                    .padding(.vertical, 4)
                }
            }
            .navigationTitle("Hello, \(session.user.displayName)")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Sign out") {
                        Task { await model.logout() }
                    }
                }
            }
            .overlay {
                if session.workspaces.isEmpty {
                    ContentUnavailableView("No workspaces", systemImage: "rectangle.3.group")
                }
            }
        }
    }
}

#Preview {
    AppView(environment: .live)
}
