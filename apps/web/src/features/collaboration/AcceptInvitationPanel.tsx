import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  type WorkspaceMembershipAcceptance,
  acceptWorkspaceInvitation,
  sessionQueryKey,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";

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
    <section className="setup-panel" aria-labelledby="accept-invitation-heading">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Joining a shared workspace</p>
          <h2 id="accept-invitation-heading">Accept an invitation</h2>
        </div>
      </div>
      {accepted ? (
        <p className="resource-state">
          You joined <strong>{accepted.workspace.name}</strong> as {accepted.member.role}.
        </p>
      ) : null}
      <form className="resource-form" onSubmit={submit}>
        <label>
          Invitation code
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
            Join workspace
          </button>
        </div>
      </form>
    </section>
  );
}
