# ArgoCD Addons Platform (AAP)

Centralized visibility and AI-powered insights for Kubernetes addon deployments managed by ArgoCD.

![CI](https://img.shields.io/badge/build-passing-brightgreen)
![License](https://img.shields.io/badge/license-AGPL--3.0-blue)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8)
![React](https://img.shields.io/badge/React-19-61DAFB)

---

<!-- TODO: Add screenshots -->

## Companion Project: ArgoCD Cluster Addons

AAP is designed as the **visibility and intelligence layer** for [ArgoCD Cluster Addons](https://github.com/MoranWeissman/argocd-cluster-addons) — a GitOps solution for managing Kubernetes addons (Datadog, Istio, Kyverno, External Secrets, etc.) across multiple clusters using ArgoCD ApplicationSets.

| Repository | Purpose |
|-----------|---------|
| [argocd-cluster-addons](https://github.com/MoranWeissman/argocd-cluster-addons) | The **engine** — GitOps config that ArgoCD watches to deploy addons |
| [argocd-addons-platform](https://github.com/MoranWeissman/argocd-addons-platform) (this repo) | The **dashboard** — read-only visibility, AI assistant, observability |

AAP reads the same Git configuration and queries the same ArgoCD instance. It never modifies deployments — all changes go through Git PRs to the cluster-addons repo.

## Features

### Dashboard and Overview
- Real-time statistics: cluster count, addon count, health status breakdown
- Aggregated health status with visual indicators
- Open pull request tracking from your Git provider
- Dark mode support

### Cluster Management
- Cluster overview with health, sync status, and addon counts
- Cluster detail view with per-addon comparison
- Configuration diff viewer between clusters
- Per-cluster value overrides inspection

### Addon Catalog
- Full addon catalog with search and filtering
- Addon detail view showing per-cluster deployment status
- Version matrix: clusters vs. addons with deployed versions at a glance
- Global default and per-cluster override configuration viewer

### Observability
- Health grouping by addon and by cluster
- Sync activity timeline across all ArgoCD applications
- Datadog metrics integration (CPU, memory, network per namespace/cluster)

### AI Assistant
- Natural language queries about your clusters and addons
- Agentic tool calling with 24 platform-aware tools
- Web search for external documentation, CVEs, and best practices
- Persistent memory across conversations
- Support for local (Ollama) and cloud (OpenAI, Gemini) providers

### Upgrade Checker
- Helm chart version comparison against upstream registries
- Values.yaml diff between current and target versions
- Release notes fetching from GitHub
- AI-powered upgrade impact analysis

### Security
- Kubernetes-native authentication (ConfigMap + Secret, bcrypt hashed)
- Rate-limited login endpoint (10 attempts/IP/minute)
- Session tokens with 24-hour expiry
- Read-only platform: no write access to any ArgoCD or cluster resources
- Non-root container with read-only filesystem
- See [docs/SECURITY.md](docs/SECURITY.md) for full details

---

## Quick Start

### Prerequisites
- Kubernetes cluster with ArgoCD installed
- Helm 3.x
- A GitHub personal access token (PAT) with repo read access

### Install with Helm

```bash
helm upgrade --install aap charts/argocd-addons-platform \
  --namespace argocd-addons-platform \
  --create-namespace \
  --set config.connections[0].name=default \
  --set config.connections[0].default=true \
  --set config.connections[0].git.provider=github \
  --set config.connections[0].git.owner=your-org \
  --set config.connections[0].git.repo=argocd-cluster-addons \
  --set secrets.GITHUB_TOKEN=ghp_your_token_here
```

### Access the UI

```bash
kubectl port-forward svc/aap-argocd-addons-platform 8080:80 -n argocd-addons-platform
```

Open http://localhost:8080 in your browser. On first install, retrieve the auto-generated admin password:

```bash
kubectl get secret aap -n argocd-addons-platform \
  -o jsonpath='{.data.admin\.initialPassword}' | base64 -d
```

---

## Architecture

AAP is a stateless Go backend serving a React single-page application. It connects to your Git provider and ArgoCD server at runtime and presents a unified view of all addon deployments.

```
React SPA  -->  Go API (/api/v1/*)  -->  GitHub API
                                     -->  ArgoCD REST API
                                     -->  Datadog API (optional)
                                     -->  AI Provider (optional)
                                     -->  Helm Registries
```

The backend is a single binary (~20 MB Docker image on Alpine) with no database. All state comes from Git and ArgoCD.

For detailed architecture diagrams (Mermaid), package layout, API endpoint map, and data flow, see [docs/architecture.md](docs/architecture.md).

---

## Configuration

AAP is configured through Helm values and a connections config file. The Helm chart manages all Kubernetes resources including RBAC, secrets, and optional Ollama sidecar deployment.

For the complete configuration reference, see [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

Key configuration areas:
- **Connections**: Git provider + ArgoCD server credentials
- **Authentication**: K8s-native user management or env-var fallback
- **AI Provider**: Ollama (local), OpenAI, Gemini (cloud)
- **Datadog**: Optional metrics integration
- **Ingress**: ALB, nginx, or port-forward

---

## AI Providers

AAP includes an AI assistant that can answer natural language questions about your clusters and addons using agentic tool calling.

| Provider | Deployment | Privacy | Tool Calling |
|----------|-----------|---------|--------------|
| Ollama   | In-cluster (local) | All data stays in your cluster | Depends on model |
| OpenAI   | Cloud API | Data sent to OpenAI | Strong |
| Gemini   | Cloud API | Data sent to Google | Strong |

For provider setup, model recommendations, available tools, and data privacy details, see [docs/AI.md](docs/AI.md).

---

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, project structure, and PR guidelines.

---

## Security

See [docs/SECURITY.md](docs/SECURITY.md) for the full security architecture, including authentication, secrets management, container hardening, RBAC, and AI agent security boundaries.

To report a security vulnerability, please contact the maintainers directly rather than opening a public issue.

---

## License

This project is licensed under the [GNU Affero General Public License v3.0 (AGPL-3.0)](LICENSE).

You are free to use, modify, and distribute this software. If you modify it and offer it as a service, you must make your modifications available under the same license.
