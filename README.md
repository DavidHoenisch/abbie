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

## Installation

### Download Pre-built Binary

Download the latest release for your platform from [GitHub Releases](https://github.com/DavidHoenisch/abbie/releases):

```bash
# Example for Linux AMD64
wget https://github.com/DavidHoenisch/abbie/releases/latest/download/abbie_Linux_x86_64.tar.gz
tar -xzf abbie_Linux_x86_64.tar.gz
chmod +x abbie

# Run it
./abbie -config config.yaml
```

### Docker

```bash
# Pull from GitHub Container Registry
docker pull ghcr.io/davidhoenisch/abbie:latest

# Run with your config
docker run -d -p 8080:8080 \
  -v $(pwd)/config.yaml:/etc/abbie/config.yaml \
  ghcr.io/davidhoenisch/abbie:latest
```

### Build from Source

```bash
git clone https://github.com/DavidHoenisch/abbie.git
cd abbie
go build ./cmd/api
./api -config config.yaml
```

## Quick Start

### Local Development

```bash
# Run with a config file (required)
go run cmd/api/main.go -config config.yaml

# Or specify custom config and port
go run cmd/api/main.go -config config.local.yaml -port 9090

# Or use environment variables
ABBIE_CONFIG=config.yaml ABBIE_PORT=8080 go run cmd/api/main.go

# Show help
go run cmd/api/main.go -h
```

### Build with Docker

```bash
# Build the image
docker build -t abbie:latest .

# Run with default config location
docker run -d -p 8080:8080 \
  -v $(pwd)/config.yaml:/etc/abbie/config.yaml \
  abbie:latest

# Run with custom config path and port
docker run -d -p 9000:9000 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  abbie:latest -config /app/config.yaml -port 9000

# Run for local development
docker run -d -p 8080:8080 \
  -v $(pwd)/config.local.yaml:/etc/abbie/config.yaml \
  abbie:latest
```

### Docker Compose (Easiest)

```bash
# Run production setup
docker-compose up -d

# Run development setup
docker-compose --profile dev up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### Build with ko (Alternative)

```bash
# Build locally with ko
ko build --local ./cmd/api

# Note: When using ko, you'll need to mount config at runtime
docker run -p 8080:8080 \
  -v $(pwd)/config.yaml:/etc/abbie/config.yaml \
  ko.local/github.com/davidhoenisch/abbie/cmd/api:latest \
  -config /etc/abbie/config.yaml
```

## Configuration

Abbie requires a YAML configuration file for all routing and backend settings. The config can be provided in multiple ways:

1. **CLI flag**: `-config /path/to/config.yaml` (recommended)
2. **Environment variable**: `ABBIE_CONFIG=/path/to/config.yaml`
3. **Default**: Looks for `config.yaml` in the current directory

### Configuration File

Create a `config.yaml` file in your project root (see `config.example.yaml` for examples):

```yaml
app:
  port: "8080"

backends:
  - name: defense-backend
    host: landing-page-a.internal
    port: 3000
    groups:
      - defense
      - government

  - name: healthcare-backend
    host: landing-page-b.internal
    port: 3000
    groups:
      - healthcare
      - medical

routing:
  strategy: query-param      # round-robin, query-param, header, cookie, static
  param_name: audience       # query param/header/cookie name to check
  default_group: defense     # fallback group when no match
```

### CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to config file | `config.yaml` |
| `-port` | Port to listen on (overrides config file) | Uses config file value |
| `-h` | Show help | - |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ABBIE_CONFIG` | Path to config file (overridden by `-config` flag) | `config.yaml` |
| `ABBIE_PORT` | Port to listen on (overridden by `-port` flag) | `8080` |

### Routing Strategies

**Round-Robin**: Distributes requests evenly across all backends
```yaml
routing:
  strategy: round-robin
```

**Query Parameter**: Routes based on query parameter (e.g., `?audience=defense`)
```yaml
routing:
  strategy: query-param
  param_name: audience
  default_group: defense
```

**Header**: Routes based on request header (e.g., `X-Customer-Type: enterprise`)
```yaml
routing:
  strategy: header
  param_name: X-Customer-Type
  default_group: standard
```

**Cookie**: Routes based on cookie value (e.g., A/B testing)
```yaml
routing:
  strategy: cookie
  param_name: ab_test_group
  default_group: A
```

**Static**: Always routes to the first configured backend
```yaml
routing:
  strategy: static
```

### Backend Groups

Backends can belong to multiple groups:

```yaml
backends:
  - name: my-backend
    host: example.com
    port: 3000
    groups:
      - defense
      - government
      - premium
```

When a request comes in with `?audience=defense`, it will be routed to any backend that has `defense` in its groups.

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

### Docker Deployment

**Using the Dockerfile (recommended):**

```bash
# Build and tag
docker build -t abbie:v1.0 .

# Run with mounted config (default location)
docker run -d -p 8080:8080 \
  -v $(pwd)/config.yaml:/etc/abbie/config.yaml \
  --name abbie \
  abbie:v1.0

# Run with custom config path and port override
docker run -d -p 9000:9000 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  --name abbie \
  abbie:v1.0 -config /app/config.yaml -port 9000

# View logs
docker logs -f abbie
```

**Using environment variables:**

```bash
docker run -d -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -e ABBIE_CONFIG=/app/config.yaml \
  -e ABBIE_PORT=8080 \
  abbie:v1.0
```

### Fly.io

```bash
# Deploy using ko
fly deploy --image $(ko build ./cmd/api)
```

Create a `fly.toml` with your config:
```toml
[env]
  ABBIE_CONFIG = "/app/config.yaml"

[files]
  "config.yaml" = "/app/config.yaml"
```

### Kubernetes

**With ConfigMap for config file:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: abbie-config
data:
  config.yaml: |
    app:
      port: "8080"
    backends:
      - name: backend-1
        host: service-1.default.svc.cluster.local
        port: 8080
        groups:
          - default
    routing:
      strategy: round-robin
---
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
        - name: ABBIE_CONFIG
          value: "/etc/abbie/config.yaml"
        volumeMounts:
        - name: config
          mountPath: /etc/abbie
      volumes:
      - name: config
        configMap:
          name: abbie-config
```

**Or use embedded default config (no ConfigMap needed):**
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

## Releases

### Creating a New Release

Abbie uses [GoReleaser](https://goreleaser.com/) with GitHub Actions for automated releases.

**To create a new release:**

```bash
# Tag your commit with semantic versioning
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

This will automatically:
- Build binaries for Linux, macOS, and Windows (AMD64, ARM64, ARM)
- Generate changelog from commit messages
- Create a GitHub Release with all artifacts

**Available artifacts:**
- Pre-compiled binaries (tar.gz/zip)
- Checksums (SHA256)

### CI/CD

The project includes two GitHub Actions workflows:

**CI Workflow** (`.github/workflows/ci.yml`)
- Runs on every push and PR
- Executes tests, linting, and builds
- Ensures code quality

**Release Workflow** (`.github/workflows/release.yml`)
- Triggers on tag push (v*)
- Builds and publishes release artifacts
- Pushes Docker images to GHCR

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.
