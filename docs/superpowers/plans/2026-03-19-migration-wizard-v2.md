# Migration Wizard v2 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development or superpowers:executing-plans.

**Goal:** Add live logs with repo context, YOLO/gates mode, value scaffolding, session management, and YAML mutator fixes.

**Architecture:** Extend Migration model with mode/logs, update executor to emit log entries, update steps with repo context, update frontend with live log stream and mode selector.

---

## Task 1: Model Changes — Mode, Logs, Repo Context

**Files:** `internal/migration/models.go`

Add to Migration struct:
- `Mode string` — "yolo" or "gates"
- `Logs []LogEntry` — activity log stream

Add LogEntry type:
```go
type LogEntry struct {
    Timestamp string `json:"timestamp"`
    Step      int    `json:"step"`
    Repo      string `json:"repo"`      // "NEW (github.com/org/repo)" or "OLD (dev.azure.com/org/project/repo)"
    Action    string `json:"action"`    // "reading", "creating", "comparing", "verifying", etc.
    Detail    string `json:"detail"`    // human-readable message
}
```

Add `StatusGated MigrationStatus = "gated"` — paused at gate, waiting for user approval.

Update `NewMigration` to accept mode parameter.

---

## Task 2: Executor — Log Emission and Gates Mode

**Files:** `internal/migration/executor.go`, `internal/migration/steps.go`

Add `addLog` helper to executor:
```go
func (e *Executor) addLog(m *Migration, step int, repo, action, detail string) {
    m.Logs = append(m.Logs, LogEntry{...})
    _ = e.store.SaveMigration(m)
}
```

Update `RunSteps` to check mode after each step:
- If mode == "gates" and step completed successfully, set status to `StatusGated` and stop (wait for user approval)
- If mode == "yolo", auto-continue

Add repo context strings:
```go
func (e *Executor) newRepoLabel() string  // "NEW (github.com/owner/repo)"
func (e *Executor) oldRepoLabel() string  // "OLD (dev.azure.com/org/project/repo)"
```

Update ALL step functions to emit logs before and after each action:
- Step 1: `addLog(m, 1, newRepo, "reading", "Reading addons-catalog.yaml...")`
- Step 3: `addLog(m, 3, newRepo, "creating", "Creating branch aap/migration/...")`
- Step 5: `addLog(m, 5, oldRepo, "reading", "Reading clusters.yaml from OLD repo...")`
- etc.

---

## Task 3: Session Management — One Active Migration

**Files:** `internal/migration/executor.go`, `internal/api/migration.go`

In `handleStartMigration`:
- Before creating, check if any active migration exists (running/paused/waiting/gated)
- If yes, return error "An active migration already exists. Cancel or complete it first."

In `handleListMigrations`:
- Include cancelled/paused/failed migrations so they can be resumed

Add `handleResumeMigration` endpoint (or reuse Continue):
- Resume from current step for paused/cancelled migrations
- Set status back to running

---

## Task 4: Configure Values — Smart Scaffolding

**Files:** `internal/migration/steps.go`

Rewrite `stepConfigureValues`:
1. Read addon global values from OLD repo: `values/addons-config/defaults.yaml` (V1) or `configuration/addons-global-values/{addon}.yaml` (V2)
2. Read addon global values from NEW repo: `configuration/addons-global-values/{addon}.yaml`
3. If NEW file doesn't exist → create it based on OLD values or cluster-example.yaml pattern
4. Read cluster values from OLD repo: `values/addons-config/overrides/{cluster}/{addon}.yaml` (V1) or `configuration/addons-clusters-values/{cluster}.yaml` (V2)
5. Read cluster values from NEW repo: `configuration/addons-clusters-values/{cluster}.yaml`
6. Compare and log differences
7. If files need creating, create them via PR

Log each step with repo context.

---

## Task 5: YAML Mutator Fixes

**Files:** `internal/gitops/yaml_mutator.go`, `internal/gitops/yaml_mutator_test.go`

Fix `labels: []` — when the labels block is an empty array, replace with:
```yaml
    labels:
      {addon}: enabled
```

Detect commented-out labels — if `# addon: enabled` exists, uncomment it instead of adding a new line.

Add tests for both cases.

---

## Task 6: Frontend — Mode Selector in New Migration Dialog

**Files:** `ui/src/components/NewMigrationDialog.tsx`

Add mode selector after scope selection:
- Two options: "YOLO (fully automatic)" and "Gates (approve each step)"
- Default: Gates
- Pass mode to `startMigration` API call

Update API request body: `{ addon_name, cluster_name, mode: "yolo" | "gates" }`

---

## Task 7: Frontend — Live Logs in Migration Detail

**Files:** `ui/src/views/MigrationDetail.tsx`, `ui/src/components/MigrationStep.tsx`

Add log stream panel below the stepper:
- Shows LogEntry items with timestamp, repo badge, action, detail
- Auto-scrolls to bottom as new logs arrive
- Repo badge: green for NEW, orange for OLD
- Collapsible (expanded by default when migration is running)

Update MigrationStep to show repo context in step header.

Add "Approve & Continue" button when status is "gated" (gates mode).

---

## Task 8: Frontend — Session Management UI

**Files:** `ui/src/views/MigrationPage.tsx`, `ui/src/components/NewMigrationDialog.tsx`

- Disable "New Migration" button if any migration is active (running/paused/waiting/gated)
- Show "Resume" button on paused/cancelled migrations
- Show active migration prominently at top of list
