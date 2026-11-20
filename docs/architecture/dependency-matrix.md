# CLUSTERX — Dependency & Compatibility Matrix (OD-07)

Tested set on 2026-11-18, macOS arm64 dev host (Colima VM for containers). CI on Linux amd64: **NOT RUN** (not pushed).
Rule (§3): adopt as a tested set; pin exact versions; re-verify advisories monthly.

| Area | Pinned | Evidence | Notes |
|---|---|---|---|
| Go toolchain | 1.27.4 | all modules build/vet/`-race` green (main, go.work) | `go.work` + per-module `replace` for `packages/proto` (Go 1.27.4 rejects workspace-only resolution: `unknown revision 000000000000`) |
| PostgreSQL | 18.7 (Homebrew local; image `postgres:18.7-bookworm@sha256:ece55f7868da684e400f82f1030a050718adbdd65c23aa500c486f6e2950dc0f`) | DDL + PoC + migration tests | PG 19 not adopted (§3) |
| pgx/v5 | v5.13.0 | race tests | |
| grpc-go | v1.85.0 | build + smoke | |
| protobuf-go / protoc-gen-go | v1.36.12 | `generate.sh` reproducible | |
| protoc-gen-go-grpc | v1.6.2 | same | |
| protoc | 29.3 (CI: release zip, sha256 `3e866620…0021a`) | same | |
| prometheus/client_golang | v1.25.0 | `/metrics` smoke | |
| Go build image | `golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d` | `docker build` arm64 | amd64 NOT RUN |
| Runtime image | `gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931` | UID 65532, RO fs, caps dropped | |
| Node.js | 26.13.1 (`.nvmrc` = 26) | pnpm install, tsc | |
| pnpm | 9.15.9 (`packageManager`) | install | newer majors exist; upgrade with lockfile review |
| TypeScript (sdk-ts) | 5.9.3 | `tsc --noEmit` exit 0 | TS 7.0.2 / 6.0.3 **not adopted**: `openapi-typescript@7.13.0` peer `typescript ^5.x` (§3 rule) |
| openapi-typescript | 7.13.0 | generated `schema.d.ts` | |
| openapi-fetch | 0.17.0 | typecheck | |
| @redocly/cli | 2.80.0 (root); 2.79.0 used in M1 evidence | lint exit 0 | |
| OpenAPI | 3.1.1 | lint | 3.2.1 not adopted (generator compat unproven) |
| Next.js | 16.4.6 (Turbopack build) | `pnpm --filter web build` exit 0 | |
| React / React DOM | 19.3.1 | build + Vitest | |
| Tailwind CSS (+ @tailwindcss/postcss) | 4.3.5 | build; generated `@theme static` from tokens.css (drift-tested) | |
| TypeScript (web) | 5.9.3 | typecheck exit 0 | `typescript-eslint` 8.73.0 caps `<6.1` |
| ESLint | 9.39.5 (+ eslint-config-next 16.4.6, typescript-eslint 8.73.0) | lint exit 0 | ESLint 10 rejected: bundled plugin peers stop at 9 |
| Vitest / Vite | 5.1.1 / 8.3.8 (jsdom 31.0.1) | 50 tests | |
| Playwright / axe | @playwright/test 1.65.1 (Chromium 159) / @axe-core/playwright 4.14.0 | e2e exit 0; axe 0 violations (59 scans) | Firefox/WebKit NOT RUN |
| marked / yaml (docs build) | 18.2.1 / 2.9.1 | build-time API reference + docs search index | web importer only |
| Motion | not installed | CSS + rAF meet motion matrix | add only if needed |
| Fonts | IBM Plex Sans/Mono, Newsreader (OFL 1.1, self-hosted) | hashes in `apps/web/public/fonts/FONTS.md` | OD-17 |
| GitHub Actions | checkout v7.0.1 `3d3c42e5…`, setup-go v7.1.0 `d8cbb804…`, setup-node v7.1.0 `949feb24…`, pnpm/action-setup v6.1.1 `73e46992…` | pinned by SHA | |
| Container runtime (dev) | Colima 0.10.3, Docker 29.10 CLI / 29.5.2 engine, compose plugin | — | registry CDN flaky from VM (DNS/IPv6); `--dns 1.1.1.1` set |

## M8 additions (TASK-M8-BE-01, 2026-11-20)

| Area | Pinned | Evidence | Notes |
|---|---|---|---|
| Prometheus | `quay.io/prometheus/prometheus:v3.16.0@sha256:9332054523223e2ddc18d8101799d54a4a8d823b030c53216f063a6c117773cd` | `promtool test rules` SUCCESS; alert drill | quay.io copy: Docker Hub `prom/prometheus` pulls stalled from the VM (Hub digest differs: `e0498764…`) |
| node-exporter | `quay.io/prometheus/node-exporter:v1.12.1@sha256:1b4e4438faca4dd7e001dd445d161a4a2091b0fededa84093b3a8dfeae1f1be0` | disk alert drill | same digest on Docker Hub; v1.13.1 (1 day old) not adopted |
| Caddy | `public.ecr.aws/docker/library/caddy:2.11.8@sha256:4fcafa98afd1c2e642e353e3c5cf9ddbe892fcee14bc990e7ba2c5b78e51f9cf` | production proof | ECR Public mirror of the Docker official image, same index digest as Docker Hub `caddy:2.11.8`; 2.11.9 (4 days old) not adopted |
| Node runtime image (web) | `public.ecr.aws/docker/library/node:26.13.1-trixie-slim@sha256:e6bedef217677b276c591795e169840edd9096dbf13f29df8ef8d1856b8820bc` (same index digest as Docker Hub) | production proof | corepack is not bundled with Node 26: pnpm 9.15.9 via `npm install -g` |
| govulncheck | v1.9.0 (`go install`) | `evidence/m8/security` | v1.10.0 (3 days old) not adopted |
| gitleaks | v8.30.1 (`go install`) | same | |
| osv-scanner | v2.7.0 (`go install`) | same | |
| syft | v1.57.0 (`go install`) | same | |
| golang.org/x/net | see go.mod (bumped for GO-2026-6860/6866, not reachable per govulncheck) | govulncheck | |
