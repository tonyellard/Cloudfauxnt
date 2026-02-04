FROM golang:1.23-bookworm AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY *.go ./

# Copy license files to builder stage
COPY LICENSE NOTICE ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o cloudfauxnt .

# Final stage
FROM debian:bookworm-slim

# Install ca-certificates for HTTPS requests to origins
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cloudfauxnt .

# Copy license files
COPY --from=builder /build/LICENSE /build/NOTICE ./

# Create directories for config and keys
RUN mkdir -p /app/keys

# Expose port
EXPOSE 8080

# Run the application
CMD ["./cloudfauxnt", "--config", "/app/config.yaml"]
