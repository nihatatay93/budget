import SwiftUI

struct AppView: View {
    @StateObject private var model: AppModel

    init(environment: AppEnvironment) {
        _model = StateObject(wrappedValue: AppModel(environment: environment))
    }

    var body: some View {
        Group {
            if model.isLoading {
                VStack(spacing: 16) {
                    BudgetWordmark(size: 62)
                    ProgressView()
                        .tint(BudgetTheme.forest)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(BudgetTheme.canvas.ignoresSafeArea())
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
    @AppStorage("budget.textSizePreference") private var textSizePreference = BudgetTextSize.balanced.rawValue

    enum Mode: CaseIterable, Identifiable {
        case login
        case register

        var id: Self { self }

        var title: String {
            switch self {
            case .login: L10n.text("auth.mode.login")
            case .register: L10n.text("auth.mode.register")
            }
        }
    }

    var body: some View {
        ScrollView {
            VStack(spacing: 22) {
                header

                BudgetCard {
                    VStack(alignment: .leading, spacing: 18) {
                        Picker(L10n.text("auth.mode.label"), selection: $mode) {
                            ForEach(Mode.allCases) { mode in
                                Text(mode.title).tag(mode)
                            }
                        }
                        .pickerStyle(.segmented)
                        .accessibilityLabel(L10n.text("auth.mode.label"))

                        VStack(alignment: .leading, spacing: 5) {
                            Text(mode == .login
                                ? L10n.text("auth.login.title")
                                : L10n.text("auth.register.title"))
                                .font(.title3.weight(.bold))
                            Text(mode == .login
                                ? L10n.text("auth.login.subtitle")
                                : L10n.text("auth.register.subtitle"))
                                .font(.subheadline)
                                .foregroundStyle(BudgetTheme.secondaryText)
                        }

                        if mode == .register {
                            registrationFields
                        }

                        credentialFields

                        if let errorMessage = model.errorMessage {
                            Label(errorMessage, systemImage: "exclamationmark.triangle.fill")
                                .font(.footnote)
                                .foregroundStyle(BudgetTheme.over)
                                .accessibilityLabel("\(L10n.text("auth.error")): \(errorMessage)")
                        }

                        Button(action: submit) {
                            HStack(spacing: 8) {
                                if model.isSubmitting {
                                    ProgressView()
                                        .tint(.white)
                                }
                                Text(mode.title)
                            }
                        }
                        .buttonStyle(BudgetPrimaryButtonStyle(isEnabled: formIsValid))
                        .disabled(model.isSubmitting || !formIsValid)
                        .accessibilityHint(L10n.text("auth.submitHint"))
                    }
                }

                DisclosureGroup(L10n.text("auth.server.title")) {
                    VStack(alignment: .leading, spacing: 8) {
                        TextField(L10n.text("auth.server.placeholder"), text: $model.serverAddress)
                            .textInputAutocapitalization(.never)
                            .keyboardType(.URL)
                            .autocorrectionDisabled()
                            .textFieldStyle(.roundedBorder)
                        Text(L10n.text("auth.server.hint"))
                            .font(.footnote)
                            .foregroundStyle(BudgetTheme.tertiaryText)
                    }
                }
                .font(.subheadline.weight(.semibold))
                .padding(.horizontal, 4)
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 28)
        }
        .scrollDismissesKeyboard(.interactively)
        .budgetScreen()
    }

    private var header: some View {
        HStack(spacing: 14) {
            BudgetWordmark(size: 58)

            VStack(alignment: .leading, spacing: 3) {
                Text(L10n.text("auth.brand.name"))
                    .font(.title2.weight(.bold))
                Text(L10n.text("auth.brand.tagline"))
                    .font(.subheadline)
                    .foregroundStyle(BudgetTheme.secondaryText)
            }
            Spacer(minLength: 0)
            Menu {
                Picker(L10n.text("appearance.textSize"), selection: $textSizePreference) {
                    ForEach(BudgetTextSize.allCases) { size in
                        Text(size.title).tag(size.rawValue)
                    }
                }
            } label: {
                Image(systemName: "textformat.size")
                    .font(.headline)
                    .frame(minWidth: 44, minHeight: 44)
                    .contentShape(Rectangle())
            }
            .accessibilityLabel(L10n.text("appearance.textSize"))
            .accessibilityHint(L10n.text("appearance.textSize.hint"))
        }
    }

    private var registrationFields: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(L10n.text("auth.workspace.title"))
                .font(.subheadline.weight(.semibold))
            TextField(L10n.text("auth.name"), text: $displayName)
                .textContentType(.name)
                .textFieldStyle(.roundedBorder)
            TextField(L10n.text("auth.workspace.name"), text: $workspaceName)
                .textFieldStyle(.roundedBorder)
            Picker(L10n.text("auth.currency"), selection: $baseCurrency) {
                ForEach(BudgetCurrency.allCases) { currency in
                    Text(currency.title).tag(currency)
                }
            }
        }
    }

    private var credentialFields: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(L10n.text("auth.credentials.title"))
                .font(.subheadline.weight(.semibold))
            TextField(L10n.text("auth.email"), text: $email)
                .textContentType(.emailAddress)
                .textInputAutocapitalization(.never)
                .keyboardType(.emailAddress)
                .autocorrectionDisabled()
                .textFieldStyle(.roundedBorder)
            // Secure entry already suppresses these, but stating them keeps the field's
            // requirements visible next to the email field's, rather than resting on an
            // implicit UIKit behaviour that a future refactor could lose.
            SecureField(L10n.text("auth.password"), text: $password)
                .textContentType(mode == .register ? .newPassword : .password)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .textFieldStyle(.roundedBorder)
            if mode == .register {
                Text(L10n.text("auth.password.hint"))
                    .font(.footnote)
                    .foregroundStyle(BudgetTheme.tertiaryText)
            }
        }
    }

    private func submit() {
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
    @SceneStorage("selectedWorkspaceID") private var selectedWorkspaceID: String?

    var body: some View {
        Group {
            if let selectedWorkspace {
                WorkspaceSetupView(
                    workspace: selectedWorkspace,
                    session: session,
                    model: model,
                    onSelectWorkspace: { selectedWorkspaceID = $0 }
                )
                .id(selectedWorkspace.id)
            } else {
                NavigationStack {
                    ScrollView {
                        VStack(alignment: .leading, spacing: 12) {
                            if session.workspaces.isEmpty {
                                BudgetCard {
                                    BudgetMessage(
                                        title: "No workspaces",
                                        systemImage: "rectangle.3.group",
                                        message: "Accept an invitation or create a workspace on the web to get started."
                                    )
                                }
                            } else {
                                Text("Choose a workspace to open.")
                                    .font(.subheadline)
                                    .foregroundStyle(BudgetTheme.secondaryText)
                                    .padding(.horizontal, 2)
                                    .padding(.bottom, 4)

                                ForEach(session.workspaces) { workspace in
                                    Button {
                                        selectedWorkspaceID = workspace.id
                                    } label: {
                                        WorkspacePickerRow(workspace: workspace)
                                    }
                                    .buttonStyle(.plain)
                                    .accessibilityHint("Opens this workspace")
                                }
                            }
                        }
                        .padding(.horizontal, BudgetTheme.Space.screen)
                        .padding(.top, 4)
                        .padding(.bottom, 32)
                    }
                    .budgetScreen()
                    .navigationTitle("Hello, \(session.user.displayName)")
                    .toolbar {
                        ToolbarItem(placement: .topBarTrailing) {
                            Button("Sign out") {
                                Task { await model.logout() }
                            }
                        }
                    }
                }
            }
        }
    }

    private var selectedWorkspace: BudgetWorkspace? {
        session.workspaces.first { $0.id == selectedWorkspaceID }
    }
}

#Preview {
    AppView(environment: .live)
}

/// One workspace in the picker. The initials tile gives the list a visual anchor, which the
/// previous plain text rows on a black canvas did not have.
private struct WorkspacePickerRow: View {
    let workspace: BudgetWorkspace

    var body: some View {
        BudgetCard(padding: 16) {
            HStack(spacing: 14) {
                ZStack {
                    RoundedRectangle(cornerRadius: 13, style: .continuous)
                        .fill(
                            LinearGradient(
                                colors: [BudgetTheme.forest, BudgetTheme.deepForest],
                                startPoint: .topLeading,
                                endPoint: .bottomTrailing
                            )
                        )
                    Text(initials)
                        .font(.system(size: 16, weight: .semibold))
                        .foregroundStyle(.white)
                }
                .frame(width: 44, height: 44)
                .accessibilityHidden(true)

                VStack(alignment: .leading, spacing: 3) {
                    Text(workspace.name)
                        .font(.headline)
                        .foregroundStyle(BudgetTheme.primaryText)
                    Text("\(workspace.baseCurrency.rawValue) · \(workspace.timezone)")
                        .font(.caption)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                        .lineLimit(1)
                }
                Spacer(minLength: 8)
                BudgetChip(text: .resolved(L10n.workspaceRole(workspace.role)), color: BudgetTheme.forest)
                Image(systemName: "chevron.right")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .accessibilityHidden(true)
            }
        }
        .accessibilityElement(children: .combine)
    }

    private var initials: String {
        let letters = workspace.name.split(separator: " ").prefix(2).compactMap { $0.first }
        return letters.isEmpty ? "B" : String(letters).uppercased()
    }
}

/// The Budget mark, shared by the launch state and the sign-in header.
struct BudgetWordmark: View {
    var size: CGFloat = 58

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: size * 0.31, style: .continuous)
                .fill(
                    LinearGradient(
                        colors: [BudgetTheme.forest, BudgetTheme.deepForest],
                        startPoint: .topLeading,
                        endPoint: .bottomTrailing
                    )
                )
                .overlay {
                    RoundedRectangle(cornerRadius: size * 0.31, style: .continuous)
                        .stroke(.white.opacity(0.12), lineWidth: 1)
                }
            Text("B")
                .font(.system(size: size * 0.46, design: .serif).weight(.bold))
                .foregroundStyle(.white)
        }
        .frame(width: size, height: size)
        .shadow(color: BudgetTheme.deepForest.opacity(0.5), radius: 12, y: 6)
        .accessibilityHidden(true)
    }
}
