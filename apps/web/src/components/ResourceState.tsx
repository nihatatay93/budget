import { EmptyState, InlineNotice, LoadingState } from "./Presentation";
import { t } from "../lib/i18n";

/** Renders the loading, error, and empty states shared by the resource panels. */
export function ResourceState({
  query,
  empty,
}: {
  query: { isPending: boolean; error: Error | null; data?: unknown[] };
  empty: string;
}) {
  if (query.isPending) return <LoadingState label={t("Loading resources")} rows={2} />;
  if (query.error) return <InlineNotice tone="danger">{query.error.message}</InlineNotice>;
  if (query.data?.length === 0) return <EmptyState compact title={empty} />;
  return null;
}
