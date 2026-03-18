# GitOps PR Creation — Design Spec

**Issue:** #16 — GitOps PR creation from UI
**Date:** 2026-03-18
**Status:** Draft

---

## 1. Overview

Add the ability for AAP to create Git pull requests that modify addon infrastructure. AAP never deploys directly — it creates PRs. ArgoCD handles the rest.

This engine also serves as the foundation for the Migration Wizard (#18), which orchestrates multi-step, multi-repo addon migrations via the AI agent.

**Principle:** AAP creates PRs. ArgoCD deploys.

---

## 2. Scope

### In Scope (v2 MVP)
- Enable/disable addon on a cluster (toggle label in `cluster-addons.yaml`)
- Change addon version (update version in `addons-catalog.yaml` or version label)
- Add/remove cluster (scaffold entry in `cluster-addons.yaml` + values file)
- Modify cluster-specific values (edit `addons-clusters-values/<cluster>.yaml`)
- Modify global addon values (edit `addons-global-values/<addon>.yaml`)
- Raw YAML editor with syntax validation for values edits
- Diff preview before PR submission
- Custom OpenAI-compatible provider support (configurable base URL, auth header, mTLS)

### Out of Scope (later)
- Bulk operations (enable addon across multiple clusters in one PR)
- AI-assisted PR descriptions
- Azure DevOps GitProvider write implementation (stub only)
- Full migration wizard UI (#18 — uses same engine but separate feature)

---

## 3. Architecture

### Layer 1 — GitProvider Write Methods

Extend the existing `GitProvider` interface with write operations:

```go
type GitProvider interface {
    // Existing read methods
    GetFileContent(ctx context.Context, path, ref string) ([]byte, error)
    ListDirectory(ctx context.Context, path, ref string) ([]string, error)
    ListPullRequests(ctx context.Context, state string) ([]PullRequest, error)
    TestConnection(ctx context.Context) error

    // New write methods
    CreateBranch(ctx context.Context, branchName, fromRef string) error
    CreateOrUpdateFile(ctx context.Context, path string, content []byte, branch, commitMessage string) error
    DeleteFile(ctx context.Context, path, branch, commitMessage string) error
    CreatePullRequest(ctx context.Context, title, body, head, base string) (*PullRequest, error)
}
```

**GitHub Implementation** (using `go-github/v68`):

- `CreateBranch`: `Git.GetRef(ctx, owner, repo, "refs/heads/{fromRef}")` → `Git.CreateRef(ctx, owner, repo, &Reference{Ref: "refs/heads/{branchName}", Object: ...})`
- `CreateOrUpdateFile`: Fetch existing file via `Repositories.GetContents` to obtain the current SHA. If file exists, call `Repositories.UpdateFile` with the SHA. If not, call `Repositories.CreateFile`. **On 422 SHA mismatch (stale SHA), retry once** by re-fetching the SHA and retrying the update. This handles concurrent modifications.
- `DeleteFile`: `Repositories.DeleteFile` (requires current SHA, same fetch-then-delete pattern)
- `CreatePullRequest`: `PullRequests.Create` with `&NewPullRequest{Title, Head, Base, Body, MaintainerCanModify: true}`

**Note:** The existing `GetFileContent` in `github.go` discards the `RepositoryContent` struct (which carries the SHA). The write implementation will use an internal `getContentsRaw(ctx, path, ref) (*github.RepositoryContent, error)` helper that returns the full struct, used by both `CreateOrUpdateFile` and `DeleteFile`.

**Branch naming convention:**

| Operation | Pattern | Example |
|-----------|---------|---------|
| Enable addon | `aap/enable-addon/{addon}/{cluster}/{ts}` | `aap/enable-addon/datadog/feedlot-dev/1711234567` |
| Disable addon | `aap/disable-addon/{addon}/{cluster}/{ts}` | `aap/disable-addon/keda/feedlot-dev/1711234567` |
| Update version | `aap/update-version/{addon}/{ts}` | `aap/update-version/keda/1711234567` |
| Edit cluster values | `aap/edit-values/{cluster}/{ts}` | `aap/edit-values/feedlot-dev/1711234567` |
| Edit global values | `aap/edit-global-values/{addon}/{ts}` | `aap/edit-global-values/datadog/1711234567` |
| Add cluster | `aap/add-cluster/{cluster}/{ts}` | `aap/add-cluster/new-cluster/1711234567` |
| Remove cluster | `aap/remove-cluster/{cluster}/{ts}` | `aap/remove-cluster/old-cluster/1711234567` |

All names are sanitized: alphanumeric and hyphens only (`[^a-zA-Z0-9-]` replaced with `-`).

### Layer 2 — ArgoCD Write Methods

Extend the ArgoCD client with write operations:

```go
// SyncApplication triggers a sync for the named application.
// POST /api/v1/applications/{name}/sync
// Body: {} (empty — app name is in URL path)
func (c *Client) SyncApplication(ctx context.Context, appName string) error

// RefreshApplication triggers a refresh (optionally hard) for the application.
// GET /api/v1/applications/{name}?refresh=hard  (hard refresh)
// GET /api/v1/applications/{name}?refresh=true  (normal refresh)
func (c *Client) RefreshApplication(ctx context.Context, appName string, hard bool) (*models.ArgocdApplication, error)
```

Implementation notes:
- `SyncApplication` uses a new `doPost(ctx, path, body)` method on Client (currently only `doGet` exists)
- `RefreshApplication` reuses `doGet` with query parameter appended

### Layer 3 — GitOps Operations (Service Layer)

New `internal/gitops/` package with typed operations:

```go
package gitops

type Operation interface {
    Validate(ctx context.Context) error
    Preview(ctx context.Context) ([]FileDiff, error)
    Execute(ctx context.Context) (*Result, error)
}

// All operations include ConnectionName to support multi-repo workflows.

type EnableAddonOp struct {
    ConnectionName string
    ClusterName    string
    AddonName      string
}

type DisableAddonOp struct {
    ConnectionName string
    ClusterName    string
    AddonName      string
}

type UpdateVersionOp struct {
    ConnectionName string
    AddonName      string
    NewVersion     string
    // If ClusterName is set, updates the cluster-specific version label.
    // If empty, updates the catalog version.
    ClusterName string
}

type EditValuesOp struct {
    ConnectionName string
    // For global: ClusterName is empty, AddonName is set
    // For cluster: both set
    ClusterName string
    AddonName   string
    NewContent  []byte
}

type AddClusterOp struct {
    ConnectionName string
    ClusterName    string
    Labels         map[string]string
    ValuesYAML     []byte // initial values file content
}

type RemoveClusterOp struct {
    ConnectionName string
    ClusterName    string
}

// FileDiff represents a change to a single file.
type FileDiff struct {
    FilePath string
    Before   string // empty string = new file (IsNew)
    After    string // empty string = file deleted (IsDelete)
}

type Result struct {
    PullRequestURL string
    BranchName     string
    FilesChanged   []string
}
```

Each operation:
1. **Validate** — checks the operation makes sense (addon exists, cluster exists, etc.)
2. **Preview** — returns `[]FileDiff` (supports multi-file operations like add/remove cluster)
3. **Execute** — creates branch, modifies file(s), opens PR

Convention for `FileDiff`:
- `Before == "" && After != ""` → new file being created
- `Before != "" && After == ""` → file being deleted
- Both non-empty → file being modified

### Layer 4 — AI Agent Tools

New write tools added to `ToolExecutor`. Each tool has a concrete JSON Schema for parameters:

```
enable_addon(connection, cluster_name, addon_name)        → creates PR to enable addon
disable_addon(connection, cluster_name, addon_name)        → creates PR to disable addon
update_addon_version(connection, addon_name, version)      → creates PR to update catalog version
update_cluster_values(connection, cluster_name, content)   → creates PR to update cluster values
update_global_values(connection, addon_name, content)      → creates PR to update global values
preview_addon_change(connection, cluster_name, addon_name, action)  → returns diff (action: "enable" or "disable")
sync_argocd_app(app_name)                                  → triggers ArgoCD sync
refresh_argocd_app(app_name, hard)                         → triggers ArgoCD refresh
```

The `preview_addon_change` tool has a typed `action` parameter (enum: `"enable"`, `"disable"`, `"update_version"`) instead of variadic params.

For the migration wizard POC, additional compound tools:

```
migrate_addon_prepare_new(addon, cluster)      → enable in NEW repo with inMigration: true
migrate_addon_remove_old(addon, cluster)       → disable in OLD repo
migrate_addon_verify(addon, cluster)           → check ArgoCD health on NEW
migrate_addon_cleanup(addon)                   → set inMigration: false
```

### Layer 5 — API Endpoints

```
POST /api/v1/gitops/preview          → preview a change (returns []FileDiff)
POST /api/v1/gitops/execute          → execute a change (creates PR)
GET  /api/v1/gitops/operations       → list supported operation types

# Request body for both preview and execute:
{
  "operation": "enable_addon",
  "connection": "default",
  "params": {
    "cluster_name": "feedlot-dev",
    "addon_name": "keda"
  }
}
```

### Layer 6 — Custom OpenAI-Compatible Provider Support

Modify `internal/ai/client.go` and `internal/ai/agent.go` to support custom OpenAI-compatible endpoints (e.g., enterprise LLM gateways):

**Config changes:**
```go
const ProviderCustomOpenAI Provider = "custom-openai"

type Config struct {
    Provider       string // "ollama", "openai", "claude", "gemini", "custom-openai"
    APIKey         string
    BaseURL        string // configurable base URL for custom endpoints
    AuthHeader     string // custom auth header name (default: "Authorization")
    AuthPrefix     string // auth value prefix (default: "Bearer ")
    MaxIterations  int    // agent loop limit (default: 8)
    // ...existing fields
    TLS struct {
        ClientCert string // path to .crt file (for mTLS)
        ClientKey  string // path to .key file
    }
}
```

**Implementation:**
1. Add `ProviderCustomOpenAI` constant to `client.go`
2. Add `case ProviderCustomOpenAI:` in `callLLM()` dispatch — calls `callCustomOpenAIChat(ctx)` which reuses the OpenAI chat logic but with configurable URL and auth header
3. Add `ProviderCustomOpenAI` to `GetAgentModel()` cloud-provider check
4. Add `ProviderCustomOpenAI` handling in `Summarize()` method
5. Build HTTP client with optional mTLS `tls.Config` when `TLS.ClientCert` is set

**Helm values** (all defaults are empty — users configure in their `values-production.yaml`):
```yaml
ai:
  provider: ""
  apiKey: ""
  baseURL: ""          # e.g., https://your-gateway.example.com/v1
  authHeader: ""       # e.g., X-Custom-APIKey (defaults to "Authorization" if empty)
  cloudModel: ""
  maxIterations: 8
  tls:
    clientCert: ""
    clientKey: ""
```

---

## 4. YAML Mutation Strategy

The config files use YAML anchors (`&env`, `*clusterName`), comments, and specific formatting. Naive marshal/unmarshal would destroy these.

**Approach: line-level manipulation for simple operations, full parse for complex ones.**

- **Enable/disable addon** (toggle label): Read file as text, find the cluster entry, find/add the label line. Simple string manipulation preserving all formatting.
- **Update version** (catalog): Read file as text, find the addon entry, replace the version value.
- **Edit values** (YAML editor): User provides the full new content from the YAML editor. Replace the entire file. Since the user is editing raw YAML, they control formatting.
- **Add cluster**: Template-based scaffolding. Read an existing cluster entry as a pattern, generate new entry.

For enable/disable specifically, the operation modifies `cluster-addons.yaml`:
```yaml
# Before
- name: feedlot-dev
  labels:
    datadog: enabled
    keda: disabled

# After (enable keda)
- name: feedlot-dev
  labels:
    datadog: enabled
    keda: enabled
```

This is a simple find-and-replace of `keda: disabled` → `keda: enabled` within the correct cluster block, or appending `keda: enabled` if the label doesn't exist.

---

## 5. Multi-Repo Support (Migration Wizard Foundation)

The migration wizard needs to operate on two repos:
- **NEW repo** (GitHub): `argocd-cluster-addons` — enable addons, set `inMigration` flag
- **OLD repo** (could be any provider): `argocd-cluster-addons` — disable addons

**Connection model:** AAP already supports multiple connections (`config.yaml` connections array). Each connection has its own GitProvider. All gitops operations accept a `ConnectionName` parameter to target a specific repo (see Section 3, Layer 3).

For the POC, the agent specifies which connection to use when calling tools:
```
enable_addon(connection="new-argocd", cluster="feedlot-dev", addon="keda")
disable_addon(connection="old-argocd", cluster="feedlot-dev", addon="keda")
```

---

## 6. Feature Flag

Disabled by default in Helm values:

```yaml
gitops:
  actions:
    enabled: false
```

When disabled:
- All `/api/v1/gitops/*` endpoints return 403
- Write tools are not registered with the AI agent
- UI does not show edit/action buttons

---

## 7. Agent Loop Limit

Current limit is 8 iterations (`agent.go` line 201). For migration workflows that need 15-20 tool calls, this needs to be configurable.

Add `MaxIterations int` to `ai.Config` struct with default of 8. Map from Helm via `ai.maxIterations`.

```go
maxIter := a.client.config.MaxIterations
if maxIter <= 0 {
    maxIter = 8
}
for i := 0; i < maxIter; i++ {
```

For migration POC, set to 25 in config.

---

## 8. Security

- **GitOps actions require authentication** — same bearer token auth as existing endpoints
- **Feature flag gates all write operations** — disabled by default
- **All changes go through Git PRs** — never direct deployment
- **Branch protection** — relies on repo-level branch protection rules (not AAP's responsibility)
- **Audit trail** — every PR created by AAP includes `aap/` prefix in branch name for traceability
- **No sensitive defaults in Helm chart** — all provider URLs, auth headers, API keys, and model names are empty in `values.yaml`. Production values go in `values-production.yaml` only.

---

## 9. Build Order

### Phase 1 — GitProvider Write + Custom Provider (enables POC)
1. Add write methods to `GitProvider` interface
2. Implement GitHub write methods (`CreateBranch`, `CreateOrUpdateFile`, `CreatePullRequest`) including `getContentsRaw` helper for SHA retrieval and retry-on-422 logic
3. Add custom OpenAI-compatible provider support:
   - Add `ProviderCustomOpenAI` constant to `client.go`
   - Add `case ProviderCustomOpenAI:` in `callLLM()` dispatch
   - Add `ProviderCustomOpenAI` to `GetAgentModel()` and `Summarize()`
   - Support configurable base URL, auth header, and optional mTLS
4. Add `SyncApplication` and `RefreshApplication` to ArgoCD client (including new `doPost` method)
5. Add write tools to AI agent with concrete JSON schemas
6. Add `MaxIterations` to `ai.Config`, make agent loop configurable
7. Test POC: migrate one addon via AI chat

### Phase 2 — GitOps Service Layer + API (enables UI)
1. Build `internal/gitops/` operations with Validate/Preview/Execute
2. Add `/api/v1/gitops/preview` and `/api/v1/gitops/execute` endpoints
3. Add feature flag gating
4. Add Helm chart config for `gitops.actions.enabled`

### Phase 3 — Frontend (enables non-AI users)
1. Side panel component for GitOps actions
2. YAML editor component (CodeMirror or Monaco)
3. Diff preview component
4. PR creation confirmation flow
5. Action buttons on cluster/addon detail pages

---

## 10. Files to Create/Modify

### New Files
- `internal/gitops/operations.go` — Operation types, interfaces, FileDiff
- `internal/gitops/enable_addon.go` — Enable addon operation
- `internal/gitops/disable_addon.go` — Disable addon operation
- `internal/gitops/update_version.go` — Update version operation
- `internal/gitops/edit_values.go` — Edit values operation
- `internal/gitops/add_cluster.go` — Add cluster operation
- `internal/gitops/remove_cluster.go` — Remove cluster operation
- `internal/gitops/yaml_mutator.go` — YAML-preserving mutation utilities
- `internal/api/gitops.go` — GitOps API handlers

### Modified Files
- `internal/gitprovider/provider.go` — Add write methods to interface
- `internal/gitprovider/github.go` — Implement GitHub write methods + `getContentsRaw` helper
- `internal/gitprovider/azuredevops.go` — Add stubs for write methods
- `internal/argocd/client.go` — Add `SyncApplication`, `RefreshApplication`, `doPost` method
- `internal/ai/tools.go` — Add write tool definitions and executors
- `internal/ai/agent.go` — Use configurable `MaxIterations`, add `ProviderCustomOpenAI` dispatch
- `internal/ai/client.go` — Add `ProviderCustomOpenAI` constant, `BaseURL`, `AuthHeader`, `AuthPrefix`, `MaxIterations`, `TLS` config fields, update `GetAgentModel()` and `Summarize()`
- `internal/api/router.go` — Add `/api/v1/gitops/*` routes
- `internal/config/store.go` — Add gitops config parsing
- `charts/argocd-addons-platform/values.yaml` — Add `gitops.actions.enabled`, empty AI provider fields
- `charts/argocd-addons-platform/templates/configmap.yaml` — Include gitops config
- `charts/argocd-addons-platform/templates/deployment.yaml` — Mount TLS certs (optional)
