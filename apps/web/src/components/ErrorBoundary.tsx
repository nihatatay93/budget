import { Component, type ErrorInfo, type ReactNode } from "react";

import { AppStatus } from "./ExperiencePrimitives";
import { t } from "../lib/i18n";

type Props = { children: ReactNode };
type State = { error: Error | null };

/**
 * Catches a render-time exception and offers a way out of it.
 *
 * Without this, React unmounts the whole tree on any thrown render and the page goes blank,
 * which for a self-hosted finance application means a person's only recourse is to guess that
 * a reload might help. A boundary turns that into a legible failure with a retry.
 *
 * It deliberately does not wrap individual features. A thrown render is a defect, not a
 * recoverable state, so the honest response is to stop and say so rather than to leave part
 * of an interface running on assumptions that have already failed somewhere.
 *
 * Data-loading failures are not this: those are ordinary outcomes each feature reports
 * itself, and they never reach here.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // The browser console is the only sink available to a client with no telemetry, and the
    // component stack is what makes the report actionable in a self-hosted deployment.
    console.error("Unhandled interface error", error, info.componentStack);
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <AppStatus
        action={(
          <button type="button" onClick={() => window.location.reload()}>
            {t("Reload Budget")}
          </button>
        )}
        description={t("Something in the interface stopped unexpectedly. Your financial data is unaffected — reloading usually restores the page.")}
        eyebrow={t("Unexpected error")}
        title={t("Budget could not continue")}
        tone="error"
      />
    );
  }
}
