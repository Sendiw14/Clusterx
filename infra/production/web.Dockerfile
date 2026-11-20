# CLUSTERX web (Next.js + BFF) production image. Build from the repository root:
#   docker build -f infra/production/web.Dockerfile -t clusterx/web:local .
# apps/web is built unchanged from the locked workspace; runs as UID 65532 with a read-only root filesystem
# (compose mounts a tmpfs for .next/cache). Base image pinned by tag and multi-arch index digest.
FROM public.ecr.aws/docker/library/node:26.13.1-trixie-slim@sha256:e6bedef217677b276c591795e169840edd9096dbf13f29df8ef8d1856b8820bc AS build
ENV NEXT_TELEMETRY_DISABLED=1 CI=1
RUN npm install -g pnpm@9.15.9 --no-fund --no-audit
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/ui/package.json packages/ui/
COPY packages/sdk-ts/package.json packages/sdk-ts/
RUN pnpm install --frozen-lockfile --filter "web..."
COPY api/ api/
COPY packages/ui/ packages/ui/
COPY packages/sdk-ts/ packages/sdk-ts/
COPY apps/web/ apps/web/
RUN pnpm --filter web build

FROM public.ecr.aws/docker/library/node:26.13.1-trixie-slim@sha256:e6bedef217677b276c591795e169840edd9096dbf13f29df8ef8d1856b8820bc
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1
WORKDIR /app/apps/web
COPY --from=build /app /app
COPY infra/production/web-entry.mjs /app/web-entry.mjs
USER 65532:65532
EXPOSE 3000
CMD ["node", "/app/web-entry.mjs"]
