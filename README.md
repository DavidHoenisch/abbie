# Abbie

A lightweight, high-performance A/B testing reverse proxy built in Go. Routes traffic between multiple backend services based on user segmentation. Specifically designed to aid marketing teams in A/B testing websites.

## Features

- **Zero Dependencies**: Pure Go stdlib implementation
- **Minimal Footprint**: Compiled to a static binary (~5MB)
- **Multi-Architecture**: Supports ARM64 and AMD64
- **Production Ready**: Proper error handling and logging
- **Distroless Base**: Built on Chainguard static image for minimal attack surface

## How It Works

Abbie acts as a reverse proxy that routes incoming HTTP requests to different backend services based on A/B test groups. Currently configured for Fly.io internal networking, but adaptable to any environment.

```
┌─────────┐
│ Client  │
└────┬────┘
     │
     ▼
┌────────────┐
│   Abbie    │  (Route based on group)
└─────┬──────┘
      │
      ├─────────► Landing Page A (Group A)
      │
      └─────────► Landing Page B (Group B)
```

## Quick Start

### Local Development

```bash
# Set the port (optional, defaults to 8080)
export ABBIE_PORT=8080

# Run directly
go run cmd/api/main.go
```

### Build with ko

```bash
# Build locally
ko build --local ./cmd/api

# Run the container
docker run -p 8080:8080 -e ABBIE_PORT=8080 ko.local/abbie:latest
```

### Build for Production

```bash
# Set your registry
export KO_DOCKER_REPO=ghcr.io/yourusername

# Build and push multi-arch images
ko build --platform=linux/amd64,linux/arm64 ./cmd/api
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ABBIE_PORT` | Port to listen on | `8080` |

### Backend Configuration

Edit `cmd/api/main.go` to configure your backend services:

```go
proxyA := newProxy("http://your-service-a:3000")
proxyB := newProxy("http://your-service-b:3000")
```

### A/B Group Logic

Implement your group assignment logic in the main handler at `cmd/api/main.go:68`. Examples:

- Cookie-based assignment
- Header-based routing
- IP-based segmentation
- Random distribution

## Project Structure

```
abbie/
├── cmd/
│   └── api/
│       └── main.go          # Application entry point
├── internal/
│   └── config/
│       └── settings.go      # Configuration management
├── .ko.yaml                 # ko build configuration
└── README.md
```

## Deployment

### Fly.io

```bash
# Deploy using ko
fly deploy --image $(ko build ./cmd/api)
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: abbie
spec:
  replicas: 3
  selector:
    matchLabels:
      app: abbie
  template:
    metadata:
      labels:
        app: abbie
    spec:
      containers:
      - name: abbie
        image: ko://github.com/DavidHoenisch/abbie/cmd/api
        ports:
        - containerPort: 8080
        env:
        - name: ABBIE_PORT
          value: "8080"
```

## Why ko?

This project uses [ko](https://ko.build/) for building container images:

- No Dockerfile needed
- Automatic multi-architecture builds
- Minimal base images (distroless)
- Fast, reproducible builds
- Built-in SBOM generation

## Performance

- **Binary Size**: ~5MB static binary
- **Image Size**: ~10MB total (with Chainguard static base)
- **Memory**: <10MB RSS under normal load
- **Latency**: <1ms routing overhead

## Security

- **Distroless Base**: No shell, no package manager, minimal attack surface
- **Static Binary**: No runtime dependencies
- **Non-root**: Runs as non-root user
- **SBOM**: Automatic software bill of materials generation

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.
