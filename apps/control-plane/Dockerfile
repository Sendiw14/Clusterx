# CLUSTERX control-plane image. Build from the repository root:
#   docker build -f apps/control-plane/Dockerfile -t clusterx-control-plane:dev .
# Static binary on distroless (nonroot UID 65532); runs with a read-only root filesystem.
# Base images are pinned by tag and multi-arch index digest; update both together.

FROM golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d AS build
ENV CGO_ENABLED=0 GOWORK=off GOFLAGS=-trimpath GOTOOLCHAIN=local
WORKDIR /src
COPY packages/proto/go.mod packages/proto/go.sum packages/proto/
COPY packages/sdk-go/go.mod packages/sdk-go/
COPY examples/basic-jobs/go.mod examples/basic-jobs/
COPY apps/control-plane/go.mod apps/control-plane/go.sum apps/control-plane/
RUN cd apps/control-plane && go mod download && go mod verify
COPY packages/proto/ packages/proto/
COPY packages/sdk-go/ packages/sdk-go/
COPY examples/basic-jobs/ examples/basic-jobs/
COPY apps/control-plane/ apps/control-plane/
ARG VERSION=dev
RUN cd apps/control-plane && go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/clusterx-control-plane ./cmd/clusterx-control-plane

FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931
COPY --from=build /out/clusterx-control-plane /usr/local/bin/clusterx-control-plane
USER 65532:65532
EXPOSE 8080 9090 9091
ENTRYPOINT ["/usr/local/bin/clusterx-control-plane"]
