# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o webhook ./cmd/webhook

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 webhook && \
    adduser -u 1000 -G webhook -s /bin/sh -D webhook

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/webhook /app/webhook

# Set ownership
RUN chown -R webhook:webhook /app

# Switch to non-root user
USER webhook

# Expose webhook port
EXPOSE 8443

# Run the webhook
ENTRYPOINT ["/app/webhook"]
