# Contributing to ArgoCD Addons Platform

Thank you for your interest in contributing to AAP. This guide covers how to report issues, set up a development environment, and submit changes.

---

## Reporting Issues

- Search [existing issues](../../issues) before opening a new one
- Include steps to reproduce, expected behavior, and actual behavior
- For bugs, include your AAP version (`cat VERSION`), Go version, and Kubernetes version
- For feature requests, describe the use case and why it would be valuable

---

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Backend |
| Node.js | 22+ | Frontend build |
| npm | 10+ | Package management |
| Docker | 20+ | Container builds |
| Helm | 3.x | Kubernetes deployment |
| kubectl | 1.28+ | Cluster access (optional for local dev) |

### Clone and Install

```bash
git clone https://github.com/moran/argocd-addons-platform.git
cd argocd-addons-platform

# Install frontend dependencies
cd ui && npm ci --legacy-peer-deps && cd ..

# Verify Go modules
go mod download
```

### Configuration

```bash
# Copy the example config
cp config.yaml.example config.yaml

# Create a secrets file (never committed)
cat > .env.secrets << 'EOF'
GITHUB_TOKEN=ghp_your_token_here
EOF
```

Edit `config.yaml` with your GitHub org/repo and ArgoCD server details.

---

## Project Structure

```
argocd-addons-platform/
  cmd/
    aap-server/           # Entry point (main.go)
  internal/
    api/                  # HTTP handlers and router (45+ endpoints)
    service/              # Business logic layer
    ai/                   # AI client, agent, tool definitions, tool executor
    argocd/               # ArgoCD REST client
    config/               # YAML config parser
    datadog/              # Datadog metrics client
    gitprovider/          # GitHub / Azure DevOps abstraction
    helm/                 # Helm chart fetcher and diff
    models/               # Shared types
    platform/             # K8s vs local detection
  ui/
    src/
      views/              # Page components (Dashboard, Clusters, Addons, etc.)
      components/         # Shared UI components
      hooks/              # React hooks (auth, connections, theme)
  charts/
    argocd-addons-platform/  # Helm chart
  scripts/                # Deployment and utility scripts
  docs/                   # Documentation
  k8s/                    # Raw Kubernetes manifests (for minikube)
```

---

## Running Locally

### Development Mode (Recommended)

```bash
make dev
```

This starts the Go backend on port 8080 and the Vite dev server on port 5173 with hot module replacement. Open http://localhost:5173.

### Build and Run

```bash
make build-go
cd ui && npm run build && cd ..
make run
```

Open http://localhost:8080.

---

## Running Tests

### Go Tests

```bash
make test-go
```

### All Tests (Go + Frontend)

```bash
make test
```

### Test Coverage

```bash
make test-coverage
```

### Linting

```bash
make lint
```

This runs `golangci-lint` for Go and ESLint for TypeScript.

---

## Code Style

### Go

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `golangci-lint` for additional checks
- Keep packages focused: handlers in `internal/api`, business logic in `internal/service`, integrations in their own packages
- Error handling: return errors up the call chain; log at the handler level
- Use `log/slog` for structured logging

### TypeScript / React

- TypeScript strict mode
- Functional components with hooks
- Tailwind CSS for styling
- ESLint for linting (`cd ui && npm run lint`)
- Component files in `ui/src/views/` for pages, `ui/src/components/` for shared components

---

## Pull Request Process

1. Fork the repository and create a feature branch from `main`
2. Make your changes, following the code style guidelines above
3. Add or update tests for any new functionality
4. Run `make test` and `make lint` to verify everything passes
5. Write a clear PR description explaining what changed and why
6. Link any related issues

### PR Guidelines

- Keep PRs focused on a single change
- Avoid mixing refactoring with feature work
- Update documentation if you change configuration options or add features
- Do not include secrets, credentials, or environment-specific values in commits

---

## Code of Conduct

We are committed to providing a welcoming and inclusive experience for everyone. Please be respectful and constructive in all interactions.
