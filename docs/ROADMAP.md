# Roadmap

## Current: v1 — Read-Only Visibility + AI

The platform provides centralized visibility, observability, and AI-powered insights for ArgoCD-managed addon deployments. Strictly read-only.

---

## v2 — GitOps Actions via PR Creation

### Overview

Allow users to make changes to their addon infrastructure directly from the AAP UI — but always through Git PRs, never direct deployment. The platform creates a branch, makes the change, and opens a PR. The actual deployment happens via ArgoCD after the PR is reviewed and merged.

**Principle:** AAP never deploys. It creates PRs. GitOps handles the rest.

### Features

#### PR-Based Addon Management
- **Enable/disable addon on a cluster** — Modifies `cluster-addons.yaml`, opens PR
- **Change addon version** — Updates version label or `addons-catalog.yaml`, opens PR
- **Add a new cluster** — Scaffolds cluster entry in `cluster-addons.yaml` + creates values file in `addons-clusters-values/`, opens PR
- **Remove a cluster** — Removes cluster entry, opens PR
- **Modify cluster-specific values** — Edits `addons-clusters-values/<cluster>.yaml`, opens PR
- **Modify global addon values** — Edits `addons-global-values/<addon>.yaml`, opens PR
- **Add a new addon to catalog** — Adds entry to `addons-catalog.yaml`, opens PR
- **Bulk operations** — Enable an addon across multiple clusters in a single PR

#### UI/UX
- Preview diff before creating PR (show exact YAML changes)
- Custom PR title and description
- Auto-generated PR description with context (what changed, why, impact)
- AI-assisted PR description ("Explain the impact of this change")
- Link to created PR after submission
- PR status tracking in the dashboard

#### Configuration
- Feature disabled by default (`gitops.actions.enabled: false` in Helm values)
- Configurable target branch (default: `main`)
- Configurable PR labels and reviewers
- Require confirmation before PR creation

### Technical Requirements
- Git provider write access (branch creation, file modification, PR creation)
- GitHub API: already supported via `go-github` library
- Azure DevOps API: needs write endpoints
- Template engine for scaffolding new cluster values files

---

## v2.1 — IDP / API Integration

### Overview

Expose REST API endpoints that allow Internal Developer Platforms (Backstage, Port, Cortex, custom portals) to programmatically manage addon infrastructure through AAP. Same GitOps principle — all changes result in Git PRs.

### API Endpoints

```
POST /api/v1/gitops/clusters              — Create a new cluster (PR)
DELETE /api/v1/gitops/clusters/{name}      — Remove a cluster (PR)
POST /api/v1/gitops/clusters/{name}/addons — Enable addon on cluster (PR)
DELETE /api/v1/gitops/clusters/{name}/addons/{addon} — Disable addon (PR)

POST /api/v1/gitops/addons                — Add new addon to catalog (PR)
PUT /api/v1/gitops/addons/{name}/version  — Update addon version (PR)

PUT /api/v1/gitops/values/global/{addon}  — Update global values (PR)
PUT /api/v1/gitops/values/cluster/{cluster}/{addon} — Update cluster values (PR)

GET /api/v1/gitops/prs                    — List open PRs created by AAP
GET /api/v1/gitops/prs/{id}/status        — PR status (open, merged, closed)
```

### Use Cases
- **Backstage template**: "Create new microservice" → provisions EKS cluster + enables standard addons via AAP API
- **CI/CD pipeline**: After cluster creation, automatically enable monitoring (Datadog) and mesh (Istio) via API call
- **Self-service portal**: Developers request addon enablement, platform team reviews the auto-generated PR
- **Automation scripts**: Bulk operations across clusters

### Authentication
- API key based (separate from user auth)
- Scoped permissions (read-only vs read-write)
- Audit log of all API-initiated changes

---

## v2.2 — Multi-ArgoCD Support

### Overview

Support connecting to multiple ArgoCD instances (dev, staging, prod) simultaneously, with a unified view across all environments.

### Features
- Multiple active connections (not just switching between them)
- Unified dashboard showing all environments
- Cross-environment addon version comparison
- Promotion workflow: "Promote datadog 3.160.1 from dev to staging" (creates PR)

---

## v3 — Provider Abstraction

### Overview

Abstract the GitOps config parser to support different addon management patterns beyond the argocd-cluster-addons structure.

### Potential Adapters
- argocd-cluster-addons V2 (current, built-in)
- argocd-cluster-addons V1 (app-of-apps pattern)
- Generic ArgoCD ApplicationSets
- Flux HelmReleases (future)

---

## Ideas / Backlog

- **GitHub Enterprise + GitHub.com support** — Ensure the Git provider works with both GitHub Enterprise Server (custom base URL) and github.com. May need configurable API base URL.
- **GitLab / Bitbucket support** — Additional Git providers
- **Makefile** — Standard `make dev`, `make test`, `make build`, `make deploy` commands
- **Helm repository** — Publish chart to a Helm repo for easier installation
- **SSO/OIDC authentication** — OAuth2 proxy or native OIDC support
- **Multi-user RBAC** — Role-based access (admin, viewer, operator)
- **Webhook receiver** — React to Git push events instead of polling
- **Cost estimation** — Resource usage trends and cost projections per addon
- **Slack/Teams notifications** — Alert when AAP detects drift or unhealthy addons
- **Plugin system** — Allow custom tools for the AI agent
