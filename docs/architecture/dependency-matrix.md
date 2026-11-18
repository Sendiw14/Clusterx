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
| Next.js / React / Tailwind / Motion | _pending M3-FE web skeleton_ | — | |
| GitHub Actions | checkout v7.0.1 `3d3c42e5…`, setup-go v7.1.0 `d8cbb804…`, setup-node v7.1.0 `949feb24…`, pnpm/action-setup v6.1.1 `73e46992…` | pinned by SHA | |
| Container runtime (dev) | Colima 0.10.3, Docker 29.10 CLI / 29.5.2 engine, compose plugin | — | registry CDN flaky from VM (DNS/IPv6); `--dns 1.1.1.1` set |
