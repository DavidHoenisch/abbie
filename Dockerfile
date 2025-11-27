# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-s -w' -o abbie ./cmd/api

# Final stage - use minimal base image
FROM alpine:latest

# Add ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/abbie .

# Create a directory for configs
RUN mkdir -p /etc/abbie

# Expose port 8080
EXPOSE 8080

# Use ENTRYPOINT so CLI args can be passed
ENTRYPOINT ["/app/abbie"]

# Default arguments (can be overridden)
CMD ["-config", "/etc/abbie/config.yaml"]
