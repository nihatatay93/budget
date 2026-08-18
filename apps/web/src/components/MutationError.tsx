import { APIError } from "../api/client";

/** Renders a failed mutation, preferring the server's message when there is one. */
export function MutationError({ mutation }: { mutation: { error: Error | null } }) {
  if (!mutation.error) return null;
  const message = mutation.error instanceof APIError ? mutation.error.message : "The change could not be saved.";
  return <p className="form-error">{message}</p>;
}
