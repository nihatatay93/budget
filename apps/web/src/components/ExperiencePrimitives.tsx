import type { ReactNode } from "react";

export type AppIconName =
  | "accounts"
  | "budget"
  | "categories"
  | "chart"
  | "home"
  | "layers"
  | "more"
  | "people"
  | "refresh"
  | "shield"
  | "transactions";

const iconPaths: Record<AppIconName, ReactNode> = {
  accounts: (
    <>
      <path d="M3 21h18" />
      <path d="M5 21V9h14v12" />
      <path d="m3 9 9-6 9 6" />
      <path d="M9 21v-6h6v6" />
    </>
  ),
  budget: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 3v9h9" />
    </>
  ),
  categories: (
    <>
      <path d="M20 13 11 22l-9-9V4h9l9 9Z" />
      <circle cx="7" cy="9" r="1.5" />
    </>
  ),
  chart: (
    <>
      <path d="M4 19V9" />
      <path d="M10 19V5" />
      <path d="M16 19v-7" />
      <path d="M22 19V2" />
    </>
  ),
  home: (
    <>
      <path d="m3 11 9-8 9 8" />
      <path d="M5 10v11h14V10" />
      <path d="M9 21v-7h6v7" />
    </>
  ),
  layers: (
    <>
      <path d="m12 2 9 5-9 5-9-5 9-5Z" />
      <path d="m3 12 9 5 9-5" />
      <path d="m3 17 9 5 9-5" />
    </>
  ),
  more: (
    <>
      <circle cx="5" cy="12" r="1" />
      <circle cx="12" cy="12" r="1" />
      <circle cx="19" cy="12" r="1" />
    </>
  ),
  people: (
    <>
      <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </>
  ),
  refresh: (
    <>
      <path d="M20 7h-5V2" />
      <path d="M20 7a9 9 0 1 0 1 8" />
    </>
  ),
  shield: (
    <>
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z" />
      <path d="m9 12 2 2 4-4" />
    </>
  ),
  transactions: (
    <>
      <path d="M4 3h16v18H4z" />
      <path d="M8 8h8" />
      <path d="M8 12h8" />
      <path d="M8 16h5" />
    </>
  ),
};

export function AppIcon({ name, size = 20 }: { name: AppIconName; size?: number }) {
  return (
    <svg
      aria-hidden="true"
      className="app-icon"
      fill="none"
      height={size}
      viewBox="0 0 24 24"
      width={size}
    >
      {iconPaths[name]}
    </svg>
  );
}

export function BrandMark({ withName = false }: { withName?: boolean }) {
  return (
    <span className="brand-lockup">
      <span aria-hidden="true" className="brand-mark">
        <svg fill="none" viewBox="0 0 40 40">
          <path d="M13 10h9a5 5 0 0 1 0 10h-9V10Z" />
          <path d="M13 20h10a5 5 0 0 1 0 10H13V20Z" />
          <path d="M13 8v24" />
        </svg>
      </span>
      {withName ? <span className="brand-name">Budget</span> : null}
    </span>
  );
}

export function AppStatus({
  action,
  description,
  eyebrow,
  title,
  tone = "loading",
}: {
  action?: ReactNode;
  description: string;
  eyebrow: string;
  title: string;
  tone?: "empty" | "error" | "loading";
}) {
  return (
    <main className="app-shell status-shell">
      <section
        aria-live={tone === "loading" ? "polite" : undefined}
        className={`status-panel status-panel-${tone}`}
        role={tone === "error" ? "alert" : "status"}
      >
        <BrandMark withName />
        <span aria-hidden="true" className="status-symbol">
          <AppIcon name={tone === "error" ? "refresh" : "layers"} size={24} />
        </span>
        <div>
          <p className="eyebrow">{eyebrow}</p>
          <h1>{title}</h1>
          <p>{description}</p>
        </div>
        {action ? <div className="status-action">{action}</div> : null}
      </section>
    </main>
  );
}
