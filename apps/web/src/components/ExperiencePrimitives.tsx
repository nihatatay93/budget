import type { ReactNode } from "react";

export type AppIconName =
  | "accounts"
  | "analysis"
  | "budget"
  | "categories"
  | "chart"
  | "car"
  | "building"
  | "ellipsis"
  | "eye"
  | "eye-off"
  | "filter"
  | "gamepad"
  | "gift"
  | "graduation-cap"
  | "heart"
  | "home"
  | "laptop"
  | "layers"
  | "more"
  | "people"
  | "refresh"
  | "repeat"
  | "refund"
  | "receipt"
  | "plane"
  | "person"
  | "shopping-bag"
  | "shopping-cart"
  | "shield"
  | "sparkles"
  | "trending-up"
  | "transactions"
  | "undo"
  | "utensils"
  | "wallet"
  | "wallet-more";

const iconPaths: Record<AppIconName, ReactNode> = {
  analysis: (
    <>
      <path d="M3 3v16a2 2 0 0 0 2 2h16" />
      <path d="m7 15 3.5-4.5 3 3L20 6" />
      <circle cx="10.5" cy="10.5" r="1" />
      <circle cx="13.5" cy="13.5" r="1" />
    </>
  ),
  eye: (
    <>
      <path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7-10-7-10-7Z" />
      <circle cx="12" cy="12" r="3" />
    </>
  ),
  "eye-off": (
    <>
      <path d="M10.7 6.2A9.9 9.9 0 0 1 12 6c6.4 0 10 6 10 6a17 17 0 0 1-3 3.5" />
      <path d="M6.2 7.6A17 17 0 0 0 2 12s3.6 7 10 7a9.6 9.6 0 0 0 4.2-.9" />
      <path d="m3 3 18 18" />
    </>
  ),
  filter: (
    <>
      <path d="M4 6h16" />
      <path d="M7 12h10" />
      <path d="M10 18h4" />
    </>
  ),
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
  car: <><path d="M5 17h14l-1.5-7h-11Z" /><path d="M7 10 9 6h6l2 4" /><circle cx="7" cy="17" r="1.5" /><circle cx="17" cy="17" r="1.5" /></>,
  building: <><path d="M4 21V4h12v17" /><path d="M16 9h4v12" /><path d="M8 8h2M8 12h2M8 16h2M16 13h1M16 17h1" /></>,
  ellipsis: <><circle cx="5" cy="12" r="1" /><circle cx="12" cy="12" r="1" /><circle cx="19" cy="12" r="1" /></>,
  gamepad: <><path d="M6 8h12a4 4 0 0 1 3.8 5.25l-1.2 4A2 2 0 0 1 17 18l-2-2H9l-2 2a2 2 0 0 1-3.6-.75l-1.2-4A4 4 0 0 1 6 8Z" /><path d="M7 12v4M5 14h4M16 13h.01M19 15h.01" /></>,
  gift: <><path d="M4 10h16v11H4zM12 10v11M2 6h20v4H2z" /><path d="M12 6H8a2 2 0 1 1 2-3c1.4 0 2 3 2 3Zm0 0h4a2 2 0 1 0-2-3c-1.4 0-2 3-2 3Z" /></>,
  "graduation-cap": <><path d="m2 10 10-5 10 5-10 5Z" /><path d="M6 12v5c3 2 9 2 12 0v-5M22 10v6" /></>,
  heart: <path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.9-8.6a5.5 5.5 0 0 0-.1-7.8Z" />,
  home: (
    <>
      <path d="m3 11 9-8 9 8" />
      <path d="M5 10v11h14V10" />
      <path d="M9 21v-7h6v7" />
    </>
  ),
  laptop: <><rect x="3" y="5" width="18" height="12" rx="1" /><path d="M2 20h20" /></>,
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
  plane: <path d="m22 2-7 20-4-9-9-4Z M11 13l4-4" />,
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
  receipt: <><path d="M5 3h14v18l-3-2-2 2-2-2-2 2-3-2Z" /><path d="M8 8h8M8 12h8" /></>,
  repeat: <><path d="m17 2 4 4-4 4" /><path d="M3 11V9a3 3 0 0 1 3-3h15" /><path d="m7 22-4-4 4-4" /><path d="M21 13v2a3 3 0 0 1-3 3H3" /></>,
  refund: <><path d="M9 7 4 12l5 5" /><path d="M4 12h10a6 6 0 1 1 0 12h-1" /></>,
  person: <><circle cx="12" cy="7" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
  "shopping-bag": <><path d="M5 8h14l-1 13H6Z" /><path d="M9 8a3 3 0 0 1 6 0" /></>,
  "shopping-cart": <><path d="M3 4h2l2 12h11l2-8H6" /><circle cx="9" cy="20" r="1" /><circle cx="18" cy="20" r="1" /></>,
  shield: (
    <>
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z" />
      <path d="m9 12 2 2 4-4" />
    </>
  ),
  sparkles: <><path d="m12 2 1.5 5.5L19 9l-5.5 1.5L12 16l-1.5-5.5L5 9l5.5-1.5Z" /><path d="m19 16 .7 2.3L22 19l-2.3.7L19 22l-.7-2.3L16 19l2.3-.7Z" /></>,
  "trending-up": <><path d="m4 16 5-5 4 4 7-8" /><path d="M15 7h5v5" /></>,
  transactions: (
    <>
      <path d="M4 3h16v18H4z" />
      <path d="M8 8h8" />
      <path d="M8 12h8" />
      <path d="M8 16h5" />
    </>
  ),
  undo: <><path d="M9 7 4 12l5 5" /><path d="M4 12h10a6 6 0 1 1 0 12h-1" /></>,
  utensils: <><path d="M4 3v8M2 3v4h4V3M4 11v10M14 3v18M14 3c4 2 4 7 0 9" /></>,
  wallet: <><path d="M4 6h15a2 2 0 0 1 2 2v11H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h13" /><path d="M17 13h.01" /></>,
  "wallet-more": <><path d="M4 6h15a2 2 0 0 1 2 2v11H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h13" /><circle cx="15" cy="13" r="1" /><circle cx="18" cy="13" r="1" /></>,
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
