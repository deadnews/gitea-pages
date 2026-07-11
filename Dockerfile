FROM --platform=${BUILDPLATFORM} golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder

ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GOARCH=${TARGETARCH} \
    GOFLAGS="-ldflags=-s" \
    GOCACHE="/cache/build" \
    GOMODCACHE="/cache/mod"

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=${GOMODCACHE} \
    go mod download

COPY --parents cmd ./
RUN --mount=type=cache,target=${GOCACHE} \
    --mount=type=cache,target=${GOMODCACHE} \
    go build -o /bin/gitea-pages ./cmd/gitea-pages

FROM gcr.io/distroless/static@sha256:d5f030ca7c5793784e9ea4178a116da360250411d13921a5af27c6cb5a5949bf AS runtime

COPY --from=ghcr.io/tarampampam/microcheck:1.4.0@sha256:c9f79cd408626de7c10f2d487d67339f49adf0ba61dde96ede65343269db1f85 /bin/httpcheck /bin/httpcheck

COPY --from=builder /bin/gitea-pages /bin/gitea-pages

USER nonroot:nonroot
HEALTHCHECK NONE
EXPOSE 8000

ENTRYPOINT ["/bin/gitea-pages"]
