# Memtrace — multi-tenant memory layer for AI agents (Go)
# Multi-stage build for minimal image size

ARG VERSION=dev

# ---------- Build stage ----------
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

ARG VERSION
RUN CGO_ENABLED=1 GOOS=linux go build \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o memtrace ./cmd/memtrace/

# ---------- Production stage ----------
FROM alpine:3.21

ARG VERSION

RUN apk add --no-cache ca-certificates curl

# Create non-root user
RUN adduser -D -u 1000 memtrace && \
    mkdir -p /app/data && \
    chown -R memtrace:memtrace /app

WORKDIR /app

COPY --from=builder --chown=memtrace:memtrace /build/memtrace .
COPY --chown=memtrace:memtrace memtrace.toml .

# Persist version inside the image for diagnostics
RUN echo "${VERSION}" > VERSION && chown memtrace:memtrace VERSION

USER memtrace

VOLUME ["/app/data"]

EXPOSE 9100

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://127.0.0.1:9100/health || exit 1

ENTRYPOINT ["./memtrace"]
