# Migration v4: Multi-Agent Architecture — Design Spec

**Date:** 2026-03-23
**Status:** Approved (brainstormed in previous session)

---

## 1. Overview

Evolve the single MigrationAgent into a three-agent system: **Planner**, **Executor**, **Verifier**. Each agent has a distinct role and permission boundary. This eliminates the hallucination problem (agent claims success without acting) by having an independent verifier confirm every action.

**Architecture:** Pipeline orchestrator dispatches three agents per step in sequence. Structured JSON output replaces free-text parsing. Shared context stored in migration state survives pod restarts.

**Key principle:** Trust nothing. The agent that performs an action is never the one that confirms it succeeded.

---

## 2. Three-Agent Pattern

```
Per migration step:

┌──────────────────────────────────────────────────────┐
│ Pipeline Executor (orchestrator)                     │
│                                                      │
│  1. PLANNER (read-only)                              │
│     - Reads current state (repos, ArgoCD, K8s)       │
│     - Produces a structured action plan (JSON)       │
│     - Tools: all read tools                          │
│     - Output: { actions: [...], context: {...} }     │
│                                                      │
│  2. EXECUTOR (read + write)                          │
│     - Receives planner's action plan                 │
│     - Performs each action using write tools          │
│     - Reports what it did (JSON)                     │
│     - Tools: all read + write tools                  │
│     - Output: { results: [...] }                     │
│                                                      │
│  3. VERIFIER (read-only)                             │
│     - Receives: planner's plan + executor's results  │
│     - Independently checks each action was done      │
│     - Gives go/no-go verdict                         │
│     - Tools: all read tools                          │
│     - Output: { verified: bool, issues: [...] }      │
│                                                      │
│  Orchestrator decides: advance, retry, or fail       │
└──────────────────────────────────────────────────────┘
```

### Agent Responsibilities

| Agent | Reads | Writes | Purpose |
|-------|-------|--------|---------|
| **Planner** | Git repos, ArgoCD, K8s | Nothing | Analyze state, determine what actions are needed |
| **Executor** | Git repos, ArgoCD | PRs, sync, refresh | Perform the actions the planner specified |
| **Verifier** | Git repos, ArgoCD, K8s | Nothing | Independently confirm actions were executed correctly |

### Why Three Agents?

1. **No hallucination** — Executor can't claim success; verifier checks independently
2. **Separation of concerns** — Planning is different from doing is different from checking
3. **Debuggability** — Each agent's output is a structured JSON record you can inspect
4. **Retry precision** — If verifier fails, you know exactly what didn't work

---

## 3. Structured Output (JSON)

Replace the `SUCCESS:`/`FAILED:` free-text parsing with a JSON result tool.

### Result Tool Definition

Each agent has a `report_result` tool it MUST call as its final action:

```json
{
  "name": "report_result",
  "description": "Report the result of your work. You MUST call this tool as your final action.",
  "parameters": {
    "status": "success | failed | needs_user_action",
    "summary": "1-2 sentence human-readable summary",
    "details": {}  // agent-specific structured data
  }
}
```

### Planner Output

```json
{
  "status": "success",
  "summary": "Found anodot addon in catalog with inMigration: true. Need to enable it in cluster-addons.yaml for cluster-prod-1.",
  "details": {
    "actions": [
      {
        "action": "create_pr",
        "repo": "new",
        "file": "configuration/cluster-addons.yaml",
        "description": "Set anodot label to enabled for cluster-prod-1",
        "required": true
      },
      {
        "action": "merge_pr",
        "repo": "new",
        "description": "Merge the enable PR",
        "required": true
      }
    ],
    "context": {
      "addon_in_catalog": true,
      "in_migration": true,
      "current_cluster_status": "disabled",
      "old_repo_format": "v2"
    }
  }
}
```

### Executor Output

```json
{
  "status": "success",
  "summary": "Created and merged PR #42 to enable anodot on cluster-prod-1.",
  "details": {
    "results": [
      {
        "action": "create_pr",
        "pr_number": 42,
        "pr_url": "https://github.com/org/repo/pull/42",
        "status": "completed"
      },
      {
        "action": "merge_pr",
        "pr_number": 42,
        "status": "completed"
      }
    ]
  }
}
```

### Verifier Output

```json
{
  "status": "success",
  "summary": "Verified: PR #42 merged, cluster-addons.yaml updated, anodot enabled for cluster-prod-1.",
  "details": {
    "checks": [
      {
        "check": "PR #42 is merged",
        "result": "pass",
        "evidence": "PR status: merged at 2026-03-23T10:00:00Z"
      },
      {
        "check": "cluster-addons.yaml has anodot: enabled for cluster-prod-1",
        "result": "pass",
        "evidence": "File content shows 'anodot: enabled' under cluster-prod-1"
      }
    ],
    "verdict": "pass"
  }
}
```

---

## 4. Shared Context

Context accumulates across steps and is shared between all three agents.

### Storage

Stored in the migration ConfigMap alongside existing state:

```json
{
  "step_context": {
    "1": {
      "planner_summary": "Found anodot in catalog with inMigration: true",
      "executor_summary": "No action needed for verification step",
      "verifier_summary": "Verified addon exists in catalog",
      "key_facts": {
        "addon_in_catalog": true,
        "in_migration": true,
        "old_repo_format": "v2"
      }
    },
    "2": { ... }
  }
}
```

### Context Budget

Each agent receives:
1. **Migration guide** (full — ~1.8K tokens)
2. **Agent-specific prompt** (~500 tokens)
3. **Step context summaries** from all previous steps (~200 tokens per step)
4. **Current step instruction** (~300 tokens)
5. **Planner output** (executor and verifier only)
6. **Executor output** (verifier only)

Total budget per agent call: ~4-5K tokens input. Well within limits.

### Context Summarization

After each step completes (all 3 agents done), the orchestrator saves:
- Planner's `summary` field
- Executor's `summary` field
- Verifier's `summary` field
- Key facts extracted from planner's `context` object

Previous step tool call details are NOT carried forward — only summaries.

---

## 5. Agent Prompts

### Planner Prompt

```
You are the PLANNER for an addon migration. Your job is to:
1. Analyze the current state by reading repos and ArgoCD
2. Determine what actions are needed for this step
3. Report your findings and action plan via report_result

You have READ-ONLY access. You cannot create PRs, sync apps, or make any changes.
You MUST use tools to verify every fact. Never assume file contents or app status.

Context from previous steps:
{step_context_summaries}

Current step: {step_number} - {step_title}
Step instructions: {step_instructions}
```

### Executor Prompt

```
You are the EXECUTOR for an addon migration. Your job is to:
1. Follow the planner's action plan exactly
2. Perform each action using your tools
3. Report what you did via report_result

You have READ and WRITE access.
Follow the planner's actions in order. Do not skip actions marked as required.
If an action fails, stop and report the failure — do not improvise alternatives.

Planner's plan:
{planner_output}

Context from previous steps:
{step_context_summaries}
```

### Verifier Prompt

```
You are the VERIFIER for an addon migration. Your job is to:
1. Check that each action the executor claims to have done was actually done
2. Use READ-ONLY tools to independently verify
3. Report your verification via report_result

You MUST NOT trust the executor's word. Check everything independently.
For each action, describe what you checked and what you found.

Planner's plan:
{planner_output}

Executor's results:
{executor_output}

Context from previous steps:
{step_context_summaries}
```

---

## 6. Orchestrator Flow

```go
func (e *Executor) runStepWithAgents(ctx context.Context, m *Migration, stepNum int) error {
    // 1. Run Planner
    plannerResult := e.runPlanner(ctx, m, stepNum)
    if plannerResult.Status == "failed" {
        return stepFailed(plannerResult.Summary)
    }

    // 2. Gates mode: show plan, wait for approval
    if m.Mode == "gates" {
        e.addLog(m, stepNum, "PLANNER", "plan", plannerResult.Summary)
        // Show actions to user, wait for approval
        if !waitForApproval(ctx, m) {
            return stepPaused()
        }
    }

    // 3. Run Executor
    executorResult := e.runExecutor(ctx, m, stepNum, plannerResult)
    if executorResult.Status == "failed" {
        return stepFailed(executorResult.Summary)
    }

    // 4. Run Verifier
    verifierResult := e.runVerifier(ctx, m, stepNum, plannerResult, executorResult)
    if verifierResult.Status == "failed" {
        // Verification failed — what executor claims doesn't match reality
        return stepFailed("Verification failed: " + verifierResult.Summary)
    }

    // 5. Save context for next steps
    e.saveStepContext(m, stepNum, plannerResult, executorResult, verifierResult)

    return stepCompleted()
}
```

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Planner fails to analyze | Step fails with planner's diagnosis |
| Planner says no actions needed | Skip executor+verifier, mark step complete |
| Executor fails an action | Step fails with executor's error report |
| Verifier finds discrepancy | Step fails with verifier's evidence |
| Agent returns no report_result | Treated as failure ("Agent did not report result") |
| Agent exceeds max iterations | Failure ("Reached max iterations") |
| LLM API error | Step fails ("AI service unavailable"), retry available |

### Retry Logic

On retry:
1. Previous attempt's verifier report included as context: "Previous attempt failed verification: ..."
2. Planner runs fresh (may gather new state)
3. Executor runs fresh with new plan
4. Verifier checks again

---

## 7. Troubleshooting Chat

When a step fails, the user can chat with a troubleshooting agent that has:
- All three agents' outputs from the failed step
- Read-only tools
- Full step context

The troubleshooting agent is a 4th role — investigator. It uses read tools to gather more evidence and explains what went wrong.

---

## 8. Tool Changes

### New Tool: `report_result`

All agents get this. It's the structured output mechanism.

### Tool Filtering by Agent Role

```go
func toolsForRole(role AgentRole) []ToolDefinition {
    switch role {
    case RolePlanner, RoleVerifier:
        return readOnlyTools()  // git_read_file, git_list_*, argocd_get_*, log, report_result
    case RoleExecutor:
        return allTools()       // read + git_create_pr, git_merge_pr, argocd_sync_*, report_result
    case RoleTroubleshooter:
        return readOnlyTools()
    }
}
```

### Existing tools stay the same
No changes to git_read_file, git_create_pr, argocd_get_app, etc.

---

## 9. What Changes vs What Stays

### New files
| File | Purpose |
|------|---------|
| `internal/migration/planner.go` | Planner agent — builds prompt, runs, parses JSON result |
| `internal/migration/verifier.go` | Verifier agent — builds prompt, runs, parses JSON result |
| `internal/migration/step_context.go` | Step context management — save/load/summarize |
| `docs/agent/planner-prompt.md` | Planner system prompt |
| `docs/agent/verifier-prompt.md` | Verifier system prompt |
| `docs/agent/executor-prompt.md` | Executor system prompt (refactored from migration-agent-prompt.md) |

### Modified files
| File | Change |
|------|--------|
| `internal/migration/migration_agent.go` | Refactor to support agent roles (planner/executor/verifier) |
| `internal/migration/agent_tools.go` | Add report_result tool, add role-based tool filtering |
| `internal/migration/executor.go` | New runStepWithAgents orchestration, step context storage |
| `internal/migration/models.go` | Add StepContext struct, AgentRole enum |
| `ui/src/views/MigrationDetail.tsx` | Show planner/executor/verifier phases in step logs |

### Unchanged
- 10 fixed steps in pipeline
- K8s ConfigMap state persistence
- Migration settings (credentials)
- Gates vs YOLO mode
- Pause/resume/cancel/retry
- Encrypted credential storage
- Existing tool implementations (git, argocd)

---

## 10. Migration from v3 to v4

The single MigrationAgent becomes three role-specific agents. The refactor:

1. Extract shared agent infrastructure (prompt building, tool calling loop, conversation management) into a base
2. Create Planner/Executor/Verifier as thin wrappers with role-specific prompts and tool sets
3. Add report_result tool for structured output
4. Modify executor.go orchestration to run P→E→V per step
5. Add step context storage/retrieval
6. Update UI to show agent phases within each step

The existing Chat method moves to a new Troubleshooter agent role.
