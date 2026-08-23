import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  type CreateWorkspaceInvitationResponse,
  type SessionResponse,
  type WorkspaceInvitation,
  createWorkspaceInvitation,
  invitationsQueryKey,
  listWorkspaceInvitations,
  revokeWorkspaceInvitation,
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
import { type InvitationRole, canListInvitations, invitableRoles } from "../../lib/workspace";

type Workspace = SessionResponse["workspaces"][number];

export function InvitationsPanel({ workspace }: { workspace: Workspace }) {
  const queryClient = useQueryClient();
  const roles = invitableRoles(workspace.role);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<InvitationRole>(roles[0] ?? "member");
  const [editorOpen, setEditorOpen] = useState(false);
  const [issued, setIssued] = useState<CreateWorkspaceInvitationResponse>();
  const [revoking, setRevoking] = useState<WorkspaceInvitation>();
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  const query = useQuery({
    queryKey: invitationsQueryKey(workspace.id),
    queryFn: () => listWorkspaceInvitations(workspace.id),
    enabled: canListInvitations(workspace.role),
  });

  const create = useMutation({
    mutationFn: () => createWorkspaceInvitation(workspace.id, { email: email.trim(), role }),
    onSuccess: async (response) => {
      setIssued(response);
      setEditorOpen(false);
      setEmail("");
      await queryClient.invalidateQueries({ queryKey: invitationsQueryKey(workspace.id) });
    },
  });
  const revoke = useMutation({
    mutationFn: (invitationId: string) => revokeWorkspaceInvitation(workspace.id, invitationId),
    onSuccess: async () => {
      setToasts([{
        id: `invitation-revoke-${revoking?.id ?? "complete"}`,
        title: "Invitation revoked",
        tone: "positive",
      }]);
      setRevoking(undefined);
      await queryClient.invalidateQueries({ queryKey: invitationsQueryKey(workspace.id) });
    },
  });

  if (!canListInvitations(workspace.role)) return null;

  function submit(event: FormEvent) {
    event.preventDefault();
    setIssued(undefined);
    create.mutate();
  }

  function openEditor() {
    create.reset();
    setEmail("");
    setRole(roles[0] ?? "member");
    setEditorOpen(true);
  }

  function confirmRevoke(invitation: WorkspaceInvitation) {
    revoke.reset();
    setRevoking(invitation);
  }

  return (
    <section className="people-panel invitations-panel" aria-labelledby="invitations-heading">
      <div className="people-panel-heading">
        <div>
          <p className="eyebrow">Pending access</p>
          <h2 id="invitations-heading">Invitations</h2>
        </div>
        <button onClick={openEditor} type="button">Invite person</button>
      </div>
      {query.isPending ? <LoadingState label="Loading workspace invitations" rows={3} /> : null}
      {query.isError ? (
        <InlineNotice
          action={<button className="secondary-button" onClick={() => void query.refetch()} type="button">Try again</button>}
          title="Invitations could not be loaded"
          tone="danger"
        >
          <p>{query.error.message}</p>
        </InlineNotice>
      ) : null}
      {!query.isPending && !query.isError && query.data?.length === 0 ? (
        <EmptyState compact description="Create an invitation when someone needs access." icon="people" title="No pending invitations" />
      ) : null}
      <div className="invitation-list">
        {query.data?.map((invitation) => (
          <article className="invitation-row" key={invitation.id}>
            <div>
              <strong>{invitation.email}</strong>
              <small>Invited by {invitation.inviter_display_name} · expires {new Date(invitation.expires_at).toLocaleDateString()}</small>
            </div>
            <StatusBadge>{invitation.role}</StatusBadge>
            <button className="text-button danger" onClick={() => confirmRevoke(invitation)} type="button">Revoke</button>
          </article>
        ))}
      </div>
      <ModalDialog
        description="The acceptance code is shown once after creation and must be shared out of band."
        footer={(
          <>
            <button className="secondary-button" onClick={() => setEditorOpen(false)} type="button">Cancel</button>
            <button disabled={create.isPending} form="invitation-editor" type="submit">
              {create.isPending ? "Creating…" : "Create invitation"}
            </button>
          </>
        )}
        onClose={() => setEditorOpen(false)}
        open={editorOpen}
        placement="drawer"
        title="Invite someone"
      >
        <form className="resource-form resource-editor-form" id="invitation-editor" onSubmit={submit}>
          <label>
            Email
            <input required type="email" maxLength={254} value={email} onChange={(event) => setEmail(event.target.value)} />
          </label>
          <label>
            Role
            <select value={role} onChange={(event) => setRole(event.target.value as InvitationRole)}>
              {roles.map((candidate) => <option key={candidate} value={candidate}>{candidate}</option>)}
            </select>
          </label>
          <MutationError mutation={create} />
        </form>
      </ModalDialog>
      <IssuedInvitation issued={issued} onClose={() => setIssued(undefined)} />
      <ModalDialog
        description={`${revoking?.email ?? "This person"} will no longer be able to use the pending acceptance code.`}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setRevoking(undefined)} type="button">Cancel</button>
            <button
              className="danger-button"
              disabled={revoke.isPending}
              onClick={() => revoking && revoke.mutate(revoking.id)}
              type="button"
            >
              {revoke.isPending ? "Revoking…" : "Revoke invitation"}
            </button>
          </>
        )}
        onClose={() => setRevoking(undefined)}
        open={Boolean(revoking)}
        title="Revoke invitation?"
      >
        <MutationError mutation={revoke} />
      </ModalDialog>
      <ToastRegion messages={toasts} onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))} />
    </section>
  );
}

function IssuedInvitation({
  issued,
  onClose,
}: {
  issued: CreateWorkspaceInvitationResponse | undefined;
  onClose: () => void;
}) {
  return (
    <ModalDialog
      description="This credential cannot be retrieved after you close this message."
      dismissible={false}
      footer={<button onClick={onClose} type="button">I saved the code</button>}
      onClose={onClose}
      open={Boolean(issued)}
      title="Invitation created"
    >
      {issued ? (
        <div className="issued-invitation">
          <p>
            Share this one-time code with <strong>{issued.invitation.email}</strong> using a secure channel.
          </p>
          <code>{issued.acceptance_token}</code>
        </div>
      ) : null}
    </ModalDialog>
  );
}
