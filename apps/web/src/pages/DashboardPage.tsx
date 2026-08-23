import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Link,
  Navigate,
  Route,
  Routes,
  useLocation,
  useNavigate,
  useParams,
} from "react-router-dom";

import { type SessionResponse, logout, sessionQueryKey } from "../api/client";
import { AppIcon, AppStatus, BrandMark } from "../components/ExperiencePrimitives";
import { PageHeader, StatusBadge } from "../components/Presentation";
import {
  type WorkspaceDestination,
  WorkspaceSetupPage,
  workspaceDestinations,
} from "./WorkspaceSetupPage";

type Workspace = SessionResponse["workspaces"][number];
type IconName = Parameters<typeof AppIcon>[0]["name"];
type NavigationItem = { destination: WorkspaceDestination; icon: IconName; label: string };

const primaryNavigation: NavigationItem[] = [
  { destination: "overview", icon: "home", label: "Overview" },
  { destination: "transactions", icon: "transactions", label: "Transactions" },
  { destination: "budget", icon: "budget", label: "Budget" },
  { destination: "reports", icon: "chart", label: "Reports" },
];

const managementNavigation: NavigationItem[] = [
  { destination: "accounts", icon: "accounts", label: "Accounts" },
  { destination: "categories", icon: "categories", label: "Categories" },
  { destination: "people", icon: "people", label: "People" },
];

const compactNavigation: NavigationItem[] = [
  ...primaryNavigation.slice(0, 3),
  { destination: "accounts", icon: "accounts", label: "Accounts" },
  { destination: "more", icon: "more", label: "More" },
];

const pageCopy: Record<WorkspaceDestination, { eyebrow: string; title: string; description: string }> = {
  overview: {
    eyebrow: "Today at a glance",
    title: "Overview",
    description: "Your posted position, pending activity, and current monthly plan.",
  },
  transactions: {
    eyebrow: "Money in motion",
    title: "Transactions",
    description: "Record and review the activity behind every balance and report.",
  },
  budget: {
    eyebrow: "Plan with intention",
    title: "Budget",
    description: "Set category targets and compare them with posted spending.",
  },
  reports: {
    eyebrow: "Understand the pattern",
    title: "Reports",
    description: "Explore balances, income, spending, and categories for a selected period.",
  },
  accounts: {
    eyebrow: "Where money lives",
    title: "Accounts",
    description: "See balances and organize the accounts represented in your ledger.",
  },
  categories: {
    eyebrow: "How money is understood",
    title: "Categories",
    description: "Keep reporting and monthly plans organized around your priorities.",
  },
  people: {
    eyebrow: "Shared with care",
    title: "People",
    description: "Manage workspace access, roles, and invitations.",
  },
  more: {
    eyebrow: "Workspace and account",
    title: "More",
    description: "Find reporting, organization, collaboration, and session controls.",
  },
};

export function DashboardPage({ session }: { session: SessionResponse }) {
  const { workspaceId } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const workspace = session.workspaces.find((candidate) => candidate.id === workspaceId);
  const destination = destinationFromPath(location.pathname);
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      // Mark the observed session as explicitly absent before changing routes. Removing the
      // query can leave its last successful value mounted long enough for the authenticated
      // catch-all route to send /login back to the workspace.
      queryClient.setQueryData<SessionResponse | null>(sessionQueryKey, null);
      navigate("/login", { replace: true });
    },
  });

  if (session.workspaces.length === 0) {
    return (
      <AppStatus
        action={(
          <button disabled={logoutMutation.isPending} onClick={() => logoutMutation.mutate()} type="button">
            Sign out
          </button>
        )}
        description="This session does not currently belong to a workspace. Ask a workspace owner for an invitation."
        eyebrow="Workspace required"
        title="No workspace available"
        tone="empty"
      />
    );
  }

  if (!workspace) {
    return <Navigate replace to={workspacePath(session.workspaces[0].id, "overview")} />;
  }

  function changeWorkspace(nextWorkspaceId: string) {
    navigate(workspacePath(nextWorkspaceId, destination));
  }

  return (
    <div className="workspace-app">
      <a className="skip-link" href="#workspace-content">Skip to content</a>
      <aside className="app-sidebar" aria-label="Workspace navigation">
        <div className="sidebar-brand"><BrandMark withName /></div>
        <WorkspacePicker
          className="sidebar-workspace-picker"
          onChange={changeWorkspace}
          session={session}
          workspace={workspace}
        />
        <WorkspaceNavigation items={primaryNavigation} workspace={workspace} />
        <WorkspaceNavigation label="Manage" items={managementNavigation} workspace={workspace} />
        <div className="sidebar-account">
          <div className="user-avatar" aria-hidden="true">{initials(session.user.display_name)}</div>
          <div>
            <strong>{session.user.display_name}</strong>
            <span>{session.user.email}</span>
          </div>
          <button
            className="text-button sidebar-signout"
            disabled={logoutMutation.isPending}
            onClick={() => logoutMutation.mutate()}
            type="button"
          >
            Sign out
          </button>
        </div>
      </aside>

      <div className="workspace-frame">
        <header className="compact-app-header">
          <BrandMark withName />
          <WorkspacePicker
            className="compact-workspace-picker"
            onChange={changeWorkspace}
            session={session}
            workspace={workspace}
          />
        </header>
        <main className="workspace-content" id="workspace-content" tabIndex={-1}>
          <PageHeader
            description={pageCopy[destination].description}
            eyebrow={pageCopy[destination].eyebrow}
            meta={<StatusBadge tone="positive">{workspace.role}</StatusBadge>}
            title={pageCopy[destination].title}
          />
          <Routes>
            <Route index element={<Navigate replace to="overview" />} />
            {workspaceDestinations.map((routeDestination) => (
              <Route
                element={(
                  <WorkspaceSetupPage
                    destination={routeDestination}
                    onSignOut={() => logoutMutation.mutate()}
                    session={session}
                    workspace={workspace}
                  />
                )}
                key={routeDestination}
                path={routeDestination}
              />
            ))}
            <Route path="*" element={<Navigate replace to="overview" />} />
          </Routes>
        </main>
        <nav className="bottom-navigation" aria-label="Primary navigation">
          {compactNavigation.map((item) => (
            <WorkspaceNavLink item={item} key={item.destination} workspace={workspace} />
          ))}
        </nav>
      </div>
    </div>
  );
}

function WorkspacePicker({
  className,
  onChange,
  session,
  workspace,
}: {
  className: string;
  onChange: (workspaceId: string) => void;
  session: SessionResponse;
  workspace: Workspace;
}) {
  return (
    <label className={className}>
      <span>Workspace</span>
      <select
        aria-label="Current workspace"
        onChange={(event) => onChange(event.target.value)}
        value={workspace.id}
      >
        {session.workspaces.map((candidate) => (
          <option key={candidate.id} value={candidate.id}>{candidate.name}</option>
        ))}
      </select>
      <small>{workspace.base_currency} · {workspace.role}</small>
    </label>
  );
}

function WorkspaceNavigation({
  items,
  label,
  workspace,
}: {
  items: NavigationItem[];
  label?: string;
  workspace: Workspace;
}) {
  return (
    <nav className="sidebar-navigation" aria-label={label ?? "Primary"}>
      {label ? <span className="sidebar-navigation-label">{label}</span> : null}
      {items.map((item) => (
        <WorkspaceNavLink item={item} key={item.destination} workspace={workspace} />
      ))}
    </nav>
  );
}

function WorkspaceNavLink({ item, workspace }: { item: NavigationItem; workspace: Workspace }) {
  const location = useLocation();
  const destination = destinationFromPath(location.pathname);
  const isActive = destination === item.destination;
  const groupedUnderMore = item.destination === "more"
    && (["more", "reports", "categories", "people"] as WorkspaceDestination[]).includes(destination);
  return (
    <Link
      aria-current={isActive || groupedUnderMore ? "page" : undefined}
      className={`workspace-nav-link${isActive || groupedUnderMore ? " active" : ""}`}
      to={workspacePath(workspace.id, item.destination)}
    >
      <AppIcon name={item.icon} />
      <span>{item.label}</span>
    </Link>
  );
}

function destinationFromPath(pathname: string): WorkspaceDestination {
  const candidate = pathname.split("/").filter(Boolean).at(-1);
  return workspaceDestinations.includes(candidate as WorkspaceDestination)
    ? candidate as WorkspaceDestination
    : "overview";
}

function workspacePath(workspaceId: string, destination: WorkspaceDestination) {
  return `/workspaces/${workspaceId}/${destination}`;
}

function initials(displayName: string) {
  return displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("") || "B";
}
