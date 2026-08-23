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
        title: "Member role updated",
        description: `${member.display_name} is now ${member.role}.`,
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
        title: isSelf ? "You left the workspace" : "Member removed",
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
          <p className="eyebrow">Who can see this workspace</p>
          <h2 id="members-heading">Members</h2>
        </div>
        <StatusBadge>{query.data?.length ?? 0} active</StatusBadge>
      </div>
      {query.isPending ? <LoadingState label="Loading workspace members" rows={4} /> : null}
      {query.isError ? (
        <InlineNotice
          action={<button className="secondary-button" onClick={() => void query.refetch()} type="button">Try again</button>}
          title="Members could not be loaded"
          tone="danger"
        >
          <p>{query.error.message}</p>
        </InlineNotice>
      ) : null}
      {!query.isPending && !query.isError && query.data?.length === 0 ? (
        <EmptyState compact description="Invite someone when you are ready to share this workspace." icon="people" title="No members yet" />
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
                <strong>{member.display_name}{isSelf ? " (you)" : ""}</strong>
                <small>
                  <span>{member.email}</span>
                  <span>Joined {new Date(member.joined_at).toLocaleDateString()}</span>
                </small>
              </div>
              <div className="member-controls">
                {roles.length > 0 ? (
                  <label className="inline-field">
                    <span className="visually-hidden">Role for {member.display_name}</span>
                    <select
                      value={member.role}
                      disabled={changeRole.isPending}
                      onChange={(event) => changeRole.mutate({
                        userId: member.user_id,
                        role: event.target.value as WorkspaceMember["role"],
                      })}
                    >
                      {roles.map((role) => <option key={role} value={role}>{role}</option>)}
                    </select>
                  </label>
                ) : <StatusBadge tone={member.role === "owner" ? "positive" : "neutral"}>{member.role}</StatusBadge>}
                {removable ? (
                  <button className="text-button danger" onClick={() => confirmRemoval(member)} type="button">
                    {isSelf ? "Leave" : "Remove"}
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
          ? `You will lose access to ${workspace.name} and its financial data.`
          : `${removing?.display_name ?? "This member"} will immediately lose access to this workspace.`}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setRemoving(undefined)} type="button">Cancel</button>
            <button
              className="danger-button"
              disabled={remove.isPending}
              onClick={() => removing && remove.mutate(removing.user_id)}
              type="button"
            >
              {remove.isPending ? "Updating…" : removing?.user_id === currentUserId ? "Leave workspace" : "Remove member"}
            </button>
          </>
        )}
        onClose={() => setRemoving(undefined)}
        open={Boolean(removing)}
        title={removing?.user_id === currentUserId ? `Leave ${workspace.name}?` : `Remove ${removing?.display_name ?? "member"}?`}
      >
        <InlineNotice title="Access changes immediately" tone="warning">
          <p>Existing financial records and audit attribution are preserved.</p>
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
