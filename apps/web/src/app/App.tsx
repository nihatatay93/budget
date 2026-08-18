import { useQuery } from "@tanstack/react-query";
import { Navigate, Route, Routes } from "react-router-dom";

import { APIError, getSession, sessionQueryKey } from "../api/client";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";
import { RegisterPage } from "../pages/RegisterPage";

export function App() {
  const session = useQuery({
    queryKey: sessionQueryKey,
    queryFn: getSession,
    retry: false,
  });

  if (session.isPending) {
    return (
      <main className="app-shell center-content">
        <section className="brand-panel compact-panel" aria-live="polite">
          <p className="eyebrow">Self-hosted personal finance</p>
          <h1>Budget</h1>
          <p>Opening your workspace…</p>
        </section>
      </main>
    );
  }

  const unauthenticated = session.error instanceof APIError && session.error.status === 401;
  if (session.isError && !unauthenticated) {
    return (
      <main className="app-shell center-content">
        <section className="brand-panel compact-panel">
          <p className="eyebrow">Connection problem</p>
          <h1>Budget</h1>
          <p>{session.error.message}</p>
          <button type="button" onClick={() => void session.refetch()}>
            Try again
          </button>
        </section>
      </main>
    );
  }

  if (unauthenticated) {
    return (
      <Routes>
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route
        path="/"
        element={session.data ? <DashboardPage session={session.data} /> : <Navigate to="/login" replace />}
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
