# Multi-stage build for optimal image size

# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
# CGO_ENABLED=0 for static binary
# -ldflags for smaller binary and version info
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X 'github.com/mo-amir99/lms-server-go/pkg/health.Version=${VERSION}' \
    -X 'github.com/mo-amir99/lms-server-go/pkg/health.GitCommit=${GIT_COMMIT}' \
    -X 'github.com/mo-amir99/lms-server-go/pkg/health.BuildTime=${BUILD_TIME}'" \
    -o lms-server ./cmd/app

# Production stage
FROM alpine:latest

# Install ca-certificates for HTTPS and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/lms-server .

# Copy public files (if any)
COPY --chown=appuser:appuser public/ ./public/

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["./lms-server"]
