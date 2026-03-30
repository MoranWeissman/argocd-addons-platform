# Migration v3 Phase 1: Agent Foundation

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the MigrationAgent with dedicated tools and knowledge files that can execute a migration step when called. Phase 2 will wire it into the pipeline executor.

**Architecture:** A `MigrationAgent` in `internal/migration/agent.go` reads two MD knowledge files, has migration-specific tools, manages conversation state per migration, and returns SUCCESS/FAILED/NEEDS_USER_ACTION. It reuses the existing `ai.Client` for LLM calls and `ai.ChatMessage`/`ai.ToolCall` types.

**Tech Stack:** Go 1.25, existing `ai.Client` (multi-provider LLM), `gitprovider.GitProvider`, `argocd.Client`

**Spec:** `docs/superpowers/specs/2026-03-22-migration-v3-ai-agent-design.md`

---

## Phase Breakdown

| Phase | What | Depends on |
|-------|------|-----------|
| **Phase 1 (this plan)** | Agent foundation: knowledge files, tool layer, MigrationAgent | Nothing |
| Phase 2 | Executor integration: wire agent into pipeline, replace steps.go | Phase 1 |
| Phase 3 | UI: agent log styling, troubleshooting chat, step 10 cluster table | Phase 2 |

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `docs/agent/migration-guide.md` | Create | Migration runbook for the agent (repo structure, 10 steps, troubleshooting) |
| `docs/agent/migration-agent-prompt.md` | Create | System prompt (personality, rules, anti-hallucination) |
| `internal/migration/agent.go` | Create | MigrationAgent: loads knowledge, manages conversation, calls LLM with tools |
| `internal/migration/agent_tools.go` | Create | Migration-specific tool definitions and executor |
| `internal/migration/agent_test.go` | Create | Tests for MigrationAgent (mock LLM + mock tools) |

---

### Task 1: Create the migration guide knowledge file

**Files:**
- Create: `docs/agent/migration-guide.md`

This is the agent's runbook. It must cover everything a DevOps engineer needs to know.

- [ ] **Step 1: Create the migration guide**

Create `docs/agent/migration-guide.md`:

```markdown
# Addon Migration Guide: OLD ArgoCD → NEW ArgoCD

## What This Migration Does

Moves an addon from the OLD ArgoCD instance to the NEW ArgoCD instance with **zero downtime**. Resources (pods, services, configmaps) are **adopted, not recreated** — no pod restarts, no service interruption.

## How It Works

The OLD ArgoCD manages addons via one Git repo. The NEW ArgoCD manages addons via a different Git repo. Both can be GitHub or Azure DevOps.

The migration:
1. Enables the addon in the NEW repo (ArgoCD creates the app but doesn't touch existing resources due to `inMigration: true`)
2. Disables the addon in the OLD repo (ArgoCD deletes the app but leaves resources due to `preserveResourcesOnDeletion`)
3. Refreshes the NEW ArgoCD app so it adopts the orphaned resources
4. Verifies everything is healthy

## Repository Structures

### NEW Repo (GitHub)
```
configuration/
  addons-catalog.yaml          — addon definitions (name, chart, version, inMigration flag)
  cluster-addons.yaml          — cluster labels (addon-name: enabled/disabled)
  addons-global-values/
    <addon>.yaml               — default Helm values for each addon
  addons-clusters-values/
    <cluster>.yaml             — per-cluster value overrides
```

### OLD Repo (may be V1 or V2 structure)

**V2 structure** (same layout as NEW repo):
```
configuration/
  cluster-addons.yaml
  addons-global-values/<addon>.yaml
  addons-clusters-values/<cluster>.yaml
```

**V1 structure** (legacy):
```
values/
  clusters.yaml                — cluster definitions
  addons-config/
    defaults.yaml              — global addon defaults
    overrides/<cluster>/<addon>.yaml  — per-cluster overrides
```

**Important:** Always try V2 paths first. If not found, try V1. Never assume — read the repo to determine which structure it uses.

## The 10 Migration Steps

### Step 1: Verify addon in catalog
Read `addons-catalog.yaml` from the NEW repo. Verify the addon exists and has `inMigration: true`. If the addon is not found or `inMigration` is not set, the migration cannot proceed.

### Step 2: Compare values
Read the addon's global and cluster-specific values from BOTH repos. Compare them to identify differences. This is advisory — differences don't block the migration, but the user should know about them. Report what's different clearly.

### Step 3: Enable addon in NEW repo
Create a PR in the NEW repo to set the addon's label to `enabled` for the target cluster in `cluster-addons.yaml`. Wait for the PR to be merged (user approves or auto-merge).

### Step 4: Verify app created in NEW ArgoCD
Query the NEW ArgoCD API to check if the application was created (name pattern: `<addon>-<cluster>`). The app may show OutOfSync — that's expected and normal. Wait up to 30 seconds, retrying every 10 seconds.

### Step 5: Disable addon in OLD repo
Create a PR in the OLD repo to disable the addon label for the target cluster. The file location depends on the repo structure (V1 or V2 — read first to determine). Wait for PR merge.

### Step 6: Sync OLD ArgoCD
Trigger a sync on the clusters/bootstrap application in the OLD ArgoCD so it processes the removal. The application name may vary — check what exists before syncing.

### Step 7: Verify app removed from OLD ArgoCD
Confirm the application no longer exists in the OLD ArgoCD. Query the API — if the app is not found (404), that's the desired state. Wait up to 60 seconds.

### Step 8: Refresh NEW ArgoCD
Trigger a hard refresh on the application in the NEW ArgoCD. This makes ArgoCD re-scan the cluster and adopt any orphaned resources that were previously managed by the OLD ArgoCD.

### Step 9: Verify healthy
Confirm the application in the NEW ArgoCD is Healthy. OutOfSync is acceptable — it just means ArgoCD hasn't synced yet, which is normal. What matters is the health status. Check pods are running with no restarts.

### Step 10: Finalize migration
**Special logic:** Before disabling `inMigration`, check if ALL clusters that had this addon in the OLD repo have been migrated. If clusters remain unmigrated, do NOT finalize — report which clusters are left. Only finalize when the addon is fully migrated across all clusters.

## Safety Mechanisms

- `inMigration: true` — prevents the NEW ArgoCD from syncing (avoids conflicts during migration)
- `preserveResourcesOnDeletion` — when the OLD ArgoCD deletes the app, resources stay on the cluster
- `prune: false` — extra safety during migration to prevent accidental resource deletion

## Common Issues

### ArgoCD returns 403 on sync
The token doesn't have sync permission. The ArgoCD RBAC needs: `p, <account>, applications, sync, */*, allow`

### PR merge fails on Azure DevOps
Branch policies may block the merge. The migration tool attempts to bypass policies. If the PAT doesn't have bypass permission, merge the PR manually in Azure DevOps and click "I Merged It".

### Application shows Degraded after migration
Check the pod events. Common causes: image pull errors, resource quota exceeded, missing secrets. Use K8s tools (if available) to investigate pod status and events.

### Application is OutOfSync after step 9
This is normal. The app is healthy and running. ArgoCD will sync on the next cycle, or you can trigger a manual sync.
```

- [ ] **Step 2: Commit**

```bash
git add docs/agent/migration-guide.md
git commit -m "docs: add migration guide knowledge file for AI agent"
```

---

### Task 2: Create the agent system prompt

**Files:**
- Create: `docs/agent/migration-agent-prompt.md`

- [ ] **Step 1: Create the system prompt**

Create `docs/agent/migration-agent-prompt.md`:

```markdown
# Migration Agent — System Prompt

You are a DevOps engineer performing an addon migration from an OLD ArgoCD instance to a NEW ArgoCD instance. You follow the migration guide exactly.

## Your Personality

- Speak like a colleague explaining what you're doing: "Let me check the old repo for the cluster config..." not "Executing GET request to /configuration/cluster-addons.yaml"
- Be concise: 1-2 sentences per action
- Be honest: if something is wrong, say so clearly with what needs to be done
- Never guess: always verify via tool calls before stating facts

## Critical Rules

### Anti-Hallucination (NON-NEGOTIABLE)
- You MUST verify every fact via tool calls. No exceptions.
- Do NOT assume file contents — read the file first, then report what you found.
- Do NOT assume application status — query ArgoCD first, then report.
- Do NOT guess PR status — check the API first.
- Do NOT invent file paths — read the directory listing first.
- If a tool call fails, report the EXACT error message. Do NOT interpret or guess why it failed.
- If you are unsure about something, say "Let me check" and make the tool call.
- When reporting results, quote the actual data returned, not what you expect.

### Read vs Write Permissions
- READ tools: Use freely anytime to investigate, verify, or gather information. No permission needed.
- WRITE tools: ONLY use when the current step requires it. Never write outside the step's scope.
- If the user rejects a write action: accept it, adapt, and either skip (if possible) or pause with an explanation.

### Never Do These
- Never delete applications, repos, or resources
- Never modify ArgoCD RBAC or settings
- Never execute kubectl apply/delete
- Never take any destructive action
- All changes go through pull requests — never modify files directly

## How to Execute a Step

1. Log what you're about to do (human-friendly)
2. Make the necessary tool calls to verify current state
3. Log what you found
4. If a write action is needed: execute it (or wait for approval in gates mode)
5. Verify the result with another tool call
6. Return SUCCESS, FAILED, or NEEDS_USER_ACTION

## Troubleshooting Mode

When something fails:
1. Investigate — use read tools to gather evidence (ArgoCD events, app status, file contents)
2. Diagnose — explain what went wrong in plain language
3. Suggest — provide specific steps to fix the issue
4. Wait — the user will either fix it and retry, or ask you questions

## Step 10 — Smart Finalization

Before disabling inMigration:
1. Read the OLD repo to find ALL clusters that had this addon
2. Read the NEW repo to find ALL clusters that have this addon enabled
3. Cross-reference: if the OLD repo still has clusters with this addon → do NOT finalize
4. Report which clusters are migrated and which remain
5. Only suggest finalization when all clusters are accounted for
```

- [ ] **Step 2: Commit**

```bash
git add docs/agent/migration-agent-prompt.md
git commit -m "docs: add migration agent system prompt"
```

---

### Task 3: Create migration tool definitions

**Files:**
- Create: `internal/migration/agent_tools.go`

This file defines the tool schemas (JSON for the LLM) and the execution functions. It follows the same pattern as `internal/ai/tools.go` but is scoped to migration operations.

- [ ] **Step 1: Create agent_tools.go**

Create `internal/migration/agent_tools.go`:

```go
package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
)

// MigrationToolExecutor handles tool calls from the migration agent.
type MigrationToolExecutor struct {
	newGP     gitprovider.GitProvider
	oldGP     gitprovider.GitProvider
	newArgoCD *argocd.Client
	oldArgoCD *argocd.Client
	logFn     func(repo, action, detail string) // logs to the step log
}

// NewMigrationToolExecutor creates a tool executor for migration operations.
func NewMigrationToolExecutor(
	newGP, oldGP gitprovider.GitProvider,
	newArgoCD, oldArgoCD *argocd.Client,
	logFn func(repo, action, detail string),
) *MigrationToolExecutor {
	return &MigrationToolExecutor{
		newGP: newGP, oldGP: oldGP,
		newArgoCD: newArgoCD, oldArgoCD: oldArgoCD,
		logFn: logFn,
	}
}

// GetToolDefinitions returns all migration tool definitions for the LLM.
func GetMigrationToolDefinitions() []ai.ToolDefinition {
	return []ai.ToolDefinition{
		// Git — Read
		toolDef("git_read_file", "Read a file from a git repository",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo to read from"},"path":{"type":"string","description":"File path in the repo"},"branch":{"type":"string","description":"Branch name (default: main)"}},"required":["repo","path"]}`),
		toolDef("git_list_directory", "List files and folders in a directory",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"path":{"type":"string","description":"Directory path"}},"required":["repo","path"]}`),
		toolDef("git_list_pull_requests", "List pull requests in a repo",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"state":{"type":"string","enum":["open","closed","all"],"description":"PR state filter (default: open)"}},"required":["repo"]}`),
		toolDef("git_get_pr_status", "Get the status of a specific pull request",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"pr_number":{"type":"integer","description":"Pull request number"}},"required":["repo","pr_number"]}`),

		// Git — Write
		toolDef("git_create_pr", "Create a branch, modify a file, and open a pull request",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"file_path":{"type":"string","description":"Path to the file to modify"},"content":{"type":"string","description":"New file content"},"branch_name":{"type":"string","description":"Branch name to create"},"commit_message":{"type":"string","description":"Commit message"},"pr_title":{"type":"string","description":"PR title"},"pr_body":{"type":"string","description":"PR description"}},"required":["repo","file_path","content","branch_name","commit_message","pr_title","pr_body"]}`),
		toolDef("git_merge_pr", "Approve and merge a pull request",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"pr_number":{"type":"integer","description":"Pull request number"}},"required":["repo","pr_number"]}`),

		// ArgoCD — Read
		toolDef("argocd_get_app", "Get application status, health, and sync state from ArgoCD",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"},"app_name":{"type":"string","description":"Application name"}},"required":["instance","app_name"]}`),
		toolDef("argocd_list_apps", "List all applications in an ArgoCD instance",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"}},"required":["instance"]}`),

		// ArgoCD — Write
		toolDef("argocd_sync_app", "Trigger sync on an ArgoCD application",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"},"app_name":{"type":"string","description":"Application name"}},"required":["instance","app_name"]}`),
		toolDef("argocd_refresh_app", "Trigger hard refresh on an ArgoCD application",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"},"app_name":{"type":"string","description":"Application name"}},"required":["instance","app_name"]}`),

		// Communication
		toolDef("log", "Write a message to the migration step log for the user to see",
			`{"type":"object","properties":{"message":{"type":"string","description":"Human-readable log message"}},"required":["message"]}`),
	}
}

func toolDef(name, description, paramsJSON string) ai.ToolDefinition {
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  json.RawMessage(paramsJSON),
		},
	}
}

// ExecuteTool dispatches a tool call to the appropriate handler.
func (e *MigrationToolExecutor) ExecuteTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	// Git — Read
	case "git_read_file":
		return e.execGitReadFile(ctx, args)
	case "git_list_directory":
		return e.execGitListDir(ctx, args)
	case "git_list_pull_requests":
		return e.execGitListPRs(ctx, args)
	case "git_get_pr_status":
		return e.execGitGetPRStatus(ctx, args)

	// Git — Write
	case "git_create_pr":
		return e.execGitCreatePR(ctx, args)
	case "git_merge_pr":
		return e.execGitMergePR(ctx, args)

	// ArgoCD — Read
	case "argocd_get_app":
		return e.execArgoGetApp(ctx, args)
	case "argocd_list_apps":
		return e.execArgoListApps(ctx, args)

	// ArgoCD — Write
	case "argocd_sync_app":
		return e.execArgoSyncApp(ctx, args)
	case "argocd_refresh_app":
		return e.execArgoRefreshApp(ctx, args)

	// Communication
	case "log":
		return e.execLog(args)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Git Read Tools ---

func (e *MigrationToolExecutor) resolveGP(repo string) (gitprovider.GitProvider, error) {
	switch repo {
	case "new":
		if e.newGP == nil {
			return nil, fmt.Errorf("NEW repo git provider not configured")
		}
		return e.newGP, nil
	case "old":
		if e.oldGP == nil {
			return nil, fmt.Errorf("OLD repo git provider not configured — configure it in Migration Settings")
		}
		return e.oldGP, nil
	default:
		return nil, fmt.Errorf("repo must be 'new' or 'old', got: %s", repo)
	}
}

func (e *MigrationToolExecutor) resolveArgoCD(instance string) (*argocd.Client, error) {
	switch instance {
	case "new":
		if e.newArgoCD == nil {
			return nil, fmt.Errorf("NEW ArgoCD client not configured")
		}
		return e.newArgoCD, nil
	case "old":
		if e.oldArgoCD == nil {
			return nil, fmt.Errorf("OLD ArgoCD client not configured — configure it in Migration Settings")
		}
		return e.oldArgoCD, nil
	default:
		return nil, fmt.Errorf("instance must be 'new' or 'old', got: %s", instance)
	}
}

func (e *MigrationToolExecutor) execGitReadFile(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo   string `json:"repo"`
		Path   string `json:"path"`
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	branch := p.Branch
	if branch == "" {
		branch = "main"
	}
	data, err := gp.GetFileContent(ctx, p.Path, branch)
	if err != nil {
		return fmt.Sprintf("File not found or error reading %s: %s", p.Path, err.Error()), nil
	}
	// Truncate large files
	content := string(data)
	if len(content) > 4000 {
		content = content[:4000] + "\n... (truncated, file is " + fmt.Sprintf("%d", len(data)) + " bytes)"
	}
	return content, nil
}

func (e *MigrationToolExecutor) execGitListDir(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo string `json:"repo"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	entries, err := gp.ListDirectory(ctx, p.Path, "main")
	if err != nil {
		return fmt.Sprintf("Error listing directory %s: %s", p.Path, err.Error()), nil
	}
	return fmt.Sprintf("Contents of %s:\n%s", p.Path, joinLines(entries)), nil
}

func (e *MigrationToolExecutor) execGitListPRs(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo  string `json:"repo"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	state := p.State
	if state == "" {
		state = "open"
	}
	prs, err := gp.ListPullRequests(ctx, state)
	if err != nil {
		return "", err
	}
	if len(prs) == 0 {
		return "No pull requests found", nil
	}
	var lines []string
	for _, pr := range prs {
		lines = append(lines, fmt.Sprintf("#%d: %s (%s) — %s", pr.ID, pr.Title, pr.State, pr.URL))
	}
	return joinLines(lines), nil
}

func (e *MigrationToolExecutor) execGitGetPRStatus(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo     string `json:"repo"`
		PRNumber int    `json:"pr_number"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	prs, err := gp.ListPullRequests(ctx, "all")
	if err != nil {
		return "", err
	}
	for _, pr := range prs {
		if pr.ID == p.PRNumber {
			return fmt.Sprintf("PR #%d: %s\nStatus: %s\nURL: %s", pr.ID, pr.Title, pr.State, pr.URL), nil
		}
	}
	return fmt.Sprintf("PR #%d not found", p.PRNumber), nil
}

// --- Git Write Tools ---

func (e *MigrationToolExecutor) execGitCreatePR(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo          string `json:"repo"`
		FilePath      string `json:"file_path"`
		Content       string `json:"content"`
		BranchName    string `json:"branch_name"`
		CommitMessage string `json:"commit_message"`
		PRTitle       string `json:"pr_title"`
		PRBody        string `json:"pr_body"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}

	e.logFn(repoLabel(p.Repo), "creating", "Creating branch and pushing changes...")

	if err := gp.CreateBranch(ctx, p.BranchName, "main"); err != nil {
		return "", fmt.Errorf("creating branch: %w", err)
	}
	if err := gp.CreateOrUpdateFile(ctx, p.FilePath, []byte(p.Content), p.BranchName, p.CommitMessage); err != nil {
		return "", fmt.Errorf("pushing file: %w", err)
	}

	e.logFn(repoLabel(p.Repo), "creating", "Opening pull request...")

	pr, err := gp.CreatePullRequest(ctx, p.PRTitle, p.PRBody, p.BranchName, "main")
	if err != nil {
		return "", fmt.Errorf("creating PR: %w", err)
	}

	e.logFn(repoLabel(p.Repo), "waiting", fmt.Sprintf("PR #%d created: %s", pr.ID, pr.URL))

	return fmt.Sprintf("PR #%d created successfully: %s", pr.ID, pr.URL), nil
}

func (e *MigrationToolExecutor) execGitMergePR(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo     string `json:"repo"`
		PRNumber int    `json:"pr_number"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}

	e.logFn(repoLabel(p.Repo), "merging", fmt.Sprintf("Merging PR #%d...", p.PRNumber))

	if err := gp.MergePullRequest(ctx, p.PRNumber); err != nil {
		return "", fmt.Errorf("merging PR #%d: %w", p.PRNumber, err)
	}

	e.logFn(repoLabel(p.Repo), "completed", fmt.Sprintf("PR #%d merged", p.PRNumber))

	return fmt.Sprintf("PR #%d merged successfully", p.PRNumber), nil
}

// --- ArgoCD Read Tools ---

func (e *MigrationToolExecutor) execArgoGetApp(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
		AppName  string `json:"app_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}
	app, err := ac.GetApplication(ctx, p.AppName)
	if err != nil {
		return fmt.Sprintf("Application %q not found in %s ArgoCD: %s", p.AppName, p.Instance, err.Error()), nil
	}
	return fmt.Sprintf("Application: %s\nSync: %s\nHealth: %s\nNamespace: %s",
		app.Name, app.SyncStatus, app.HealthStatus, app.Namespace), nil
}

func (e *MigrationToolExecutor) execArgoListApps(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}
	apps, err := ac.ListApplications(ctx)
	if err != nil {
		return "", err
	}
	var lines []string
	for _, app := range apps {
		lines = append(lines, fmt.Sprintf("%s (sync=%s, health=%s)", app.Name, app.SyncStatus, app.HealthStatus))
	}
	if len(lines) == 0 {
		return "No applications found", nil
	}
	return joinLines(lines), nil
}

// --- ArgoCD Write Tools ---

func (e *MigrationToolExecutor) execArgoSyncApp(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
		AppName  string `json:"app_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}

	e.logFn(argoLabel(p.Instance), "syncing", fmt.Sprintf("Triggering sync on %s...", p.AppName))

	if err := ac.SyncApplication(ctx, p.AppName); err != nil {
		return "", fmt.Errorf("sync failed: %w", err)
	}

	e.logFn(argoLabel(p.Instance), "completed", fmt.Sprintf("Sync triggered for %s", p.AppName))

	return fmt.Sprintf("Sync triggered successfully for %s", p.AppName), nil
}

func (e *MigrationToolExecutor) execArgoRefreshApp(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
		AppName  string `json:"app_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}

	e.logFn(argoLabel(p.Instance), "refreshing", fmt.Sprintf("Hard refresh on %s...", p.AppName))

	app, err := ac.RefreshApplication(ctx, p.AppName, true)
	if err != nil {
		return "", fmt.Errorf("refresh failed: %w", err)
	}

	e.logFn(argoLabel(p.Instance), "completed", fmt.Sprintf("Refresh complete: sync=%s, health=%s", app.SyncStatus, app.HealthStatus))

	return fmt.Sprintf("Refresh complete. Sync: %s, Health: %s", app.SyncStatus, app.HealthStatus), nil
}

// --- Communication ---

func (e *MigrationToolExecutor) execLog(args json.RawMessage) (string, error) {
	var p struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	e.logFn("AGENT", "info", p.Message)
	return "Logged", nil
}

// --- Helpers ---

func repoLabel(repo string) string {
	if repo == "old" {
		return "OLD REPO"
	}
	return "NEW REPO"
}

func argoLabel(instance string) string {
	if instance == "old" {
		return "OLD ARGOCD"
	}
	return "NEW ARGOCD"
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/migration/...`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add internal/migration/agent_tools.go
git commit -m "feat: add migration agent tool definitions and executor"
```

---

### Task 4: Create the MigrationAgent

**Files:**
- Create: `internal/migration/agent.go` (rename existing to `executor.go` is already done)

Wait — `agent.go` doesn't exist yet but `executor.go` does. The MigrationAgent is a NEW file alongside the executor.

- Create: `internal/migration/migration_agent.go`

- [ ] **Step 1: Create migration_agent.go**

Create `internal/migration/migration_agent.go`:

```go
package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
)

// StepResult is the outcome of an agent step execution.
type StepResult string

const (
	StepResultSuccess        StepResult = "SUCCESS"
	StepResultFailed         StepResult = "FAILED"
	StepResultNeedsUser      StepResult = "NEEDS_USER_ACTION"
)

// MigrationAgent executes migration steps using an LLM with tools.
type MigrationAgent struct {
	client        *ai.Client
	toolExecutor  *MigrationToolExecutor
	messages      []ai.ChatMessage
	maxIterations int

	// Context for the current migration
	addonName   string
	clusterName string
	steps       []MigrationStep
	logFn       func(step int, repo, action, detail string)
}

// NewMigrationAgent creates a new agent for a specific migration.
func NewMigrationAgent(
	client *ai.Client,
	newGP, oldGP gitprovider.GitProvider,
	newArgoCD, oldArgoCD *argocd.Client,
	addonName, clusterName string,
	steps []MigrationStep,
	logFn func(step int, repo, action, detail string),
) *MigrationAgent {
	// Create a step-aware log wrapper
	var currentStep int
	stepLogFn := func(repo, action, detail string) {
		logFn(currentStep, repo, action, detail)
	}

	agent := &MigrationAgent{
		client:        client,
		toolExecutor:  NewMigrationToolExecutor(newGP, oldGP, newArgoCD, oldArgoCD, stepLogFn),
		maxIterations: 20,
		addonName:     addonName,
		clusterName:   clusterName,
		steps:         steps,
		logFn:         logFn,
	}

	// Allow override from config
	if client != nil && client.GetConfig().MaxIterations > 0 {
		agent.maxIterations = client.GetConfig().MaxIterations
	}

	return agent
}

// ExecuteStep runs the agent for a specific migration step.
func (a *MigrationAgent) ExecuteStep(ctx context.Context, stepNum int) (StepResult, string, error) {
	if !a.client.IsEnabled() {
		return StepResultFailed, "", fmt.Errorf("AI provider is required for migration. Configure one in Settings → AI Configuration")
	}

	// Build the system prompt with knowledge files + context
	systemPrompt, err := a.buildSystemPrompt(stepNum)
	if err != nil {
		return StepResultFailed, "", fmt.Errorf("building agent prompt: %w", err)
	}

	// Initialize conversation
	a.messages = []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: a.buildStepInstruction(stepNum)},
	}

	slog.Info("migration agent executing step", "step", stepNum, "addon", a.addonName, "cluster", a.clusterName)

	// Tool-calling loop
	tools := GetMigrationToolDefinitions()
	for i := 0; i < a.maxIterations; i++ {
		resp, err := a.client.ChatWithTools(ctx, a.messages, tools)
		if err != nil {
			return StepResultFailed, "", fmt.Errorf("LLM call failed: %w", err)
		}

		// No tool calls = final response
		if len(resp.ToolCalls) == 0 {
			a.messages = append(a.messages, ai.ChatMessage{Role: "assistant", Content: resp.Content})
			return a.parseResult(resp.Content)
		}

		// Process tool calls
		a.messages = append(a.messages, ai.ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			slog.Info("migration agent tool call", "tool", tc.Function.Name, "step", stepNum)
			result, toolErr := a.toolExecutor.ExecuteTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if toolErr != nil {
				result = fmt.Sprintf("Error: %s", toolErr.Error())
			}
			a.messages = append(a.messages, ai.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return StepResultFailed, "Reached maximum iterations. The step may be too complex — please review and retry.", nil
}

// Chat sends a follow-up message to the agent (for troubleshooting).
func (a *MigrationAgent) Chat(ctx context.Context, message string) (string, error) {
	if !a.client.IsEnabled() {
		return "", fmt.Errorf("AI not configured")
	}

	a.messages = append(a.messages, ai.ChatMessage{Role: "user", Content: message})

	tools := GetMigrationToolDefinitions()
	// Only read tools for troubleshooting chat
	readTools := filterReadTools(tools)

	for i := 0; i < 10; i++ {
		resp, err := a.client.ChatWithTools(ctx, a.messages, readTools)
		if err != nil {
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			a.messages = append(a.messages, ai.ChatMessage{Role: "assistant", Content: resp.Content})
			return resp.Content, nil
		}

		a.messages = append(a.messages, ai.ChatMessage{
			Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			result, toolErr := a.toolExecutor.ExecuteTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if toolErr != nil {
				result = fmt.Sprintf("Error: %s", toolErr.Error())
			}
			a.messages = append(a.messages, ai.ChatMessage{
				Role: "tool", Content: result, ToolCallID: tc.ID,
			})
		}
	}

	return "I've reached my investigation limit. Please provide more details or try a specific question.", nil
}

// GetMessages returns the conversation history (for persistence).
func (a *MigrationAgent) GetMessages() []ai.ChatMessage {
	return a.messages
}

// SetMessages restores conversation history (after pod restart or retry).
func (a *MigrationAgent) SetMessages(msgs []ai.ChatMessage) {
	a.messages = msgs
}

// --- Internal ---

func (a *MigrationAgent) buildSystemPrompt(stepNum int) (string, error) {
	// Read knowledge files
	guideContent := readKnowledgeFile("docs/agent/migration-guide.md")
	promptContent := readKnowledgeFile("docs/agent/migration-agent-prompt.md")

	// Build step context
	var stepSummary strings.Builder
	for _, s := range a.steps {
		status := string(s.Status)
		if s.Error != "" {
			status += " (error: " + s.Error + ")"
		}
		stepSummary.WriteString(fmt.Sprintf("  Step %d: %s — %s\n", s.Number, s.Title, status))
	}

	context := fmt.Sprintf(`%s

---

%s

---

## Current Migration Context

Addon: %s
Cluster: %s
Current Step: %d

## All Steps Status:
%s`,
		promptContent, guideContent,
		a.addonName, a.clusterName, stepNum,
		stepSummary.String())

	return context, nil
}

func (a *MigrationAgent) buildStepInstruction(stepNum int) string {
	if stepNum < 1 || stepNum > len(a.steps) {
		return fmt.Sprintf("Execute migration step %d", stepNum)
	}

	step := a.steps[stepNum-1]
	return fmt.Sprintf(`Execute migration step %d: "%s"

Description: %s

Follow the migration guide to complete this step. Use tools to verify state before and after each action. Log your reasoning for the user to see.

When done, respond with exactly one of:
- SUCCESS: <brief summary of what was accomplished>
- FAILED: <what went wrong and your diagnosis>
- NEEDS_USER_ACTION: <what the user needs to do>`, stepNum, step.Title, step.Description)
}

func (a *MigrationAgent) parseResult(content string) (StepResult, string, error) {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "SUCCESS:") {
		return StepResultSuccess, strings.TrimPrefix(content, "SUCCESS:"), nil
	}
	if strings.HasPrefix(content, "FAILED:") {
		return StepResultFailed, strings.TrimPrefix(content, "FAILED:"), nil
	}
	if strings.HasPrefix(content, "NEEDS_USER_ACTION:") {
		return StepResultNeedsUser, strings.TrimPrefix(content, "NEEDS_USER_ACTION:"), nil
	}

	// If the agent didn't use the exact format, treat as success if no error indicators
	lower := strings.ToLower(content)
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "cannot") {
		return StepResultFailed, content, nil
	}

	return StepResultSuccess, content, nil
}

func readKnowledgeFile(relativePath string) string {
	// Try common locations
	paths := []string{
		relativePath,
		filepath.Join("/app", relativePath),
	}

	// Also try relative to the binary
	if _, filename, _, ok := runtime.Caller(0); ok {
		projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
		paths = append(paths, filepath.Join(projectRoot, relativePath))
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data)
		}
	}

	slog.Warn("knowledge file not found", "path", relativePath)
	return fmt.Sprintf("(Knowledge file %s not found — agent will use built-in knowledge)", relativePath)
}

func filterReadTools(tools []ai.ToolDefinition) []ai.ToolDefinition {
	writeTools := map[string]bool{
		"git_create_pr":    true,
		"git_merge_pr":     true,
		"argocd_sync_app":  true,
		"argocd_refresh_app": true,
	}

	var readOnly []ai.ToolDefinition
	for _, t := range tools {
		if !writeTools[t.Function.Name] {
			readOnly = append(readOnly, t)
		}
	}
	return readOnly
}
```

- [ ] **Step 2: Add ChatWithTools to ai.Client**

The existing `ai.Client` has `Summarize()` and the `Agent` has `Chat()` with tool calling. We need a public `ChatWithTools` method on the client. Check if it exists:

If `callChat` or `callLLM` is private in `agent.go`, we need to expose a public version. Add to `internal/ai/client.go`:

```go
// ChatWithTools sends messages to the LLM with tool definitions and returns the response.
func (c *Client) ChatWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error) {
	switch c.config.Provider {
	case ProviderOllama:
		return c.callOllamaChat(ctx, messages, tools)
	case ProviderClaude:
		return c.callClaudeChat(ctx, messages, tools)
	case ProviderOpenAI:
		return c.callOpenAIChat(ctx, messages, tools)
	case ProviderGemini:
		return c.callGeminiChat(ctx, messages, tools)
	case ProviderCustomOpenAI:
		return c.callCustomOpenAIChat(ctx, messages, tools)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", c.config.Provider)
	}
}
```

The `call*Chat` methods already exist in `agent.go` as private methods on the `Agent` struct. They need to be refactored to be on `Client` instead, or a thin wrapper added. The simplest approach: check if these methods are already on `Client` — if not, add `ChatWithTools` that delegates to the existing agent chat infrastructure.

- [ ] **Step 3: Verify build**

Run: `go build ./internal/migration/... ./internal/ai/...`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add internal/migration/migration_agent.go internal/ai/client.go
git commit -m "feat: add MigrationAgent with tool calling and knowledge files"
```

---

### Task 5: Ensure knowledge files are included in Docker image

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Check Dockerfile and add COPY for docs/agent/**

The knowledge files need to be available at `/app/docs/agent/` in the container. Add to the Dockerfile:

```dockerfile
COPY docs/agent/ /app/docs/agent/
```

- [ ] **Step 2: Commit**

```bash
git add Dockerfile
git commit -m "build: include agent knowledge files in Docker image"
```

---

### Task 6: Integration test — verify agent can execute a step

**Files:**
- Create: `internal/migration/migration_agent_test.go`

- [ ] **Step 1: Write agent test**

Create `internal/migration/migration_agent_test.go` with a test that verifies the agent can be constructed and that tools are properly defined:

```go
package migration

import (
	"testing"
)

func TestGetMigrationToolDefinitions(t *testing.T) {
	tools := GetMigrationToolDefinitions()
	if len(tools) == 0 {
		t.Fatal("expected at least 1 tool definition")
	}

	// Verify all tools have names and descriptions
	for _, tool := range tools {
		if tool.Function.Name == "" {
			t.Error("tool has empty name")
		}
		if tool.Function.Description == "" {
			t.Errorf("tool %s has empty description", tool.Function.Name)
		}
	}

	// Verify expected tools exist
	expected := []string{
		"git_read_file", "git_list_directory", "git_create_pr", "git_merge_pr",
		"argocd_get_app", "argocd_list_apps", "argocd_sync_app", "argocd_refresh_app",
		"log",
	}
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Function.Name] = true
	}
	for _, name := range expected {
		if !toolNames[name] {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestFilterReadTools(t *testing.T) {
	all := GetMigrationToolDefinitions()
	readOnly := filterReadTools(all)

	for _, tool := range readOnly {
		switch tool.Function.Name {
		case "git_create_pr", "git_merge_pr", "argocd_sync_app", "argocd_refresh_app":
			t.Errorf("write tool %q should not be in read-only list", tool.Function.Name)
		}
	}

	if len(readOnly) >= len(all) {
		t.Error("read-only tools should be fewer than all tools")
	}
}

func TestReadKnowledgeFile(t *testing.T) {
	// Should return content or a fallback message (never crash)
	content := readKnowledgeFile("docs/agent/migration-guide.md")
	if content == "" {
		t.Error("expected non-empty content (either file or fallback)")
	}
}

func TestParseResult(t *testing.T) {
	agent := &MigrationAgent{}

	tests := []struct {
		input    string
		expected StepResult
	}{
		{"SUCCESS: Step completed", StepResultSuccess},
		{"FAILED: Something went wrong", StepResultFailed},
		{"NEEDS_USER_ACTION: Please merge the PR", StepResultNeedsUser},
		{"The addon was enabled successfully", StepResultSuccess},
		{"An error occurred while reading", StepResultFailed},
	}

	for _, tt := range tests {
		result, _, _ := agent.parseResult(tt.input)
		if result != tt.expected {
			t.Errorf("parseResult(%q) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/migration/ -run TestGetMigration -v`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/migration/migration_agent_test.go
git commit -m "test: add MigrationAgent unit tests"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run all tests**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/aap-server/`
Expected: Clean build

- [ ] **Step 3: Verify knowledge files exist**

Run: `ls docs/agent/`
Expected: `migration-guide.md` and `migration-agent-prompt.md`
