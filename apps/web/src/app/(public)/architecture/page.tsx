import type { Metadata } from "next";
import Link from "next/link";
import { Notice, Reveal } from "@clusterx/ui";
import { ArchitectureDiagram } from "@/features/architecture-diagram";
import "@/features/home.css";

export const metadata: Metadata = {
  title: "Architecture",
  description: "Topology, trust boundaries, the job and lease path, trade-offs, failure domains and the stated limits of CLUSTERX.",
};

const LAYERS = [
  { name: "Browser", text: "Public site and console over HTTPS. Holds a session cookie only: no API tokens, no worker credentials.", trust: "session cookie only", built: false },
  { name: "Next.js server (BFF)", text: "Human sessions, CSRF and origin checks, and a server-side proxy. Signs a short-lived, audience-bound service assertion for every upstream request. It never decides authorization.", trust: "trust boundary", built: false },
  { name: "Go control plane", text: "Job admission, due-job promotion, transactional claims, fencing, retries, cancellation, recovery, worker registration and REST authorization on every call.", trust: "enforces roles and scopes", built: true },
  { name: "PostgreSQL", text: "The only authoritative store for jobs, attempts, leases and events. Time for leases is the database clock.", trust: "single source of truth", built: true },
  { name: "Workers", text: "Trusted, precompiled Go handlers. They pull work, renew leases and report over authenticated gRPC. They cannot write to the database.", trust: "credential per worker", built: true },
];

const CONNECTIONS = [
  { from: "Browser", to: "Next.js server", proto: "HTTPS", cred: "Session cookie (server-held session)", status: "Specified, not built", src: "ADR-0005, ADR-0006" },
  { from: "Next.js server", to: "Control plane REST", proto: "HTTP, /api/v1", cred: "Signed service assertion, 60 s at most", status: "Specified, not built", src: "ADR-0005, ADR-0006" },
  { from: "API client", to: "Control plane REST", proto: "HTTP, /api/v1", cred: "Scoped API token (cxt_ prefix)", status: "Implemented, tested", src: "ADR-0005, api/openapi.yaml" },
  { from: "Worker", to: "Control plane gRPC", proto: "gRPC, pull model", cred: "Worker credential (cxw_ prefix), checked on every call", status: "Implemented, tested", src: "ADR-0005, packages/proto" },
  { from: "Control plane", to: "PostgreSQL", proto: "SQL (pgx), READ COMMITTED", cred: "Least-privilege runtime role", status: "Implemented, tested", src: "ADR-0001, ADR-0004" },
];

const PATH = [
  { h: "Admit.", t: "A job is acknowledged only after its transaction commits: the job row, an admitted event and any idempotency record go in together. A database outage answers 503, never a premature success." },
  { h: "Promote.", t: "A scheduler loop moves due scheduled jobs to queued. It runs in the coordinator, in short transactions, about once a second." },
  { h: "Claim.", t: "A worker asks for work for the handlers it advertised. One transaction creates an attempt, increments the lease generation and marks the job running. Rows are locked with SKIP LOCKED so concurrent claimers never take the same job." },
  { h: "Renew.", t: "While the handler runs, the SDK renews the lease. Renewing a lease and sending a worker heartbeat are two separate signals; a heartbeat never extends a lease." },
  { h: "Report.", t: "A result is accepted only for the current attempt, lease generation, worker incarnation and a non-revoked credential, judged on the database clock after row locks are held." },
  { h: "Recover.", t: "A lease that runs out is closed by the scheduler. The job returns to the queue after backoff while attempt budget remains; otherwise it fails. A late report from the old worker is rejected." },
];

const TIMING = [
  { k: "Lease TTL", v: "30 s", b: "5 to 300 s" },
  { k: "Heartbeat interval", v: "10 s", b: "2 to 60 s" },
  { k: "Liveness timeout", v: "30 s", b: "at least 3 x heartbeat" },
  { k: "Cancel grace", v: "15 s", b: "1 to 120 s" },
  { k: "Drain grace", v: "60 s", b: "0 to 1 h" },
  { k: "Scheduler tick", v: "1 s", b: "0.2 to 10 s" },
  { k: "Retry backoff", v: "2 s base, 5 min cap, jittered", b: "base 0.1 s to 60 s" },
  { k: "Attempts per job", v: "3 (includes the first run)", b: "1 to 5" },
];

const FAILURES = [
  { f: "A worker crashes", w: "Its lease stops being renewed and expires on the database clock. The job is retried if budget remains. If the worker comes back, its late report is rejected.", s: "Covered by in-process tests; a real process kill is not yet verified" },
  { f: "A worker is partitioned but keeps running", w: "Its handler may still run and cause side effects. Its reports are rejected once the lease expires. This is why execution is at-least-once.", s: "Fence covered by tests" },
  { f: "The control plane restarts", w: "REST and gRPC are unavailable. Nothing uncommitted was acknowledged. Leases may expire during the outage; recovery runs on start.", s: "Restart exercised in the M4 curl guide" },
  { f: "PostgreSQL is down", w: "Readiness fails, REST writes answer 503, claims pause, scheduler loops back off. State resumes from what was committed.", s: "Outage tests not yet run" },
  { f: "The live channel drops", w: "The design says the console shows it and reconciles from a snapshot. Live messages are hints, never data.", s: "Design only: console not built" },
  { f: "The BFF cannot reach Go", w: "The design shows an infrastructure error with a request ID and does not ask the user to sign in again.", s: "Design only: BFF not built" },
];

const SOURCES = [
  "docs/architecture/overview.md",
  "docs/architecture/state-machine.md",
  "docs/architecture/adrs/0001-single-coordinator-postgres-authority.md",
  "docs/architecture/adrs/0002-state-machine-cancel-success-precedence.md",
  "docs/architecture/adrs/0003-leases-heartbeat-fencing-timing.md",
  "docs/architecture/adrs/0004-ids-idempotency-bounds-retention.md",
  "docs/architecture/adrs/0005-trust-boundaries.md",
  "docs/architecture/adrs/0006-bff-session-contract.md",
  "api/openapi.yaml",
  "packages/proto/clusterx/worker/v1/worker.proto",
  "apps/control-plane/internal/config/config.go",
];

export default function ArchitecturePage() {
  return (
    <div className="section-page">
      <div className="wrap">
        <p className="eyebrow">Architecture</p>
        <h1 className="page-title" style={{ marginTop: "var(--space-4)", maxWidth: "20ch" }}>One coordinator, one database, workers that pull.</h1>
        <p className="lede" style={{ marginTop: "var(--space-4)" }}>Where each part sits, which credential crosses each boundary, what is built and what only exists as a specification. Source files are listed at the end; every statement here traces to one.</p>

        <section aria-labelledby="topo-h" style={{ marginTop: "var(--space-7)" }}>
          <h2 id="topo-h" className="h2">Topology</h2>
          <p className="muted" style={{ marginTop: "var(--space-3)", maxWidth: "44rem" }}>Browser to Next.js to the Go control plane to PostgreSQL. Workers connect to the control plane, never to the database. The diagram is hidden on very narrow screens; the layer list and connection table below say the same thing.</p>
          <Reveal style={{ marginTop: "var(--space-5)" }}><ArchitectureDiagram /></Reveal>
        </section>

        <section aria-labelledby="layers-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="layers-h" className="h2">Layers</h2>
          <ol className="arch" style={{ marginTop: "var(--space-5)" }} aria-label="Architecture layers, top to bottom on narrow screens, left to right on wide ones">
            {LAYERS.map((l, i) => (
              <li key={l.name}>
                <p className="tier">Layer {i + 1}</p><h3>{l.name}</h3><p>{l.text}</p>
                <span className="trust">{l.trust}</span>
                <span className="stat" data-s={l.built ? "ok" : "plan"}>{l.built ? "Implemented, tested" : "Specified, not built"}</span>
              </li>
            ))}
          </ol>
        </section>

        <section aria-labelledby="conn-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="conn-h" className="h2">Connections and credentials</h2>
          <div className="card tbl-wrap" role="region" tabIndex={0} aria-label="Connections table" style={{ marginTop: "var(--space-4)" }}>
            <table className="tbl">
              <caption>Every connection, what protects it, and its build status</caption>
              <thead><tr><th scope="col">From</th><th scope="col">To</th><th scope="col">Protocol</th><th scope="col">Credential</th><th scope="col">Status</th><th scope="col">Source</th></tr></thead>
              <tbody>
                {CONNECTIONS.map((c) => (
                  <tr key={c.from + c.to}>
                    <td data-label="From" data-primary>{c.from}</td><td data-label="To">{c.to}</td><td data-label="Protocol">{c.proto}</td><td data-label="Credential">{c.cred}</td><td data-label="Status">{c.status}</td><td data-label="Source">{c.src}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section aria-labelledby="path-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="path-h" className="h2">The job and lease path</h2>
          <ol className="path-list" style={{ marginTop: "var(--space-5)" }}>
            {PATH.map((p) => <li key={p.h}><p><strong>{p.h}</strong> {p.t}</p></li>)}
          </ol>
          <h3 className="h3" style={{ marginTop: "var(--space-7)" }}>Timing defaults</h3>
          <p className="muted" style={{ marginTop: "var(--space-2)", maxWidth: "44rem" }}>These are the defaults compiled into the control plane. The coordinator refuses to start with a value outside its bounds. They are starting values and have not been tuned against a workload.</p>
          <div className="card tbl-wrap" role="region" tabIndex={0} aria-label="Timing defaults table" style={{ marginTop: "var(--space-3)" }}>
            <table className="tbl">
              <caption>Default timing and bounds</caption>
              <thead><tr><th scope="col">Setting</th><th scope="col">Default</th><th scope="col">Allowed</th></tr></thead>
              <tbody>{TIMING.map((t) => <tr key={t.k}><td data-label="Setting" data-primary>{t.k}</td><td data-label="Default">{t.v}</td><td data-label="Allowed">{t.b}</td></tr>)}</tbody>
            </table>
          </div>
        </section>

        <section aria-labelledby="trade-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="trade-h" className="h2">Trade-offs</h2>
          <p className="muted" style={{ marginTop: "var(--space-3)", maxWidth: "44rem" }}>Each choice buys something and costs something. The costs are part of the design.</p>
          <div className="tradeoffs" style={{ marginTop: "var(--space-5)" }}>
            <div>
              <h3>What the design buys</h3>
              <ul>
                <li>One source of truth: no broker, no second queue that can disagree with the database.</li>
                <li>Atomic claims: an attempt, a lease and a status change commit together or not at all.</li>
                <li>Workers need no database access and open no inbound ports.</li>
                <li>Leases are judged on one clock, the database&apos;s, so worker clock drift does not matter.</li>
                <li>No arbitrary code upload: handlers are trusted and precompiled.</li>
              </ul>
            </div>
            <div>
              <h3>What it costs</h3>
              <ul>
                <li>Throughput is bounded by PostgreSQL write capacity. No numbers are published yet.</li>
                <li>One coordinator means downtime when it restarts. This is not a high-availability design.</li>
                <li>Row-locking claims are not strict FIFO, and low priority can starve under sustained high priority load.</li>
                <li>Fencing protects the database record, not the outside world: a handler can run twice.</li>
                <li>A new handler means shipping a new worker build.</li>
              </ul>
            </div>
          </div>
        </section>

        <section aria-labelledby="fail-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="fail-h" className="h2">Failure domains</h2>
          <div className="card tbl-wrap" role="region" tabIndex={0} aria-label="Failure domains table" style={{ marginTop: "var(--space-4)" }}>
            <table className="tbl">
              <caption>What fails, what happens, and how far that is verified</caption>
              <thead><tr><th scope="col">Failure</th><th scope="col">What happens</th><th scope="col">Evidence</th></tr></thead>
              <tbody>
                {FAILURES.map((f) => <tr key={f.f}><td data-label="Failure" data-primary>{f.f}</td><td data-label="What happens">{f.w}</td><td data-label="Evidence">{f.s}</td></tr>)}
              </tbody>
            </table>
          </div>
        </section>

        <section aria-labelledby="lim-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="lim-h" className="h2">Limitations, stated plainly</h2>
          <ul className="main-prose" style={{ marginTop: "var(--space-4)", paddingLeft: "var(--space-5)", display: "grid", gap: "var(--space-2)" }}>
            <li>Execution is at-least-once. A handler can run twice; make side effects idempotent.</li>
            <li>Fencing protects the authoritative database record. It cannot unrun a handler that already acted outside.</li>
            <li>There is one coordinator, so it is a single point of downtime.</li>
            <li>Priority scheduling on PostgreSQL has eligibility and fairness limits; it is not strict FIFO.</li>
            <li>Deadlines and cancellation are cooperative. The engine cannot kill a handler that ignores its context.</li>
            <li>Results and log tails are size-limited and expire, so old information disappears by design.</li>
            <li>The public demo is illustrative and isolated, not live telemetry.</li>
            <li>No throughput or recovery-time numbers are promised until a benchmark is published.</li>
          </ul>
          <div style={{ marginTop: "var(--space-5)", maxWidth: "44rem" }}>
            <Notice variant="info" lead="Not in 1.0." role="none">Versioned workflow definitions are planned for a later release and are not part of this architecture. See <Link href="/docs/reliability">Reliability</Link> for what is and is not guaranteed.</Notice>
          </div>
        </section>

        <section aria-labelledby="src-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="src-h" className="h2">Sources</h2>
          <p className="muted" style={{ marginTop: "var(--space-3)", maxWidth: "44rem" }}>Paths are relative to the repository root.</p>
          <ul className="src-list" style={{ marginTop: "var(--space-4)" }}>
            {SOURCES.map((s) => <li key={s}><code>{s}</code></li>)}
          </ul>
        </section>
      </div>
    </div>
  );
}
