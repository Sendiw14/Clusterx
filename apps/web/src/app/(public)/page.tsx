import type { Metadata } from "next";
import Link from "next/link";
import { ButtonLink, CodeBlock, Notice, Reveal } from "@clusterx/ui";
import { Atlas } from "@/features/atlas";
import { HOME_QUICKSTART, VERIFICATION } from "@/content/commands";
import { CONSOLE_CAPTURE } from "@/content/console-capture";
import "@/features/home.css";

export const metadata: Metadata = {
  title: "Jobs that outlive their workers",
  description: "CLUSTERX runs every job at least once. Leases and fencing keep a returning worker from overwriting what happened while it was gone.",
};

export default function HomePage() {
  return (
    <>
      <section className="hero" aria-labelledby="hero-h">
        <div className="wrap hero-grid">
          <div className="hero-in">
            <p className="eyebrow" style={{ "--i": 0 } as React.CSSProperties}>Open-source distributed job engine · self-hosted</p>
            <h1 id="hero-h" className="display" style={{ "--i": 1, marginTop: "var(--space-5)" } as React.CSSProperties}>Jobs that outlive their <em>workers.</em></h1>
            <p className="lede" style={{ "--i": 2 } as React.CSSProperties}>CLUSTERX runs every job at least once. Leases and fencing make sure a worker that vanished, and then came back, cannot overwrite what happened while it was gone.</p>
            <div className="cta-row" style={{ "--i": 3 } as React.CSSProperties}>
              <ButtonLink href="/demo" variant="primary" size="lg">Explore the demo</ButtonLink>
              <ButtonLink href="/docs/quickstart" size="lg">Read the quickstart</ButtonLink>
            </div>
            <p className="hero-note" style={{ "--i": 3 } as React.CSSProperties}>At-least-once, not exactly-once: a handler can run twice, so side effects need to be idempotent. The engine guarantees only that the database never records a stale result.</p>
          </div>
          <figure className="plate hero-rise" style={{ margin: 0 }}>
            <div className="plate-h"><span>Fig. 1 · one job, two workers</span><span>Illustrative, scripted</span></div>
            <div className="tbl-wrap" role="region" tabIndex={0} aria-label="Scripted example ledger">
              <table className="ledger">
                <caption className="sr-only">Ledger of the scripted example: time, event, resulting status</caption>
                <thead><tr><th scope="col">Time</th><th scope="col">Event</th><th scope="col">Status</th></tr></thead>
                <tbody>
                  <tr><td>t+00.0 s</td><td>admit <span className="who">· committed</span></td><td>queued</td></tr>
                  <tr><td>t+00.4 s</td><td>claim, lease gen 1 <span className="who">· worker A</span></td><td>running</td></tr>
                  <tr><td>t+42.4 s</td><td>lease expired</td><td>retry_wait</td></tr>
                  <tr><td>t+44.9 s</td><td>claim, lease gen 2 <span className="who">· worker B</span></td><td>running</td></tr>
                  <tr><td>t+46.1 s</td><td>result committed</td><td>succeeded</td></tr>
                  <tr className="rej"><td>t+53.7 s</td><td>late report from worker A <span className="who">· rejected</span></td><td>succeeded <span className="who">(unchanged)</span></td></tr>
                </tbody>
              </table>
            </div>
            <figcaption className="plate-f">Worker A went silent, B finished the job, and A&apos;s late report was fenced off. The handler ran twice; the record is still single and coherent.</figcaption>
          </figure>
        </div>
        <div className="wrap" aria-hidden="true"><hr className="tick-rule" style={{ marginTop: "var(--space-7)" }} /></div>
      </section>

      <section className="section" aria-labelledby="atlas-h" style={{ paddingTop: 0 }}>
        <div className="wrap">
          <Reveal className="sec-h"><p className="eyebrow">The Execution Atlas</p><h2 id="atlas-h" className="h2">Follow one job through a worker failure.</h2><p className="lede">Eight steps, scripted and deterministic. Nothing on this diagram is live telemetry; the real console draws the same shapes from persisted state.</p></Reveal>
          <Atlas />
        </div>
      </section>

      <section className="section alt" aria-labelledby="pill-h">
        <div className="wrap">
          <Reveal className="sec-h"><p className="eyebrow">Three things it does</p><h2 id="pill-h" className="h2">Small surface. Stated guarantees.</h2></Reveal>
          <Reveal className="pillars">
            <article><p className="no">01 / Admission</p><h3>Acknowledged means committed.</h3><p>A job gets a success response only after its transaction commits in PostgreSQL. A coordinator restart does not lose acknowledged work.</p><p className="proof">POST /jobs → 201 after commit</p></article>
            <article><p className="no">02 / Execution</p><h3>Leases and fencing.</h3><p>Every claim creates an attempt and a lease generation in one transaction. Reports from a stale attempt, generation, or revoked credential are refused.</p><p className="proof">attempt + generation + incarnation</p></article>
            <article><p className="no">03 / Inspection</p><h3>Every attempt is on the record.</h3><p>Jobs, attempts, events and workers are read from the database over the REST API. The console that renders them live is still in development; until it ships, the API and the demo are the way in.</p><p className="proof">GET /jobs/&#123;id&#125;/attempts</p></article>
          </Reveal>
        </div>
      </section>

      <section className="section" aria-labelledby="arch-h">
        <div className="wrap">
          <Reveal className="sec-h"><p className="eyebrow">Architecture</p><h2 id="arch-h" className="h2">One coordinator, one database, workers that pull.</h2><p className="lede">Workers never touch the database. They ask the coordinator for work and report back over authenticated gRPC.</p></Reveal>
          <Reveal as="ol" className="arch" >
            <li><p className="tier">Layer 1</p><h3>Browser</h3><p>Console and public site over HTTPS. Holds no API or worker tokens.</p><span className="trust">session cookie only</span><span className="stat" data-s="plan">Specified, not built</span></li>
            <li><p className="tier">Layer 2</p><h3>Next.js server</h3><p>Human sessions and the server-side proxy. Signs short-lived service assertions.</p><span className="trust">trust boundary</span><span className="stat" data-s="plan">Specified, not built</span></li>
            <li><p className="tier">Layer 3</p><h3>Go coordinator</h3><p>REST, gRPC, scheduler and authorization on every call.</p><span className="trust">enforces roles</span><span className="stat" data-s="ok">Implemented, tested</span></li>
            <li><p className="tier">Layer 4</p><h3>PostgreSQL</h3><p>The only authoritative store for jobs, attempts, leases and events.</p><span className="trust">single source of truth</span><span className="stat" data-s="ok">Implemented, tested</span></li>
            <li><p className="tier">Layer 5</p><h3>Workers</h3><p>Trusted, precompiled handlers. Pull work, renew leases, report results.</p><span className="trust">credential per worker</span><span className="stat" data-s="ok">Implemented, tested</span></li>
          </Reveal>
          <p style={{ marginTop: "var(--space-4)" }}><Link href="/architecture">Read the full architecture, trade-offs and failure domains</Link></p>
        </div>
      </section>

      <section className="section alt" aria-labelledby="story-h">
        <div className="wrap story">
          <Reveal className="sec-h" style={{ marginBottom: 0 }}><p className="eyebrow">The worker-failure story</p><h2 id="story-h" className="h2">What happens when a worker dies mid-job.</h2><p className="lede">The same scripted example as the Atlas, in plain terms: no retry is hidden, no result is overwritten. The recovery path is covered by in-process tests; killing a real worker process to watch it is not yet verified.</p></Reveal>
          <Reveal as="ol" className="timeline">
            <li><span className="t">t+00 s</span><p><strong>Claimed.</strong> Worker A holds attempt 1 under a lease. The default is 30 s and it is configurable.</p></li>
            <li data-kind="warn"><span className="t">t+42 s</span><p><strong>Lease expires.</strong> The database clock decides, not the worker. Attempt 1 counts against the job&apos;s budget.</p></li>
            <li><span className="t">t+44 s</span><p><strong>Retried.</strong> With budget left the job goes back to the queue; worker B claims attempt 2.</p></li>
            <li data-kind="ok"><span className="t">t+46 s</span><p><strong>Committed.</strong> Worker B&apos;s result is recorded with its attempt and generation.</p></li>
            <li data-kind="bad"><span className="t">t+53 s</span><p><strong>Rejected.</strong> Worker A reports late. The result is refused and a diagnostic event is kept. The job still says succeeded.</p></li>
          </Reveal>
        </div>
      </section>

      <section className="section" aria-labelledby="prev-h">
        <div className="wrap">
          <Reveal className="sec-h"><p className="eyebrow">Console</p><h2 id="prev-h" className="h2">Open a job and see every attempt.</h2></Reveal>
          <Reveal as="figure" className="preview" style={{ margin: 0 }}>
            <div className="preview-bar"><span>{CONSOLE_CAPTURE.route}</span><span>real capture, {CONSOLE_CAPTURE.date}, commit {CONSOLE_CAPTURE.commit}</span></div>
            <div className="preview-in">
              {/* eslint-disable-next-line @next/next/no-img-element -- a fixed static capture; no optimizer needed */}
              <img src={CONSOLE_CAPTURE.file} width={CONSOLE_CAPTURE.width} height={CONSOLE_CAPTURE.height} alt="Screenshot of the console jobs list on live data: succeeded, failed, running and retry-wait jobs from the three example handlers, each with its status, handler, priority, creation time and attempts." loading="lazy" decoding="async" style={{ width: "100%", height: "auto", display: "block" }} />
            </div>
            <figcaption className="plate-f" style={{ background: "var(--color-white)" }}>A screenshot of the real console, not a mock-up: taken on {CONSOLE_CAPTURE.date} from commit <span className="mono">{CONSOLE_CAPTURE.commit}</span> against a local stack (PostgreSQL 18, the control plane and one worker running the three example handlers). Job IDs and times are from that run. The <Link href="/demo">demo</Link> shows the same views on labelled fixture data.</figcaption>
          </Reveal>
        </div>
      </section>

      <section className="section alt" aria-labelledby="qs-h">
        <div className="wrap stack-grid">
          <Reveal><p className="eyebrow">Quickstart</p><h2 id="qs-h" className="h2" style={{ marginTop: "var(--space-4)" }}>Three workers on your machine.</h2><p className="lede" style={{ marginTop: "var(--space-4)" }}>A local development stack: PostgreSQL, the coordinator and three example workers. It is not a production setup, and it does not include the web console yet.</p></Reveal>
          <Reveal style={{ maxWidth: "48rem" }}>
            <CodeBlock code={HOME_QUICKSTART} language="Shell" label="Quickstart commands" />
            <div style={{ marginTop: "var(--space-4)", display: "grid", gap: "var(--space-3)" }}>
              <Notice variant="success" lead="Verified." className="qs-notice" role="none">{VERIFICATION.verified}</Notice>
              <Notice variant="warning" lead="Not yet verified." className="qs-notice" role="none">{VERIFICATION.open}</Notice>
            </div>
            <p style={{ marginTop: "var(--space-4)" }}><Link href="/docs/quickstart">Full quickstart, with what each step does</Link></p>
          </Reveal>
        </div>
      </section>

      <section className="section" aria-labelledby="faq-h">
        <div className="wrap stack-grid">
          <Reveal><p className="eyebrow">Limitations and questions</p><h2 id="faq-h" className="h2" style={{ marginTop: "var(--space-4)" }}>What it does not promise.</h2></Reveal>
          <Reveal className="faq" style={{ maxWidth: "48rem" }}>
            <details><summary>Is processing exactly-once?</summary><p>No. Execution is at-least-once. If a worker loses its lease while a handler is still running, the handler may run again on another worker. The database only records a result from the current attempt, so make external side effects idempotent.</p></details>
            <details><summary>Can the engine stop a handler that ignores cancellation?</summary><p>No. Deadlines and cancellation are cooperative: they stop the engine from accepting results, not a goroutine from running. Handlers should honor their context.</p></details>
            <details><summary>What if the coordinator goes down?</summary><p>There is one coordinator. While it is down, no new claims or reports happen; workers must not assume a lease still holds. Processing resumes from PostgreSQL when it returns. This is a single-coordinator design, not a high-availability one.</p></details>
            <details><summary>Is job order strict FIFO?</summary><p>No. Eligible jobs are claimed by priority, due time and creation time, but concurrent claimers can see rows out of strict order.</p></details>
            <details><summary>Does fencing stop a stale handler from running?</summary><p>No. Fencing protects what the database records. It cannot unrun a handler, so an outdated worker may still cause external side effects until it notices it lost its lease.</p></details>
            <details><summary>How long are results and logs kept?</summary><p>Results are limited to 128 KiB and are never truncated; larger ones fail the job. Attempt log tails are limited to 16 KiB and expire after 7 days, and terminal jobs are deleted after 30 days. These are proposed values, not yet exercised in a release.</p></details>
            <details><summary>Can low-priority jobs starve?</summary><p>Yes. Claiming is by priority, so sustained priority 9 load can keep priority 0 jobs waiting. It is monitored, not prevented, and fairness is not a promise.</p></details>
            <details><summary>How fast is it?</summary><p>There are no published throughput numbers. A 1,000-job, three-worker test passes in-process; it is a correctness test, not a benchmark.</p></details>
            <details><summary>Does the demo show real data?</summary><p>No. The public demo is seeded fixture data in your browser, with no connection to a running system. The real engine runs locally with Docker Compose.</p></details>
          </Reveal>
        </div>
      </section>

      <section className="cta-band" aria-labelledby="cta-h">
        <div className="wrap" style={{ display: "grid", gap: "var(--space-5)" }}>
          <h2 id="cta-h" className="h2">Run it, break a worker, read the record.</h2>
          <div className="cta-row" style={{ marginTop: 0 }}>
            <ButtonLink href="/docs/quickstart" variant="primary" size="lg">Quickstart</ButtonLink>
            <ButtonLink href="/architecture" size="lg">Read the architecture</ButtonLink>
          </div>
        </div>
      </section>
      
    </>
  );
}
