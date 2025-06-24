# Multi-stage build for TA Watcher
# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o bin/ta-watcher \
    ./cmd/watcher

# Runtime stage
FROM scratch

# Copy ca-certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/bin/ta-watcher /ta-watcher

# Copy config files
COPY --from=builder /app/config.example.yaml /config.yaml

# Expose port (if needed for health checks or metrics)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/ta-watcher"]

# Default command arguments
CMD ["-config", "/config.yaml"]
