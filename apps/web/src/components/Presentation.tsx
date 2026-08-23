import { type KeyboardEvent, type ReactNode, useEffect, useId, useRef } from "react";

import { type Currency, formatMoney } from "../lib/currency";
import { type AppIconName, AppIcon } from "./ExperiencePrimitives";

export function PageHeader({
  description,
  eyebrow,
  meta,
  title,
}: {
  description: string;
  eyebrow: string;
  meta?: ReactNode;
  title: string;
}) {
  return (
    <header className="workspace-page-header">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {meta}
    </header>
  );
}

export function SurfaceCard({
  children,
  className = "",
  labelledBy,
}: {
  children: ReactNode;
  className?: string;
  labelledBy?: string;
}) {
  return (
    <section aria-labelledby={labelledBy} className={`surface-card ${className}`.trim()}>
      {children}
    </section>
  );
}

export function MoneyAmount({
  amount,
  currency,
  emphasis = "normal",
  signed = false,
}: {
  amount: number;
  currency: Currency;
  emphasis?: "hero" | "normal" | "quiet";
  signed?: boolean;
}) {
  const formatted = formatMoney(amount, currency);
  const value = signed && amount > 0 && formatted !== "Amount unavailable" ? `+${formatted}` : formatted;
  return <span className={`money-amount money-amount-${emphasis}`}>{value}</span>;
}

export function StatusBadge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "danger" | "neutral" | "positive" | "warning";
}) {
  return <span className={`status-badge status-badge-${tone}`}>{children}</span>;
}

export function ProgressMeter({
  label,
  max = 100,
  tone = "positive",
  value,
}: {
  label: string;
  max?: number;
  tone?: "danger" | "positive" | "warning";
  value: number;
}) {
  return (
    <progress
      aria-label={label}
      className={`progress-meter progress-meter-${tone}`}
      max={max}
      value={Math.max(0, Math.min(max, value))}
    />
  );
}

export function InlineNotice({
  action,
  children,
  title,
  tone = "neutral",
}: {
  action?: ReactNode;
  children: ReactNode;
  title?: string;
  tone?: "danger" | "neutral" | "positive" | "warning";
}) {
  return (
    <div className={`inline-notice inline-notice-${tone}`} role={tone === "danger" ? "alert" : undefined}>
      <div>
        {title ? <strong>{title}</strong> : null}
        <div>{children}</div>
      </div>
      {action ? <div className="inline-notice-action">{action}</div> : null}
    </div>
  );
}

export function EmptyState({
  action,
  compact = false,
  description,
  icon = "layers",
  title,
}: {
  action?: ReactNode;
  compact?: boolean;
  description?: string;
  icon?: AppIconName;
  title: string;
}) {
  return (
    <div className={`empty-state${compact ? " empty-state-compact" : ""}`}>
      <span aria-hidden="true" className="empty-state-icon"><AppIcon name={icon} /></span>
      <div>
        <strong>{title}</strong>
        {description ? <p>{description}</p> : null}
      </div>
      {action ? <div>{action}</div> : null}
    </div>
  );
}

export function LoadingState({ label = "Loading…", rows = 3 }: { label?: string; rows?: number }) {
  return (
    <div aria-label={label} className="loading-state" role="status">
      <span className="visually-hidden">{label}</span>
      {Array.from({ length: rows }, (_, index) => (
        <span className="skeleton-line" key={index} style={{ width: `${92 - index * 13}%` }} />
      ))}
    </div>
  );
}

export function ModalDialog({
  children,
  description,
  dismissible = true,
  footer,
  onClose,
  open,
  placement = "dialog",
  title,
}: {
  children: ReactNode;
  description?: string;
  dismissible?: boolean;
  footer?: ReactNode;
  onClose: () => void;
  open: boolean;
  placement?: "dialog" | "drawer";
  title: string;
}) {
  const titleId = useId();
  const descriptionId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialogRef.current?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus();
    };
  }, [open]);

  if (!open) return null;

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape" && dismissible) {
      event.preventDefault();
      onClose();
      return;
    }
    if (event.key !== "Tab") return;
    const focusable = Array.from(
      dialogRef.current?.querySelectorAll<HTMLElement>(
        "button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])",
      ) ?? [],
    );
    if (focusable.length === 0) {
      event.preventDefault();
      dialogRef.current?.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable.at(-1)!;
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  return (
    <div
      className="modal-backdrop"
      onMouseDown={(event) => {
        if (dismissible && event.target === event.currentTarget) onClose();
      }}
    >
      <div
        aria-describedby={description ? descriptionId : undefined}
        aria-labelledby={titleId}
        aria-modal="true"
        className={`modal-surface modal-surface-${placement}`}
        onKeyDown={handleKeyDown}
        ref={dialogRef}
        role="dialog"
        tabIndex={-1}
      >
        <header className="modal-header">
          <div>
            <h2 id={titleId}>{title}</h2>
            {description ? <p id={descriptionId}>{description}</p> : null}
          </div>
          {dismissible ? <button aria-label="Close" className="modal-close" onClick={onClose} type="button">×</button> : null}
        </header>
        <div className="modal-body">{children}</div>
        {footer ? <footer className="modal-footer">{footer}</footer> : null}
      </div>
    </div>
  );
}

export type ToastMessage = {
  description?: string;
  id: string;
  title: string;
  tone?: "danger" | "neutral" | "positive" | "warning";
};

export function ToastRegion({
  messages,
  onDismiss,
}: {
  messages: ToastMessage[];
  onDismiss: (id: string) => void;
}) {
  return (
    <div aria-label="Notifications" aria-live="polite" className="toast-region" role="region">
      {messages.map((message) => (
        <article className={`toast toast-${message.tone ?? "neutral"}`} key={message.id}>
          <div>
            <strong>{message.title}</strong>
            {message.description ? <p>{message.description}</p> : null}
          </div>
          <button aria-label={`Dismiss ${message.title}`} onClick={() => onDismiss(message.id)} type="button">×</button>
        </article>
      ))}
    </div>
  );
}
