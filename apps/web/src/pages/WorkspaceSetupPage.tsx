import type { SessionResponse } from "../api/client";
import { AccountsPanel } from "../features/accounts/AccountsPanel";
import { BudgetPanel } from "../features/budgets/BudgetPanel";
import { CategoriesPanel } from "../features/categories/CategoriesPanel";
import { AcceptInvitationPanel } from "../features/collaboration/AcceptInvitationPanel";
import { InvitationsPanel } from "../features/collaboration/InvitationsPanel";
import { MembersPanel } from "../features/collaboration/MembersPanel";
import { FinancialDashboard } from "../features/dashboard/FinancialDashboard";
import { TransactionsPanel } from "../features/transactions/TransactionsPanel";
import { canManageWorkspace } from "../lib/workspace";

type Workspace = SessionResponse["workspaces"][number];

/** Composes the workspace feature panels; each panel owns its own data and mutations. */
export function WorkspaceSetupPage({
  workspace,
  currentUserId,
}: {
  workspace: Workspace;
  currentUserId: string;
}) {
  const canManage = canManageWorkspace(workspace);
  return (
    <>
      <FinancialDashboard workspace={workspace} />
      <BudgetPanel key={workspace.id} workspace={workspace} canManage={canManage} />
      <div className="setup-grid">
        <AccountsPanel workspace={workspace} canManage={canManage} />
        <CategoriesPanel workspace={workspace} canManage={canManage} />
      </div>
      <TransactionsPanel workspace={workspace} canManage={canManage} />
      <div className="setup-grid">
        <MembersPanel workspace={workspace} currentUserId={currentUserId} />
        <InvitationsPanel workspace={workspace} />
      </div>
      <AcceptInvitationPanel />
    </>
  );
}
