# Migration Wizard Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Migration Wizard page with AI-driven 10-step addon migration, visual stepper UI, PR tracking, and multi-provider Git support.

**Architecture:** Backend migration state machine with JSON persistence, Azure DevOps GitProvider implementation, migration API endpoints, React frontend with stepper component and settings panel. AI agent evaluates health/status at key steps.

**Tech Stack:** Go 1.25, Azure DevOps REST API v7.1, React 19, shadcn/ui, Tailwind CSS, Lucide icons

**Spec:** `docs/superpowers/specs/2026-03-19-migration-wizard-design.md`

---

## File Structure

### New Backend Files
| File | Responsibility |
|------|---------------|
| `internal/migration/models.go` | Migration, MigrationStep, MigrationSettings types |
| `internal/migration/store.go` | JSON file persistence for migrations and settings |
| `internal/migration/executor.go` | Step-by-step migration orchestration, AI integration |
| `internal/migration/steps.go` | Individual step implementations (10 steps) |
| `internal/migration/store_test.go` | Store persistence tests |
| `internal/migration/executor_test.go` | Executor logic tests |
| `internal/gitprovider/azuredevops_impl.go` | Full Azure DevOps read + write implementation |
| `internal/gitprovider/azuredevops_impl_test.go` | Azure DevOps provider tests |
| `internal/api/migration.go` | Migration API handlers |

### New Frontend Files
| File | Responsibility |
|------|---------------|
| `ui/src/views/MigrationPage.tsx` | Main page: settings panel + migration list |
| `ui/src/views/MigrationDetail.tsx` | Detail view: visual stepper with live updates |
| `ui/src/components/MigrationStepper.tsx` | Vertical stepper component |
| `ui/src/components/MigrationStep.tsx` | Individual step card |
| `ui/src/components/MigrationSettings.tsx` | OLD repo + OLD ArgoCD settings form |
| `ui/src/components/NewMigrationDialog.tsx` | Start migration dialog |

### Modified Files
| File | Change |
|------|--------|
| `internal/gitprovider/azuredevops.go` | Replace stubs with PAT auth, base URL with token |
| `internal/api/router.go` | Add `/api/v1/migration/*` routes |
| `cmd/aap-server/main.go` | Wire migration service |
| `ui/src/App.tsx` | Add `/migration` and `/migration/:id` routes |
| `ui/src/components/Layout.tsx` | Add Migration nav item |
| `ui/src/services/api.ts` | Add migration API methods |

---

## Task 1: Azure DevOps GitProvider — Full Implementation

**Files:**
- Modify: `internal/gitprovider/azuredevops.go` — add PAT auth, token storage
- Create: `internal/gitprovider/azuredevops_impl.go` — read + write method implementations
- Create: `internal/gitprovider/azuredevops_impl_test.go` — tests with httptest

The Azure DevOps REST API uses:
- **Auth:** Basic auth with empty username and PAT as password: `Authorization: Basic base64(:PAT)`
- **Base URL:** `https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}`
- **API version:** `api-version=7.1` query param on every request

### Methods to implement:

**Read methods:**
- `GetFileContent(path, ref)` — `GET /items?path={path}&versionDescriptor.version={ref}&api-version=7.1`
- `ListDirectory(path, ref)` — `GET /items?path={path}&recursionLevel=OneLevel&versionDescriptor.version={ref}&api-version=7.1`
- `ListPullRequests(state)` — `GET /pullrequests?searchCriteria.status={state}&api-version=7.1`
- `TestConnection()` — `GET ?api-version=7.1` (get repo info)

**Write methods:**
- `CreateBranch(branchName, fromRef)` — Azure DevOps creates branches via pushes. Use `POST /pushes?api-version=7.1` with a refUpdate that creates a new ref from the source ref's objectId. First get the source ref via `GET /refs?filter=heads/{fromRef}&api-version=7.1`.
- `CreateOrUpdateFile(path, content, branch, commitMessage)` — `POST /pushes?api-version=7.1` with a change of type "edit" (or "add" for new files). First check if file exists via GET items. The push body includes the base64 content and commit message.
- `DeleteFile(path, branch, commitMessage)` — `POST /pushes?api-version=7.1` with change type "delete".
- `CreatePullRequest(title, body, head, base)` — `POST /pullrequests?api-version=7.1` with sourceRefName and targetRefName.

### Azure DevOps Push API body format:
```json
{
  "refUpdates": [{"name": "refs/heads/branch", "oldObjectId": "old-sha"}],
  "commits": [{
    "comment": "commit message",
    "changes": [{
      "changeType": "edit",
      "item": {"path": "/path/to/file"},
      "newContent": {"content": "base64content", "contentType": "base64encoded"}
    }]
  }]
}
```

- [ ] **Step 1: Update azuredevops.go — add PAT and auth helper**

Add `pat` field to struct, update constructor, add `doGet`/`doPost` helpers with Basic auth.

```go
type AzureDevOpsProvider struct {
    client       *http.Client
    organisation string
    project      string
    repository   string
    pat          string
    baseURL      string
}

func NewAzureDevOpsProvider(organisation, project, repository, token string) *AzureDevOpsProvider {
    return &AzureDevOpsProvider{
        client:       &http.Client{},
        organisation: organisation,
        project:      project,
        repository:   repository,
        pat:          token,
        baseURL:      fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/repositories/%s", organisation, project, repository),
    }
}
```

- [ ] **Step 2: Write tests for Azure DevOps read methods**

Test GetFileContent, ListPullRequests, TestConnection with httptest.

- [ ] **Step 3: Implement Azure DevOps read methods in azuredevops_impl.go**

- [ ] **Step 4: Write tests for Azure DevOps write methods**

Test CreateBranch (get ref + create push), CreateOrUpdateFile (push with edit change), CreatePullRequest.

- [ ] **Step 5: Implement Azure DevOps write methods**

- [ ] **Step 6: Run all tests and verify build**

Run: `go test ./internal/gitprovider/ -v && go build ./...`

- [ ] **Step 7: Commit**

```bash
git add internal/gitprovider/
git commit -m "feat: implement Azure DevOps GitProvider (read + write methods)"
```

---

## Task 2: Migration Models and Store

**Files:**
- Create: `internal/migration/models.go`
- Create: `internal/migration/store.go`
- Create: `internal/migration/store_test.go`

- [ ] **Step 1: Write models.go**

Define all types: `Migration`, `MigrationStep`, `MigrationSettings`, `MigrationStatus`, `StepStatus`, step title constants.

The 10 steps with titles and descriptions:
```go
var MigrationStepDefinitions = []struct {
    Title       string
    Description string
}{
    {Title: "Verify addon in NEW catalog", Description: "Check that the addon exists in addons-catalog.yaml with inMigration: true"},
    {Title: "Configure values in NEW repo", Description: "Verify global and cluster values match the OLD repo configuration"},
    {Title: "Enable addon on cluster", Description: "Create PR to set addon label to 'enabled' in cluster-addons.yaml"},
    {Title: "Verify app created in NEW ArgoCD", Description: "Check that ArgoCD created the application (may show OutOfSync — this is normal)"},
    {Title: "Disable addon in OLD repo", Description: "Create PR to disable the addon label in the OLD repository"},
    {Title: "Sync clusters app in OLD ArgoCD", Description: "Trigger sync so OLD ArgoCD removes the application"},
    {Title: "Verify app removed from OLD ArgoCD", Description: "Confirm the application no longer exists in OLD ArgoCD"},
    {Title: "Hard refresh in NEW ArgoCD", Description: "Trigger hard refresh so NEW ArgoCD adopts orphaned resources"},
    {Title: "Verify healthy in NEW ArgoCD", Description: "Confirm application is Synced + Healthy with no pod restarts"},
    {Title: "Disable migration mode", Description: "Create PR to set inMigration: false (optional — can be deferred)"},
}
```

- [ ] **Step 2: Write store.go**

JSON file-based persistence:
- `MigrationStore` struct with data directory path
- `SaveMigration(m *Migration) error` — writes `{dataDir}/{id}.json`
- `GetMigration(id string) (*Migration, error)` — reads JSON
- `ListMigrations() ([]*Migration, error)` — reads all files in data dir
- `SaveSettings(s *MigrationSettings) error` — writes `{dataDir}/settings.json`
- `GetSettings() (*MigrationSettings, error)` — reads settings

- [ ] **Step 3: Write store tests**

Test save/get/list migrations, settings persistence.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/migration/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/migration/
git commit -m "feat: add migration models and JSON file store"
```

---

## Task 3: Migration Executor — Step Orchestration

**Files:**
- Create: `internal/migration/executor.go`
- Create: `internal/migration/steps.go`

- [ ] **Step 1: Implement executor.go**

```go
type Executor struct {
    store       *MigrationStore
    newGP       gitprovider.GitProvider  // NEW repo (active connection)
    oldGP       gitprovider.GitProvider  // OLD repo (from migration settings)
    newArgoCD   *argocd.Client           // NEW ArgoCD
    oldArgoCD   *argocd.Client           // OLD ArgoCD (from migration settings)
    aiClient    *ai.Client               // for AI evaluation steps
}

func (e *Executor) StartMigration(addonName, clusterName string) (*Migration, error)
func (e *Executor) RunNextStep(migrationID string) error
func (e *Executor) ContinueAfterPR(migrationID string) error
func (e *Executor) PauseMigration(migrationID string) error
func (e *Executor) RetryStep(migrationID string) error
func (e *Executor) CancelMigration(migrationID string) error
```

`RunNextStep` finds the current step, executes it, updates state, and decides whether to auto-advance or wait.

- [ ] **Step 2: Implement steps.go**

Each step is a function: `func (e *Executor) executeStep(ctx context.Context, m *Migration, step int) error`

The function:
1. Sets step status to `running`
2. Calls the step-specific logic
3. For PR steps: creates branch, mutates YAML, creates PR, sets status to `waiting`
4. For verification steps: calls ArgoCD API, uses AI to evaluate, sets status to `completed` or `failed`
5. For ArgoCD operation steps: triggers sync/refresh, sets status to `completed`

Step-specific implementations:
- `stepVerifyCatalog` — read addons-catalog.yaml from NEW, check addon exists with inMigration: true
- `stepConfigureValues` — read values from both OLD and NEW repos, AI compares them
- `stepEnableAddon` — use gitops.EnableAddonLabel + create PR on NEW repo
- `stepVerifyAppCreated` — call NEW ArgoCD GetApplication, AI evaluates
- `stepDisableAddonOld` — use gitops.DisableAddonLabel on OLD repo + create PR
- `stepSyncOldArgoCD` — call OLD ArgoCD SyncApplication
- `stepVerifyAppRemoved` — call OLD ArgoCD GetApplication, expect 404
- `stepHardRefresh` — call NEW ArgoCD RefreshApplication(hard=true)
- `stepVerifyHealthy` — call NEW ArgoCD GetApplication, AI checks Synced + Healthy
- `stepDisableMigrationMode` — update addons-catalog.yaml inMigration: false + create PR

**AI evaluation pattern:**
```go
func (e *Executor) aiEvaluate(ctx context.Context, prompt string) (continueOk bool, message string, err error) {
    response, err := e.aiClient.Summarize(ctx, prompt)
    // Parse response for continue/stop decision and explanation
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/migration/executor.go internal/migration/steps.go
git commit -m "feat: add migration executor with 10-step orchestration"
```

---

## Task 4: Migration API Endpoints

**Files:**
- Create: `internal/api/migration.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/aap-server/main.go`

- [ ] **Step 1: Implement migration.go API handlers**

```go
func (s *Server) handleGetMigrationSettings(w, r)
func (s *Server) handleSaveMigrationSettings(w, r)
func (s *Server) handleTestMigrationConnection(w, r)
func (s *Server) handleListMigrations(w, r)
func (s *Server) handleStartMigration(w, r)
func (s *Server) handleGetMigration(w, r)
func (s *Server) handleContinueMigration(w, r)
func (s *Server) handlePauseMigration(w, r)
func (s *Server) handleRetryMigration(w, r)
func (s *Server) handleCancelMigration(w, r)
```

`handleStartMigration` creates the migration, starts step 1 in a goroutine, returns the migration ID immediately.

`handleContinueMigration` resumes execution after PR merge — runs next steps in a goroutine.

The executor runs steps sequentially in the goroutine, updating state after each step. The frontend polls `handleGetMigration` for updates.

- [ ] **Step 2: Add routes to router.go**

```go
r.HandleFunc("/api/v1/migration/settings", s.handleGetMigrationSettings).Methods("GET")
r.HandleFunc("/api/v1/migration/settings", s.handleSaveMigrationSettings).Methods("POST")
r.HandleFunc("/api/v1/migration/settings/test", s.handleTestMigrationConnection).Methods("POST")
r.HandleFunc("/api/v1/migration/list", s.handleListMigrations).Methods("GET")
r.HandleFunc("/api/v1/migration/start", s.handleStartMigration).Methods("POST")
r.HandleFunc("/api/v1/migration/{id}", s.handleGetMigration).Methods("GET")
r.HandleFunc("/api/v1/migration/{id}/continue", s.handleContinueMigration).Methods("POST")
r.HandleFunc("/api/v1/migration/{id}/pause", s.handlePauseMigration).Methods("POST")
r.HandleFunc("/api/v1/migration/{id}/retry", s.handleRetryMigration).Methods("POST")
r.HandleFunc("/api/v1/migration/{id}/cancel", s.handleCancelMigration).Methods("POST")
```

- [ ] **Step 3: Wire migration service in main.go**

Create migration store, executor, and add to server.

- [ ] **Step 4: Verify build**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/api/migration.go internal/api/router.go cmd/aap-server/main.go
git commit -m "feat: add migration API endpoints"
```

---

## Task 5: Frontend — API Service + Types

**Files:**
- Modify: `ui/src/services/api.ts`
- Create: `ui/src/types/migration.ts` (if types are not inline)

- [ ] **Step 1: Add migration types and API methods**

Add to `api.ts`:

```typescript
// Migration types
interface MigrationStep {
  number: number
  title: string
  description: string
  status: 'pending' | 'running' | 'waiting' | 'completed' | 'failed' | 'skipped'
  message: string
  pr_url?: string
  pr_status?: string
  started_at?: string
  completed_at?: string
  error?: string
}

interface Migration {
  id: string
  addon_name: string
  cluster_name: string
  status: 'pending' | 'running' | 'waiting' | 'paused' | 'completed' | 'failed' | 'cancelled'
  current_step: number
  steps: MigrationStep[]
  created_at: string
  updated_at: string
  completed_at?: string
  error?: string
}

interface MigrationSettings {
  old_git: { provider: string; owner?: string; repo?: string; token?: string; organization?: string; project?: string; repository?: string; pat?: string }
  old_argocd: { server_url: string; token: string; namespace: string; insecure?: boolean }
  configured: boolean
}

// API methods
getMigrationSettings(): Promise<MigrationSettings>
saveMigrationSettings(settings: MigrationSettings): Promise<void>
testMigrationConnection(): Promise<{ git: boolean; argocd: boolean }>
listMigrations(): Promise<Migration[]>
startMigration(data: { addon_name: string; cluster_name: string; scope: string }): Promise<Migration>
getMigration(id: string): Promise<Migration>
continueMigration(id: string): Promise<void>
pauseMigration(id: string): Promise<void>
retryMigration(id: string): Promise<void>
cancelMigration(id: string): Promise<void>
```

- [ ] **Step 2: Commit**

```bash
git add ui/src/services/api.ts
git commit -m "feat: add migration API types and service methods"
```

---

## Task 6: Frontend — Migration Settings Component

**Files:**
- Create: `ui/src/components/MigrationSettings.tsx`

A form panel for configuring the OLD repo and OLD ArgoCD connection. Follows the Connections.tsx form pattern.

- [ ] **Step 1: Build MigrationSettings.tsx**

Features:
- Git provider selector (GitHub / Azure DevOps)
- Conditional fields based on provider (owner/repo/token for GitHub, org/project/repo/PAT for Azure DevOps)
- OLD ArgoCD: server URL, token, namespace, insecure toggle
- "Test Connection" button for both Git and ArgoCD — shows green checkmark or red X
- "Save Settings" button — disabled until both tests pass
- Shows current configuration status (configured / not configured)

Use same input/label CSS classes as Connections.tsx.

- [ ] **Step 2: Commit**

```bash
git add ui/src/components/MigrationSettings.tsx
git commit -m "feat: add migration settings form component"
```

---

## Task 7: Frontend — Migration Stepper Components

**Files:**
- Create: `ui/src/components/MigrationStepper.tsx`
- Create: `ui/src/components/MigrationStep.tsx`

- [ ] **Step 1: Build MigrationStep.tsx**

A single step card:
- Left: circle with step number (green check for done, blue spinner for running, clock for waiting, gray for pending, red X for failed)
- Connecting vertical line between steps (solid green for done section, dashed gray for pending)
- Right: title (bold), description (gray), AI message (if present), PR link (if present)
- Completed steps: reduced opacity (`opacity-60`), collapsed
- Active step: highlighted with blue border, expanded
- Failed step: red border, error message, retry button
- Waiting step: amber border, "Waiting for PR merge" with link, "Continue" button

- [ ] **Step 2: Build MigrationStepper.tsx**

Vertical layout wrapping MigrationStep components:
- Takes `steps: MigrationStep[]` and `currentStep: number` as props
- Action callbacks: `onContinue`, `onRetry`
- Auto-scrolls to active step

- [ ] **Step 3: Commit**

```bash
git add ui/src/components/MigrationStepper.tsx ui/src/components/MigrationStep.tsx
git commit -m "feat: add migration stepper UI components"
```

---

## Task 8: Frontend — Migration Pages

**Files:**
- Create: `ui/src/views/MigrationPage.tsx`
- Create: `ui/src/views/MigrationDetail.tsx`
- Create: `ui/src/components/NewMigrationDialog.tsx`

- [ ] **Step 1: Build NewMigrationDialog.tsx**

Dialog with:
- Scope selector: "Single Addon" / "Entire Cluster" radio buttons
- Addon dropdown (populated from API — addon catalog)
- Cluster dropdown (populated from API — cluster list)
- If "Entire Cluster": show list of addons that will be migrated
- "Start Migration" button

- [ ] **Step 2: Build MigrationPage.tsx**

Two sections:
- **Top**: MigrationSettings component
- **Bottom**: Migration list table with columns: Addon, Cluster, Status (badge), Current Step, Started, Actions
- "New Migration" button (disabled if settings not configured)
- Click row → navigate to `/migration/{id}`

- [ ] **Step 3: Build MigrationDetail.tsx**

- Header: addon name + cluster name + overall status badge
- MigrationStepper showing all 10 steps
- Control buttons: Continue (when waiting), Pause (when running), Cancel
- Polls `GET /api/v1/migration/{id}` every 3s when running, 10s when waiting
- Stops polling when completed/failed/cancelled

- [ ] **Step 4: Commit**

```bash
git add ui/src/views/MigrationPage.tsx ui/src/views/MigrationDetail.tsx ui/src/components/NewMigrationDialog.tsx
git commit -m "feat: add migration wizard pages and new migration dialog"
```

---

## Task 9: Frontend — Route Registration + Nav

**Files:**
- Modify: `ui/src/App.tsx`
- Modify: `ui/src/components/Layout.tsx`

- [ ] **Step 1: Add routes to App.tsx**

```tsx
import MigrationPage from './views/MigrationPage'
import MigrationDetail from './views/MigrationDetail'

// Inside routes:
<Route path="/migration" element={<MigrationPage />} />
<Route path="/migration/:id" element={<MigrationDetail />} />
```

- [ ] **Step 2: Add nav item to Layout.tsx**

Add after "Upgrade Checker" in the navItems array:

```tsx
import { GitPullRequest } from 'lucide-react'

{ to: '/migration', label: 'Migration', icon: GitPullRequest },
```

- [ ] **Step 3: Verify UI renders**

Run: `cd ui && npm run build`

- [ ] **Step 4: Commit**

```bash
git add ui/src/App.tsx ui/src/components/Layout.tsx
git commit -m "feat: add migration route and sidebar navigation"
```

---

## Summary

After all 9 tasks:

| Layer | What's built |
|-------|-------------|
| **Azure DevOps** | Full GitProvider (read + write) with PAT auth |
| **Migration backend** | Models, JSON store, 10-step executor with AI evaluation |
| **API** | 10 endpoints for settings, CRUD, and step control |
| **UI — Settings** | OLD repo + ArgoCD config form with connection testing |
| **UI — Stepper** | Visual step-by-step component with status indicators |
| **UI — Pages** | Migration list, new migration dialog, detail view with live polling |
| **Navigation** | `/migration` route + sidebar entry |

The migration wizard will allow a DevOps engineer to: configure OLD repo once → start migration → watch the AI execute each step → approve PRs when prompted → see the migration complete with full audit trail.
