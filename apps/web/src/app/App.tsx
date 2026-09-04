import { useQuery } from "@tanstack/react-query";
import { Navigate, Route, Routes } from "react-router-dom";

import { APIError, type SessionResponse, getSession, sessionQueryKey } from "../api/client";
import { AppStatus } from "../components/ExperiencePrimitives";
import { t } from "../lib/i18n";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";
import { RegisterPage } from "../pages/RegisterPage";

export function App() {
  const session = useQuery<SessionResponse | null>({
    queryKey: sessionQueryKey,
    queryFn: getSession,
    retry: false,
  });

  if (session.isPending) {
    return (
      <AppStatus
        description={t("Restoring your secure session and preparing your financial workspace.")}
        eyebrow={t("Private personal finance")}
        title={t("Opening Budget")}
      />
    );
  }

  const unauthenticated = session.data === null
    || (session.error instanceof APIError && session.error.status === 401);
  if (session.isError && !unauthenticated) {
    return (
      <AppStatus
        action={(
          <button type="button" onClick={() => void session.refetch()}>
            {t("Try again")}
          </button>
        )}
        description={session.error.message}
        eyebrow={t("Connection problem")}
        title={t("Budget is unavailable")}
        tone="error"
      />
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
      <Route path="/" element={session.data ? <DashboardPage session={session.data} /> : null} />
      <Route
        path="/workspaces/:workspaceId/*"
        element={session.data ? <DashboardPage session={session.data} /> : null}
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
