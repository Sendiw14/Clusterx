"use client";
import { useState, type FormEvent } from "react";
import { Button, CopyButton, DataTable, Dialog, EmptyState, Notice, Pagination, Select, Tabs, Timestamp, TextInput, useOnline, useToast, type Column } from "@clusterx/ui";
import { useConsoleProvider } from "@/data/context";
import type { ApiToken, ApiTokenCreate, WorkerCredential } from "@/data/console-provider";
import type { ApiFailure, Page, Result } from "@/data/provider";
import { Load } from "./common";
import { ConfirmDialog } from "./confirm";
import { describeFailure, fieldIssues, newKey } from "./failure";
import { useResource } from "./use-resource";

const SCOPES = ["jobs:read", "jobs:write", "workers:read", "workers:write", "metrics:read", "audit:read"] as const;
type Scope = (typeof SCOPES)[number];
const DAY = 86_400_000;
const NEVER = () => false;

interface Revealed { what: string; name: string; secret: string; revokeAfter?: { id: string; name: string } }

/** One-time reveal: a blocking dialog (no Esc, no backdrop close) that holds the secret only in this component's state. */
function RevealDialog({ r, onDone }: { r: Revealed | null; onDone: (r: Revealed) => void }) {
  const [saved, setSaved] = useState(false);
  return (
    <Dialog
      open={Boolean(r)}
      onClose={() => {}}
      blocking
      title={`Copy your new ${r?.what ?? "secret"} now`}
      footer={<Button variant="primary" unavailable={!saved} onClick={() => { if (r) { setSaved(false); onDone(r); } }}>Done, I have stored it</Button>}
    >
      <p><strong>{r?.name}</strong> is shown <strong>once</strong>. CLUSTERX keeps only a hash, so it cannot be shown again. If you lose it, revoke it and create a new one.</p>
      <pre className="jsonblock" tabIndex={0} aria-label={`New ${r?.what}`} data-testid="secret-value" style={{ wordBreak: "break-all", whiteSpace: "pre-wrap" }}>{r?.secret}</pre>
      <p><CopyButton text={r?.secret ?? ""} label="Copy secret" ariaLabel={`Copy the new ${r?.what}`} /></p>
      <label style={{ display: "flex", gap: "var(--space-2)", alignItems: "center" }}>
        <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} /> I have copied it somewhere safe
      </label>
    </Dialog>
  );
}

function CreateDialog({ kind, open, onClose, onCreated }: { kind: "token" | "worker"; open: boolean; onClose: () => void; onCreated: (r: Revealed | { replayed: true }) => void }) {
  const p = useConsoleProvider();
  const [name, setName] = useState("");
  const [role, setRole] = useState<"operator" | "viewer">("viewer");
  const [scopes, setScopes] = useState<Scope[]>(["jobs:read"]);
  const [days, setDays] = useState(kind === "token" ? "90" : "180");
  const [key, setKey] = useState(newKey);
  const [pending, setPending] = useState(false);
  const [err, setErr] = useState<ApiFailure | null>(null);
  const bad = name.trim() === "" || name.length > 100 || (kind === "token" && scopes.length === 0);
  const [touched, setTouched] = useState(false);

  async function go(e: FormEvent) {
    e.preventDefault();
    setTouched(true);
    if (bad || pending) return;
    setPending(true);
    const expires = days === "never" ? null : Number(days);
    const r = kind === "token"
      ? await p.createToken({ name: name.trim(), role, scopes, expires_in_days: expires } as ApiTokenCreate, key)
      : await p.createWorkerCredential({ name: name.trim(), expires_in_days: expires }, key);
    setPending(false);
    if (!r.ok) return setErr(r.error);
    if (r.replayed || !("secret" in r.data)) return onCreated({ replayed: true });
    onCreated({ what: kind === "token" ? "API token" : "worker credential", name: r.data.name, secret: r.data.secret });
    setName("");
    setKey(newKey());
  }
  const fi = err ? fieldIssues(err) : [];
  return (
    <Dialog open={open} onClose={() => { if (!pending) onClose(); }} title={kind === "token" ? "Create API token" : "Create worker credential"}>
      <form onSubmit={(e) => void go(e)} noValidate style={{ display: "grid", gap: "var(--space-3)" }}>
        <TextInput label="Name" required maxLength={100} value={name} onChange={(e) => { setName(e.target.value); setKey(newKey()); setErr(null); }} error={touched && name.trim() === "" ? "Give it a name you will recognise." : fi.find((i) => i.field === "/name")?.message} />
        {kind === "token" ? (
          <>
            <Select label="Role" help="Use-time authority is the scopes intersected with the owner's current role." value={role} onChange={(e) => { const r = e.target.value as "operator" | "viewer"; setRole(r); if (r === "viewer") setScopes((s) => s.filter((x) => x.endsWith(":read"))); setKey(newKey()); }}>
              <option value="viewer">viewer (read scopes)</option>
              <option value="operator">operator (read and write scopes)</option>
            </Select>
            <fieldset style={{ border: 0, padding: 0 }}>
              <legend className="label">Scopes</legend>
              {SCOPES.map((s) => {
                const blocked = role === "viewer" && !s.endsWith(":read");
                return (
                  <label key={s} style={{ display: "flex", gap: "var(--space-2)", alignItems: "center", opacity: blocked ? 0.55 : 1 }}>
                    <input type="checkbox" disabled={blocked} checked={scopes.includes(s)} onChange={(e) => { setScopes(e.target.checked ? [...scopes, s] : scopes.filter((x) => x !== s)); setKey(newKey()); }} />
                    <span className="mono">{s}</span>{blocked ? <span className="meta">needs the operator role</span> : null}
                  </label>
                );
              })}
              {touched && scopes.length === 0 ? <span className="field-error">Choose at least one scope.</span> : null}
            </fieldset>
          </>
        ) : null}
        <Select label="Expires" value={days} onChange={(e) => { setDays(e.target.value); setKey(newKey()); }}>
          {["30", "90", "180", "365"].concat(kind === "worker" ? ["730"] : []).map((d) => <option key={d} value={d}>in {d} days</option>)}
          <option value="never">never</option>
        </Select>
        {err && !fi.length ? <Notice variant="error" lead="Not created." role="alert">{describeFailure(err)}{err.code === "idempotency_conflict" ? " Edit a field to start a new request." : ""}</Notice> : null}
        {fi.length ? <Notice variant="error" lead="Not created." role="alert"><ul>{fi.map((i, n) => <li key={n}>{i.message}</li>)}</ul></Notice> : null}
        {err?.code === "timeout" ? <p className="meta">The outcome is unknown. If it was created, the secret was lost with the response: revoke it from the list and create a new one.</p> : null}
        <div className="dlg-f" style={{ padding: 0 }}>
          <Button onClick={onClose} unavailable={pending}>Cancel</Button>
          <Button type="submit" variant="primary" pending={pending}>{pending ? "Creating…" : "Create"}</Button>
        </div>
      </form>
    </Dialog>
  );
}

type Row = ApiToken | WorkerCredential;
const isToken = (r: Row): r is ApiToken => "scopes" in r;

function Panel({ kind }: { kind: "token" | "worker" }) {
  const p = useConsoleProvider();
  const toast = useToast();
  const online = useOnline();
  const [showRevoked, setShowRevoked] = useState(false);
  const [stack, setStack] = useState<string[]>([]);
  const cursor = stack.at(-1);
  const [creating, setCreating] = useState(false);
  const [reveal, setReveal] = useState<Revealed | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<{ id: string; name: string } | null>(null);
  const [rotating, setRotating] = useState<ApiToken | null>(null);
  const [pending, setPending] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const word = kind === "token" ? "token" : "credential";

  const res = useResource(() => (kind === "token" ? p.listTokens({ include_revoked: showRevoked, cursor, limit: 50 }) : p.listWorkerCredentials({ include_revoked: showRevoked, cursor, limit: 50 })) as Promise<Result<Page<Row>>>, `cred-${kind}-${showRevoked}-${cursor}`, NEVER);

  async function revoke(id: string) {
    setPending(true);
    const r = await (kind === "token" ? p.revokeToken(id) : p.revokeWorkerCredential(id));
    setPending(false);
    if (r.ok) {
      setRevoking(null);
      toast.show(`${kind === "token" ? "Token" : "Credential"} revoked`);
      res.reload();
    } else setErr(describeFailure(r.error));
  }
  async function rotate(t: ApiToken) {
    setPending(true);
    const left = t.expires_at ? Math.min(365, Math.max(1, Math.ceil((Date.parse(t.expires_at) - Date.now()) / DAY))) : null;
    const r = await p.createToken({ name: `${t.name} (rotated)`.slice(0, 100), role: t.role, scopes: t.scopes, expires_in_days: left }, newKey());
    setPending(false);
    setRotating(null);
    if (r.ok && "secret" in r.data) setReveal({ what: "API token", name: r.data.name, secret: r.data.secret, revokeAfter: { id: t.id, name: t.name } });
    else if (r.ok) setNotice("A rotation with this key already exists; its secret cannot be shown again.");
    else setNotice(describeFailure(r.error));
  }

  const columns: Column<Row>[] = [
    { key: "name", header: "Name", primary: true, render: (r) => <>{r.name} {r.revoked_at ? <span className="badge">revoked</span> : null}</> },
    { key: "prefix", header: "Prefix", render: (r) => <span className="mono">{r.prefix}</span> },
    ...(kind === "token" ? [{ key: "scopes", header: "Role and scopes", render: (r: Row) => (isToken(r) ? <>{r.role}: <span className="mono">{r.scopes.join(" ")}</span></> : null) }] : [{ key: "bound", header: "Bound workers", numeric: true, render: (r: Row) => ("bound_worker_ids" in r ? (r.bound_worker_ids?.length ?? 0) : 0) }]),
    { key: "created", header: "Created", render: (r) => <Timestamp value={r.created_at} /> },
    { key: "exp", header: "Expires", render: (r) => (r.expires_at ? <Timestamp value={r.expires_at} /> : <span className="meta">never</span>) },
    { key: "used", header: "Last used", render: (r) => (r.last_used_at ? <Timestamp value={r.last_used_at} /> : <span className="meta">never</span>) },
    { key: "act", header: "Actions", render: (r) => (r.revoked_at ? <span className="meta">none</span> : (
      <span className="toolbar">
        {kind === "token" && isToken(r) ? <Button size="sm" unavailable={!online} onClick={() => setRotating(r)}>Rotate</Button> : null}
        <Button size="sm" variant="danger" unavailable={!online} onClick={() => { setErr(null); setRevoking({ id: r.id, name: r.name }); }}>Revoke</Button>
      </span>
    )) },
  ];

  return (
    <>
      <div className="toolbar" style={{ padding: "var(--space-3) var(--space-4)", justifyContent: "space-between" }}>
        <label style={{ display: "flex", gap: "var(--space-2)", alignItems: "center" }}><input type="checkbox" checked={showRevoked} onChange={(e) => { setShowRevoked(e.target.checked); setStack([]); }} /> Show revoked</label>
        <Button variant="primary" unavailable={!online} onClick={() => setCreating(true)}>{kind === "token" ? "Create API token" : "Create worker credential"}</Button>
      </div>
      {notice ? <div style={{ padding: "0 var(--space-4) var(--space-3)" }}><Notice variant="info" lead="Note." actions={<Button size="sm" onClick={() => setNotice(null)}>Dismiss</Button>}>{notice}</Notice></div> : null}
      <Load res={res} rows={3} label={`Loading ${word}s`} errorHeading={`The ${word} list could not be loaded`}>
        {(page) => page.items.length === 0 ? (
          <EmptyState title={`No ${word}s yet`}>{kind === "token" ? "API tokens let scripts call the API directly, with scopes you choose." : "Worker credentials let a worker register with the control plane."}</EmptyState>
        ) : (
          <>
            <DataTable caption={<>{kind === "token" ? "API tokens" : "Worker credentials"}, {page.items.length} shown · secrets are never listed · as of <Timestamp value={page.as_of} /></>} columns={columns} rows={page.items} rowKey={(r) => r.id} />
            <Pagination hasPrev={stack.length > 0} hasNext={Boolean(page.next_cursor)} onPrev={() => setStack((s) => s.slice(0, -1))} onNext={() => page.next_cursor && setStack((s) => [...s, page.next_cursor as string])} note="50 per page · newest first" />
          </>
        )}
      </Load>
      <CreateDialog kind={kind} open={creating} onClose={() => setCreating(false)} onCreated={(r) => { setCreating(false); if ("replayed" in r) setNotice("A credential with this request already exists. Its secret cannot be shown again: revoke it and create a new one."); else setReveal(r); res.reload(); }} />
      <RevealDialog r={reveal} onDone={(r) => { setReveal(null); res.reload(); if (r.revokeAfter) setRevoking(r.revokeAfter); }} />
      <ConfirmDialog open={Boolean(revoking)} title={`Revoke this ${word}?`} verb="Revoke" danger pending={pending} onConfirm={() => revoking && void revoke(revoking.id)} onCancel={() => { if (!pending) setRevoking(null); }} error={err}>
        <p><strong>{revoking?.name}</strong> stops working immediately. {kind === "token" ? "Requests that use it get 401." : "The bound worker is denied on its next call and its active attempts are recovered at lease expiry."} This cannot be undone.</p>
      </ConfirmDialog>
      <ConfirmDialog open={Boolean(rotating)} title="Rotate this token?" verb="Create replacement" pending={pending} onConfirm={() => rotating && void rotate(rotating)} onCancel={() => { if (!pending) setRotating(null); }}>
        <p>This creates a <strong>new token</strong> with the same role and scopes as <strong>{rotating?.name}</strong>, shows it once, then asks whether to revoke the old one. Switch your clients before revoking.</p>
      </ConfirmDialog>
    </>
  );
}

export function TokensView() {
  return (
    <section className="card" aria-label="Tokens and credentials">
      <Tabs label="Credential type" items={[{ id: "tokens", label: "API tokens", panel: <Panel kind="token" /> }, { id: "workers", label: "Worker credentials", panel: <Panel kind="worker" /> }]} />
    </section>
  );
}
