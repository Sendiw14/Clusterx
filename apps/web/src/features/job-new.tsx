"use client";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState, type FormEvent } from "react";
import { Button, Card, Notice, Select, TextInput, Textarea, useOnline, useToast } from "@clusterx/ui";
import { useConsoleProvider } from "@/data/context";
import type { Handler } from "@/data/console-provider";
import type { ApiFailure, Config, Job } from "@/data/provider";
import { checkSchema } from "@/lib/json-schema";
import { Load } from "./common";
import { describeFailure, fieldIssues, newKey } from "./failure";
import { useResource } from "./use-resource";

const DRAFT = "clusterx:new-job-draft";
interface Draft { handler: string; version: string; payload: string; priority: string; attempts: string; timeout: string; runAfter: string }

// OQ-01 (ADR-0006 section 6): per-tab sessionStorage, cleared on submit success and on logout (tab close). Never localStorage, never sent anywhere.
function readDraft(): Partial<Draft> {
  try {
    return JSON.parse(sessionStorage.getItem(DRAFT) ?? "{}") as Partial<Draft>;
  } catch {
    return {};
  }
}
function writeDraft(d: Draft | null) {
  try {
    if (d) sessionStorage.setItem(DRAFT, JSON.stringify(d));
    else sessionStorage.removeItem(DRAFT);
  } catch {
    /* storage unavailable: the form still works, the draft is just not kept */
  }
}

interface Issue { field: string; message: string }

function validate(d: Draft, h: Handler | undefined, cfg: Config): { body?: Record<string, unknown>; issues: Issue[] } {
  const issues: Issue[] = [];
  const v = h?.versions.find((x) => String(x.version) === d.version);
  if (!h || !v) issues.push({ field: "/handler_name", message: "Choose a handler and version." });
  let payload: unknown;
  try {
    payload = JSON.parse(d.payload);
    if (new TextEncoder().encode(d.payload).length > cfg.limits.payload_max_bytes) issues.push({ field: "/payload", message: `The payload is larger than ${cfg.limits.payload_max_bytes / 1024} KiB.` });
    else if (v) {
      for (const e of checkSchema(v.payload_schema, payload).issues) issues.push({ field: `/payload${e.path}`, message: `${e.path || "payload"} ${e.message}` });
    }
  } catch (e) {
    issues.push({ field: "/payload", message: `The payload is not valid JSON (${(e as Error).message}).` });
  }
  const int = (s: string, lo: number, hi: number, field: string, name: string) => {
    const n = Number(s);
    if (s.trim() === "" || !Number.isInteger(n) || n < lo || n > hi) issues.push({ field, message: `${name} must be a whole number from ${lo} to ${hi}.` });
    return n;
  };
  const priority = int(d.priority, cfg.limits.priority_min, cfg.limits.priority_max, "/priority", "Priority");
  const max_attempts = int(d.attempts, cfg.limits.max_attempts_min, cfg.limits.max_attempts_max, "/max_attempts", "Attempts");
  const timeout_seconds = int(d.timeout, 1, cfg.limits.timeout_seconds_max, "/timeout_seconds", "Timeout");
  let run_after: string | null = null;
  if (d.runAfter) {
    const t = Date.parse(`${d.runAfter}:00Z`);
    if (Number.isNaN(t)) issues.push({ field: "/run_after", message: "Enter a valid date and time." });
    else run_after = new Date(t).toISOString();
  }
  if (issues.length) return { issues };
  return { issues, body: { handler_name: h!.name, handler_version: v!.version, payload, priority, max_attempts, timeout_seconds, run_after } };
}

function Form({ handlers, cfg }: { handlers: Handler[]; cfg: Config }) {
  const p = useConsoleProvider();
  const router = useRouter();
  const toast = useToast();
  const online = useOnline();
  const [d, setD] = useState<Draft>(() => {
    const s = typeof window === "undefined" ? {} : readDraft();
    const h0 = handlers[0];
    return { handler: s.handler ?? h0?.name ?? "", version: s.version ?? String(h0?.versions.at(-1)?.version ?? ""), payload: s.payload ?? "{}", priority: s.priority ?? String(cfg.limits.priority_min), attempts: s.attempts ?? String(cfg.limits.max_attempts_default), timeout: s.timeout ?? "600", runAfter: s.runAfter ?? "" };
  });
  const [key, setKey] = useState(newKey);
  const [pending, setPending] = useState(false);
  const [local, setLocal] = useState<Issue[]>([]);
  const [fail, setFail] = useState<ApiFailure | null>(null);
  const [replay, setReplay] = useState<Job | null>(null);
  const h = handlers.find((x) => x.name === d.handler);
  const ver = h?.versions.find((x) => String(x.version) === d.version);

  // Any edit is a new intent, so it gets a new idempotency key (same key + different body is a 409).
  function set(patch: Partial<Draft>) {
    const next = { ...d, ...patch };
    setD(next);
    writeDraft(next);
    setKey(newKey());
    setFail(null);
    setReplay(null);
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (pending) return;
    const v = validate(d, h, cfg);
    setLocal(v.issues);
    setFail(null);
    if (!v.body) return;
    setPending(true);
    const r = await p.createJob(v.body as never, key);
    setPending(false);
    if (r.ok) {
      if (r.replayed) return setReplay(r.data);
      writeDraft(null);
      toast.show("Job submitted");
      router.push(`/app/jobs/${r.data.id}`);
    } else setFail(r.error);
  }

  const server = fail ? fieldIssues(fail) : [];
  const errFor = (field: string) => [...local, ...server].find((i) => i.field === field)?.message;
  const summary = [...local, ...server];

  return (
    <form onSubmit={(e) => void submit(e)} noValidate style={{ display: "grid", gap: "var(--space-4)", maxWidth: "46rem" }}>
      {replay ? <Notice variant="info" lead="This job was already submitted." actions={<Link className="btn btn-sm" href={`/app/jobs/${replay.id}`}>Open job {replay.id.slice(-8)}</Link>}>The same idempotency key and request were seen before, so nothing new was created.</Notice> : null}
      {fail && !server.length ? <Notice variant="error" lead={fail.code === "idempotency_conflict" ? "Key conflict." : "The job was not submitted."} role="alert">{describeFailure(fail)}{fail.requestId ? <> Request id <span className="mono">{fail.requestId}</span>.</> : null}</Notice> : null}
      {summary.length ? (
        <Notice variant="error" lead={`${summary.length} problem${summary.length === 1 ? "" : "s"} to fix.`} role="alert">
          <ul>{summary.map((i, n) => <li key={n}><span className="mono">{i.field}</span> {i.message}</li>)}</ul>
        </Notice>
      ) : null}
      <Card>
        <div className="card-h"><h2>Handler</h2></div>
        <div className="card-pad" style={{ display: "grid", gap: "var(--space-3)" }}>
          <Select label="Handler" required value={d.handler} onChange={(e) => { const nh = handlers.find((x) => x.name === e.target.value); set({ handler: e.target.value, version: String(nh?.versions.at(-1)?.version ?? "") }); }} error={errFor("/handler_name")}>
            {handlers.map((x) => <option key={x.name} value={x.name}>{x.name}</option>)}
          </Select>
          <Select label="Version" required value={d.version} onChange={(e) => set({ version: e.target.value })}>
            {h?.versions.map((x) => <option key={x.version} value={x.version}>v{x.version}</option>)}
          </Select>
          {h?.description ? <p className="muted">{h.description}</p> : null}
          {ver ? (
            ver.live_capable_workers === 0 ? (
              <Notice variant="warning" lead="No live worker can run this version." role="none">{ver.capable_workers ? `${ver.capable_workers} registered, none live.` : "None are registered."} The job will be accepted and wait (shown as unschedulable) until one is.</Notice>
            ) : <p className="meta">{ver.live_capable_workers} live worker{ver.live_capable_workers === 1 ? "" : "s"} can run this version.</p>
          ) : null}
          <details>
            <summary>Payload schema for v{d.version}</summary>
            <pre className="jsonblock" tabIndex={0} aria-label="JSON Schema">{JSON.stringify(ver?.payload_schema ?? {}, null, 2)}</pre>
          </details>
        </div>
      </Card>
      <Card>
        <div className="card-h"><h2>Payload</h2></div>
        <div className="card-pad">
          <Textarea label="Payload (JSON)" required rows={8} spellCheck={false} className="mono" value={d.payload} onChange={(e) => set({ payload: e.target.value })} error={errFor("/payload") ?? local.find((i) => i.field.startsWith("/payload"))?.message} help={`Checked against the schema above, at most ${cfg.limits.payload_max_bytes / 1024} KiB. Do not put secrets in a payload. This draft stays in this tab only.`} />
        </div>
      </Card>
      <Card>
        <div className="card-h"><h2>Scheduling</h2></div>
        <div className="card-pad filters">
          <TextInput label="Priority" help="0 lowest, 9 highest" inputMode="numeric" value={d.priority} onChange={(e) => set({ priority: e.target.value })} error={errFor("/priority")} />
          <TextInput label="Max attempts" inputMode="numeric" value={d.attempts} onChange={(e) => set({ attempts: e.target.value })} error={errFor("/max_attempts")} />
          <TextInput label="Timeout (seconds)" inputMode="numeric" value={d.timeout} onChange={(e) => set({ timeout: e.target.value })} error={errFor("/timeout_seconds")} />
          <TextInput label="Run after (UTC)" type="datetime-local" help="Empty runs as soon as a worker is free." value={d.runAfter} onChange={(e) => set({ runAfter: e.target.value })} error={errFor("/run_after")} />
        </div>
      </Card>
      <Card>
        <div className="card-h"><h2>Idempotency</h2></div>
        <div className="card-pad">
          <TextInput label="Idempotency key" help="Sent with the request. Submitting twice with the same key and the same content creates one job. It changes when you edit the form." value={key} onChange={(e) => { setKey(e.target.value); setFail(null); }} className="mono" autoComplete="off" spellCheck={false} />
        </div>
      </Card>
      <div className="toolbar">
        <Button type="submit" variant="primary" pending={pending} unavailable={!online}>{pending ? "Submitting…" : "Submit job"}</Button>
        {!online ? <span className="meta">You are offline. Changes are disabled until you reconnect.</span> : null}
        {fail?.code === "timeout" ? <span className="meta">The outcome is unknown. Submitting again with this key is safe: it cannot create a second job.</span> : null}
      </div>
    </form>
  );
}

export function JobNewView() {
  const p = useConsoleProvider();
  const handlers = useResource(() => p.listHandlers(), "handlers");
  const cfg = useResource(() => p.getConfig(), "config-new");
  const both = useMemo(() => (handlers.state === "ok" && cfg.state === "ok" ? { h: handlers.data.items, c: cfg.data } : null), [handlers, cfg]);
  if (!both) return <Load res={handlers.state === "error" ? handlers : cfg.state === "error" ? cfg : handlers} rows={4} label="Loading the handler catalogue" errorHeading="The handler catalogue could not be loaded">{() => null}</Load>;
  if (both.h.length === 0) return <Notice variant="info" lead="No handlers are registered." role="none">Jobs can only target allowlisted handlers. See the <Link href="/docs/workers">Worker SDK docs</Link>.</Notice>;
  return <Form handlers={both.h} cfg={both.c} />;
}
