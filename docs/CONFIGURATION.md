# Configuration Reference

This document covers all configuration options for the ArgoCD Addons Platform (AAP).

---

## Table of Contents

- [Helm Values Reference](#helm-values-reference)
- [Environment Variables](#environment-variables)
- [Connection Configuration](#connection-configuration)
- [Secrets Management](#secrets-management)
- [Production Deployment Guide](#production-deployment-guide)
- [Local Development Setup](#local-development-setup)

---

## Helm Values Reference

All values are defined in `charts/argocd-addons-platform/values.yaml`.

### Image

| Value | Default | Description |
|-------|---------|-------------|
| `replicaCount` | `1` | Number of pod replicas |
| `image.repository` | `ghcr.io/moranweissman/argocd-addons-platform` | Container image repository |
| `image.tag` | `""` (uses Chart `appVersion`) | Image tag override |
| `image.pullPolicy` | `IfNotPresent` | Kubernetes image pull policy |
| `imagePullSecrets` | `[]` | Registry pull secret references |
| `nameOverride` | `""` | Override the chart name |
| `fullnameOverride` | `""` | Override the full release name |

### Service Account and RBAC

| Value | Default | Description |
|-------|---------|-------------|
| `serviceAccount.create` | `true` | Create a ServiceAccount for the pod |
| `serviceAccount.name` | `""` | Override ServiceAccount name (auto-generated if empty) |
| `serviceAccount.annotations` | `{}` | Annotations on the ServiceAccount (e.g., for IAM roles) |
| `rbac.create` | `true` | Create RBAC roles for ArgoCD read access |
| `rbac.argocdNamespace` | `argocd` | Namespace where ArgoCD is installed |

### Connections

| Value | Default | Description |
|-------|---------|-------------|
| `config.connections` | (see below) | List of Git + ArgoCD connection pairs |
| `config.connections[].name` | `""` | Unique name for this connection |
| `config.connections[].default` | `false` | Whether this is the default active connection |
| `config.connections[].git.provider` | `github` | Git provider: `github` or `azuredevops` |
| `config.connections[].git.owner` | `""` | GitHub organization or user |
| `config.connections[].git.repo` | `""` | Repository name containing addon definitions |
| `config.connections[].git.token` | `${GITHUB_TOKEN}` | Git PAT (use env var reference for secret injection) |
| `config.connections[].argocd.server_url` | `""` | ArgoCD server URL. Leave empty for in-cluster auto-discovery via K8s DNS |
| `config.connections[].argocd.token` | `""` | ArgoCD API bearer token. Not needed if RBAC is enabled for in-cluster mode |
| `config.connections[].argocd.namespace` | `argocd` | ArgoCD installation namespace |
| `config.connections[].argocd.insecure` | `true` | Skip TLS verification (acceptable for in-cluster traffic) |

### Secrets

| Value | Default | Description |
|-------|---------|-------------|
| `existingSecret` | `""` | Name of a pre-existing K8s Secret. If set, the chart does not create one |
| `secrets.GITHUB_TOKEN` | `""` | GitHub PAT for Git provider access |
| `secrets.ARGOCD_TOKEN` | `""` | ArgoCD API bearer token (not needed in in-cluster mode with RBAC) |

### Authentication

Authentication is managed via Kubernetes-native resources. On first install, a random admin password is generated.

Retrieve the initial password:
```bash
kubectl get secret <release-name> -n <namespace> \
  -o jsonpath='{.data.admin\.initialPassword}' | base64 -d
```

For local development (outside K8s), use environment variables instead:

| Env Var | Description |
|---------|-------------|
| `AAP_AUTH_USER` | Username for local auth |
| `AAP_AUTH_PASSWORD` | Password (plaintext or bcrypt hash) for local auth |

### AI Provider

| Value | Default | Description |
|-------|---------|-------------|
| `ai.enabled` | `false` | Enable the AI assistant |
| `ai.provider` | `""` | Provider: `ollama`, `openai`, or `gemini` |
| `ai.apiKey` | `""` | API key for cloud providers (stored in K8s Secret) |
| `ai.cloudModel` | `""` | Model name (e.g., `gpt-4o`, `gemini-2.5-flash`) |

#### Ollama Settings

| Value | Default | Description |
|-------|---------|-------------|
| `ai.ollama.deploy` | `false` | Deploy an Ollama pod alongside AAP |
| `ai.ollama.image` | `ollama/ollama:latest` | Ollama container image (~1.2 GB compressed, ~3 GB on disk) |
| `ai.ollama.url` | `""` | Ollama URL (auto-set when `deploy=true`; override for external Ollama) |
| `ai.ollama.model` | `llama3.2` | Model for simple queries |
| `ai.ollama.agentModel` | `""` | Separate model for tool-calling agent (defaults to `model` if empty) |
| `ai.ollama.gpu` | `false` | Enable GPU support (requires NVIDIA device plugin) |
| `ai.ollama.persistence` | `false` | Persist downloaded models across restarts (strongly recommended) |
| `ai.ollama.storageClassName` | `""` | Storage class for persistence (empty = cluster default) |
| `ai.ollama.storageSize` | `10Gi` | PVC size (10 Gi fits 1-2 small models; 50 Gi+ for larger ones) |
| `ai.ollama.resources.requests.memory` | `2Gi` | Memory request for Ollama pod |
| `ai.ollama.resources.requests.cpu` | `500m` | CPU request for Ollama pod |
| `ai.ollama.resources.limits.memory` | `4Gi` | Memory limit for Ollama pod |
| `ai.ollama.resources.limits.cpu` | `2000m` | CPU limit for Ollama pod |

### Datadog

| Value | Default | Description |
|-------|---------|-------------|
| `datadog.enabled` | `false` | Enable Datadog metrics integration |
| `datadog.apiKey` | `""` | Datadog API key (stored in K8s Secret) |
| `datadog.appKey` | `""` | Datadog application key (stored in K8s Secret) |
| `datadog.site` | `datadoghq.com` | Datadog site (`datadoghq.com` or `datadoghq.eu`) |

### Service

| Value | Default | Description |
|-------|---------|-------------|
| `service.type` | `ClusterIP` | Kubernetes service type (`ClusterIP`, `LoadBalancer`, `NodePort`) |
| `service.port` | `80` | Service port |

### Ingress

| Value | Default | Description |
|-------|---------|-------------|
| `ingress.enabled` | `false` | Enable Kubernetes Ingress |
| `ingress.className` | `""` | Ingress class (e.g., `alb`, `nginx`) |
| `ingress.annotations` | `{}` | Ingress annotations |
| `ingress.hosts` | `[{host: aap.example.com, paths: [{path: /, pathType: Prefix}]}]` | Host and path rules |
| `ingress.tls` | `[]` | TLS configuration |

### Resources

| Value | Default | Description |
|-------|---------|-------------|
| `resources.requests.memory` | `128Mi` | Memory request |
| `resources.requests.cpu` | `100m` | CPU request |
| `resources.limits.memory` | `512Mi` | Memory limit |
| `resources.limits.cpu` | `500m` | CPU limit |

### Pod Security

| Value | Default | Description |
|-------|---------|-------------|
| `podSecurityContext.runAsNonRoot` | `true` | Enforce non-root execution |
| `podSecurityContext.runAsUser` | `1001` | UID for the pod |
| `podSecurityContext.runAsGroup` | `1001` | GID for the pod |
| `podSecurityContext.fsGroup` | `1001` | Filesystem group |
| `securityContext.allowPrivilegeEscalation` | `false` | Block privilege escalation |
| `securityContext.readOnlyRootFilesystem` | `true` | Read-only root filesystem |
| `securityContext.capabilities.drop` | `[ALL]` | Drop all Linux capabilities |

### Health Probes

| Value | Default | Description |
|-------|---------|-------------|
| `livenessProbe.httpGet.path` | `/api/v1/health` | Liveness probe endpoint |
| `livenessProbe.initialDelaySeconds` | `5` | Delay before first probe |
| `livenessProbe.periodSeconds` | `10` | Probe interval |
| `readinessProbe.httpGet.path` | `/api/v1/health` | Readiness probe endpoint |
| `readinessProbe.initialDelaySeconds` | `3` | Delay before first probe |
| `readinessProbe.periodSeconds` | `5` | Probe interval |

### Scheduling

| Value | Default | Description |
|-------|---------|-------------|
| `nodeSelector` | `{}` | Node selector labels |
| `tolerations` | `[]` | Pod tolerations |
| `affinity` | `{}` | Pod affinity rules |

### Extra Environment Variables

| Value | Default | Description |
|-------|---------|-------------|
| `extraEnv` | `[]` | Additional env vars as `name`/`value` pairs |

Example:
```yaml
extraEnv:
  - name: AAP_CORS_ORIGIN
    value: "https://aap.example.com"
```

---

## Environment Variables

These environment variables are used by the AAP server at runtime.

| Variable | Required | Description |
|----------|----------|-------------|
| `GITHUB_TOKEN` | Yes | GitHub personal access token for Git provider |
| `ARGOCD_TOKEN` | No | ArgoCD API bearer token (not needed with in-cluster RBAC) |
| `AAP_AUTH_USER` | No | Username for local development auth (outside K8s) |
| `AAP_AUTH_PASSWORD` | No | Password for local development auth (outside K8s) |
| `AAP_CORS_ORIGIN` | No | CORS allowed origin. Empty = same-origin; `*` = all (dev only) |
| `AAP_NAMESPACE` | No | K8s namespace for ConfigMap/Secret auth discovery |
| `AAP_STATIC_DIR` | No | Path to static files directory (default: `/app/static`) |
| `AAP_PORT` | No | Server listen port (default: `8080`) |
| `AI_API_KEY` | No | API key for cloud AI provider |
| `AI_PROVIDER` | No | AI provider override (`ollama`, `openai`, `gemini`) |
| `AI_CLOUD_MODEL` | No | Cloud model name override |
| `DATADOG_API_KEY` | No | Datadog API key |
| `DATADOG_APP_KEY` | No | Datadog application key |

---

## Connection Configuration

AAP uses a `config.yaml` file to define connections to Git providers and ArgoCD servers. Environment variables like `${GITHUB_TOKEN}` are resolved at runtime.

### Example: GitHub + In-Cluster ArgoCD

```yaml
connections:
  - name: production
    default: true
    git:
      provider: github
      owner: your-org
      repo: argocd-cluster-addons
      token: ${GITHUB_TOKEN}
    argocd:
      server_url: ""  # Empty = auto-discover via K8s DNS
      token: ""       # Empty = use RBAC ServiceAccount
      namespace: argocd
      insecure: true
```

### Example: GitHub + External ArgoCD

```yaml
connections:
  - name: production
    default: true
    git:
      provider: github
      owner: your-org
      repo: argocd-cluster-addons
      token: ${GITHUB_TOKEN}
    argocd:
      server_url: "https://argocd.example.com"
      token: ${ARGOCD_TOKEN}
      namespace: argocd
      insecure: false
```

### Example: Azure DevOps (Legacy)

```yaml
connections:
  - name: azure-legacy
    git:
      provider: azuredevops
      organization: https://dev.azure.com/YourOrg
      project: DevOps
      repository: argocd-cluster-addons
      pat: ${AZURE_DEVOPS_PAT}
    argocd:
      server_url: "https://argocd.example.com"
      token: ${ARGOCD_TOKEN}
      namespace: argocd
```

### Multiple Connections

You can define multiple connections and switch between them in the UI:

```yaml
connections:
  - name: dev
    default: true
    git:
      provider: github
      owner: your-org
      repo: argocd-cluster-addons-dev
      token: ${GITHUB_TOKEN}
    argocd:
      server_url: ""
      namespace: argocd
      insecure: true

  - name: prod
    git:
      provider: github
      owner: your-org
      repo: argocd-cluster-addons-prod
      token: ${GITHUB_TOKEN}
    argocd:
      server_url: "https://argocd-prod.example.com"
      token: ${ARGOCD_PROD_TOKEN}
      namespace: argocd
```

---

## Secrets Management

### What Goes in the Kubernetes Secret

The Helm chart creates a Secret containing:

| Key | Source | Description |
|-----|--------|-------------|
| `admin.password` | Auto-generated | Bcrypt hash of admin password |
| `admin.initialPassword` | Auto-generated | Plaintext initial password (removed after first change) |
| `GITHUB_TOKEN` | `secrets.GITHUB_TOKEN` or `existingSecret` | GitHub PAT |
| `ARGOCD_TOKEN` | `secrets.ARGOCD_TOKEN` or `existingSecret` | ArgoCD API token |
| `AI_API_KEY` | `ai.apiKey` or `existingSecret` | Cloud AI provider API key |
| `DATADOG_API_KEY` | `datadog.apiKey` or `existingSecret` | Datadog API key |
| `DATADOG_APP_KEY` | `datadog.appKey` or `existingSecret` | Datadog application key |

### Using an Existing Secret

For production, it is recommended to manage secrets externally (e.g., via Sealed Secrets, External Secrets Operator, or Vault):

```yaml
existingSecret: "my-aap-secrets"
```

The existing Secret must contain the same keys listed above.

### Secrets File for the Install Script

The `scripts/helm-install.sh` script reads secrets from a `.env.secrets` file:

```bash
# .env.secrets (never commit this file)
GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
ARGOCD_TOKEN=eyJhbGciOi...
AI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxx
AI_PROVIDER=gemini
AI_CLOUD_MODEL=gemini-2.5-flash
DATADOG_API_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
DATADOG_APP_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

---

## Production Deployment Guide

### Step 1: Prepare Secrets

Create a `.env.secrets` file with your credentials (see above). Never commit this file.

### Step 2: Configure Values

Create or edit `charts/argocd-addons-platform/values-production.yaml`:

```yaml
replicaCount: 1

image:
  repository: ghcr.io/your-org/argocd-addons-platform
  tag: "1.10.7"
  pullPolicy: Always

config:
  connections:
    - name: production
      default: true
      git:
        provider: github
        owner: your-org
        repo: argocd-cluster-addons
        token: ${GITHUB_TOKEN}
      argocd:
        server_url: ""
        namespace: argocd
        insecure: true

rbac:
  create: true
  argocdNamespace: argocd

ai:
  enabled: true
  provider: gemini
  cloudModel: gemini-2.5-flash

ingress:
  enabled: true
  className: alb
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTP":80},{"HTTPS":443}]'
    alb.ingress.kubernetes.io/ssl-redirect: "443"
  hosts:
    - host: aap.example.com
      paths:
        - path: /
          pathType: Prefix
```

### Step 3: Deploy

Using the install script:
```bash
./scripts/helm-install.sh .env.secrets
```

Or manually with Helm:
```bash
helm upgrade --install aap charts/argocd-addons-platform \
  --namespace argocd-addons-platform \
  --create-namespace \
  -f charts/argocd-addons-platform/values-production.yaml \
  --set secrets.GITHUB_TOKEN=$GITHUB_TOKEN \
  --set ai.apiKey=$AI_API_KEY
```

### Step 4: Verify

```bash
kubectl -n argocd-addons-platform get pods
kubectl -n argocd-addons-platform logs -f deploy/aap-argocd-addons-platform
```

### Step 5: Retrieve Admin Password

```bash
kubectl get secret aap -n argocd-addons-platform \
  -o jsonpath='{.data.admin\.initialPassword}' | base64 -d
```

---

## Local Development Setup

### Prerequisites

- Go 1.25+
- Node.js 22+
- A `config.yaml` file (copy from `config.yaml.example`)
- A `.env.secrets` file with at least `GITHUB_TOKEN`

### Running Locally

1. Copy the example config:
   ```bash
   cp config.yaml.example config.yaml
   # Edit config.yaml with your org/repo and ArgoCD details
   ```

2. Create `.env.secrets`:
   ```bash
   GITHUB_TOKEN=ghp_your_token_here
   ```

3. Start the development servers (Go backend + Vite dev server):
   ```bash
   make dev
   ```

   This runs the Go backend on port 8080 and the Vite dev server on port 5173 with hot-reload.

4. Open http://localhost:5173 in your browser.

### Alternative: Build and Run

```bash
# Build the Go binary
make build-go

# Build the UI
cd ui && npm ci --legacy-peer-deps && npm run build && cd ..

# Run the server with static files
make run
```

Open http://localhost:8080.

### Running Tests

```bash
# All tests (Go + UI)
make test

# Go tests only
make test-go

# Linting (Go + ESLint)
make lint

# Test coverage
make test-coverage
```
