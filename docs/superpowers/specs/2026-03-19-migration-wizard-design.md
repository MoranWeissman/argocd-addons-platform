# Migration Wizard — Design Spec

**Issue:** #18 — Migration Wizard
**Date:** 2026-03-19
**Status:** Draft

---

## 1. Overview

A dedicated Migration page in AAP that automates the 10-step addon migration from OLD ArgoCD to NEW ArgoCD. The OLD repo can be either GitHub or Azure DevOps. The AI agent acts as the DevOps engineer — executing steps, verifying health between steps, and deciding whether to continue or stop.

Each migration is a trackable session with persistent state. The UI shows a visual stepper where completed steps gray out, the current step is highlighted, and each step has an explanation of what's happening and why.

**Principle:** The AI does the work. The human approves PRs and monitors progress.

---

## 2. User Flow

### 2.1 Migration Page (`/migration`)

Two sections:

**Top: Migration Settings (one-time setup)**
- OLD Git repo config: provider (GitHub or Azure DevOps), then provider-specific fields (owner/repo/token for GitHub, organization/project/repository/PAT for Azure DevOps)
- OLD ArgoCD config: server URL, token, namespace
- "Test Connection" buttons for both — must pass before migrations are allowed
- These settings persist (saved in config store) so they're not re-entered per migration

**Bottom: Migration List**
- Table of all migrations: addon name, cluster name, status (active/completed/failed), current step, started at, PR links
- "New Migration" button (disabled until settings are configured and tested)

### 2.2 New Migration Dialog

1. Select scope: "Single addon" or "Entire cluster"
2. If single addon: pick addon from dropdown (populated from NEW repo catalog), pick cluster from dropdown
3. If entire cluster: pick cluster, shows all addons that exist in OLD but will be migrated
4. Confirm — creates a migration session, redirects to migration detail view

### 2.3 Migration Detail View (`/migration/:id`)

**Visual stepper** showing all 10 steps vertically:

```
 [1] ✅ Verify addon in NEW catalog          Done — istiod found with inMigration: true
 [2] ✅ Configure values in NEW repo          Done — PR #42 created and merged
 [3] ✅ Enable addon on cluster (NEW)         Done — PR #43 created and merged
 [4] ✅ Verify app created in NEW ArgoCD      Done — istiod-feedlot-dev exists, OutOfSync
 [5] 🔄 Disable addon in OLD repo             In progress — AI creating PR on Azure DevOps...
 [6] ⏳ Sync clusters app in OLD ArgoCD       Pending
 [7] ⏳ Verify app removed from OLD ArgoCD    Pending
 [8] ⏳ Hard refresh in NEW ArgoCD            Pending
 [9] ⏳ Verify healthy in NEW ArgoCD          Pending
 [10] ⏳ Disable migration mode               Pending
```

Each step shows:
- Step number and title
- Status icon: ✅ completed (grayed out), 🔄 in progress (highlighted), ⏳ pending, ❌ failed
- Explanation text: what the AI did, what it found, why it's proceeding or stopping
- PR link (if a PR was created at this step)
- "Waiting for PR merge" indicator with link when applicable

**Controls:**
- "Continue" button — enabled when waiting for user action (PR merge)
- "Pause" button — stops the AI from proceeding to next step
- "Retry" button — on failed steps, retry the current step
- "Cancel Migration" — stops and marks as cancelled

### 2.4 AI Decision Points

The AI evaluates health/status between steps and decides:
- **Continue**: Everything looks good, proceed to next step
- **Wait**: PR created, waiting for merge (shows "Continue" button)
- **Pause**: Something unexpected (e.g., app stuck OutOfSync after 3 refreshes) — explains what's wrong, asks user what to do
- **Fail**: Critical error (e.g., ArgoCD unreachable, file not found) — stops migration, shows error

---

## 3. Architecture

### 3.1 Migration State Machine (Backend)

New package `internal/migration/`:

```go
type MigrationStatus string

const (
    StatusPending    MigrationStatus = "pending"
    StatusRunning    MigrationStatus = "running"
    StatusWaiting    MigrationStatus = "waiting"     // waiting for PR merge
    StatusPaused     MigrationStatus = "paused"       // AI paused, needs user input
    StatusCompleted  MigrationStatus = "completed"
    StatusFailed     MigrationStatus = "failed"
    StatusCancelled  MigrationStatus = "cancelled"
)

type StepStatus string

const (
    StepPending    StepStatus = "pending"
    StepRunning    StepStatus = "running"
    StepWaiting    StepStatus = "waiting"
    StepCompleted  StepStatus = "completed"
    StepFailed     StepStatus = "failed"
    StepSkipped    StepStatus = "skipped"
)

type MigrationStep struct {
    Number      int        `json:"number"`
    Title       string     `json:"title"`
    Description string     `json:"description"`    // what this step does
    Status      StepStatus `json:"status"`
    Message     string     `json:"message"`         // AI explanation of what happened
    PRURL       string     `json:"pr_url,omitempty"`
    PRStatus    string     `json:"pr_status,omitempty"` // open, merged, closed
    StartedAt   string     `json:"started_at,omitempty"`
    CompletedAt string     `json:"completed_at,omitempty"`
    Error       string     `json:"error,omitempty"`
}

type Migration struct {
    ID           string           `json:"id"`
    AddonName    string           `json:"addon_name"`
    ClusterName  string           `json:"cluster_name"`
    Status       MigrationStatus  `json:"status"`
    CurrentStep  int              `json:"current_step"`
    Steps        []MigrationStep  `json:"steps"`
    CreatedAt    string           `json:"created_at"`
    UpdatedAt    string           `json:"updated_at"`
    CompletedAt  string           `json:"completed_at,omitempty"`
    Error        string           `json:"error,omitempty"`
}
```

**State persistence:** Migrations are stored as JSON files in a data directory (`/data/migrations/` in K8s, `./data/migrations/` locally). Each migration gets its own file: `{id}.json`.

### 3.2 Migration Steps Definition

The 10 steps, mapped to the migration guide:

| Step | Title | Action | Creates PR? | Repo | Needs AI Decision? |
|------|-------|--------|-------------|------|-------------------|
| 1 | Verify addon in NEW catalog | Check addons-catalog.yaml has the addon with inMigration: true | No | NEW (GitHub) | Yes — verify config |
| 2 | Configure values in NEW repo | Verify global + cluster values match OLD repo | No (or PR if values need updating) | NEW (GitHub) | Yes — compare values |
| 3 | Enable addon on cluster | Set addon: enabled in cluster-addons.yaml | Yes — PR on GitHub | NEW (GitHub) | No |
| 4 | Verify app created in NEW ArgoCD | Check ArgoCD for the new application | No | — | Yes — check status |
| 5 | Disable addon in OLD repo | Comment out/disable addon label in OLD clusters.yaml | Yes — PR on OLD repo | OLD (GitHub or Azure DevOps) | No |
| 6 | Sync clusters app in OLD ArgoCD | Trigger sync on OLD ArgoCD | No | — | No |
| 7 | Verify app removed from OLD | Check OLD ArgoCD no longer has the application | No | — | Yes — verify removal |
| 8 | Hard refresh in NEW ArgoCD | Trigger hard refresh to adopt orphaned resources | No | — | No |
| 9 | Verify healthy in NEW ArgoCD | Check app is Synced + Healthy, no pod restarts | No | — | Yes — critical check |
| 10 | Disable migration mode | Set inMigration: false in addons-catalog.yaml | Yes — PR on GitHub | NEW (GitHub) | No |

### 3.3 Migration Executor

The executor runs steps sequentially. For each step:

1. Set step status to `running`
2. Execute the step action
3. AI evaluates the result
4. Set step to `completed`, `waiting`, or `failed` with explanation message
5. If `waiting` (PR created), stop and wait for user to click "Continue"
6. If `completed`, advance to next step
7. If `failed`, stop migration

The AI evaluation is key — it's not just checking a boolean. It reads ArgoCD app details, compares expected vs actual state, and writes a human-readable explanation.

### 3.4 Azure DevOps Write Implementation

Implement the GitProvider write methods for Azure DevOps:

```go
// Azure DevOps REST API endpoints:
// Create branch:   POST https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/refs?api-version=7.1
// Create/update:   POST https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/pushes?api-version=7.1
// Create PR:       POST https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/pullrequests?api-version=7.1
// Get PR status:   GET  https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/pullrequests/{id}?api-version=7.1
```

Auth: PAT as Basic auth (`Authorization: Basic base64(:PAT)`).

### 3.5 Migration Settings Storage

Add to the config store — migration settings are separate from connections:

```go
type MigrationSettings struct {
    OldGit    GitRepoConfig  `json:"old_git" yaml:"old_git"`
    OldArgocd ArgocdConfig   `json:"old_argocd" yaml:"old_argocd"`
    Configured bool          `json:"configured"`   // true after successful connection test
}
```

Stored in config.yaml under a `migration:` key, or as a separate file.

### 3.6 API Endpoints

```
# Migration settings
GET    /api/v1/migration/settings              — get migration settings
POST   /api/v1/migration/settings              — save migration settings
POST   /api/v1/migration/settings/test         — test OLD connections

# Migration CRUD
GET    /api/v1/migration/list                  — list all migrations
POST   /api/v1/migration/start                 — start new migration
GET    /api/v1/migration/:id                   — get migration detail + steps
POST   /api/v1/migration/:id/continue          — continue after PR merge
POST   /api/v1/migration/:id/pause             — pause migration
POST   /api/v1/migration/:id/retry             — retry failed step
POST   /api/v1/migration/:id/cancel            — cancel migration

# Request body for start:
{
  "addon_name": "istiod",
  "cluster_name": "feedlot-dev",
  "scope": "addon"  // or "cluster" for all addons
}
```

### 3.7 Frontend Components

**New route:** `/migration` and `/migration/:id`

**New components:**
- `MigrationPage.tsx` — main page with settings panel + migration list
- `MigrationDetail.tsx` — detail view with visual stepper
- `MigrationStepper.tsx` — the visual step-by-step component
- `MigrationStep.tsx` — individual step card with status, message, PR link
- `MigrationSettings.tsx` — settings form for OLD repo + OLD ArgoCD
- `NewMigrationDialog.tsx` — dialog to start a new migration

**Stepper design:**
- Vertical layout, each step is a card
- Left side: step number in a circle (green ✓ for done, blue spinner for active, gray for pending, red ✗ for failed)
- Connecting line between circles (solid green for done, dashed gray for pending)
- Right side: title, description, AI message, PR link
- Completed steps have reduced opacity and collapsed detail
- Active step is expanded with full detail and any action buttons
- Failed step shows error in red with retry button

---

## 4. OLD Repo Structure Differences

The migration wizard needs to understand BOTH repo structures:

**NEW repo (GitHub) — V2 pattern:**
- `configuration/cluster-addons.yaml` — clusters with labels
- `configuration/addons-catalog.yaml` — addon definitions

**OLD repo (Azure DevOps) — V1 pattern:**
- `values/clusters.yaml` — clusters with labels
- `values/addons-list.yaml` — addon definitions per environment
- `values/addons-config/overrides/{cluster}/{addon}.yaml` — per-cluster configs

The YAML mutator needs to handle both structures. For the OLD repo, disabling an addon means finding the cluster in `values/clusters.yaml` and setting the label to `disabled` (same pattern, different file path).

---

## 5. AI Agent Integration

The migration executor doesn't run the AI agent for every step. Instead:

**Steps that need AI reasoning:**
- Step 1: AI reads catalog, checks inMigration flag, reports findings
- Step 2: AI compares OLD and NEW values, identifies mismatches
- Step 4: AI checks NEW ArgoCD app status, interprets OutOfSync as expected
- Step 7: AI verifies OLD app is gone, handles edge cases (stuck finalizers)
- Step 9: AI checks Synced + Healthy, verifies no pod restarts, decides if migration succeeded

**Steps that are deterministic:**
- Step 3: Create PR (mechanical — use GitProvider directly)
- Step 5: Create PR (mechanical)
- Step 6: Trigger sync (mechanical)
- Step 8: Trigger hard refresh (mechanical)
- Step 10: Create PR (mechanical)

For AI steps, the executor calls the AI agent with a focused prompt like:
```
Migration step 4: Verify that application {addon}-{cluster} exists in NEW ArgoCD.
Expected state: Application exists, may be OutOfSync (normal during migration).
Current application status: {json from ArgoCD API}
Analyze: Is this expected? Should we continue to step 5?
```

The AI returns a structured response with `continue: true/false` and an explanation message that's shown in the UI.

---

## 6. Polling and Real-time Updates

The migration detail page polls `GET /api/v1/migration/:id` every 3 seconds while the migration is running. This returns the full migration state including all step statuses and messages.

When the migration is in `waiting` status (PR created), polling slows to every 10 seconds.

When completed/failed/cancelled, polling stops.

---

## 7. Security

- Migration settings (OLD repo PAT, OLD ArgoCD token) stored in the same config store as connections — env var expansion supported
- PR creation requires write access to both repos
- All actions are auditable — each step logs what was done and by whom
- Migration data persists across server restarts

---

## 8. Build Order

### Phase 1 — Backend (migration state machine + Azure DevOps write)
1. Azure DevOps GitProvider write methods
2. Migration models and state persistence (JSON files)
3. Migration executor (10-step workflow)
4. Migration settings API endpoints
5. Migration CRUD API endpoints
6. AI integration for decision steps

### Phase 2 — Frontend (Migration page + stepper UI)
1. Migration settings panel component
2. Migration list component
3. New migration dialog
4. Migration stepper component
5. Migration detail page
6. Route registration and sidebar nav

---

## 9. Files to Create/Modify

### New Files
- `internal/migration/models.go` — Migration, MigrationStep, MigrationSettings types
- `internal/migration/store.go` — JSON file-based migration persistence
- `internal/migration/executor.go` — Step-by-step migration execution
- `internal/migration/steps.go` — Individual step implementations
- `internal/api/migration.go` — API handlers for migration endpoints
- `internal/gitprovider/azuredevops_write.go` — Azure DevOps write methods
- `internal/gitprovider/azuredevops_write_test.go` — Tests
- `ui/src/views/MigrationPage.tsx` — Main migration page
- `ui/src/views/MigrationDetail.tsx` — Detail view with stepper
- `ui/src/components/MigrationStepper.tsx` — Visual stepper component
- `ui/src/components/MigrationStep.tsx` — Individual step card
- `ui/src/components/MigrationSettings.tsx` — Settings form
- `ui/src/components/NewMigrationDialog.tsx` — New migration dialog

### Modified Files
- `internal/gitprovider/azuredevops.go` — Replace stubs with real implementation
- `internal/api/router.go` — Add `/api/v1/migration/*` routes
- `internal/config/store.go` — Add migration settings storage
- `cmd/aap-server/main.go` — Wire migration service
- `ui/src/App.tsx` — Add `/migration` route
- `ui/src/components/Layout.tsx` — Add Migration to sidebar nav
- `ui/src/services/api.ts` — Add migration API methods
