"use client";
import { useEffect, useState, type ReactNode } from "react";
import { CopyButton } from "./copy";
import { Icon, type IconName } from "./icon";
import { Notice } from "./notice";
import { cx } from "./util";

/** True only after `ms` have passed while `active` stays true (S-LOAD shows skeletons after 200 ms to avoid flashes). */
export function useDelayed(active: boolean, ms = 200): boolean {
  const [shown, setShown] = useState(false);
  useEffect(() => {
    if (!active) {
      setShown(false);
      return;
    }
    const t = setTimeout(() => setShown(true), ms);
    return () => clearTimeout(t);
  }, [active, ms]);
  return active && shown;
}

/** S-LOAD: shape-matched rows, `aria-busy`, 1 s pulse (static under reduced motion). */
export function SkeletonRows({ rows = 5, label = "Loading" }: { rows?: number; label?: string }) {
  const show = useDelayed(true, 200);
  return (
    <div aria-busy="true" data-testid="skeleton">
      {show ? Array.from({ length: rows }, (_, i) => <div key={i} className="skel skel-row" />) : <div style={{ minHeight: `${rows * 3.25}rem` }} />}
      <p className="sr-only" role="status">{label}</p>
    </div>
  );
}

export function SkeletonBlock({ height = "6rem" }: { height?: string }) {
  return <div className="skel" style={{ height }} aria-hidden="true" />;
}

/** S-EMPTY-FIRST / S-EMPTY-FILTER: title, one sentence, role-aware primary action, docs link. Never a cartoon. */
export function EmptyState({ icon = "g-queued", title, children, actions }: { icon?: IconName; title: string; children?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="empty" data-testid="empty-state">
      <Icon name={icon} size="md" />
      <h3>{title}</h3>
      {children ? <p>{children}</p> : null}
      {actions ? <div className="actions">{actions}</div> : null}
    </div>
  );
}

export interface ErrorInfo {
  /** Stable machine code (`not_implemented`, `unavailable`, `internal`, ...). Copy comes from the code, never the raw message. */
  code: string;
  requestId?: string;
  status?: number;
}

const ERROR_COPY: Record<string, { title: string; body: string }> = {
  not_implemented: { title: "Not connected yet", body: "This build has no backend session, so the console cannot load data. The layout is real; the data arrives with the BFF in a later milestone." },
  unavailable: { title: "The service is temporarily unavailable", body: "The control plane is not ready. Your last data stays on screen, marked stale. Try again shortly." },
  upstream_unavailable: { title: "The control plane is unreachable", body: "The console could not reach the engine. Nothing was changed." },
  upstream_auth: { title: "The console could not reach its own API", body: "This is an infrastructure fault, not a problem with your account, so you are not asked to sign in again." },
  timeout: { title: "The request timed out", body: "No response arrived in 10 seconds. If you were changing something, check its state before trying again." },
  internal: { title: "Something went wrong on the server", body: "The server returned an internal error. Your filters are kept." },
  csrf_failed: { title: "The request was rejected", body: "Reload the page and try again. Nothing was changed." },
  forbidden: { title: "You do not have access to this", body: "Your role cannot perform this action." },
  rate_limited: { title: "Too many requests", body: "Wait a moment, then try again." },
  network: { title: "The request did not reach the server", body: "Check your connection and try again." },
};

/** S-ERR-*: plain-language cause, request id with copy, recovery action. role=alert only for blocking errors. */
export function ErrorState({ error, onRetry, blocking = false, extra, heading }: { error: ErrorInfo; onRetry?: () => void; blocking?: boolean; extra?: ReactNode; heading?: string }) {
  const c = ERROR_COPY[error.code] ?? ERROR_COPY.internal!;
  return (
    <div className="empty" data-testid="error-state" data-error-code={error.code} role={blocking ? "alert" : undefined}>
      <Icon name="i-alert" size="md" className="text-danger" />
      <h3>{heading ?? c.title}</h3>
      <p>{c.body}</p>
      <p className="mono meta">
        code <span>{error.code}</span>
        {error.status ? <> · HTTP {error.status}</> : null}
        {error.requestId ? (
          <>
            {" "}· request id <span data-testid="request-id">{error.requestId}</span> <CopyButton text={error.requestId} ariaLabel="Copy request id" />
          </>
        ) : null}
      </p>
      <div className="actions">
        {onRetry ? (
          <button type="button" className="btn btn-primary" onClick={onRetry}>Retry</button>
        ) : null}
        {extra}
      </div>
    </div>
  );
}

/** S-OFFLINE: banner; last data stays labelled "as of"; mutations disabled with reason; recovers on `online`. */
export function OfflineNotice({ asOf }: { asOf?: ReactNode }) {
  return (
    <Notice variant="warning" icon="i-signal-off" lead="You are offline.">
      {asOf ? <>Showing the last data you loaded, as of {asOf}. </> : null}Changes are disabled until you reconnect.
    </Notice>
  );
}

/** S-STALE: snapshot age beyond the threshold or the live channel is not live. */
export function StaleNotice({ asOf, onRefresh }: { asOf: ReactNode; onRefresh?: () => void }) {
  return (
    <Notice
      variant="warning"
      icon="g-stale"
      lead="This data is stale."
      actions={onRefresh ? <button type="button" className="btn btn-sm" onClick={onRefresh}>Refresh</button> : undefined}
    >
      The last snapshot is from {asOf} and the live channel is not connected.
    </Notice>
  );
}

/** Tracks navigator.onLine (S-OFFLINE). Server snapshot is online so the first render matches. */
export function useOnline(): boolean {
  const [online, setOnline] = useState(true);
  useEffect(() => {
    setOnline(navigator.onLine);
    const on = () => setOnline(true);
    const off = () => setOnline(false);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
    };
  }, []);
  return online;
}

export function Region({ title, headingLevel = 2, children, className, id }: { title: string; headingLevel?: 2 | 3; children: ReactNode; className?: string; id?: string }) {
  const H = `h${headingLevel}` as "h2" | "h3";
  return (
    <section className={cx("card sect", className)} aria-labelledby={`${id ?? title}-h`}>
      <div className="card-h"><H id={`${id ?? title}-h`}>{title}</H></div>
      {children}
    </section>
  );
}
