import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  type CreateWorkspaceInvitationResponse,
  type SessionResponse,
  createWorkspaceInvitation,
  invitationsQueryKey,
  listWorkspaceInvitations,
  revokeWorkspaceInvitation,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";
import { ResourceState } from "../../components/ResourceState";
import { type InvitationRole, canListInvitations, invitableRoles } from "../../lib/workspace";

type Workspace = SessionResponse["workspaces"][number];

export function InvitationsPanel({ workspace }: { workspace: Workspace }) {
  const queryClient = useQueryClient();
  const roles = invitableRoles(workspace.role);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<InvitationRole>(roles[0] ?? "member");
  const [issued, setIssued] = useState<CreateWorkspaceInvitationResponse>();

  const query = useQuery({
    queryKey: invitationsQueryKey(workspace.id),
    queryFn: () => listWorkspaceInvitations(workspace.id),
    // Pending invitations expose the addresses of people who are not members, so this is
    // never requested for a role the server would refuse.
    enabled: canListInvitations(workspace.role),
  });

  const create = useMutation({
    mutationFn: () => createWorkspaceInvitation(workspace.id, { email: email.trim(), role }),
    onSuccess: async (response) => {
      setIssued(response);
      setEmail("");
      await queryClient.invalidateQueries({ queryKey: invitationsQueryKey(workspace.id) });
    },
  });
  const revoke = useMutation({
    mutationFn: (invitationId: string) => revokeWorkspaceInvitation(workspace.id, invitationId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invitationsQueryKey(workspace.id) }),
  });

  if (!canListInvitations(workspace.role)) return null;

  function submit(event: FormEvent) {
    event.preventDefault();
    setIssued(undefined);
    create.mutate();
  }

  return (
    <section className="setup-panel" aria-labelledby="invitations-heading">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Pending access</p>
          <h2 id="invitations-heading">Invitations</h2>
        </div>
        <span>{query.data?.length ?? 0} open</span>
      </div>
      <ResourceState query={query} empty="No invitations are waiting to be accepted." />
      <div className="resource-list">
        {query.data?.map((invitation) => (
          <article className="resource-row" key={invitation.id}>
            <div>
              <strong>{invitation.email}</strong>
              <small>
                {invitation.role} · invited by {invitation.inviter_display_name} · expires{" "}
                {new Date(invitation.expires_at).toLocaleDateString()}
              </small>
            </div>
            <div className="resource-actions">
              <button
                className="text-button danger"
                type="button"
                disabled={revoke.isPending}
                onClick={() => {
                  if (window.confirm(`Revoke the invitation for ${invitation.email}?`)) {
                    revoke.mutate(invitation.id);
                  }
                }}
              >
                Revoke
              </button>
            </div>
          </article>
        ))}
      </div>
      {issued ? <IssuedInvitation issued={issued} /> : null}
      <form className="resource-form" onSubmit={submit}>
        <h3>Invite someone</h3>
        <label>
          Email
          <input
            required
            type="email"
            maxLength={254}
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </label>
        <label>
          Role
          <select value={role} onChange={(event) => setRole(event.target.value as InvitationRole)}>
            {roles.map((candidate) => (
              <option key={candidate} value={candidate}>
                {candidate}
              </option>
            ))}
          </select>
        </label>
        <MutationError mutation={create} />
        <div className="form-actions">
          <button disabled={create.isPending} type="submit">
            Create invitation
          </button>
        </div>
      </form>
      <MutationError mutation={revoke} />
    </section>
  );
}

/**
 * The acceptance token is disclosed once, at creation. There is no email delivery yet, so the
 * inviter passes it on themselves; it is shown here rather than stored, and it is a
 * credential, so it is never written to the console or a query string.
 */
function IssuedInvitation({ issued }: { issued: CreateWorkspaceInvitationResponse }) {
  return (
    <div className="issued-invitation">
      <p>
        Share this one-time code with <strong>{issued.invitation.email}</strong>. It is shown
        only now and cannot be retrieved again.
      </p>
      <code>{issued.acceptance_token}</code>
    </div>
  );
}
