# =============================================================================
# Stage 1: Builder
# =============================================================================
# Using Alpine for smaller base image and faster downloads
FROM golang:1.25-alpine AS builder

# Install git for go mod download (some dependencies may need it)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Generate Swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

# Build the application
# CGO_ENABLED=0: Pure Go build (no C dependencies)
# -ldflags="-s -w": Strip debug info for smaller binary
# -trimpath: Remove file system paths from binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=${VERSION:-dev}" \
    -trimpath \
    -o main ./cmd/api

# =============================================================================
# Stage 2: Production Runtime
# =============================================================================
# Using distroless for minimal attack surface
# - No shell, no package manager, no unnecessary binaries
# - Only contains: ca-certs, timezone data, /tmp directory
FROM gcr.io/distroless/static-debian12:nonroot

# Labels for container metadata
LABEL org.opencontainers.image.title="people-service-registry" \
    org.opencontainers.image.description="People and Company Registry API" \
    org.opencontainers.image.vendor="Appmax" \
    org.opencontainers.image.source="https://bitbucket.org/appmax-space/people-service-registry"

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Copy migrations (needed for goose at startup)
COPY --from=builder /app/internal/infrastructure/db/postgres/migration ./migrations

# Expose API port
EXPOSE 8080

# Health check endpoint
# Note: distroless doesn't have curl, so we rely on Kubernetes probes
# HEALTHCHECK is documented but not functional in distroless

# Run as non-root user (UID 65532 is the nonroot user in distroless)
USER nonroot:nonroot

# Start the application
ENTRYPOINT ["./main"]
