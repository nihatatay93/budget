export function workspaceMonth(timezone: string, now = new Date()): string {
  try {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: timezone,
      year: "numeric",
      month: "2-digit",
    }).formatToParts(now);
    const year = parts.find((part) => part.type === "year")?.value;
    const month = parts.find((part) => part.type === "month")?.value;
    if (year && month) return `${year}-${month}`;
  } catch {
    // Invalid workspace timezone data is reported by the API; retain a usable local fallback.
  }
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

export function monthLabel(month: string): string {
  const [year, value] = month.split("-").map(Number);
  if (!year || !value) return month;
  return new Intl.DateTimeFormat(undefined, {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  }).format(new Date(Date.UTC(year, value - 1, 1)));
}
