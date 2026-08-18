import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

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
import { ResourceState } from "../../components/ResourceState";
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
  const query = useQuery({
    queryKey: membersQueryKey(workspace.id),
    queryFn: () => listWorkspaceMembers(workspace.id),
  });

  // Leaving or losing a role changes what this user may see, so the session is refreshed
  // alongside the member list.
  async function refresh() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: membersQueryKey(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: sessionQueryKey }),
    ]);
  }

  const changeRole = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: WorkspaceMember["role"] }) =>
      updateWorkspaceMemberRole(workspace.id, userId, role),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (userId: string) => removeWorkspaceMember(workspace.id, userId),
    onSuccess: refresh,
  });

  return (
    <section className="setup-panel" aria-labelledby="members-heading">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Who can see this workspace</p>
          <h2 id="members-heading">Members</h2>
        </div>
        <span>{query.data?.length ?? 0} active</span>
      </div>
      <ResourceState query={query} empty="No other members yet." />
      <div className="resource-list">
        {query.data?.map((member) => {
          const isSelf = member.user_id === currentUserId;
          const roles = assignableRoles(workspace.role, member.role);
          const removable = canRemoveMember(
            currentUserId,
            member.user_id,
            workspace.role,
            member.role,
          );
          return (
            <article className="resource-row" key={member.user_id}>
              <div>
                <strong>
                  {member.display_name}
                  {isSelf ? " (you)" : ""}
                </strong>
                <small>{member.email}</small>
              </div>
              <div className="resource-actions">
                {roles.length > 0 ? (
                  <label className="inline-field">
                    <span className="visually-hidden">Role for {member.display_name}</span>
                    <select
                      value={member.role}
                      disabled={changeRole.isPending}
                      onChange={(event) =>
                        changeRole.mutate({
                          userId: member.user_id,
                          role: event.target.value as WorkspaceMember["role"],
                        })
                      }
                    >
                      {roles.map((role) => (
                        <option key={role} value={role}>
                          {role}
                        </option>
                      ))}
                    </select>
                  </label>
                ) : (
                  <span>{member.role}</span>
                )}
                {removable ? (
                  <button
                    className="text-button danger"
                    type="button"
                    disabled={remove.isPending}
                    onClick={() => {
                      const prompt = isSelf
                        ? `Leave ${workspace.name}? You will lose access to its data.`
                        : `Remove ${member.display_name} from ${workspace.name}?`;
                      if (window.confirm(prompt)) remove.mutate(member.user_id);
                    }}
                  >
                    {isSelf ? "Leave" : "Remove"}
                  </button>
                ) : null}
              </div>
            </article>
          );
        })}
      </div>
      <MutationError mutation={changeRole} />
      <MutationError mutation={remove} />
    </section>
  );
}
