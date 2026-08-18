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
import { type Currency, SUPPORTED_CURRENCIES, currencyLabel } from "../lib/currency";

type AuthMode = "login" | "register";

export function AuthPage({ mode }: { mode: AuthMode }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [workspaceName, setWorkspaceName] = useState("Personal");
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
        <p className="eyebrow">Private by design</p>
        <h1>Budget</h1>
        <p>One calm place for the money decisions that shape your everyday life.</p>
      </section>
      <section className="form-panel">
        <div>
          <p className="eyebrow">{registering ? "Create your home base" : "Welcome back"}</p>
          <h2>{registering ? "Start a workspace" : "Sign in"}</h2>
        </div>
        <form onSubmit={submit}>
          {registering ? (
            <>
              <label>
                Your name
                <input
                  autoComplete="name"
                  required
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                />
              </label>
              <label>
                Workspace name
                <input
                  required
                  value={workspaceName}
                  onChange={(event) => setWorkspaceName(event.target.value)}
                />
              </label>
              <label>
                Base currency
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
                  The reporting currency for this workspace. It cannot be changed later.
                </span>
              </label>
            </>
          ) : null}
          <label>
            Email
            <input
              autoComplete="email"
              required
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </label>
          <label>
            Password
            <input
              autoComplete={registering ? "new-password" : "current-password"}
              minLength={registering ? 15 : undefined}
              maxLength={128}
              required
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
            {registering ? <span>Use at least 15 characters. Spaces are welcome.</span> : null}
          </label>
          {mutation.isError ? <p className="form-error">{mutation.error.message}</p> : null}
          <button disabled={mutation.isPending} type="submit">
            {mutation.isPending ? "Working…" : registering ? "Create workspace" : "Sign in"}
          </button>
        </form>
        <p className="form-switch">
          {registering ? "Already have an account?" : "New to Budget?"}{" "}
          <Link to={registering ? "/login" : "/register"}>
            {registering ? "Sign in" : "Create a workspace"}
          </Link>
        </p>
      </section>
    </main>
  );
}
