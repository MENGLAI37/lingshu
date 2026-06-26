# =============================================================================
# Multi-stage build for ops-ai-agent
# =============================================================================

# Stage 1: Build
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache --virtual .build-deps \
    git \
    ca-certificates \
    tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG BINARY_NAME=ops-ai
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# Build flags
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Build the binary
RUN echo "Building ${BINARY_NAME}..." && \
    go build \
        -ldflags="-s -w \
            -X main.Version=${VERSION} \
            -X main.GitCommit=${GIT_COMMIT} \
            -X main.BuildDate=${BUILD_DATE}" \
        -o /${BINARY_NAME} \
        ./cmd/${BINARY_NAME}

# Stage 2: Build alertd
FROM builder AS builder-alertd

ARG BINARY_NAME=ops-ai-alertd

RUN echo "Building ${BINARY_NAME}..." && \
    go build \
        -ldflags="-s -w \
            -X main.Version=${VERSION} \
            -X main.GitCommit=${GIT_COMMIT} \
            -X main.BuildDate=${BUILD_DATE}" \
        -o /${BINARY_NAME} \
        ./cmd/${BINARY_NAME}

# Stage 3: Final distroless image
FROM gcr.io/distroless/static-debian12:nonroot AS ops-ai

# Labels
LABEL org.opencontainers.image.title="ops-ai-agent"
LABEL org.opencontainers.image.description="AI-native SRE Agent for Kubernetes"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.source="https://github.com/lingshu/ops-ai"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Copy certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Create non-root user
RUN addgroup --system --gid 1000 opsai && \
    adduser --system --uid 1000 --ingroup opsai opsai

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /ops-ai /app/ops-ai

# Set ownership
RUN chown -R opsai:opsai /app

# Switch to non-root user
USER opsai

# Set environment
ENV PATH="/app:${PATH}"
ENV HOME="/app"

# Expose ports
EXPOSE 8080 9090 26379

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["/app/ops-ai"]
CMD ["--help"]

# =============================================================================
# Stage 3b: Final image for alertd
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS ops-ai-alertd

# Labels
LABEL org.opencontainers.image.title="ops-ai-alertd"
LABEL org.opencontainers.image.description="Alert Webhook Server for ops-ai"
LABEL org.opencontainers.image.version="${VERSION}"

# Copy certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Create non-root user
RUN addgroup --system --gid 1000 opsai && \
    adduser --system --uid 1000 --ingroup opsai opsai

# Set working directory
WORKDIR /app

# Copy alertd binary
COPY --from=builder-alertd /ops-ai-alertd /app/ops-ai-alertd

# Set ownership
RUN chown -R opsai:opsai /app

# Switch to non-root user
USER opsai

# Set environment
ENV PATH="/app:${PATH}"
ENV HOME="/app"

# Expose ports
EXPOSE 8080 26379

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["/app/ops-ai-alertd"]
CMD ["--help"]

# =============================================================================
# Final target for multi-architecture builds
# =============================================================================
FROM ops-ai AS latest
FROM ops-ai-alertd AS alertd-latest

# Build args for versioning
# VERSION: Image tag version (e.g., v0.1.0)
# GIT_COMMIT: Git commit SHA
# BUILD_DATE: Build timestamp
