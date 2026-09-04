import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import {
  type SessionResponse,
  type WorkspaceMember,
  listWorkspaceMembers,
  membersQueryKey,
  removeWorkspaceMember,
  sessionQueryKey,
  updateWorkspaceMemberRole,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  ModalDialog,
  StatusBadge,
  ToastRegion,
  type ToastMessage,
} from "../../components/Presentation";
import { assignableRoles, canRemoveMember } from "../../lib/workspace";
import { roleLabel, t } from "../../lib/i18n";

type Workspace = SessionResponse["workspaces"][number];

export function MembersPanel({
  workspace,
  currentUserId,
}: {
  workspace: Workspace;
  currentUserId: string;
}) {
  const queryClient = useQueryClient();
  const [removing, setRemoving] = useState<WorkspaceMember>();
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const query = useQuery({
    queryKey: membersQueryKey(workspace.id),
    queryFn: () => listWorkspaceMembers(workspace.id),
  });

  async function refresh() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: membersQueryKey(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: sessionQueryKey }),
    ]);
  }

  const changeRole = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: WorkspaceMember["role"] }) =>
      updateWorkspaceMemberRole(workspace.id, userId, role),
    onSuccess: async (member) => {
      setToasts([{
        id: `member-role-${member.user_id}-${member.role}`,
        title: t("Member role updated"),
        description: t("{name} is now {role}.", { name: member.display_name, role: roleLabel(member.role) }),
        tone: "positive",
      }]);
      await refresh();
    },
  });
  const remove = useMutation({
    mutationFn: (userId: string) => removeWorkspaceMember(workspace.id, userId),
    onSuccess: async () => {
      const isSelf = removing?.user_id === currentUserId;
      setToasts([{
        id: `member-remove-${removing?.user_id ?? "complete"}`,
        title: isSelf ? t("You left the workspace") : t("Member removed"),
        tone: "positive",
      }]);
      setRemoving(undefined);
      await refresh();
    },
  });

  function confirmRemoval(member: WorkspaceMember) {
    remove.reset();
    setRemoving(member);
  }

  return (
    <section className="people-panel members-panel" aria-labelledby="members-heading">
      <div className="people-panel-heading">
        <div>
          <p className="eyebrow">{t("Who can see this workspace")}</p>
          <h2 id="members-heading">{t("Members")}</h2>
        </div>
        <StatusBadge>{t("{count} active", { count: query.data?.length ?? 0 })}</StatusBadge>
      </div>
      {query.isPending ? <LoadingState label={t("Loading workspace members")} rows={4} /> : null}
      {query.isError ? (
        <InlineNotice
          action={<button className="secondary-button" onClick={() => void query.refetch()} type="button">{t("Try again")}</button>}
          title={t("Members could not be loaded")}
          tone="danger"
        >
          <p>{query.error.message}</p>
        </InlineNotice>
      ) : null}
      {!query.isPending && !query.isError && query.data?.length === 0 ? (
        <EmptyState compact description={t("Invite someone when you are ready to share this workspace.")} icon="people" title={t("No members yet")} />
      ) : null}
      <div className="member-list">
        {query.data?.map((member) => {
          const isSelf = member.user_id === currentUserId;
          const roles = assignableRoles(workspace.role, member.role);
          const removable = canRemoveMember(currentUserId, member.user_id, workspace.role, member.role);
          return (
            <article className="member-row" key={member.user_id}>
              <div aria-hidden="true" className="member-avatar">{initials(member.display_name)}</div>
              <div className="member-identity">
                <strong>{member.display_name}{isSelf ? ` ${t("(you)")}` : ""}</strong>
                <small>
                  <span>{member.email}</span>
                  <span>{t("Joined {date}", { date: new Date(member.joined_at).toLocaleDateString() })}</span>
                </small>
              </div>
              <div className="member-controls">
                {roles.length > 0 ? (
                  <label className="inline-field">
                    <span className="visually-hidden">{t("Role for {name}", { name: member.display_name })}</span>
                    <select
                      value={member.role}
                      disabled={changeRole.isPending}
                      onChange={(event) => changeRole.mutate({
                        userId: member.user_id,
                        role: event.target.value as WorkspaceMember["role"],
                      })}
                    >
                      {roles.map((role) => <option key={role} value={role}>{roleLabel(role)}</option>)}
                    </select>
                  </label>
                ) : <StatusBadge tone={member.role === "owner" ? "positive" : "neutral"}>{roleLabel(member.role)}</StatusBadge>}
                {removable ? (
                  <button className="text-button danger" onClick={() => confirmRemoval(member)} type="button">
                    {isSelf ? t("Leave") : t("Remove")}
                  </button>
                ) : null}
              </div>
            </article>
          );
        })}
      </div>
      <MutationError mutation={changeRole} />
      <ModalDialog
        description={removing?.user_id === currentUserId
          ? t("You will lose access to {workspace} and its financial data.", { workspace: workspace.name })
          : t("{member} will immediately lose access to this workspace.", { member: removing?.display_name ?? t("This member") })}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setRemoving(undefined)} type="button">{t("Cancel")}</button>
            <button
              className="danger-button"
              disabled={remove.isPending}
              onClick={() => removing && remove.mutate(removing.user_id)}
              type="button"
            >
              {remove.isPending ? t("Updating…") : removing?.user_id === currentUserId ? t("Leave workspace") : t("Remove member")}
            </button>
          </>
        )}
        onClose={() => setRemoving(undefined)}
        open={Boolean(removing)}
        title={removing?.user_id === currentUserId ? t("Leave {workspace}?", { workspace: workspace.name }) : t("Remove {member}?", { member: removing?.display_name ?? t("member") })}
      >
        <InlineNotice title={t("Access changes immediately")} tone="warning">
          <p>{t("Existing financial records and audit attribution are preserved.")}</p>
        </InlineNotice>
        <MutationError mutation={remove} />
      </ModalDialog>
      <ToastRegion messages={toasts} onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))} />
    </section>
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
