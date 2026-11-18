import type { Metadata } from "next";
import { Notice } from "@clusterx/ui";

export const metadata: Metadata = { title: "Architecture", description: "Topology, trust boundaries, the job and lease path, failure domains and the limits of CLUSTERX." };

const LAYERS = [
  { name: "Browser", text: "Public site and console over HTTPS. Holds a session cookie only: no API tokens, no worker credentials.", trust: "session cookie only" },
  { name: "Next.js server (BFF)", text: "Human sessions, CSRF and origin checks, and a server-side proxy. Signs a short-lived, audience-bound service assertion for every upstream request. It never decides authorization.", trust: "trust boundary" },
  { name: "Go control plane", text: "Job admission, due-job promotion, transactional claims, fencing, retries, cancellation, recovery, worker registration and REST authorization on every call.", trust: "enforces roles and scopes" },
  { name: "PostgreSQL", text: "The only authoritative store for jobs, attempts, leases and events. Time for leases is the database clock.", trust: "single source of truth" },
  { name: "Workers", text: "Trusted, precompiled Go handlers. They pull work, renew leases and report over authenticated gRPC. They cannot write to the database.", trust: "credential per worker" },
];

export default function ArchitecturePage() {
  return (
    <div className="section-page">
      <div className="wrap">
        <p className="eyebrow">Architecture</p>
        <h1 className="page-title" style={{ marginTop: "var(--space-4)", maxWidth: "20ch" }}>One coordinator, one database, workers that pull.</h1>
        <p className="lede" style={{ marginTop: "var(--space-4)" }}>This page states the real topology and where each trust boundary sits. A full diagram, source links and measured numbers arrive with the documentation milestone.</p>

        <section aria-labelledby="layers-h" style={{ marginTop: "var(--space-7)" }}>
          <h2 id="layers-h" className="h2">Layers</h2>
          <p className="muted" style={{ marginTop: "var(--space-3)", maxWidth: "44rem" }}>Browser to Next.js to the Go control plane to PostgreSQL; workers connect to the control plane, never to the database.</p>
          <ol className="arch" style={{ marginTop: "var(--space-5)" }} aria-label="Architecture layers, top to bottom on narrow screens, left to right on wide ones">
            {LAYERS.map((l, i) => (
              <li key={l.name}><p className="tier">Layer {i + 1}</p><h3>{l.name}</h3><p>{l.text}</p><span className="trust">{l.trust}</span></li>
            ))}
          </ol>
        </section>

        <section aria-labelledby="path-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="path-h" className="h2">The job and lease path</h2>
          <ol className="main-prose" style={{ marginTop: "var(--space-4)", paddingLeft: "var(--space-5)", display: "grid", gap: "var(--space-2)" }}>
            <li><strong>Admit.</strong> A job is acknowledged only after its transaction commits.</li>
            <li><strong>Promote.</strong> Due scheduled jobs become queued.</li>
            <li><strong>Claim.</strong> One transaction creates an attempt and a lease generation.</li>
            <li><strong>Renew.</strong> The worker extends the lease while its handler runs.</li>
            <li><strong>Report.</strong> A result is accepted only for the current attempt, generation and worker identity.</li>
            <li><strong>Recover.</strong> An expired lease returns the job to the queue if attempt budget remains.</li>
          </ol>
        </section>

        <section aria-labelledby="fail-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="fail-h" className="h2">Failure domains</h2>
          <div className="card tbl-wrap" style={{ marginTop: "var(--space-4)" }}>
            <table className="tbl">
              <caption>What fails, and what the system promises when it does</caption>
              <thead><tr><th scope="col">Failure</th><th scope="col">What happens</th></tr></thead>
              <tbody>
                <tr><td data-label="Failure">A worker disappears</td><td data-label="What happens">Its lease expires on the database clock; the job is retried if budget remains. A late report from it is rejected.</td></tr>
                <tr><td data-label="Failure">The control plane is down</td><td data-label="What happens">No new claims or reports. Work resumes from PostgreSQL when it returns. There is one coordinator in 1.0.</td></tr>
                <tr><td data-label="Failure">The live channel drops</td><td data-label="What happens">The console says so and reconciles from a snapshot. Live messages are hints, never data.</td></tr>
                <tr><td data-label="Failure">The BFF cannot reach Go</td><td data-label="What happens">The console shows an infrastructure error with a request ID. The user is not asked to sign in again.</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section aria-labelledby="lim-h" style={{ marginTop: "var(--space-8)" }}>
          <h2 id="lim-h" className="h2">Limitations, stated plainly</h2>
          <ul className="main-prose" style={{ marginTop: "var(--space-4)", paddingLeft: "var(--space-5)", display: "grid", gap: "var(--space-2)" }}>
            <li>Delivery is at-least-once. A handler can run twice; make side effects idempotent.</li>
            <li>There is one coordinator. This is not a high-availability design.</li>
            <li>Claiming uses row locks that skip locked rows, so order is by priority and time but not strictly FIFO.</li>
            <li>Cancellation and deadlines are cooperative. The engine cannot stop a handler that ignores its context.</li>
          </ul>
          <div style={{ marginTop: "var(--space-5)", maxWidth: "44rem" }}><Notice variant="info" lead="Source links pending.">Links to the architecture decision records are added when the repository is published.</Notice></div>
        </section>
      </div>
    </div>
  );
}
