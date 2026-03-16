# ArgoCD Addons Platform — Rebuild Plan

## Overview

Full rebuild of the ArgoCD Addons Platform (AAP) with modern stack:
- **Backend**: Go (replacing Python/FastAPI)
- **Frontend**: React + shadcn/ui + Tailwind CSS (replacing React + MUI)
- **Storage**: Stateless — K8s Secrets/ConfigMaps for connections (replacing SQLite)
- **Git Provider**: GitHub support (replacing Azure DevOps-only), with Azure DevOps kept as a pluggable provider
- **Config Parsing**: Updated to match the new `argocd-cluster-addons` repo structure

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Single Binary                      │
│  ┌───────────────┐    ┌───────────────────────────┐ │
│  │  Static Files  │    │       Go HTTP Server       │ │
│  │  (React SPA)   │◄──│  chi/echo router           │ │
│  │  shadcn + TW   │    │  /api/v1/...              │ │
│  └───────────────┘    └───────────┬───────────────┘ │
│                                   │                  │
│         ┌─────────────────────────┼──────────┐      │
│         │                         │          │      │
│  ┌──────▼──────┐  ┌──────────▼──────┐ ┌────▼────┐ │
│  │  Git Provider│  │  ArgoCD Client  │ │  Config │ │
│  │  (interface) │  │  (REST/gRPC)    │ │  Store  │ │
│  │  ├─ GitHub   │  │                 │ │  (K8s)  │ │
│  │  └─ AzDevOps │  │  In-cluster:    │ │         │ │
│  │              │  │    ServiceAcct   │ │  or     │ │
│  │              │  │  External:       │ │  (File) │ │
│  │              │  │    Token/PAT     │ │         │ │
│  └──────────────┘  └─────────────────┘ └─────────┘ │
└─────────────────────────────────────────────────────┘
```

### Dual-mode connection support

| Mode | Git Auth | ArgoCD Auth | Config Storage | Use case |
|------|----------|-------------|----------------|----------|
| **Local/Dev** | PAT (env var or config file) | Token/PAT (env var or config file) | Local YAML file | minikube development |
| **In-cluster** | K8s Secret or GitHub App | ServiceAccount + RBAC | K8s Secrets/ConfigMaps | Production on same cluster as ArgoCD |

The app auto-detects: if running inside K8s (`KUBERNETES_SERVICE_HOST` env var exists), use in-cluster mode. Otherwise, local mode with config file.

---

## Phase 1: Go Backend Foundation

**Goal**: Working Go backend that serves the same API as the current Python backend, with GitHub support and new repo structure parsing.

### 1.1 Project Structure

```
/
├── cmd/
│   └── aap-server/
│       └── main.go                 # Entry point
├── internal/
│   ├── api/
│   │   ├── router.go               # chi/echo router setup, middleware
│   │   ├── clusters.go             # GET /api/v1/clusters, /api/v1/clusters/{name}, /api/v1/clusters/{name}/comparison
│   │   ├── addons.go               # GET /api/v1/addons/list, /api/v1/addons/catalog, /api/v1/addons/{name}
│   │   ├── dashboard.go            # GET /api/v1/dashboard/stats, /api/v1/dashboard/pull-requests
│   │   ├── connections.go          # CRUD /api/v1/connections/
│   │   └── health.go               # GET /api/v1/health
│   ├── models/
│   │   ├── cluster.go              # Cluster, ClusterHealthStats, ClusterComparisonResponse
│   │   ├── addon.go                # Addon, AddonCatalogItem, AddonComparisonStatus
│   │   ├── connection.go           # Connection, GitRepoConfig, ArgocdConfig
│   │   ├── dashboard.go            # DashboardStatisticsResponse, PR models
│   │   └── argocd.go               # ArgocdCluster, ArgocdApplication
│   ├── service/
│   │   ├── cluster.go              # Cluster business logic (list, detail, comparison)
│   │   ├── addon.go                # Addon catalog and detail logic
│   │   ├── dashboard.go            # Dashboard aggregation
│   │   └── connection.go           # Connection management
│   ├── gitprovider/
│   │   ├── provider.go             # Interface: GitProvider
│   │   ├── github.go               # GitHub implementation (REST API via go-github)
│   │   └── azuredevops.go          # Azure DevOps implementation (existing logic ported)
│   ├── argocd/
│   │   ├── client.go               # ArgoCD REST client (supports token + in-cluster)
│   │   └── service.go              # ArgoCD data transformation
│   ├── config/
│   │   ├── parser.go               # Parse cluster-addons.yaml, addons-catalog.yaml, cluster values
│   │   └── store.go                # Connection config store (K8s Secrets or local file)
│   └── platform/
│       └── detect.go               # Auto-detect local vs in-cluster mode
├── ui/                             # React frontend (Phase 2)
├── k8s/                            # Kubernetes manifests
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

### 1.2 Git Provider Interface

```go
type GitProvider interface {
    // Read repo content
    GetFileContent(ctx context.Context, path string) ([]byte, error)
    ListDirectory(ctx context.Context, path string) ([]string, error)

    // Pull Requests
    ListPullRequests(ctx context.Context, state string) ([]PullRequest, error)

    // Connection test
    TestConnection(ctx context.Context) error
}
```

Two implementations:
- `GitHubProvider` — uses `google/go-github` library, authenticates with PAT or GitHub App
- `AzureDevOpsProvider` — ports existing `azure_client.py` logic to Go

### 1.3 Config Parser (new repo structure)

The parser reads 3 files from the Git provider:

| File | Content |
|------|---------|
| `configuration/cluster-addons.yaml` | All clusters + labels (which addons enabled/disabled + version overrides) |
| `configuration/addons-catalog.yaml` | All addon definitions (appName, repoURL, chart, version, inMigration) |
| `configuration/addons-clusters-values/{cluster}.yaml` | Per-cluster value overrides (clusterGlobalValues + per-addon config) |

This replaces the old tree-walking logic that assumed per-cluster directories.

### 1.4 Connection Config Store

Two backends behind one interface:

```go
type ConfigStore interface {
    ListConnections(ctx context.Context) ([]Connection, error)
    GetConnection(ctx context.Context, name string) (*Connection, error)
    SaveConnection(ctx context.Context, conn Connection) error
    DeleteConnection(ctx context.Context, name string) error
    GetActiveConnection(ctx context.Context) (string, error)
    SetActiveConnection(ctx context.Context, name string) error
}
```

- `FileConfigStore` — reads/writes a local YAML file (for local dev / minikube)
- `K8sConfigStore` — reads/writes K8s Secrets and a ConfigMap (for in-cluster)

### 1.5 API Endpoints (same as current)

All existing endpoints preserved with identical response shapes:

| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/v1/clusters` | List clusters with health stats |
| GET | `/api/v1/clusters/{name}` | Cluster detail with addons |
| GET | `/api/v1/clusters/{name}/comparison` | Git vs ArgoCD comparison |
| GET | `/api/v1/addons/list` | List addons from Git |
| GET | `/api/v1/addons/catalog` | Addon catalog with deployment stats |
| GET | `/api/v1/addons/{name}` | Addon detail |
| GET | `/api/v1/connections/` | List connections |
| POST | `/api/v1/connections/` | Create connection |
| PUT | `/api/v1/connections/{name}` | Update connection |
| DELETE | `/api/v1/connections/{name}` | Delete connection |
| POST | `/api/v1/connections/test` | Test connectivity |
| POST | `/api/v1/connections/active` | Set active connection |
| GET | `/api/v1/dashboard/stats` | Dashboard statistics |
| GET | `/api/v1/dashboard/pull-requests` | Recent PRs |
| GET | `/api/v1/health` | Health check |

### 1.6 Testing

- Table-driven unit tests for each service (Go convention)
- Mock interfaces for GitProvider, ArgoCD client, ConfigStore
- Integration tests using httptest for API handlers
- Test data derived from current Python test expectations

---

## Phase 2: Frontend Rebuild (shadcn/ui + Tailwind)

**Goal**: Same views and functionality, modern stack.

### 2.1 Tech Stack

| Current | New |
|---------|-----|
| React 19 | React 19 (keep) |
| MUI 7 | shadcn/ui components |
| MUI theming | Tailwind CSS |
| Recharts | Recharts (keep — works fine with Tailwind) |
| React Router 7 | React Router 7 (keep) |
| Vite 7 | Vite 7 (keep) |

### 2.2 Migration Approach

Scaffold a fresh Vite + React + TypeScript + Tailwind + shadcn/ui project in `ui/`, then rebuild each view:

| View | Components | Notes |
|------|-----------|-------|
| **Dashboard** | Stats cards, pie charts, PR list | Recharts stays, cards become shadcn Card |
| **Clusters Overview** | Data table, filter bar, health KPIs | shadcn Table + Command for search |
| **Cluster Detail** | Comparison table, status badges | shadcn Table + Badge |
| **Addon Catalog** | Searchable list, expandable rows | shadcn Accordion + Table |
| **Addon Detail** | Deployment info, cluster list | shadcn Card + Table |
| **Manage Connections** | Form, list, test button | shadcn Form + Dialog |
| **Add/Edit Connection** | Form with validation | shadcn Form + Input + Select |
| **Settings** | (minimal) | shadcn Card |
| **Layout** | Sidebar nav, header, connection selector | shadcn Sidebar + DropdownMenu |

### 2.3 API Service Layer

`ui/src/services/apiService.ts` stays mostly the same — it's just fetch calls to the backend. The response types (TypeScript interfaces) match the Go API responses.

### 2.4 Testing

- Vitest + React Testing Library (same as typical shadcn/ui projects)
- Component tests for each view
- API mock with MSW (Mock Service Worker) for integration tests

---

## Phase 3: Deployment & Makefile

### 3.1 Dockerfile (multi-stage)

```dockerfile
# Stage 1: Build UI
FROM node:22-alpine AS ui-build
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.23-alpine AS go-build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o aap-server ./cmd/aap-server

# Stage 3: Final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=go-build /app/aap-server /usr/local/bin/
COPY --from=ui-build /app/ui/dist /app/static
EXPOSE 8080
ENTRYPOINT ["aap-server"]
```

Result: ~30-50MB image (vs current ~300MB+ with Python + Node).

### 3.2 Makefile Targets

Simplified, keeping the patterns you're used to:

```makefile
# Development
make dev              # Run Go backend + Vite dev server locally (hot reload)
make test             # Run all tests (Go + frontend)
make test-go          # Run Go tests only
make test-ui          # Run frontend tests only
make lint             # Lint everything (golangci-lint + eslint)

# Build
make build            # Build Docker image (auto-increment patch version)
make build-go         # Build Go binary locally

# Minikube
make deploy           # Build + deploy to minikube
make update           # Quick update (build + restart)
make status           # Show deployment status
make logs             # Show pod logs
make undeploy         # Remove deployment

# Secrets
make create-secrets   # Create K8s secrets from .env.secrets
make list-secrets     # List secrets in namespace
```

### 3.3 K8s Manifests

Same namespace (`argocd-addons-platform`), simplified:
- `k8s/namespace.yaml` — namespace
- `k8s/deployment.yaml` — single-pod deployment (no PVC needed)
- `k8s/service.yaml` — ClusterIP service
- `k8s/ingress.yaml` — ingress
- `k8s/rbac.yaml` — ServiceAccount + RBAC for in-cluster ArgoCD access (optional)

No `database-pvc.yaml` — stateless.

### 3.4 Local Dev Config

For minikube / local development, connections configured via `config.yaml`:

```yaml
# config.yaml (local dev — not committed, in .gitignore)
mode: local  # "local" or "kubernetes" (auto-detected if omitted)

connections:
  - name: dev
    default: true
    git:
      provider: github  # or "azuredevops"
      # GitHub fields
      owner: merck-ahtl
      repo: argocd-cluster-addons
      token: ${GITHUB_TOKEN}  # env var reference
      # Azure DevOps fields (if provider: azuredevops)
      # organization: https://dev.azure.com/AHITL
      # project: DevOps
      # repository: argocd-cluster-addons
      # pat: ${AZURE_DEVOPS_PAT}
    argocd:
      server_url: https://argocd.example.com
      token: ${ARGOCD_TOKEN}
      namespace: argocd
```

---

## Phase 4: Datadog Integration (future)

Not part of the initial rebuild, but designed for:

- **Datadog API client** in Go (`internal/datadog/client.go`)
- Fetch monitor status, alert counts per cluster/addon
- Display alongside ArgoCD health in the cluster detail view
- Potentially embed Datadog dashboard widgets via iframe or Graph Snapshot API

---

## Phase 5: AI/LLM Integration (future)

- Log analysis: send ArgoCD sync failure logs to an LLM for troubleshooting suggestions
- Natural language queries: "which clusters have out-of-sync addons?"
- Could use Claude API or any LLM endpoint

---

## Execution Order

| Step | What | Depends on | Estimated scope |
|------|------|------------|-----------------|
| 1 | Go project scaffold + models + config parser | — | Foundation |
| 2 | Git provider interface + GitHub implementation | Step 1 | Core integration |
| 3 | ArgoCD client (REST, dual-mode auth) | Step 1 | Core integration |
| 4 | Connection config store (file-based for dev) | Step 1 | Storage |
| 5 | Service layer (clusters, addons, dashboard) | Steps 2-4 | Business logic |
| 6 | API handlers + router + middleware | Step 5 | HTTP layer |
| 7 | Go tests (unit + integration) | Steps 1-6 | Quality |
| 8 | Frontend scaffold (Vite + shadcn/ui + Tailwind) | — (parallel with Go) | UI foundation |
| 9 | Frontend views (Dashboard, Clusters, Addons, Connections) | Step 8 | UI features |
| 10 | Frontend tests | Step 9 | Quality |
| 11 | Dockerfile + Makefile + K8s manifests | Steps 7, 10 | Deployment |
| 12 | End-to-end testing on minikube | Step 11 | Validation |

Steps 1-7 (Go backend) and 8-10 (frontend) can run **in parallel**.

---

## What We're Keeping

- Same API endpoint paths and response shapes (frontend doesn't break during migration)
- Same views and user experience
- Same namespace and deployment pattern (single binary, minikube-first)
- Same Makefile-driven workflow
- Recharts for data visualization
- React Router for navigation

## What We're Dropping

- Python/FastAPI backend
- SQLite database + PVC
- MUI component library
- Azure DevOps as sole Git provider
- Hardcoded default connection with PAT in source code
- Old repo structure assumptions (directory-per-cluster)

## What We're Adding

- Go backend (single static binary)
- GitHub Git provider
- shadcn/ui + Tailwind CSS
- Stateless config (K8s Secrets or local YAML file)
- In-cluster auto-discovery mode
- Dual-mode auth (PAT for dev, ServiceAccount for production)
- New config parser for current repo structure
