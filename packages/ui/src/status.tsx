import type { ReactNode } from "react";
import { Icon, type IconName } from "./icon";
import { cx } from "./util";

/** Seven job states (1.0) plus worker states. `blocked` and `skipped` are reserved for 1.1 and never rendered. */
export const JOB_STATES = ["scheduled", "queued", "running", "retry_wait", "succeeded", "failed", "canceled"] as const;
export type JobState = (typeof JOB_STATES)[number];
export const WORKER_STATES = ["online", "stale", "draining", "revoked", "deregistered"] as const;
export type WorkerState = (typeof WORKER_STATES)[number];
export type StatusKey = JobState | WorkerState;

const META: Record<StatusKey, { label: string; glyph: IconName; description: string }> = {
  scheduled: { label: "Scheduled", glyph: "g-scheduled", description: "Waiting for its run_after time" },
  queued: { label: "Queued", glyph: "g-queued", description: "Due and waiting for a worker claim" },
  running: { label: "Running", glyph: "g-running", description: "An attempt holds a lease" },
  retry_wait: { label: "Retry wait", glyph: "g-retry", description: "Waiting out a backoff before the next attempt" },
  succeeded: { label: "Succeeded", glyph: "g-succeeded", description: "Finished; the result is recorded" },
  failed: { label: "Failed", glyph: "g-failed", description: "Finished without success (dead-letter is this filtered view)" },
  canceled: { label: "Canceled", glyph: "g-canceled", description: "Canceled before success was committed" },
  online: { label: "Online", glyph: "g-online", description: "Heartbeat within the liveness timeout" },
  stale: { label: "Stale", glyph: "g-stale", description: "No heartbeat within the liveness timeout" },
  draining: { label: "Draining", glyph: "g-draining", description: "Finishing active attempts, taking no new work" },
  revoked: { label: "Credential revoked", glyph: "g-revoked", description: "Its credential was revoked" },
  deregistered: { label: "Deregistered", glyph: "g-off", description: "No longer registered" },
};

export function statusLabel(s: StatusKey): string {
  return META[s].label;
}

/** Text plus a glyph with a distinct silhouette; color only reinforces. The glyph is aria-hidden. */
export function StatusBadge({ state }: { state: StatusKey }) {
  const m = META[state];
  return (
    <span className="status" data-s={state}>
      <Icon name={m.glyph} />
      {m.label}
    </span>
  );
}

/** Canonical legend: every glyph with its words. Reused on /docs/concepts and the demo. */
export function StatusLegend({ states = JOB_STATES, label = "Status legend" }: { states?: readonly StatusKey[]; label?: string }) {
  return (
    <ul className="legend" aria-label={label}>
      {states.map((s) => (
        <li key={s}>
          <StatusBadge state={s} /> <span>{META[s].description}</span>
        </li>
      ))}
    </ul>
  );
}

/** Unschedulable is a reason beside the Queued badge, never a status. */
export function ReasonChip({ children }: { children: ReactNode }) {
  return (
    <span className="chip chip-reason">
      <Icon name="i-alert" />
      {children}
    </span>
  );
}

/** S-MASKED: explicit placeholder, never blank, never null-looking. Raw values must not be in the DOM for viewers. */
export function MaskedField({ what }: { what?: string }) {
  return (
    <span className="masked" data-masked="true">
      <Icon name="i-lock" />
      Hidden for viewer role{what ? <span className="sr-only"> ({what})</span> : null}
    </span>
  );
}

export type LiveState = "live" | "reconnecting" | "polling" | "stale" | "offline" | "paused" | "demo";
const LIVE: Record<LiveState, { label: string; glyph: IconName }> = {
  live: { label: "Live", glyph: "g-online" },
  reconnecting: { label: "Reconnecting", glyph: "i-refresh" },
  polling: { label: "Polling", glyph: "i-refresh" },
  stale: { label: "Stale", glyph: "g-stale" },
  offline: { label: "Offline", glyph: "i-signal-off" },
  paused: { label: "Paused", glyph: "i-pausebar" },
  demo: { label: "Demo data", glyph: "i-dot" },
};

/** One component, six console states plus the demo pill. Announces politely only when the state changes (role=status). */
export function LiveIndicator({ state, detail, className }: { state: LiveState; detail?: string; className?: string }) {
  const l = LIVE[state];
  return (
    <span className={cx("live", className)} role="status" data-live={state} aria-label={`Connection state: ${l.label}${detail ? `, ${detail}` : ""}`}>
      <Icon name={l.glyph} />
      <span className="live-t">{l.label}</span>
      {detail ? <span className="meta">{detail}</span> : null}
    </span>
  );
}
