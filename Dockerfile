# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/keygen ./cmd/keygen

FROM debian:bookworm-slim AS runtime

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --gid 10001 app && \
    useradd --uid 10001 --gid 10001 --no-create-home --home-dir /app --shell /usr/sbin/nologin app && \
    install -d -m 0755 /app/config /app/storage && \
    install -d -m 0755 -o 10001 -g 10001 /app/storage/uploads

WORKDIR /app
ENV TZ=Asia/Ho_Chi_Minh
COPY --from=build --chmod=0555 /out/api /app/api
COPY --from=build --chmod=0555 /out/keygen /app/keygen
# Keep directory traversal permissions separate from the read-only file mode.
# COPY --chmod can also apply its mode to newly created parent directories.
COPY config/app.docker.yaml /app/config/app.yaml
RUN chmod 0444 /app/config/app.yaml

USER 10001:10001
EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/app/api"]
CMD ["--config=/app/config/app.yaml"]
