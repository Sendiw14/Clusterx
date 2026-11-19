/**
 * Layered topology of CLUSTERX 1.0. Static SVG, no script. Shapes follow the Execution Atlas grammar
 * (docs/design/execution-atlas.md): rounded block = service, chamfered block = worker, slotted block = store.
 * Dashed outlines mean "specified, not built yet". Dashed vertical rules are trust boundaries.
 * The same facts are repeated as text in the layer list and the connections table on /architecture.
 */
export function ArchitectureDiagram() {
  return (
    <div className="arch-fig">
      <div className="arch-stage" role="region" aria-label="Topology diagram (scrolls horizontally on narrow screens)" tabIndex={0}>
        <svg viewBox="0 0 1000 520" role="img" aria-describedby="arch-desc" focusable="false" id="arch-svg">
          <title>CLUSTERX topology: browser, Next.js server, Go control plane, PostgreSQL and workers</title>
          <desc id="arch-desc">
            Left to right: the browser talks to the Next.js server over HTTPS with a session cookie. The Next.js server talks to the Go control plane REST API with a short-lived signed service assertion. Both of those layers are specified but not built yet. API clients talk to the same REST API with an API token. The Go control plane contains the REST API, the scheduler loops and the worker gRPC service, and it is the only component that connects to PostgreSQL. Three workers below connect to the gRPC service with a worker credential and never to the database. Dashed vertical lines mark trust boundaries.
          </desc>
          <defs>
            <marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" className="ahead" /></marker>
          </defs>

          <g className="tb" aria-hidden="true">
            <line x1="205" y1="40" x2="205" y2="505" /><line x1="470" y1="40" x2="470" y2="505" /><line x1="815" y1="40" x2="815" y2="505" />
            <text x="213" y="30">boundary 1</text><text x="478" y="30">boundary 2</text><text x="823" y="30">boundary 3</text>
          </g>

          <g className="node">
            <rect x="240" y="60" width="190" height="64" rx="8" />
            <text x="335" y="88" className="t" textAnchor="middle">API clients</text>
            <text x="335" y="110" className="s" textAnchor="middle">curl, scripts</text>
          </g>
          <g className="node plan dashed">
            <rect x="20" y="200" width="160" height="110" rx="8" />
            <text x="100" y="236" className="t" textAnchor="middle">Browser</text>
            <text x="100" y="260" className="s" textAnchor="middle">console, site</text>
            <text x="100" y="284" className="s" textAnchor="middle">holds no tokens</text>
          </g>
          <g className="node plan dashed">
            <rect x="240" y="200" width="190" height="110" rx="8" />
            <text x="335" y="236" className="t" textAnchor="middle">Next.js server</text>
            <text x="335" y="260" className="s" textAnchor="middle">sessions, CSRF, proxy</text>
            <text x="335" y="288" className="tag" textAnchor="middle">NOT BUILT YET</text>
          </g>

          <g className="node big">
            <rect x="500" y="90" width="290" height="270" rx="10" />
            <text x="520" y="120" className="t">Go control plane</text>
            <text x="520" y="140" className="s">authorizes every call</text>
            <g className="sub"><rect x="520" y="160" width="250" height="44" rx="6" /><text x="535" y="188">REST /api/v1</text></g>
            <g className="sub"><rect x="520" y="218" width="250" height="44" rx="6" /><text x="535" y="246">Scheduler: promote, recover</text></g>
            <g className="sub"><rect x="520" y="276" width="250" height="44" rx="6" /><text x="535" y="304">Worker gRPC service</text></g>
            <text x="520" y="346" className="tag">IMPLEMENTED, TESTED</text>
          </g>

          <g className="node store">
            <rect x="850" y="200" width="130" height="110" rx="8" />
            <path d="M880 200v110M910 200v110M940 200v110" className="slot" />
            <text x="915" y="334" className="t" textAnchor="middle">PostgreSQL</text>
            <text x="915" y="354" className="s" textAnchor="middle">only authority</text>
          </g>

          {[0, 1, 2].map((i) => {
            const x = 500 + i * 100;
            return (
              <g className="node worker" key={i}>
                <polygon points={`${x},430 ${x + 76},430 ${x + 90},444 ${x + 90},490 ${x + 14},490 ${x},476`} />
                <text x={x + 45} y="456" className="s" textAnchor="middle">worker</text>
                <text x={x + 45} y="478" className="t2" textAnchor="middle">{`W${i + 1}`}</text>
              </g>
            );
          })}
          <text x="810" y="468" className="tag">3 IN LOCAL COMPOSE</text>
          <text x="810" y="488" className="tag">NO DB ACCESS</text>

          <g fill="none">
            <path d="M180 255H238" markerEnd="url(#ah)" markerStart="url(#ah)" className="e dash" />
            <path d="M430 255H498" markerEnd="url(#ah)" className="e dash" />
            <path d="M430 92H485V182H518" markerEnd="url(#ah)" className="e" />
            <path d="M790 255H848" markerEnd="url(#ah)" markerStart="url(#ah)" className="e" />
            <path d="M545 428V322" markerEnd="url(#ah)" markerStart="url(#ah)" className="e" />
            <path d="M645 428V322" markerEnd="url(#ah)" markerStart="url(#ah)" className="e" />
            <path d="M745 428V322" markerEnd="url(#ah)" markerStart="url(#ah)" className="e" />
          </g>
          <g className="lbl">
            <text x="188" y="244">HTTPS</text>
            <text x="440" y="244">service</text><text x="440" y="270">assertion</text>
            <text x="440" y="80">API token</text>
            <text x="796" y="244">SQL</text>
            <text x="535" y="388" textAnchor="end">gRPC, pull,</text><text x="535" y="406" textAnchor="end">worker credential</text>
          </g>
        </svg>
      </div>
      <ul className="atlas-legend arch-legend" aria-label="Diagram legend">
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><rect x="4" y="5" width="36" height="14" rx="3" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /></svg><span><strong>Solid outline</strong> implemented and tested in the repository</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><rect x="4" y="5" width="36" height="14" rx="3" fill="none" stroke="var(--color-ink-2)" strokeWidth="1.5" strokeDasharray="4 3" /></svg><span><strong>Dashed outline</strong> specified, not built yet</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><polygon points="6,5 34,5 40,10 40,19 12,19 6,14" fill="none" stroke="var(--color-ink)" strokeWidth="1.5" /></svg><span><strong>Chamfered</strong> worker</span></li>
        <li><svg viewBox="0 0 44 24" aria-hidden="true"><path d="M22 2v20" stroke="var(--color-line-strong)" strokeWidth="1.5" strokeDasharray="3 3" /></svg><span><strong>Dashed rule</strong> trust boundary</span></li>
      </ul>
    </div>
  );
}
