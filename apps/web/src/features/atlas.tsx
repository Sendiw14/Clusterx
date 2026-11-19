"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Icon } from "@clusterx/ui";
import { ATLAS_SVG_INNER } from "./atlas-svg";

interface Step { t: string; job: string; res?: string; title: string; short: string; text: string; path?: string[]; ring?: [string, number] }
const STEPS: Step[] = [
  { t: "t+00 s", job: "queued", title: "Admit", short: "Committed before the API answers.", text: "The API commits the job to PostgreSQL, then answers. Nothing is acknowledged before commit. Status: queued.", path: ["e1"] },
  { t: "t+00.4 s", job: "running", title: "Claim", short: "Attempt 1, lease generation 1.", text: "Worker A asks the coordinator for work. One transaction creates attempt 1 and lease generation 1, valid for 30 s (the default; configurable). Status: running.", path: ["e2"], ring: ["ring1", 100] },
  { t: "t+12 s", job: "running", title: "Renew", short: "Worker A extends its lease.", text: "While the handler runs, worker A renews the lease before it runs out. The liveness heartbeat and the lease renewal are separate signals.", path: ["e5"], ring: ["ring1", 100] },
  { t: "t+14 s", job: "running", title: "Silence", short: "Worker A stops renewing.", text: "Worker A crashes or is cut off. Renewals stop. Its handler may still be running somewhere; the engine cannot know that.", ring: ["ring1", 55] },
  { t: "t+42.4 s", job: "retry_wait", title: "Expire", short: "Lease ends on the database clock.", text: "The lease runs out on the database clock. Attempt 1 is marked lease_expired and counts against the job's attempt budget. Status: retry_wait.", ring: ["ring1", 0] },
  { t: "t+44.9 s", job: "running", title: "Recover", short: "Back to queued; worker B claims it.", text: "Budget remains, so the job returns to the queue after backoff and worker B claims it: attempt 2, lease generation 2. Status: running.", path: ["e7", "e3"], ring: ["ring2", 100] },
  { t: "t+46.1 s", job: "succeeded", res: "committed", title: "Commit", short: "Worker B's result is accepted.", text: "Worker B reports success. The coordinator accepts it because the attempt, generation and worker identity are all current. Status: succeeded.", path: ["e9"], ring: ["ring2", 100] },
  { t: "t+53.7 s", job: "succeeded", res: "committed", title: "Fence", short: "Worker A's late report is rejected.", text: "Worker A returns and reports a result for attempt 1. The fence rejects it: that attempt is stale. The committed result stands. The handler ran twice; the record was never overwritten.", path: ["e8"], ring: ["ring2", 100] },
];

const inRange = (r: string, n: number) => { const [a = "0", b = "0"] = r.split("-"); return n >= +a && n <= +b; };
const reduced = () => document.documentElement.getAttribute("data-rm") === "1" || window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/**
 * Execution Atlas (marketing). Illustrative and scripted: a deterministic 8-step script, never telemetry. Controls are
 * native buttons (Play has aria-pressed), the step list is always visible, the caption is a polite status.
 * Auto-plays once when 60 percent visible (never under reduced motion), never loops, pauses when scrolled out of view
 * or the tab is hidden. Reduced motion: stepping only, the token jumps to the connector end.
 */
export function Atlas() {
  const stage = useRef<SVGSVGElement>(null);
  const [step, setStep] = useState(0);
  const [playing, setPlaying] = useState(false);
  const raf = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const stepRef = useRef(0);
  const auto = useRef(false);
  const tickRef = useRef<() => void>(() => {});

  const render = useCallback((n: number) => {
    const svg = stage.current;
    if (!svg) return;
    svg.querySelectorAll<SVGElement>(".g").forEach((g) => {
      const vis = g.getAttribute("data-vis"), hl = g.getAttribute("data-hl"), dim = g.getAttribute("data-dim"), ok = g.getAttribute("data-ok");
      if (vis) g.classList.toggle("is-off", !(n > 0 && inRange(vis, n)));
      g.classList.toggle("is-hl", !!hl && hl.split(",").includes(String(n)));
      g.classList.toggle("is-dim", !!dim && n > 0 && inRange(dim, n));
      g.classList.toggle("is-ok", !!ok && n > 0 && inRange(ok, n));
    });
    const s = STEPS[n - 1];
    const set = (id: string, v: string) => { const el = svg.querySelector(`#${id}`); if (el) el.textContent = v; };
    set("job-status", s?.job ?? "​");
    set("res-status", s?.res ?? "​");
    set("clk", s?.t ?? "t+00 s");
    if (n === 0) ["ring1", "ring2"].forEach((id) => svg.querySelector(`#${id}`)?.setAttribute("stroke-dasharray", "0 100"));
    else if (s?.ring) svg.querySelector(`#${s.ring[0]}`)?.setAttribute("stroke-dasharray", `${s.ring[1]} ${100 - s.ring[1]}`);
    stepRef.current = n;
    setStep(n);
  }, []);

  const travel = useCallback((ids?: string[]) => {
    const svg = stage.current;
    const token = svg?.querySelector<SVGCircleElement>("#token");
    cancelAnimationFrame(raf.current);
    if (!svg || !token) return;
    token.setAttribute("opacity", "0");
    if (!ids?.length) return;
    const paths = ids.map((id) => svg.querySelector<SVGPathElement>(`#${id}`)).filter((p): p is SVGPathElement => !!p);
    if (!paths.length) return;
    const place = (p: SVGPathElement, k: number) => { const pt = p.getPointAtLength(p.getTotalLength() * k); token.setAttribute("cx", String(pt.x)); token.setAttribute("cy", String(pt.y)); };
    token.setAttribute("opacity", "1");
    if (reduced()) { place(paths[paths.length - 1]!, 1); return; }
    const dur = 600 * paths.length;
    let t0: number | null = null;
    const frame = (t: number) => {
      t0 ??= t;
      const k = Math.min(1, (t - t0) / dur);
      const idx = Math.min(paths.length - 1, Math.floor(k * paths.length));
      place(paths[idx]!, k === 1 ? 1 : Math.min(1, k * paths.length - idx));
      if (k < 1) raf.current = requestAnimationFrame(frame);
    };
    raf.current = requestAnimationFrame(frame);
  }, []);

  const go = useCallback((n: number) => { render(n); travel(STEPS[n - 1]?.path); }, [render, travel]);
  const stop = useCallback(() => { setPlaying(false); if (timer.current) clearTimeout(timer.current); }, []);

  const tick = useCallback(() => {
    if (stepRef.current >= 8) { setPlaying(false); return; }
    go(stepRef.current + 1);
    timer.current = setTimeout(() => tickRef.current(), reduced() ? 2600 : 2400);
  }, [go]);
  useEffect(() => { tickRef.current = tick; }, [tick]);
  const play = useCallback(() => {
    if (stepRef.current >= 8) { go(0); }
    setPlaying(true);
    tick();
  }, [go, tick]);

  useEffect(() => {
    render(0);
    const svg = stage.current;
    if (!svg) return;
    const io = new IntersectionObserver((entries) => {
      for (const e of entries) {
        if (e.isIntersecting && !auto.current && !reduced()) { auto.current = true; play(); }
        if (!e.isIntersecting) stop();
      }
    }, { threshold: 0.6 });
    io.observe(svg);
    const onVis = () => { if (document.hidden) stop(); };
    document.addEventListener("visibilitychange", onVis);
    return () => { io.disconnect(); document.removeEventListener("visibilitychange", onVis); stop(); cancelAnimationFrame(raf.current); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const s = STEPS[step - 1];
  return (
    <div className="atlas-card" role="group" aria-labelledby="atlas-h">
      <div className="atlas-h"><span className="tag">Illustrative, scripted</span><span className="meta">Step {step} of 8</span></div>
      <div className="atlas-body">
        <div>
          <div className="atlas-stage" tabIndex={0} role="region" aria-label="Execution Atlas diagram (scrolls horizontally on narrow screens)">
            <svg
              ref={stage}
              id="atlas"
              viewBox="0 0 1000 500"
              role="img"
              aria-describedby="atlas-desc"
              focusable="false"
              dangerouslySetInnerHTML={{ __html: `<title>Execution Atlas: one job, a lease expiry and a rejected late result</title><desc id="atlas-desc">A job is admitted into a queue. Worker A claims it as attempt 1 under a 30 second lease and renews it, then goes silent. The lease expires, a recovery step puts the job back in the queue, and worker B claims it as attempt 2. Worker B commits the result. A late report from worker A is rejected because its attempt is stale. The same eight steps are listed beside the diagram.</desc>${ATLAS_SVG_INNER}` }}
            />
          </div>
          <div className="atlas-ctl" role="group" aria-label="Atlas controls">
            <button type="button" className="btn btn-sm" aria-pressed={playing} onClick={() => (playing ? stop() : play())}>
              <Icon name={playing ? "i-pause" : "i-play"} /><span>{playing ? "Pause" : step >= 8 ? "Replay" : "Play"}</span>
            </button>
            <button type="button" className="btn btn-sm" onClick={() => { stop(); go(step >= 8 ? 1 : step + 1); }}><Icon name="i-step" />Step</button>
            <button type="button" className="btn btn-sm" onClick={() => { stop(); cancelAnimationFrame(raf.current); stage.current?.querySelector("#token")?.setAttribute("opacity", "0"); render(0); }}><Icon name="i-reset" />Reset</button>
            <span className="meta atlas-note">Plays once on first view, then rests. Reduced motion: step through only.</span>
          </div>
          <div className="atlas-cap" role="status" aria-live="polite">
            <b>Step {step} of 8{s ? ` · ${s.title}` : ""}</b>
            <span>{s ? s.text : "Press Play or Step. The steps are listed alongside; the diagram only mirrors them."}</span>
          </div>
        </div>
        <div className="steps">
          <h3 className="h3" style={{ fontSize: "var(--text-ui-lg)" }}>The eight steps</h3>
          <ol>
            {STEPS.map((x, i) => (
              <li key={x.title} aria-current={step === i + 1 ? "step" : undefined}>
                <div><strong>{x.title}</strong><span>{x.short}</span></div>
              </li>
            ))}
          </ol>
        </div>
      </div>
      <ul className="atlas-legend" aria-label="Atlas legend">
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><rect x="4" y="5" width="36" height="14" rx="3" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /></svg><span><strong>Job</strong> rounded block, status written beneath</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><rect x="3" y="4" width="38" height="16" rx="3" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /><path d="M15 8v8M27 8v8" stroke="var(--color-line-strong)" strokeWidth="1.5" /></svg><span><strong>Queue</strong> slotted block, depth shown by filled slots</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><circle cx="22" cy="12" r="6" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /></svg><span><strong>Attempt</strong> numbered circle, one per claim</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><circle cx="22" cy="12" r="9" fill="none" stroke="var(--color-accent)" strokeWidth="3" strokeDasharray="40 17" strokeLinecap="round" /></svg><span><strong>Lease</strong> ring around an attempt; arc length is time left</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><polygon points="6,5 34,5 40,10 40,19 12,19 6,14" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /></svg><span><strong>Worker</strong> chamfered block, with its incarnation</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><polygon points="22,2 36,12 22,22 8,12" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /></svg><span><strong>Recovery</strong> diamond: the engine decides retry or fail</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><rect x="4" y="5" width="36" height="14" rx="3" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /><rect x="4" y="5" width="6" height="14" rx="2" fill="var(--color-ink)" /></svg><span><strong>Result</strong> block with a solid spine</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><path d="M3 12h34" stroke="var(--color-ink)" strokeWidth="1.75" /><path d="M3 20h30" stroke="var(--color-accent)" strokeWidth="1.75" strokeDasharray="5 4" /><path d="M3 5h30" stroke="var(--color-danger)" strokeWidth="1.75" strokeDasharray="5 4" /></svg><span><strong>Edges</strong> solid committed, blue dashed renewal, red dashed rejected</span></li>
      </ul>
    </div>
  );
}
