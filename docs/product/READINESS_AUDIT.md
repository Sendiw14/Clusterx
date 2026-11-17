# CLUSTERX — Final Evidence-Based Readiness Audit and Decisions

**Date:** 2026-11-17  
**Target:** The 2026-11-17 54 KB `CLUSTERX_FINAL_ULTIMATE_IMPLEMENTATION_MASTER_PROMPT.md`, the `CLUSTERX_MULTI_MODEL_DEVELOPMENT_WORKFLOW.md`, the previous implementation plan, audit reports, and the generated 10-file Claude Code kit.  
**Output:** `CLUSTERX_FINAL_MASTER_PROMPT_REVISED_AND_INTEGRATED.md` (Revision R2, ~8,100 words), plus a revised operating kit.  
**Review class:** Documentation/technical-consistency review and external standards verification; **not** a code security audit, running-system evaluation, or model entitlement test.  
**Repository state:** Connected GitHub `Sendiw14/Clusterx`, public, empty, size 0, default branch main on this date. **No repository changes made.**

## Evidence sources and claim limits

- **Official sources checked:** Anthropic Claude model catalog, Claude Code model config/settings-reference and custom subagents; Go release history; PostgreSQL 18.7 and PostgreSQL 19 RC2 announcement; Node.js LTS schedule; Next.js 16 support/upgrading; TypeScript home; OpenAPI 3.2.1; WCAG 2.2; OWASP ASVS 5.0; RFC 9700/10017; GitHub Actions hardening.
- **File checks:** Compared the previous 54 KB master with the multi-model operating protocol and kit (`CLAUDE.md`, `.claude/settings.json`, three agent definitions, two templates). The earlier artifacts were detailed but did not incorporate model ownership into the actual master, creating two possible interpretations of authority.
- **Not verified:** local Claude CLI version, account entitlements, an Ultracode session actually running, runtime compatibility of exact pinned patches, SQL correctness, UI rendered at all sizes, deployed OAuth/credentials, benchmark/SLOs, provider cost or a production release. Treat all as explicit gates.

## Overall verdict

**GO for controlled implementation after owner adoption of R2 and M0/M1 architectural preflight. CONDITIONAL NO-GO for claiming full autonomous readiness or production release.** The master is coherent and actionable as a *contract for staged implementation*; it cannot guarantee zero missing requirements, zero software defects, or that a single agent run will finish safely.

## Material findings, remedies and acceptance gates

| ID | Severity | Finding in previous master/kit | Revised disposition | Evidence required |
|---|---|---|---|---|
| F01 | P0 | Multi-model ownership lived only in a companion protocol, despite demand for one authoritative prompt | Integrated model governance as normative Section 1 in R2; sidecars are execution aides | Agents read one spec; ticket/PR references master revision and contract SHA |
| F02 | P0 | Two agent lanes could alter shared OpenAPI/client/root files without an enforceable ordering | Exclusive integrator ownership of shared files; frozen generated contracts and sequential integration | Concurrent worktree merge test and contract CI |
| F03 | P0 | 'Sonnet high / Opus medium / Ultracode' could be assumed to run by merely committing settings | Validate CLI, model, effort, workflows, account; explicit fallback when not available | Actual `/model`, `/effort`, `/status` evidence; no silent substitutions |
| F04 | P0 | Agent reviews might be mistaken for independent human verification | Separate reviewer session plus deterministic tests; human owner controls release/security exception | Non-author reviewer report, passing tests, owner release decision |
| F05 | P0 | Prior product policies could still diverge from operating protocol | Explicit precedence: safety/owner > unified master > approved ADR/contract > ticket > agent preference | No unresolved conflicting normative rules in release review |
| F06 | P0 | Concurrent job cancellation may race completion or recovery | One DB transaction decides outcome; cancel request prevents later success, expired ownership fenced | Controlled cancel/success/expiry/partition races yield valid terminal outcome |
| F07 | P0 | Hard deadline might be mistaken for physical termination of an uncooperative Go goroutine | Separate authorization fencing from real external execution; optional process isolation explicitly tested | Run stubborn handler; stale result rejected; no false forced-stop claim |
| F08 | P0 | Browser/Next BFF/Go/direct API/worker/public demo trust boundaries may be implemented inconsistently | Defined signed audience-bound BFF assertions, scoped direct tokens, authenticated gRPC, fixture-only public demo | Cross-boundary authorization tests; no privileged token in browser/demo |
| F09 | P1 | Scope too broad for one unrestricted implementation run | Stage Alpha/Beta/RC/1.0 and separate 1.1; no feature silently dropped | Milestone-specific acceptance mapping and release freeze |
| F10 | P1 | 2026 model defaults are surface-dependent | Explicit efforts per lane (Sonnet high, Opus medium) independent of CLI defaults; Ultracode separate | Session configuration checkpoint |
| F11 | P1 | Claude Code custom subagents are not the same as independent parallel worktrees | Recommend two isolated sessions/worktrees; optional agents for bounded review | Two branches, ticket/contract handoff, no shared write conflict |
| F12 | P1 | Task ownership unclear for Next.js auth BFF and generated client | Sonnet implements BFF UI under Opus-owned auth contract; integrator owns generated TS client | Permission tests and codegen diff on each change |
| F13 | P1 | Some deployment/hosting values, OAuth identities and license remain owner-dependent | Keep owner approvals explicitly gated and distinguish local-only safe defaults | Decision register records approval before publish/spend/credentials |
| F14 | P1 | Backend benchmark lacks measured SLO targets | Freeze latency/capacity/recovery targets after pilot, before RC | Fixture, hardware and results with pass/fail thresholds |
| F15 | P1 | Browser semantics for live events could be overclaimed | HTTP snapshot authoritative, WebSocket advisory, reconcile after missing messages | Reconnect/drop/reorder E2E reconciles to persisted values |
| F16 | P1 | Data retention vs immutable history not clearly distinguished | Immutability applies while retained; retention/archival rules are separate | Retention tests and honest expired-history UI |
| F17 | P1 | Dependency version and standard drift risk | Version/patch compatibility matrix, pinned deps, scheduled review, stable standards | Build/test against pinned set; advisory review and reproducible codegen |
| F18 | P1 | UI originality may reward motion over operational clarity | Purposeful Execution Atlas + accessible table/graph equivalent; marketing and console motion separated | Responsive screenshots, keyboard/a11y, performance and design review |
| F19 | P1 | Agent reporting may present generated files as done | Standard task states and HANDOFF/PR evidence fields | Traceability goes implemented→integrated→tested→verified only with tests |
| F20 | P2 | Several prior source references may change after 2026 | R2 links official sources but requires revalidation during implementation | Dependency review changelog and ADR notes |

**P0 in this table are approval gates**, not evidence that running software is defective. Past audit count totals should not be added to this table as if they are new distinct defects.

## Scope, page and discipline comparison

| Discipline | Current R2 outcome | Remaining release evidence |
|---|---|---|
| Product/personas/scope | Included developer/operator/viewer/evaluator journeys, Alpha/Beta/RC, 1.0 vs 1.1 non-goals | Real user task walkthrough and scope sign-off |
| Go engine/scheduler/worker | Formal states, ownership, leases, retries, cancel, eligibility and worker lifecycle | DB/worker test suite with fault injection |
| Data architecture | Authoritative PostgreSQL, versioned migrations, bounded JSON, retention and restore | Schema review, crash consistency, restore drill |
| API/proto/SDK | REST, gRPC, generated client and contract change protocol | Generated code actually compiles; version skew tests |
| Identity/security | GitHub OAuth **proposed**, server sessions/BFF, direct tokens, worker creds, read-only demo | Owner identity selection, threat model, negative auth tests |
| Public site/docs | All approved public routes, section content and error states | Real responsive high-fidelity screens, tested links and copy |
| Console | All approved console routes, interactions, status, permission states | Live E2E, high-volume table/topology tests |
| Creative/motion | Original white-first Precision Under Motion / Execution Atlas, animation matrix | Manual and automated reduced-motion/resizing/scroll QA |
| Accessibility | WCAG 2.2 AA and 320px reflow target; graph equivalents | Criterion-by-criterion manual/audited evidence |
| Performance/observability | Core Web Vitals, backend provisional limits, traces/metrics/alerts | Pilot thresholds, load measurements, real incident drill |
| Infrastructure and maintenance | Single Linux VM Compose reference; backups, upgrades and support policy | Dry-run install, restoration, rollback and real costs |
| Multi-model execution | Sonnet/Opus routing, one master, separate worktrees/branches, handoffs, independent review | Actual model/CLI availability and successful collaboration evidence |
| CI/GitHub release | PR gates, race/chaos/release checks, docs/license/README | Working pipelines, provenance, clean clone and authorized tag/release |

## Key decisions carried into revised master

- Preserve original Go + PostgreSQL single-authority architecture, one coordinator and trusted handler registry; do **not** add Kafka/Kubernetes/extra queue to look modern.
- At-least-once job execution, with unique current attempt lease only; no exactly-once side effects.
- Automatic retries = attempts on same job; terminal manual retry = linked new job; immutable terminal state while retained.
- `cancel_requested_at` commits before a completion means subsequent success is denied; if completion commits first, cancellation cannot reverse it. Process kill cannot be equated to context cancellation.
- PostgreSQL clock controls lease and due time; worker credentials and incarnation checked on claim/renew/report; stale results rejected.
- Authenticated UI uses real API; public demo is fixture-backed read-only with **no production Go API connection**.
- Snapshot/reconciliation authoritative; WebSocket only advisory.
- Main interface white-first, Execution Atlas original; WCAG2.2 AA, motion alternatives, quantified performance testing.
- **Multi-model:** Sonnet 5.5 high frontend, Opus 5.5 medium backend/lead; independent Opus high for critical review; not a permanent army of agents.
- Use source schemas and a single contract custodian; no incompatible hand-written client DTOs.

## 2026+ version and standards verification summary

Official documentation as checked on 2026-11-17: Go 1.27.4 security-patched; PostgreSQL 18.7 stable while PostgreSQL 19 RC2 (GA planned November 19); Node 26 LTS; Next.js 16 Active LTS; TypeScript 7 stable candidate subject to tooling compatibility; Sonnet 5.5 and Opus 5.5 IDs exist; Claude Code ultracode may run with medium/high if version >=2.1.284 and workflows permitted. Model access cannot be inferred from published availability. OpenAPI 3.2.1 is published, generator compatibility must be checked; WCAG 2.2 AA, OWASP ASVS 5.0 and OAuth RFC 9700/10017 are current references. Avoid adopting new standards just for recency.

## Outstanding owner actions (cannot be answered by specification alone)

1. Explicitly adopt R2 as the canonical repository master before implementation; approvals may be recorded during M0 rather than blocking read-only planning.
2. Select open-source license (MIT/Apache-2.0/other) before publishing a tagged release; check dependency licenses.
3. Approve OAuth provider, first operator identities and credential provisioning before any real authenticated deployment.
4. Choose host/domain/budget and external services, if any; do not deploy or spend by default.
5. Supply/test local Claude entitlement and model/effort/Ultracode status before launching the multi-model workflow.
6. Approve any high-risk security exception explicitly; unwaived critical exploitable issues block release.

## Further validation the prompt itself cannot complete

- Exact dependency patch compatibility and build with Next/React/TypeScript/Node, Go/protobuf/API generators, auth library and tooling.
- Correctness under real DB contention, partitions, stale workers, task timeouts, retry exhaustion and cancellation races.
- Security validation of actual signed delegation/session key rotation, sensitive field access and public demo isolation.
- Verified UI, assets/licenses, keyboard/screen-reader tests, Core Web Vitals and large graph performance on actual devices.
- Numerical backend SLOs/capacity from a reproducible benchmark; deployment RPO/RTO and restore drill; recurring operating costs.
- Actual GitHub repository deployment, CI, commits, licensing and release state.

## Final readiness verdict

**Master prompt itself:** READY as a **controlled, staged implementation instruction set** with explicit owner/ADR and technical proof gates.  
**Project implementation:** NOT STARTED / NOT VERIFIED on reviewed public repository.  
**Unrestricted autonomous build:** NOT APPROVED until M0 owner decisions and M1 trust/lease/invariant contracts are accepted.  
**Production / release:** NO-GO until all critical tests, UX/a11y/security/deployment gates pass and human publication approval exists.  

The goal is defensible engineering and a remarkable accessible UI, not a claim that completeness of prose guarantees success.

## Official references

- Anthropic models: https://platform.claude.com/docs/en/models/overview
- Claude Code configuration: https://code.claude.com/docs/en/model-config
- Claude Code settings: https://code.claude.com/docs/en/settings-reference
- Claude Code subagents: https://code.claude.com/docs/en/sub-agents
- Go: https://go.dev/doc/devel/release
- PostgreSQL: https://www.postgresql.org/about/news/
- Node.js: https://nodejs.org/en/about/previous-releases
- Next.js: https://nextjs.org/support-policy
- TypeScript: https://www.typescriptlang.org/
- OpenAPI: https://spec.openapis.org/oas/latest.html
- OWASP: https://asvs.owasp.org/
- WCAG: https://www.w3.org/TR/WCAG22/
- OAuth: https://datatracker.ietf.org/doc/rfc9700/ and https://datatracker.ietf.org/doc/rfc10017/
- GitHub Actions: https://docs.github.com/en/actions/reference/security/secure-use
