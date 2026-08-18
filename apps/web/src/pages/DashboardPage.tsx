import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { type SessionResponse, logout, sessionQueryKey } from "../api/client";
import { WorkspaceSetupPage } from "./WorkspaceSetupPage";

export function DashboardPage({ session }: { session: SessionResponse }) {
  const queryClient = useQueryClient();
  const [workspaceId, setWorkspaceId] = useState(session.workspaces[0]?.id ?? "");
  const workspace = session.workspaces.find((candidate) => candidate.id === workspaceId);
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: sessionQueryKey });
      window.location.assign("/login");
    },
  });

  return (
    <main className="app-shell dashboard-shell">
      <header className="dashboard-header">
        <div>
          <p className="eyebrow">Your workspaces</p>
          <h1>Good to see you, {session.user.display_name}</h1>
        </div>
        <button
          className="secondary-button"
          disabled={logoutMutation.isPending}
          onClick={() => logoutMutation.mutate()}
          type="button"
        >
          Sign out
        </button>
      </header>
      {session.workspaces.length > 1 ? (
        <label className="workspace-picker">
          Workspace
          <select value={workspaceId} onChange={(event) => setWorkspaceId(event.target.value)}>
            {session.workspaces.map((candidate) => (
              <option key={candidate.id} value={candidate.id}>
                {candidate.name}
              </option>
            ))}
          </select>
        </label>
      ) : null}
      {workspace ? (
        <>
          <section className="workspace-summary" aria-label="Selected workspace">
            <div className="workspace-mark">{workspace.name.slice(0, 1).toUpperCase()}</div>
            <div>
              <h2>{workspace.name}</h2>
              <p>
                {workspace.base_currency} · {workspace.timezone} · {workspace.role}
              </p>
            </div>
          </section>
          <WorkspaceSetupPage workspace={workspace} currentUserId={session.user.id} />
        </>
      ) : (
        <p className="milestone-note">No workspace is available for this session.</p>
      )}
    </main>
  );
}
