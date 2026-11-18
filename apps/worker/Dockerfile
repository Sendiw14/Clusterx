# CLUSTERX worker image. Build from the repository root:
#   docker build -f apps/worker/Dockerfile -t clusterx-worker:dev .
# Static binary on distroless (nonroot UID 65532); runs with a read-only root filesystem.
# Base images are pinned by tag and multi-arch index digest; update both together.

FROM golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d AS build
ENV CGO_ENABLED=0 GOWORK=off GOFLAGS=-trimpath GOTOOLCHAIN=local
WORKDIR /src
COPY packages/proto/go.mod packages/proto/go.sum packages/proto/
COPY apps/worker/go.mod apps/worker/go.sum apps/worker/
RUN cd apps/worker && go mod download && go mod verify
COPY packages/proto/ packages/proto/
COPY apps/worker/ apps/worker/
ARG VERSION=dev
RUN cd apps/worker && go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/clusterx-worker ./cmd/clusterx-worker

FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931
COPY --from=build /out/clusterx-worker /usr/local/bin/clusterx-worker
USER 65532:65532
# The worker listens on no port; it dials the control plane.
ENTRYPOINT ["/usr/local/bin/clusterx-worker"]
