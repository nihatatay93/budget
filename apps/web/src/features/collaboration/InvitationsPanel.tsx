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
import { roleLabel, t } from "../../lib/i18n";

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
        title: t("Invitation revoked"),
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
          <p className="eyebrow">{t("Pending access")}</p>
          <h2 id="invitations-heading">{t("Invitations")}</h2>
        </div>
        <button onClick={openEditor} type="button">{t("Invite person")}</button>
      </div>
      {query.isPending ? <LoadingState label={t("Loading workspace invitations")} rows={3} /> : null}
      {query.isError ? (
        <InlineNotice
          action={<button className="secondary-button" onClick={() => void query.refetch()} type="button">{t("Try again")}</button>}
          title={t("Invitations could not be loaded")}
          tone="danger"
        >
          <p>{query.error.message}</p>
        </InlineNotice>
      ) : null}
      {!query.isPending && !query.isError && query.data?.length === 0 ? (
        <EmptyState compact description={t("Create an invitation when someone needs access.")} icon="people" title={t("No pending invitations")} />
      ) : null}
      <div className="invitation-list">
        {query.data?.map((invitation) => (
          <article className="invitation-row" key={invitation.id}>
            <div>
              <strong>{invitation.email}</strong>
              <small>{t("Invited by {name} · expires {date}", { name: invitation.inviter_display_name, date: new Date(invitation.expires_at).toLocaleDateString() })}</small>
            </div>
            <StatusBadge>{roleLabel(invitation.role)}</StatusBadge>
            <button className="text-button danger" onClick={() => confirmRevoke(invitation)} type="button">{t("Revoke")}</button>
          </article>
        ))}
      </div>
      <ModalDialog
        description={t("The acceptance code is shown once after creation and must be shared out of band.")}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setEditorOpen(false)} type="button">{t("Cancel")}</button>
            <button disabled={create.isPending} form="invitation-editor" type="submit">
              {create.isPending ? t("Creating…") : t("Create invitation")}
            </button>
          </>
        )}
        onClose={() => setEditorOpen(false)}
        open={editorOpen}
        placement="drawer"
        title={t("Invite someone")}
      >
        <form className="resource-form resource-editor-form" id="invitation-editor" onSubmit={submit}>
          <label>
            {t("Email")}
            <input required type="email" maxLength={254} value={email} onChange={(event) => setEmail(event.target.value)} />
          </label>
          <label>
            {t("Role")}
            <select value={role} onChange={(event) => setRole(event.target.value as InvitationRole)}>
              {roles.map((candidate) => <option key={candidate} value={candidate}>{roleLabel(candidate)}</option>)}
            </select>
          </label>
          <MutationError mutation={create} />
        </form>
      </ModalDialog>
      <IssuedInvitation issued={issued} onClose={() => setIssued(undefined)} />
      <ModalDialog
        description={t("{person} will no longer be able to use the pending acceptance code.", { person: revoking?.email ?? t("This person") })}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setRevoking(undefined)} type="button">{t("Cancel")}</button>
            <button
              className="danger-button"
              disabled={revoke.isPending}
              onClick={() => revoking && revoke.mutate(revoking.id)}
              type="button"
            >
              {revoke.isPending ? t("Revoking…") : t("Revoke invitation")}
            </button>
          </>
        )}
        onClose={() => setRevoking(undefined)}
        open={Boolean(revoking)}
        title={t("Revoke invitation?")}
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
      description={t("This credential cannot be retrieved after you close this message.")}
      dismissible={false}
      footer={<button onClick={onClose} type="button">{t("I saved the code")}</button>}
      onClose={onClose}
      open={Boolean(issued)}
      title={t("Invitation created")}
    >
      {issued ? (
        <div className="issued-invitation">
          <p>
            {t("Share this one-time code with {email} using a secure channel.", { email: issued.invitation.email })}
          </p>
          <code>{issued.acceptance_token}</code>
        </div>
      ) : null}
    </ModalDialog>
  );
}
