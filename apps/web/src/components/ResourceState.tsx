/** Renders the loading, error, and empty states shared by the resource panels. */
export function ResourceState({
  query,
  empty,
}: {
  query: { isPending: boolean; error: Error | null; data?: unknown[] };
  empty: string;
}) {
  if (query.isPending) return <p className="resource-state">Loading…</p>;
  if (query.error) return <p className="form-error">{query.error.message}</p>;
  if (query.data?.length === 0) return <p className="resource-state">{empty}</p>;
  return null;
}
