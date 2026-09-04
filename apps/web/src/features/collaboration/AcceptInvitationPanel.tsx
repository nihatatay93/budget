import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  type WorkspaceMembershipAcceptance,
  acceptWorkspaceInvitation,
  sessionQueryKey,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";
import { InlineNotice } from "../../components/Presentation";
import { roleLabel, t } from "../../lib/i18n";

const TOKEN_LENGTH = 43;

/**
 * Accepts an invitation code the user received out of band. The code identifies the
 * invitation on its own, so it is submitted in the request body rather than a URL, keeping it
 * out of browser history and server logs.
 */
export function AcceptInvitationPanel() {
  const queryClient = useQueryClient();
  const [token, setToken] = useState("");
  const [accepted, setAccepted] = useState<WorkspaceMembershipAcceptance>();

  const accept = useMutation({
    mutationFn: () => acceptWorkspaceInvitation(token.trim()),
    onSuccess: async (result) => {
      setAccepted(result);
      setToken("");
      // Joining adds a workspace to the session, which drives the workspace switcher.
      await queryClient.invalidateQueries({ queryKey: sessionQueryKey });
    },
  });

  function submit(event: FormEvent) {
    event.preventDefault();
    setAccepted(undefined);
    accept.mutate();
  }

  return (
    <section className="people-panel join-workspace-panel" aria-labelledby="accept-invitation-heading">
      <div className="people-panel-heading">
        <div>
          <p className="eyebrow">{t("Joining a shared workspace")}</p>
          <h2 id="accept-invitation-heading">{t("Accept an invitation")}</h2>
          <p>{t("Paste the one-time code you received. It is sent only in the request body.")}</p>
        </div>
      </div>
      {accepted ? (
        <InlineNotice title={t("Workspace joined")} tone="positive">
          <p>{t("You joined {workspace} as {role}.", { workspace: accepted.workspace.name, role: roleLabel(accepted.member.role) })}</p>
        </InlineNotice>
      ) : null}
      <form className="accept-invitation-form" onSubmit={submit}>
        <label>
          {t("Invitation code")}
          <input
            required
            value={token}
            minLength={TOKEN_LENGTH}
            maxLength={TOKEN_LENGTH}
            autoComplete="off"
            spellCheck={false}
            onChange={(event) => setToken(event.target.value)}
          />
        </label>
        <MutationError mutation={accept} />
        <div className="form-actions">
          <button disabled={accept.isPending || token.trim().length !== TOKEN_LENGTH} type="submit">
            {t("Join workspace")}
          </button>
        </div>
      </form>
    </section>
  );
}
