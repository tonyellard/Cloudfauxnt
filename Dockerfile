FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o cloudfauxnt .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests to origins
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cloudfauxnt .

# Create directories for config and keys
RUN mkdir -p /app/keys

# Expose port
EXPOSE 8080

# Run the application
CMD ["./cloudfauxnt", "--config", "/app/config.yaml"]
