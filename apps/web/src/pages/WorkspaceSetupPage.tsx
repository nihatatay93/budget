import { Link } from "react-router-dom";

import type { SessionResponse } from "../api/client";
import { AppIcon } from "../components/ExperiencePrimitives";
import { StatusBadge, SurfaceCard } from "../components/Presentation";
import { AccountsPanel } from "../features/accounts/AccountsPanel";
import { BudgetPanel } from "../features/budgets/BudgetPanel";
import { CategoriesPanel } from "../features/categories/CategoriesPanel";
import { AcceptInvitationPanel } from "../features/collaboration/AcceptInvitationPanel";
import { InvitationsPanel } from "../features/collaboration/InvitationsPanel";
import { MembersPanel } from "../features/collaboration/MembersPanel";
import { FinancialDashboard } from "../features/dashboard/FinancialDashboard";
import { OverviewDashboard } from "../features/dashboard/OverviewDashboard";
import { TransactionsPanel } from "../features/transactions/TransactionsPanel";
import { canListInvitations, canManageWorkspace } from "../lib/workspace";

type Workspace = SessionResponse["workspaces"][number];

export const workspaceDestinations = [
  "overview",
  "transactions",
  "budget",
  "reports",
  "accounts",
  "categories",
  "people",
  "more",
] as const;

export type WorkspaceDestination = typeof workspaceDestinations[number];

/** Resolves one focused workspace destination while each feature retains its own server state. */
export function WorkspaceSetupPage({
  destination,
  onSignOut,
  session,
  workspace,
}: {
  destination: WorkspaceDestination;
  onSignOut: () => void;
  session: SessionResponse;
  workspace: Workspace;
}) {
  const canManage = canManageWorkspace(workspace);

  switch (destination) {
    case "overview":
      return <OverviewDashboard canManage={canManage} workspace={workspace} />;
    case "reports":
      return <FinancialDashboard workspace={workspace} />;
    case "budget":
      return <BudgetPanel key={workspace.id} workspace={workspace} canManage={canManage} />;
    case "accounts":
      return <AccountsPanel workspace={workspace} canManage={canManage} />;
    case "categories":
      return <CategoriesPanel workspace={workspace} canManage={canManage} />;
    case "transactions":
      return <TransactionsPanel workspace={workspace} canManage={canManage} />;
    case "people":
      return (
        <div className="people-destination">
          <SurfaceCard className="people-role-summary">
            <div>
              <p className="eyebrow">Your access</p>
              <strong>{workspace.name}</strong>
              <span>Role permissions are enforced by the server on every membership operation.</span>
            </div>
            <StatusBadge tone="positive">{workspace.role}</StatusBadge>
          </SurfaceCard>
          <div className={`people-layout${canListInvitations(workspace.role) ? "" : " people-layout-single"}`}>
            <MembersPanel workspace={workspace} currentUserId={session.user.id} />
            {canListInvitations(workspace.role) ? <InvitationsPanel workspace={workspace} /> : null}
          </div>
        </div>
      );
    case "more":
      return <MoreDestination onSignOut={onSignOut} session={session} workspace={workspace} />;
  }
}

function MoreDestination({
  onSignOut,
  session,
  workspace,
}: {
  onSignOut: () => void;
  session: SessionResponse;
  workspace: Workspace;
}) {
  const destinations = [
    { path: "reports", icon: "chart" as const, title: "Reports", detail: "Balances and category activity" },
    { path: "categories", icon: "categories" as const, title: "Categories", detail: "Reporting organization" },
    { path: "people", icon: "people" as const, title: "People", detail: "Members, roles, and invitations" },
  ];

  return (
    <div className="more-destination">
      <SurfaceCard className="workspace-profile-card" labelledBy="workspace-profile-heading">
        <div className="workspace-mark">{workspace.name.slice(0, 1).toUpperCase()}</div>
        <div>
          <p className="eyebrow">Current workspace</p>
          <h2 id="workspace-profile-heading">{workspace.name}</h2>
          <p>{workspace.base_currency} · {workspace.timezone} · {workspace.role}</p>
        </div>
      </SurfaceCard>
      {session.workspaces.length > 1 ? (
        <SurfaceCard className="workspace-switch-card" labelledBy="workspace-switch-heading">
          <div>
            <p className="eyebrow">Workspace switcher</p>
            <h2 id="workspace-switch-heading">Your workspaces</h2>
          </div>
          <nav aria-label="Available workspaces">
            {session.workspaces.map((candidate) => (
              <Link
                aria-current={candidate.id === workspace.id ? "page" : undefined}
                key={candidate.id}
                to={`/workspaces/${candidate.id}/more`}
              >
                <span className="workspace-list-mark">{candidate.name.slice(0, 1).toUpperCase()}</span>
                <span><strong>{candidate.name}</strong><small>{candidate.base_currency} · {candidate.role}</small></span>
                {candidate.id === workspace.id ? <StatusBadge tone="positive">Current</StatusBadge> : null}
              </Link>
            ))}
          </nav>
        </SurfaceCard>
      ) : null}
      <nav className="more-link-grid" aria-label="More workspace destinations">
        {destinations.map((item) => (
          <Link key={item.path} to={`/workspaces/${workspace.id}/${item.path}`}>
            <span className="more-link-icon"><AppIcon name={item.icon} /></span>
            <span><strong>{item.title}</strong><small>{item.detail}</small></span>
          </Link>
        ))}
      </nav>
      <div className="settings-grid">
        <SurfaceCard className="account-profile-card" labelledBy="account-profile-heading">
          <div className="user-avatar" aria-hidden="true">{initials(session.user.display_name)}</div>
          <div>
            <p className="eyebrow">Signed-in account</p>
            <h2 id="account-profile-heading">{session.user.display_name}</h2>
            <span>{session.user.email}</span>
          </div>
        </SurfaceCard>
        <SurfaceCard className="session-card">
          <div>
            <p className="eyebrow">Session security</p>
            <strong>Secure browser session</strong>
            <span>Sign out here without affecting your other sessions.</span>
          </div>
          <button className="secondary-button" onClick={onSignOut} type="button">Sign out</button>
        </SurfaceCard>
      </div>
      <AcceptInvitationPanel />
    </div>
  );
}

function initials(displayName: string) {
  return displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("") || "B";
}
