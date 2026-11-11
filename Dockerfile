# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.22 AS builder
WORKDIR /src/bridge
ENV CGO_ENABLED=0
COPY phev2mqtt/go.mod phev2mqtt/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY phev2mqtt/ ./
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/phev2mqtt .
WORKDIR /src/launcher
ARG BUILD_VERSION=development
ARG SOURCE_REVISION=unknown
COPY launcher/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w -X main.version=${BUILD_VERSION} -X main.revision=${SOURCE_REVISION}" -o /out/launcher .

FROM scratch
ENV HOME=/data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/phev2mqtt /opt/phev2mqtt
COPY --from=builder /out/launcher /opt/launcher
COPY LICENSE /LICENSE
CMD ["/opt/launcher"]
