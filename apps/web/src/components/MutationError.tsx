import { APIError } from "../api/client";
import { InlineNotice } from "./Presentation";
import { t } from "../lib/i18n";

/** Renders a failed mutation, preferring the server's message when there is one. */
export function MutationError({ mutation }: { mutation: { error: Error | null } }) {
  if (!mutation.error) return null;
  const message = mutation.error instanceof APIError ? mutation.error.message : t("The change could not be saved.");
  return <InlineNotice tone="danger">{message}</InlineNotice>;
}
