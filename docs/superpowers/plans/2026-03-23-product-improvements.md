# Product Improvements Plan

> Ordered by priority. Execute sequentially, commit after each item.

---

## 1. Session Token Memory Leak

**Problem:** `activeSessions` map in `internal/api/router.go` grows unbounded. Expired tokens are checked on validation but never cleaned from the map.

**Fix:** Add a background goroutine in `NewRouter()` that cleans expired tokens every hour.

**Files:**
- Modify: `internal/api/router.go`

**Implementation:**
```go
// In NewRouter(), after activeSessions is initialized, add:
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    for range ticker.C {
        sessionsMu.Lock()
        now := time.Now()
        for token, expiry := range activeSessions {
            if now.After(expiry) {
                delete(activeSessions, token)
            }
        }
        sessionsMu.Unlock()
    }
}()
```

- [ ] Add cleanup goroutine
- [ ] Verify build
- [ ] Commit: `fix: add session token cleanup goroutine to prevent memory leak`

---

## 2. Request Body Size Limits

**Problem:** No limit on JSON payload size. A large POST could cause OOM.

**Fix:** Add `http.MaxBytesReader` as middleware wrapping request bodies. 1MB default.

**Files:**
- Modify: `internal/api/router.go`

**Implementation:**
Add middleware function:
```go
func maxBodySize(next http.Handler, maxBytes int64) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Body != nil {
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
        }
        next.ServeHTTP(w, r)
    })
}
```

Wrap the mux in `NewRouter()`:
```go
return maxBodySize(mux, 1<<20) // 1MB limit
```

- [ ] Add maxBodySize middleware
- [ ] Wrap mux with it
- [ ] Verify build
- [ ] Commit: `fix: add 1MB request body size limit to prevent OOM`

---

## 3. Observability — Hide Columns Without Datadog

**Problem:** CPU/Memory columns show "N/A" when Datadog isn't configured, making the page feel broken.

**Fix:** In `ui/src/views/Observability.tsx`, check `datadogEnabled` state and conditionally render CPU/Memory columns only when Datadog is configured.

**Files:**
- Modify: `ui/src/views/Observability.tsx`

**Implementation:**
- Find where the table headers for CPU/Memory are rendered
- Wrap them in `{datadogEnabled && ...}`
- Do the same for the data cells
- When Datadog is not configured, those columns simply don't appear

- [ ] Find datadogEnabled state variable
- [ ] Conditionally render CPU/Memory table headers
- [ ] Conditionally render CPU/Memory table cells
- [ ] Verify UI build
- [ ] Commit: `fix: hide CPU/Memory columns in observability when Datadog not configured`

---

## 4. YAML Parsing — Replace String Scanning with yaml.v3

**Problem:** `isAddonAlreadySet()` in `steps.go` and `inMigration` checks in `stepVerifyCatalog()` use line-by-line string scanning. Fragile against whitespace, comments, unusual formatting.

**Fix:** Replace READ operations with `gopkg.in/yaml.v3` unmarshaling. Keep WRITE operations (yaml_mutator.go) line-based to preserve formatting.

**Files:**
- Modify: `internal/migration/steps.go` — replace `isAddonAlreadySet` and catalog checks
- Keep: `internal/gitops/yaml_mutator.go` — unchanged (writes need formatting preservation)

**Implementation:**

Replace `isAddonAlreadySet`:
```go
// clusterAddonsFile represents the YAML structure of cluster-addons.yaml
type clusterAddonsFile struct {
    Clusters []struct {
        Name   string            `yaml:"name"`
        Labels map[string]string `yaml:"labels"`
    } `yaml:"clusters"`
}

func isAddonAlreadySet(data []byte, clusterName, addonName, value string) bool {
    var f clusterAddonsFile
    if err := yaml.Unmarshal(data, &f); err != nil {
        return false
    }
    for _, c := range f.Clusters {
        if c.Name == clusterName {
            return c.Labels[addonName] == value
        }
    }
    return false
}
```

Replace catalog `inMigration` check in `stepVerifyCatalog`:
```go
type addonsCatalog struct {
    ApplicationSets []struct {
        AppName     string `yaml:"appName"`
        InMigration bool   `yaml:"inMigration"`
    } `yaml:"applicationsets"`
}

func isAddonInMigration(data []byte, addonName string) (found bool, inMigration bool) {
    var c addonsCatalog
    if err := yaml.Unmarshal(data, &c); err != nil {
        return false, false
    }
    for _, a := range c.ApplicationSets {
        if a.AppName == addonName {
            return true, a.InMigration
        }
    }
    return false, false
}
```

- [ ] Add yaml.v3 import (already a dependency via go.mod)
- [ ] Replace `isAddonAlreadySet` with YAML-based version
- [ ] Replace catalog string checks with `isAddonInMigration`
- [ ] Update `stepVerifyCatalog` to use new function
- [ ] Verify build + run existing tests
- [ ] Commit: `fix: replace fragile string-based YAML parsing with yaml.v3 unmarshaling`

---

## 5. Batch Migration UI

**Problem:** Batch API exists but the migration page has no batch-aware UI. Users can't see queue progress.

**Fix:** Add a batch progress component to the migration page that shows:
- Current addon being migrated (with link to detail)
- Queue of pending/completed addons
- Overall progress bar

**Files:**
- Create: `ui/src/components/BatchProgress.tsx`
- Modify: `ui/src/views/Migration.tsx` — show batch banner when active batch exists

**Implementation:**

`BatchProgress.tsx`:
```tsx
// Shows: "Migrating 3 of 12 addons on cluster-prod-1"
// Progress bar
// List of addons: ✓ completed | ► current (link) | ○ pending
// Each addon shows its migration status
```

`Migration.tsx`:
```tsx
// On load, call api.getActiveBatch()
// If batch exists, show BatchProgress component above the migration list
// Poll batch status every 5 seconds
// When batch completes, show success banner
```

- [ ] Create BatchProgress component
- [ ] Add batch state to Migration view
- [ ] Poll for batch updates
- [ ] Show completed/current/pending addons with status
- [ ] Link current addon to its migration detail page
- [ ] Build UI
- [ ] Commit: `feat: batch migration progress UI with queue visualization`

---

## 6. Migration Rollback

**Problem:** If migration fails mid-way (e.g., step 5 done but step 9 fails), there's no way to undo. Addon is in split state.

**Fix:** Add a "Rollback" action that reverses completed PR steps.

**Files:**
- Add: `internal/migration/rollback.go`
- Modify: `internal/migration/executor.go` — add `RollbackMigration` method
- Modify: `internal/api/migration.go` — add `POST /migration/{id}/rollback` endpoint
- Modify: `internal/api/router.go` — register route
- Modify: `ui/src/views/MigrationDetail.tsx` — add Rollback button on failed migrations

**Implementation:**

Rollback logic (reverses steps in order):
- If step 3 completed (addon enabled in NEW): create PR to disable it in NEW repo
- If step 5 completed (addon disabled in OLD): create PR to re-enable it in OLD repo
- Steps 1, 2, 4, 6-10 are read-only/verification — no rollback needed
- Mark migration as "rolled_back" status

```go
func (e *Executor) RollbackMigration(ctx context.Context, migrationID string) error {
    m, err := e.store.GetMigration(migrationID)
    // Check status is failed or cancelled

    // Reverse step 5 if completed: re-enable addon in OLD repo
    if m.Steps[4].Status == StepCompleted {
        // Read OLD repo cluster file
        // Use gitops.EnableAddonLabel to re-enable
        // Create PR
    }

    // Reverse step 3 if completed: disable addon in NEW repo
    if m.Steps[2].Status == StepCompleted {
        // Read NEW repo cluster-addons.yaml
        // Use gitops.DisableAddonLabel
        // Create PR
    }

    m.Status = "rolled_back"
    return e.store.SaveMigration(m)
}
```

- [ ] Create rollback.go with RollbackMigration
- [ ] Add API endpoint
- [ ] Register route
- [ ] Add Rollback button in UI (only on failed/cancelled migrations)
- [ ] Build + verify
- [ ] Commit: `feat: migration rollback — reverses PR steps on failed migrations`

---

## 7. Documentation Overhaul

**Problem:** Docs page is static JSX in `ui/src/views/Docs.tsx`. Not searchable, not editable without deploys, not readable on GitHub.

**Fix:** Move docs to markdown files in `docs/user-guide/`. Render in UI with a markdown renderer. Same files are readable on GitHub.

**Files:**
- Create: `docs/user-guide/01-overview.md`
- Create: `docs/user-guide/02-features.md`
- Create: `docs/user-guide/03-managing-addons.md`
- Create: `docs/user-guide/04-values-guide.md`
- Create: `docs/user-guide/05-troubleshooting.md`
- Create: `docs/user-guide/06-migration.md`
- Create: `docs/user-guide/07-ai-assistant.md`
- Modify: `ui/src/views/Docs.tsx` — replace static JSX with markdown fetcher + renderer
- Add: `react-markdown` + `remark-gfm` dependencies to ui/package.json
- Modify: API to serve markdown files OR embed them at build time

**Implementation approach:**
- Embed markdown files into the Go binary (like static UI files)
- Add API endpoint: `GET /api/v1/docs/{slug}` returns markdown content
- UI fetches markdown, renders with react-markdown
- Sidebar generated from file list
- Search: simple client-side text search across all loaded docs
- GitHub: same files visible at `docs/user-guide/` in the repo

**Content to extract from current Docs.tsx:**
- Overview: what AAP does, architecture diagram
- Features: cluster monitoring, addon catalog, version matrix, observability, migration
- Managing Addons: how to add/remove/configure
- Values Guide: global defaults, cluster overrides, precedence
- Troubleshooting: common issues, ArgoCD errors, connectivity
- Migration: step-by-step guide, modes, troubleshooting
- AI Assistant: what it can do, example prompts, provider setup

- [ ] Extract current docs content from Docs.tsx into markdown files
- [ ] Add react-markdown + remark-gfm to UI dependencies
- [ ] Create API endpoint to serve docs (or embed at build time)
- [ ] Rewrite Docs.tsx to fetch and render markdown
- [ ] Add sidebar navigation from file list
- [ ] Add simple search across doc content
- [ ] Verify GitHub readability
- [ ] Build UI + Go
- [ ] Commit: `feat: move documentation to markdown files, render in UI, readable on GitHub`
