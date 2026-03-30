# Migration v3: AI Agent-Driven Migration — Design Spec

**Date:** 2026-03-22
**Status:** Draft

---

## 1. Overview

Replace the hardcoded 10-step migration executor with an AI agent that acts as a DevOps engineer. The agent reads a migration guide (markdown), uses tools to interact with Git repos and ArgoCD, and explains its reasoning in human-friendly language.

**Architecture:** Hybrid — the pipeline structure (10 fixed steps, state persistence, retry, gates/YOLO) remains as the safety net. The AI agent executes within each step, providing intelligence, error handling, and troubleshooting.

**Key principle:** Pipeline catches failures. Agent provides the brain. If the LLM fails, the step fails cleanly — state is saved, user can retry.

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────┐
│ Pipeline Executor (existing, modified)              │
│ - 10 fixed steps, visual UI, state in K8s ConfigMap │
│ - Gates mode: pause between steps for approval      │
│ - YOLO mode: agent runs autonomously                │
│ - Catches agent failures → step fails → retry       │
└───────────────┬─────────────────────────────────────┘
                │ Per step: calls MigrationAgent
                ▼
┌─────────────────────────────────────────────────────┐
│ MigrationAgent                                      │
│ - Dedicated LLM agent per migration session         │
│ - System prompt: docs/agent/migration-agent-prompt  │
│ - Knowledge: docs/agent/migration-guide.md          │
│ - Context: all 10 steps + statuses + previous logs  │
│ - Reasoning logged in human-friendly tone           │
│ - On error: investigates, diagnoses, suggests fixes │
│ - On write rejection: adapts, skips or waits        │
└───────────────┬─────────────────────────────────────┘
                │ Tool calls
                ▼
┌─────────────────────────────────────────────────────┐
│ Tool Layer                                          │
│ - Git: read/write via GitProvider interface         │
│ - ArgoCD: read/write via ArgoCD client              │
│ - K8s: read-only (optional, requires ClusterRole)   │
│ - Migration: compare values, check all migrated     │
│ - Communication: log to step log                    │
└─────────────────────────────────────────────────────┘
```

---

## 3. Anti-Hallucination Rules

The agent MUST verify every fact via tool calls. This is non-negotiable.

- NEVER assume file contents — always read the file first
- NEVER assume app status — always query ArgoCD API first
- NEVER guess PR status — always check via API
- NEVER invent file paths — read repo structure first, then decide
- If a tool call fails — report the exact error, do not interpret or guess
- If unsure — say "Let me check" and make the tool call
- When reporting results — reference actual API/tool response, not expectations

The system prompt enforces these rules explicitly. The tool layer also prevents hallucination by design — the agent can only interact with real systems through tools.

---

## 4. Tool Set

### Git — Read (anytime, no permission needed)

| Tool | Description |
|------|-------------|
| `git_read_file` | Read file from OLD or NEW repo (specify repo, path, branch) |
| `git_list_directory` | List files/folders in a repo path |
| `git_list_pull_requests` | List open/closed PRs in a repo |

### Git — Write (step-authorized only)

| Tool | Description |
|------|-------------|
| `git_create_pr` | Create branch, modify file, open PR. In gates mode: requires user approval before executing. |
| `git_merge_pr` | Approve + merge a PR. Handles GitHub (squash merge) and Azure DevOps (PATCH). Policy bypass is configurable via migration settings (opt-in, not default). |
| `git_get_pr_status` | Get status of a specific PR by number (open, merged, closed, blocked by checks) |

### ArgoCD — Read (anytime, no permission needed)

| Tool | Description |
|------|-------------|
| `argocd_get_app` | Get application status, health, sync state, resources |
| `argocd_list_apps` | List all applications |
| `argocd_get_app_events` | Get recent events for an application (for troubleshooting) |

### ArgoCD — Write (step-authorized only)

| Tool | Description |
|------|-------------|
| `argocd_sync_app` | Trigger sync on an application |
| `argocd_refresh_app` | Trigger hard refresh on an application |

### K8s — Read (optional, anytime)

Available only if `config.agentK8sAccess: true` in Helm values (deploys a read-only ClusterRole).

| Tool | Description |
|------|-------------|
| `k8s_get_pods` | List pods in a namespace (status, restarts, age) |
| `k8s_get_events` | Get recent events for a namespace or resource |
| `k8s_get_pod_logs` | Read logs from a specific pod (tail N lines) |
| `k8s_get_resources` | Get any K8s resource (deployments, services, configmaps, etc.) |

If K8s access is not configured, the agent explains: "I don't have cluster access to check pod status. Enable it with `config.agentK8sAccess: true` in Helm values."

### Migration-specific — Read (anytime)

| Tool | Description |
|------|-------------|
| `compare_values` | Diff addon values between OLD and NEW repos |
| `check_all_clusters_migrated` | Cross-reference OLD and NEW repos to determine if all clusters have been migrated for this addon |

### Communication

| Tool | Description |
|------|-------------|
| `log` | Write a message to the step log (reasoning, action result, diagnosis) |

---

## 5. Read vs Write Permissions

```
READ (agent can do anytime, no permission needed):
  - Read any file from OLD or NEW repo
  - List directories, branches, PRs
  - Query ArgoCD: app status, health, events, resources
  - Read K8s resources (if ClusterRole deployed)
  - Compare values between repos

WRITE (only within the step's scope):
  - Create/merge PRs — only when the step requires it
  - Sync/refresh ArgoCD — only when the step requires it
  - All writes go through PRs, never direct file modification

NEVER (hard-blocked, not in tool set):
  - Delete applications
  - Delete repos or branches outside migration scope
  - Modify ArgoCD RBAC or settings
  - kubectl apply/delete
  - Any destructive action
```

### Write Action Rejection

When the user rejects a write action (gates mode):

1. Agent logs: "User chose not to proceed with this action."
2. If the step can continue without it → skip and proceed
3. If the step cannot continue → pause with explanation: "This step requires the PR to proceed. Resume when ready."
4. Agent never argues, never retries rejected actions, never works around the decision.

---

## 6. Step Execution Flow

### Per-step execution

```
Pipeline Executor
    │
    ├─ Set step status = "running"
    ├─ Build agent context:
    │   - Migration guide MD (docs/agent/migration-guide.md)
    │   - System prompt MD (docs/agent/migration-agent-prompt.md)
    │   - All 10 steps with current statuses
    │   - Logs from all previous steps
    │   - Current step number, title, description
    │   - Addon name, cluster name, OLD/NEW repo info
    │
    ├─ Call MigrationAgent: "Execute step N"
    │   │
    │   ├─ Agent reasons, makes tool calls, logs to step log
    │   ├─ Agent returns: SUCCESS | FAILED | NEEDS_USER_ACTION
    │   │
    │   ├─ SUCCESS → step completed, advance
    │   ├─ FAILED → agent investigated + diagnosed
    │   │           → error + diagnosis + suggestions shown in log
    │   │           → "Talk to agent" chat available
    │   │           → Retry button
    │   └─ NEEDS_USER_ACTION → agent explains what's needed
    │                        → step pauses, user acts, resumes
    │
    ├─ Gates mode: pause before next step, show "Approve"
    └─ YOLO mode: continue to next step automatically
```

### Troubleshooting Mode

When a step fails, the agent enters troubleshooting mode automatically:

1. **Investigate** — uses read tools to gather evidence (ArgoCD events, pod status, file contents, PR state)
2. **Diagnose** — explains what went wrong in plain language
3. **Suggest** — provides specific fix steps (runbook-style)
4. **Wait** — step shows as failed with diagnosis. User can:
   - Click "Talk to Agent" to ask follow-up questions
   - Fix the issue externally, then click Retry
   - Skip the step if applicable

The agent's conversation state is preserved on failure, so the chat has full context from the investigation.

---

## 7. Smart Finalization (Step 10)

Step 10 (`inMigration: false`) has special logic because `inMigration` is addon-level, not cluster-level.

Before the agent disables `inMigration`:

1. **Read OLD repo** — find all clusters that had this addon enabled
2. **Read NEW repo** — find all clusters that have this addon enabled
3. **Cross-reference:**
   - If OLD still has clusters with this addon enabled → DO NOT finalize
   - Show: "3 of 5 clusters migrated. Remaining: cluster-a, cluster-b"
   - Suggest migrating remaining clusters first
4. **All clusters migrated** → agent explains what finalization does, any risks, and offers a "Finalize" button
5. **User confirms** → agent creates PR to set `inMigration: false`

---

## 8. Agent Knowledge Files

### `docs/agent/migration-guide.md`

The migration runbook in markdown. Contains:
- What the migration does and why (zero downtime, resource adoption)
- NEW repo structure (configuration/addons-catalog.yaml, cluster-addons.yaml, etc.)
- OLD repo structure (V2 paths + V1 legacy paths)
- How to detect V1 vs V2 (try V2 first, fall back to V1)
- All 10 steps with: what to do, why, what to verify, common problems
- Safety mechanisms (preserveResourcesOnDeletion, prune: false, inMigration)
- Common issues and fixes (troubleshooting runbook)

### `docs/agent/migration-agent-prompt.md`

The system prompt. Contains:
- Personality: speak like a colleague, concise, honest
- Anti-hallucination rules (see section 3)
- Read vs write permission rules (see section 5)
- Troubleshooting mode instructions
- Step 10 smart finalization logic
- How to handle write action rejections
- Tone guidelines: "Reading the cluster config..." not "GET /api/v1/..."

Both files are read by the agent at the start of each step execution. They can be updated without code changes.

---

## 9. Agent Personality & Tone

The agent speaks like a DevOps colleague, not a machine:

| Instead of | Say |
|-----------|-----|
| "Executing GET request to /configuration/cluster-addons.yaml" | "Let me read the cluster configuration from the new repo..." |
| "HTTP 403 from POST /api/v1/applications/clusters/sync" | "The sync failed because the ArgoCD token doesn't have sync permission. You'll need to add sync access for this account." |
| "V2 path not found, trying V1: values/clusters.yaml" | "The old repo uses the legacy folder structure. Reading from the V1 config path instead." |
| "Application status: sync=OutOfSync, health=Healthy" | "The app is healthy and running. It shows OutOfSync which is expected right after migration — ArgoCD will sync it on the next cycle." |

Reasoning entries are 1-2 sentences. Actions are logged with their results. Errors include diagnosis and fix suggestions.

---

## 10. UI Changes

Minimal changes to the existing Azure Pipeline-style UI:

1. **Agent reasoning in logs** — log entries from the agent get a distinct icon/color (e.g., cyan "AGENT" tag) to distinguish from system logs
2. **"Talk to Agent" button** — appears on failed steps. Expands a chat input below the log panel. Agent has full context from the step.
3. **Write action approval (gates mode)** — agent logs what it wants to do → inline Approve/Reject buttons appear in the log
4. **Step 10 cluster table** — shows migration status per cluster instead of auto-finalizing. "Finalize" button only when all clusters accounted for.

---

## 11. What Changes vs What Stays

### New files
| File | Purpose |
|------|---------|
| `docs/agent/migration-guide.md` | Migration runbook for the agent |
| `docs/agent/migration-agent-prompt.md` | Agent system prompt |
| `internal/migration/agent.go` | MigrationAgent — dedicated agent with prompt, tools, conversation state |
| `internal/migration/tools.go` | Migration-specific tool definitions for the agent |

### Modified files
| File | Change |
|------|--------|
| `internal/migration/executor.go` | Replace `executeStep()` hardcoded logic with MigrationAgent call |
| `internal/migration/models.go` | Minor — keep step definitions for UI |
| `ui/src/views/MigrationDetail.tsx` | Add chat input, agent log styling, step 10 cluster table |
| `charts/argocd-addons-platform/values.yaml` | Add `config.agentK8sAccess: false` |
| `charts/argocd-addons-platform/templates/rbac.yaml` | Optional ClusterRole for K8s read access |

### Deleted files
| File | Reason |
|------|--------|
| `internal/migration/steps.go` | Logic moves to agent + migration guide MD |

### Unchanged
- 10 fixed steps in the UI pipeline
- K8s ConfigMap state persistence per migration
- Migration settings (OLD repo/ArgoCD connection config)
- Gates vs YOLO mode selection
- Pipeline executor retry/pause/resume/cancel logic
- Encrypted credential storage

---

## 12. Helm Chart Changes

```yaml
config:
  # Enable K8s read access for the migration agent (troubleshooting)
  # Deploys a ClusterRole with get/list/watch on pods, events, deployments, etc.
  agentK8sAccess: false
```

When `true`, the chart deploys:
- `ClusterRole` with read-only verbs (`get`, `list`, `watch`) on common resources
- `ClusterRoleBinding` to the AAP ServiceAccount

The agent checks at runtime if it has K8s access. If not, it works without it and explains the limitation when relevant.

---

## 13. Resource Limits & Timeouts

### Step timeout
The current 2-minute timeout per step is too short for agent-based execution (multiple LLM round-trips + tool calls). New defaults:
- **Normal steps** (verify, sync, refresh): 5 minutes
- **Complex steps** (compare values, create PR): 10 minutes
- **Troubleshooting investigation**: 3 minutes (after step failure, before presenting diagnosis)
- Configurable via `ai.migrationStepTimeout` in Helm values

### Token/cost budget
- **Max tool-call iterations per step:** 20 (configurable via `ai.maxMigrationIterations`)
- If limit hit → step fails cleanly with: "Reached maximum iterations. This usually means the step is more complex than expected. Please review and retry."
- No infinite loops — the pipeline executor enforces the limit

### Context window budget
Previous step logs are **summarized, not included in full**:
- Steps 1-N: include only the final status line + any errors (not all log entries)
- Current step: full conversation history
- Migration guide: included in full (it's the agent's primary knowledge)
- This keeps the prompt under ~8K tokens of context + the guide

---

## 14. Conversation State & Retries

### On step failure
- Agent's conversation history for the failed step is saved in the migration ConfigMap (JSON array of messages)
- The "Talk to Agent" chat appends to this conversation
- Agent retains full context from its investigation

### On retry
- **Default:** Start fresh — new conversation, clean slate. Previous attempt's final error is included as context: "Previous attempt failed with: ..."
- **After chat:** If the user chatted with the agent about the error, the conversation summary is included so the agent knows what was discussed
- This avoids stale context from a previous attempt interfering with the retry

### On pod restart
- Conversation state is in the ConfigMap, so it survives restarts
- Agent can resume troubleshooting chat after a pod restart

---

## 15. NEEDS_USER_ACTION Status

Maps to the existing `StepWaiting` status with a sub-reason:

```go
// Step status remains: pending, running, waiting, completed, failed, skipped
// When agent returns NEEDS_USER_ACTION:
step.Status = StepWaiting
step.Message = "Agent explanation of what the user needs to do"
// The executor pauses (same as PR-waiting behavior)
// User clicks "Done" or "Continue" to resume
```

No new status enum needed. The UI already handles `waiting` steps with action buttons. The agent's message explains what action is needed (e.g., "Please merge the PR in Azure DevOps, then click Continue").

---

## 16. AI Not Configured Fallback

If no AI provider is configured when starting a migration:
- **Refuse to start** with clear message: "AI provider is required for migration. Configure one in Settings → AI Configuration."
- The old hardcoded `steps.go` is deleted — there is no fallback to non-AI execution
- This is intentional: the agent IS the migration engine, not an optional enhancement

---

## 17. Troubleshooting Chat API

New endpoint: `POST /api/v1/migration/{id}/chat`

```json
// Request
{ "message": "Why did the sync fail?" }

// Response
{ "response": "The sync failed because...", "tool_calls": [...] }
```

- Scoped to the specific migration (not the general `/agent/chat`)
- Agent receives: migration state, current step, all logs, the user's question
- Agent can make read-only tool calls during the chat (investigate further)
- Chat history stored in the migration ConfigMap
