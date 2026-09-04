import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";

import {
  type AuthResponse,
  type LoginRequest,
  type RegisterRequest,
  login,
  register,
  sessionQueryKey,
} from "../api/client";
import { AppIcon, BrandMark } from "../components/ExperiencePrimitives";
import { type Currency, SUPPORTED_CURRENCIES, currencyLabel } from "../lib/currency";
import { t } from "../lib/i18n";

type AuthMode = "login" | "register";

export function AuthPage({ mode }: { mode: AuthMode }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [workspaceName, setWorkspaceName] = useState(t("Personal"));
  const [baseCurrency, setBaseCurrency] = useState<Currency>("USD");
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";

  const mutation = useMutation({
    mutationFn: async () => {
      if (mode === "login") {
        const input: LoginRequest = { email, password, transport: "cookie" };
        return login(input);
      }
      const input: RegisterRequest = {
        email,
        password,
        display_name: displayName,
        workspace_name: workspaceName,
        base_currency: baseCurrency,
        timezone,
        transport: "cookie",
      };
      return register(input);
    },
    onSuccess: (response: AuthResponse) => {
      queryClient.setQueryData(sessionQueryKey, {
        user: response.user,
        workspaces: response.workspaces,
      });
      navigate("/", { replace: true });
    },
  });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    mutation.mutate();
  }

  const registering = mode === "register";
  return (
    <main className="app-shell auth-shell">
      <section className="brand-panel auth-brand">
        <BrandMark />
        <p className="eyebrow">{t("Private by design")}</p>
        <h1>Budget</h1>
        <p className="auth-promise">
          {t("See where you stand, plan what comes next, and keep every number on infrastructure you control.")}
        </p>
        <ul className="auth-values" aria-label={t("Budget principles")}>
          <li>
            <span className="auth-value-icon"><AppIcon name="chart" /></span>
            <span><strong>{t("Clear by default")}</strong>{t("Balances, spending, and plans stay easy to scan.")}</span>
          </li>
          <li>
            <span className="auth-value-icon"><AppIcon name="shield" /></span>
            <span><strong>{t("Yours by design")}</strong>{t("Self-hosted data with secure web and native sessions.")}</span>
          </li>
          <li>
            <span className="auth-value-icon"><AppIcon name="layers" /></span>
            <span><strong>{t("Built on the ledger")}</strong>{t("Every total traces back to real financial activity.")}</span>
          </li>
        </ul>
      </section>
      <section className="form-panel">
        <div className="form-panel-heading">
          <p className="eyebrow">{registering ? t("Create your home base") : t("Welcome back")}</p>
          <h2>{registering ? t("Start a workspace") : t("Sign in")}</h2>
          <p>
            {registering
              ? t("Create the private space where your accounts, plans, and people come together.")
              : t("Continue to your private financial workspace.")}
          </p>
        </div>
        <form onSubmit={submit}>
          {registering ? (
            <>
              <label>
                {t("Your name")}
                <input
                  autoComplete="name"
                  required
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                />
              </label>
              <label>
                {t("Workspace name")}
                <input
                  required
                  value={workspaceName}
                  onChange={(event) => setWorkspaceName(event.target.value)}
                />
              </label>
              <label>
                {t("Base currency")}
                <select
                  aria-describedby="currency-help"
                  required
                  value={baseCurrency}
                  onChange={(event) => setBaseCurrency(event.target.value as Currency)}
                >
                  {SUPPORTED_CURRENCIES.map((code) => (
                    <option key={code} value={code}>
                      {code} · {currencyLabel(code)}
                    </option>
                  ))}
                </select>
                <span id="currency-help">
                  {t("The reporting currency for this workspace. It cannot be changed later.")}
                </span>
              </label>
            </>
          ) : null}
          <label>
            {t("Email")}
            <input
              autoComplete="email"
              required
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </label>
          <label>
            {t("Password")}
            <input
              autoComplete={registering ? "new-password" : "current-password"}
              minLength={registering ? 15 : undefined}
              maxLength={128}
              required
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
            {registering ? <span>{t("Use at least 15 characters. Spaces are welcome.")}</span> : null}
          </label>
          {mutation.isError ? <p className="form-error" role="alert">{mutation.error.message}</p> : null}
          <button disabled={mutation.isPending} type="submit">
            {mutation.isPending ? t("Working…") : registering ? t("Create workspace") : t("Sign in")}
          </button>
        </form>
        <p className="form-switch">
          {registering ? t("Already have an account?") : t("New to Budget?")}{" "}
          <Link to={registering ? "/login" : "/register"}>
            {registering ? t("Sign in") : t("Create a workspace")}
          </Link>
        </p>
        <p className="auth-footnote"><AppIcon name="shield" size={16} /> {t("Secure session · No third-party analytics")}</p>
      </section>
    </main>
  );
}
