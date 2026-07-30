# ──────────────────────────────────────────────
#  AEGIS Multi-Stage Docker Build
# ──────────────────────────────────────────────
# Produces a minimal, scratch-based container (~15MB).
# No shell, no package manager — defense in depth.

# ── Stage 1: Build ────────────────────────────
FROM golang:1.25-alpine AS builder

ARG SERVICE_NAME=question-bank
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with hardened flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w \
        -X 'main.version=${VERSION}' \
        -X 'main.commit=${COMMIT}' \
        -X 'main.buildTime=${BUILD_TIME}'" \
    -trimpath \
    -o /app/service \
    ./cmd/${SERVICE_NAME}/

# ── Stage 2: Production ──────────────────────
FROM scratch

# Import CA certificates for TLS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Import timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/service /service

# Copy migrations (for init container use)
COPY --from=builder /build/migrations /migrations

# Non-root user (UID 65534 = nobody)
USER 65534:65534

# Health check port
EXPOSE 8080 9090

ENTRYPOINT ["/service"]
