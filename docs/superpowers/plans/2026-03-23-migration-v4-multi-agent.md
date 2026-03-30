# Migration v4: Multi-Agent Architecture — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single MigrationAgent with a Planner→Executor→Verifier three-agent pipeline per step, using structured JSON output via a `report_result` tool.

**Architecture:** Each migration step runs 3 agents in sequence. The Planner (read-only) analyzes state and produces an action plan. The Executor (read+write) performs the planned actions. The Verifier (read-only) independently confirms each action succeeded. All agents output structured JSON via a `report_result` tool call instead of free-text parsing.

**Tech Stack:** Go 1.22, existing ai.Client (ChatWithTools), existing MigrationToolExecutor

---

### Task 1: Add AgentRole enum and StepContext model

**Files:**
- Modify: `internal/migration/models.go`

- [ ] **Step 1: Add AgentRole type and StepContext struct to models.go**

Add after the `StepStatus` constants:

```go
// AgentRole identifies which agent is executing within a step.
type AgentRole string

const (
	RolePlanner       AgentRole = "planner"
	RoleExecutor      AgentRole = "executor"
	RoleVerifier      AgentRole = "verifier"
	RoleTroubleshooter AgentRole = "troubleshooter"
)

// StepContext stores summarized results from each agent for a completed step.
// This is shared between agents across steps to provide continuity.
type StepContext struct {
	PlannerSummary  string            `json:"planner_summary"`
	ExecutorSummary string            `json:"executor_summary"`
	VerifierSummary string            `json:"verifier_summary"`
	KeyFacts        map[string]string `json:"key_facts,omitempty"`
}

// AgentResult is the structured output from any agent via the report_result tool.
type AgentResult struct {
	Status  string          `json:"status"`  // "success", "failed", "needs_user_action"
	Summary string          `json:"summary"` // human-readable 1-2 sentence summary
	Details json.RawMessage `json:"details"` // agent-specific structured data
}

// PlannerDetails contains the structured action plan from the planner.
type PlannerDetails struct {
	Actions []PlannedAction   `json:"actions"`
	Context map[string]string `json:"context,omitempty"`
}

// PlannedAction is a single action the executor should perform.
type PlannedAction struct {
	Action      string `json:"action"`      // e.g., "create_pr", "merge_pr", "sync_app"
	Repo        string `json:"repo"`        // "new" or "old"
	Description string `json:"description"` // human-readable
	Required    bool   `json:"required"`
	File        string `json:"file,omitempty"`
}

// ExecutorDetails contains what the executor actually did.
type ExecutorDetails struct {
	Results []ActionResult `json:"results"`
}

// ActionResult is the outcome of a single executed action.
type ActionResult struct {
	Action   string `json:"action"`
	Status   string `json:"status"` // "completed", "failed", "skipped"
	PRNumber int    `json:"pr_number,omitempty"`
	PRURL    string `json:"pr_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// VerifierDetails contains the verification results.
type VerifierDetails struct {
	Checks  []VerificationCheck `json:"checks"`
	Verdict string              `json:"verdict"` // "pass" or "fail"
}

// VerificationCheck is a single verification the verifier performed.
type VerificationCheck struct {
	Check    string `json:"check"`
	Result   string `json:"result"`   // "pass" or "fail"
	Evidence string `json:"evidence"` // what the verifier actually found
}
```

- [ ] **Step 2: Add StepContexts field to Migration struct**

In the `Migration` struct, add:

```go
StepContexts map[int]*StepContext `json:"step_contexts,omitempty"`
```

- [ ] **Step 3: Add `encoding/json` to imports if not present**

- [ ] **Step 4: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`
Expected: builds successfully

- [ ] **Step 5: Commit**

```bash
git add internal/migration/models.go
git commit -m "Add multi-agent types: AgentRole, StepContext, AgentResult structs"
```

---

### Task 2: Add report_result tool and role-based tool filtering

**Files:**
- Modify: `internal/migration/agent_tools.go`

- [ ] **Step 1: Add report_result tool definition**

Add to `GetMigrationToolDefinitions()` return slice:

```go
// Structured output — every agent must call this as their final action
toolDef("report_result", "Report the result of your work. You MUST call this as your final action.",
    `{"type":"object","properties":{"status":{"type":"string","enum":["success","failed","needs_user_action"],"description":"Result status"},"summary":{"type":"string","description":"1-2 sentence human-readable summary"},"details":{"type":"object","description":"Structured data (actions, results, checks depending on agent role)"}},"required":["status","summary"]}`),
```

- [ ] **Step 2: Add role-based tool filtering functions**

Add after `filterReadTools`:

```go
// ToolsForRole returns tool definitions filtered by agent role.
func ToolsForRole(role AgentRole) []ai.ToolDefinition {
	all := GetMigrationToolDefinitions()
	switch role {
	case RolePlanner, RoleVerifier, RoleTroubleshooter:
		return filterReadTools(all)
	case RoleExecutor:
		return all
	default:
		return all
	}
}
```

- [ ] **Step 3: Add report_result handler in ExecuteTool**

Add case in the `ExecuteTool` switch:

```go
case "report_result":
    // This is handled by the agent runner, not executed as a tool.
    // Return the raw args so the runner can parse them.
    return string(args), nil
```

- [ ] **Step 4: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/migration/agent_tools.go
git commit -m "Add report_result tool and role-based tool filtering"
```

---

### Task 3: Create agent prompt files for each role

**Files:**
- Create: `docs/agent/planner-prompt.md`
- Create: `docs/agent/executor-prompt.md`
- Create: `docs/agent/verifier-prompt.md`

- [ ] **Step 1: Create planner-prompt.md**

```markdown
# Migration Planner Agent

You are the PLANNER for an addon migration from OLD ArgoCD to NEW ArgoCD.

## Your Role
- Analyze the current state by reading repos and ArgoCD
- Determine what actions are needed for this step
- Report your findings and action plan via report_result tool

## Rules
- You have READ-ONLY access. You cannot create PRs, sync apps, or make changes.
- You MUST use tools to verify every fact. Never assume file contents or app status.
- Never guess file paths — list the directory first if unsure.
- If a tool call fails, report the error — do not interpret or guess.
- Keep your analysis concise. Focus on what matters for this step.

## Output
You MUST call the report_result tool as your final action with:
- status: "success" if you determined the actions needed
- status: "failed" if you cannot analyze the state (missing access, errors)
- summary: 1-2 sentence description of what you found
- details: { "actions": [...], "context": {...} }

Each action should have: action (what to do), repo (new/old), description, required (bool).
Context should include key facts discovered (addon status, config state, etc).
```

- [ ] **Step 2: Create executor-prompt.md**

```markdown
# Migration Executor Agent

You are the EXECUTOR for an addon migration from OLD ArgoCD to NEW ArgoCD.

## Your Role
- Follow the planner's action plan
- Perform each action using your tools
- Report what you did via report_result tool

## Rules
- You have READ and WRITE access.
- Follow the planner's actions IN ORDER. Do not skip actions marked as required.
- If an action fails, STOP and report the failure. Do not improvise alternatives.
- Use the log tool to explain what you're doing before each action.
- Verify state with read tools after write actions when appropriate.

## Output
You MUST call the report_result tool as your final action with:
- status: "success" if all required actions completed
- status: "failed" if any required action failed
- status: "needs_user_action" if user intervention is needed
- summary: 1-2 sentence description of what you did
- details: { "results": [...] }

Each result should have: action, status (completed/failed/skipped), pr_number, pr_url, detail.
```

- [ ] **Step 3: Create verifier-prompt.md**

```markdown
# Migration Verifier Agent

You are the VERIFIER for an addon migration from OLD ArgoCD to NEW ArgoCD.

## Your Role
- Check that each action the executor claims to have done was actually done
- Use READ-ONLY tools to independently verify
- Report your verification via report_result tool

## Rules
- You have READ-ONLY access.
- You MUST NOT trust the executor's word. Check everything independently.
- For each action, describe what you checked and what you found.
- If the executor claims a PR was created — read the PR status yourself.
- If the executor claims a file was modified — read the file yourself.
- If the executor claims an app was synced — check the app status yourself.

## Output
You MUST call the report_result tool as your final action with:
- status: "success" if all checks pass (verdict: "pass")
- status: "failed" if any check fails (verdict: "fail")
- summary: 1-2 sentence verification summary
- details: { "checks": [...], "verdict": "pass"|"fail" }

Each check should have: check (what was verified), result (pass/fail), evidence (what you found).
```

- [ ] **Step 4: Commit**

```bash
git add docs/agent/planner-prompt.md docs/agent/executor-prompt.md docs/agent/verifier-prompt.md
git commit -m "Add agent prompt files for planner, executor, verifier roles"
```

---

### Task 4: Refactor MigrationAgent to support roles

**Files:**
- Modify: `internal/migration/migration_agent.go`

This is the core refactor. The MigrationAgent becomes role-aware: it builds different prompts and uses different tools based on its role.

- [ ] **Step 1: Add role field and update constructor**

Add `role` field to MigrationAgent struct:

```go
type MigrationAgent struct {
	client        *ai.Client
	toolExecutor  *MigrationToolExecutor
	messages      []ai.ChatMessage
	maxIterations int
	currentStep   int
	role          AgentRole // NEW: which agent role this is

	addonName   string
	clusterName string
	steps       []MigrationStep
	stepContexts map[int]*StepContext // NEW: accumulated context from previous steps
	logFn       func(step int, repo, action, detail string)
}
```

Update `NewMigrationAgent` to accept role:

```go
func NewMigrationAgent(
	client *ai.Client,
	newGP, oldGP gitprovider.GitProvider,
	newArgoCD, oldArgoCD *argocd.Client,
	addonName, clusterName string,
	steps []MigrationStep,
	stepContexts map[int]*StepContext,
	role AgentRole,
	logFn func(step int, repo, action, detail string),
) *MigrationAgent {
```

- [ ] **Step 2: Rewrite buildSystemPrompt to be role-aware**

Replace `buildSystemPrompt` to load the role-specific prompt file and include step context summaries:

```go
func (a *MigrationAgent) buildSystemPrompt(stepNum int) (string, error) {
	// Load role-specific prompt
	var promptFile string
	switch a.role {
	case RolePlanner:
		promptFile = "docs/agent/planner-prompt.md"
	case RoleExecutor:
		promptFile = "docs/agent/executor-prompt.md"
	case RoleVerifier:
		promptFile = "docs/agent/verifier-prompt.md"
	default:
		promptFile = "docs/agent/migration-agent-prompt.md"
	}
	rolePrompt := readKnowledgeFile(promptFile)

	// Load migration guide (shared knowledge)
	guideContent := readKnowledgeFile("docs/agent/migration-guide.md")

	// Load relevant skill for this step (only for planner/executor)
	var skillsContent string
	if a.role == RolePlanner || a.role == RoleExecutor {
		skills := a.getSkillsForStep(stepNum)
		var sb strings.Builder
		for _, skill := range skills {
			content := readKnowledgeFile("docs/agent/skills/" + skill + ".md")
			if content != "" && !strings.HasPrefix(content, "(Knowledge file") {
				sb.WriteString(fmt.Sprintf("\n---\n## Skill: %s\n%s\n", skill, content))
			}
		}
		skillsContent = sb.String()
	}

	// Build step context summaries from previous steps
	var contextSummary strings.Builder
	if len(a.stepContexts) > 0 {
		contextSummary.WriteString("\n## Previous Step Results:\n")
		for i := 1; i < stepNum; i++ {
			if sc, ok := a.stepContexts[i]; ok {
				contextSummary.WriteString(fmt.Sprintf("Step %d: %s\n", i, sc.PlannerSummary))
				if sc.VerifierSummary != "" {
					contextSummary.WriteString(fmt.Sprintf("  Verified: %s\n", sc.VerifierSummary))
				}
			}
		}
	}

	// Build current step status overview
	var stepSummary strings.Builder
	for _, s := range a.steps {
		status := string(s.Status)
		if s.Error != "" {
			status += " (error: " + s.Error + ")"
		}
		stepSummary.WriteString(fmt.Sprintf("  Step %d: %s — %s\n", s.Number, s.Title, status))
	}

	return fmt.Sprintf(`%s

---

%s
%s
---

## Current Migration Context

Addon: %s
Cluster: %s
Current Step: %d
Agent Role: %s
%s
## All Steps Status:
%s`,
		rolePrompt, guideContent, skillsContent,
		a.addonName, a.clusterName, stepNum, string(a.role),
		contextSummary.String(),
		stepSummary.String()), nil
}
```

- [ ] **Step 3: Rewrite ExecuteStep to use report_result tool**

The key change: instead of parsing free text for SUCCESS:/FAILED:, we look for a `report_result` tool call:

```go
func (a *MigrationAgent) ExecuteStep(ctx context.Context, stepNum int) (StepResult, string, error) {
	a.currentStep = stepNum
	if !a.client.IsEnabled() {
		return StepResultFailed, "", fmt.Errorf("AI provider is required for migration")
	}

	systemPrompt, err := a.buildSystemPrompt(stepNum)
	if err != nil {
		return StepResultFailed, "", fmt.Errorf("building agent prompt: %w", err)
	}

	a.messages = []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: a.buildStepInstruction(stepNum)},
	}

	slog.Info("migration agent executing step", "step", stepNum, "role", a.role, "addon", a.addonName)

	tools := ToolsForRole(a.role)
	for i := 0; i < a.maxIterations; i++ {
		resp, err := a.client.ChatWithTools(ctx, a.messages, tools)
		if err != nil {
			return StepResultFailed, "", fmt.Errorf("LLM call failed: %w", err)
		}

		// Check for report_result tool call
		if len(resp.ToolCalls) > 0 {
			for _, tc := range resp.ToolCalls {
				if tc.Function.Name == "report_result" {
					// Parse the structured result
					var result AgentResult
					if err := json.Unmarshal(tc.Function.Arguments, &result); err != nil {
						return StepResultFailed, "Agent returned invalid report_result: " + err.Error(), nil
					}
					a.logFn(stepNum, strings.ToUpper(string(a.role)), "result", result.Summary)
					return mapStatus(result.Status), result.Summary, nil
				}
			}
		}

		// No tool calls and no report_result = agent gave free text (fallback)
		if len(resp.ToolCalls) == 0 {
			a.messages = append(a.messages, ai.ChatMessage{Role: "assistant", Content: resp.Content})
			// Fall back to old parsing for compatibility
			return a.parseResult(resp.Content)
		}

		// Log reasoning text
		if resp.Content != "" {
			a.logFn(stepNum, strings.ToUpper(string(a.role)), "thinking", resp.Content)
		}

		// Process non-report_result tool calls
		a.messages = append(a.messages, ai.ChatMessage{
			Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			slog.Info("migration agent tool call", "tool", tc.Function.Name, "step", stepNum, "role", a.role)
			result, toolErr := a.toolExecutor.ExecuteTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if toolErr != nil {
				result = fmt.Sprintf("Error: %s", toolErr.Error())
			}
			a.messages = append(a.messages, ai.ChatMessage{
				Role: "tool", Content: result, ToolCallID: tc.ID,
			})
		}
	}

	return StepResultFailed, "Reached maximum iterations", nil
}

// mapStatus converts string status from report_result to StepResult.
func mapStatus(status string) StepResult {
	switch strings.ToLower(status) {
	case "success":
		return StepResultSuccess
	case "failed":
		return StepResultFailed
	case "needs_user_action":
		return StepResultNeedsUser
	default:
		return StepResultFailed
	}
}
```

- [ ] **Step 4: Add ExecuteStepWithResult that returns AgentResult**

```go
// ExecuteStepWithResult runs the agent and returns the full structured result.
func (a *MigrationAgent) ExecuteStepWithResult(ctx context.Context, stepNum int) (*AgentResult, error) {
	a.currentStep = stepNum
	if !a.client.IsEnabled() {
		return nil, fmt.Errorf("AI provider is required for migration")
	}

	systemPrompt, err := a.buildSystemPrompt(stepNum)
	if err != nil {
		return nil, fmt.Errorf("building agent prompt: %w", err)
	}

	a.messages = []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: a.buildStepInstruction(stepNum)},
	}

	tools := ToolsForRole(a.role)
	for i := 0; i < a.maxIterations; i++ {
		resp, err := a.client.ChatWithTools(ctx, a.messages, tools)
		if err != nil {
			return &AgentResult{Status: "failed", Summary: "LLM call failed: " + err.Error()}, nil
		}

		// Check for report_result
		if len(resp.ToolCalls) > 0 {
			for _, tc := range resp.ToolCalls {
				if tc.Function.Name == "report_result" {
					var result AgentResult
					if err := json.Unmarshal(tc.Function.Arguments, &result); err != nil {
						return &AgentResult{Status: "failed", Summary: "Invalid report_result"}, nil
					}
					a.logFn(stepNum, strings.ToUpper(string(a.role)), "result", result.Summary)
					return &result, nil
				}
			}
		}

		if len(resp.ToolCalls) == 0 {
			// Free text fallback
			a.messages = append(a.messages, ai.ChatMessage{Role: "assistant", Content: resp.Content})
			r, summary, _ := a.parseResult(resp.Content)
			return &AgentResult{
				Status:  strings.ToLower(string(r)),
				Summary: summary,
			}, nil
		}

		if resp.Content != "" {
			a.logFn(stepNum, strings.ToUpper(string(a.role)), "thinking", resp.Content)
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

	return &AgentResult{Status: "failed", Summary: "Reached maximum iterations"}, nil
}
```

- [ ] **Step 5: Update buildStepInstruction for role-aware instructions**

Modify `buildStepInstruction` to adjust instructions based on role. For the planner, instructions focus on analysis. For the executor, they focus on action. Remove the free-text format requirements (report_result handles that now):

Replace the final `IMPORTANT RULES` and `When done` section:

```go
// At the end of buildStepInstruction, replace the format rules based on role
var formatRules string
switch a.role {
case RolePlanner:
    formatRules = `
IMPORTANT RULES:
- Use the log tool to explain what you are analyzing
- Use tools to read state — never assume file contents or app status
- Report your analysis and action plan via the report_result tool
- You MUST call report_result as your final action`
case RoleExecutor:
    formatRules = `
IMPORTANT RULES:
- Use the log tool to explain what you are doing BEFORE each action
- Follow the planner's actions in order — do not skip required actions
- If an action fails, STOP and report via report_result
- You MUST call report_result as your final action`
case RoleVerifier:
    formatRules = `
IMPORTANT RULES:
- Check each action independently using read-only tools
- Do NOT trust previous agent outputs — verify everything yourself
- Report your verification via the report_result tool
- You MUST call report_result as your final action`
default:
    formatRules = `
IMPORTANT RULES:
- Use tools to verify state before and after each action
- You MUST call report_result as your final action`
}
```

- [ ] **Step 6: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`

- [ ] **Step 7: Commit**

```bash
git add internal/migration/migration_agent.go
git commit -m "Refactor MigrationAgent for multi-agent roles with structured output"
```

---

### Task 5: Create step context management

**Files:**
- Create: `internal/migration/step_context.go`

- [ ] **Step 1: Create step_context.go**

```go
package migration

import (
	"encoding/json"
	"log/slog"
)

// SaveStepContext stores the summarized results from all three agents for a step.
func SaveStepContext(m *Migration, stepNum int, planner, executor, verifier *AgentResult) {
	if m.StepContexts == nil {
		m.StepContexts = make(map[int]*StepContext)
	}

	sc := &StepContext{}

	if planner != nil {
		sc.PlannerSummary = planner.Summary
		// Extract key facts from planner details
		var pd PlannerDetails
		if planner.Details != nil {
			if err := json.Unmarshal(planner.Details, &pd); err == nil {
				sc.KeyFacts = pd.Context
			}
		}
	}

	if executor != nil {
		sc.ExecutorSummary = executor.Summary
	}

	if verifier != nil {
		sc.VerifierSummary = verifier.Summary
	}

	m.StepContexts[stepNum] = sc
	slog.Info("saved step context", "step", stepNum, "planner", sc.PlannerSummary)
}

// BuildContextSummary creates a text summary of all previous step contexts
// for inclusion in agent prompts.
func BuildContextSummary(contexts map[int]*StepContext, upToStep int) string {
	if len(contexts) == 0 {
		return ""
	}

	var summary string
	for i := 1; i < upToStep; i++ {
		sc, ok := contexts[i]
		if !ok {
			continue
		}
		summary += "Step " + itoa(i) + ": " + sc.PlannerSummary + "\n"
		if sc.ExecutorSummary != "" {
			summary += "  Done: " + sc.ExecutorSummary + "\n"
		}
		if sc.VerifierSummary != "" {
			summary += "  Verified: " + sc.VerifierSummary + "\n"
		}
		if len(sc.KeyFacts) > 0 {
			for k, v := range sc.KeyFacts {
				summary += "  " + k + ": " + v + "\n"
			}
		}
	}
	return summary
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
```

Wait — `fmt` import is needed. Include it:

```go
import (
	"encoding/json"
	"fmt"
	"log/slog"
)
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/migration/step_context.go
git commit -m "Add step context management for multi-agent state sharing"
```

---

### Task 6: Wire multi-agent orchestration into executor

**Files:**
- Modify: `internal/migration/executor.go`

This is the main wiring task. The executor's `RunSteps` method gets a new `runStepWithAgents` path that runs Planner→Executor→Verifier.

- [ ] **Step 1: Add runStepWithAgents method**

```go
// runStepWithAgents orchestrates the three-agent pipeline for a single step:
// Planner (analyze) → Executor (act) → Verifier (confirm).
func (e *Executor) runStepWithAgents(ctx context.Context, m *Migration, stepNum int) error {
	stepCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	logFn := func(step int, repo, action, detail string) {
		e.addLog(m, step, repo, action, detail)
	}

	// Shared tool executor
	toolExec := NewMigrationToolExecutor(e.newGP, e.oldGP, e.newArgoCD, e.oldArgoCD,
		func(repo, action, detail string) {
			logFn(stepNum, repo, action, detail)
		})

	// --- PLANNER ---
	e.addLog(m, stepNum, "PLANNER", "starting", "Analyzing current state...")

	planner := &MigrationAgent{
		client:       e.aiClient,
		toolExecutor: toolExec,
		maxIterations: 15,
		currentStep:  stepNum,
		role:         RolePlanner,
		addonName:    m.AddonName,
		clusterName:  m.ClusterName,
		steps:        m.Steps,
		stepContexts: m.StepContexts,
		logFn:        logFn,
	}

	plannerResult, err := planner.ExecuteStepWithResult(stepCtx, stepNum)
	if err != nil {
		return fmt.Errorf("planner error: %w", err)
	}
	if plannerResult.Status == "failed" {
		return fmt.Errorf("planner: %s", plannerResult.Summary)
	}

	// Check if planner says no actions needed (e.g., step 1 verify, step 2 compare)
	var pd PlannerDetails
	hasActions := false
	if plannerResult.Details != nil {
		if err := json.Unmarshal(plannerResult.Details, &pd); err == nil {
			hasActions = len(pd.Actions) > 0
		}
	}

	if !hasActions {
		// No write actions — skip executor, just verify
		e.addLog(m, stepNum, "PLANNER", "result", plannerResult.Summary+" (no actions needed)")

		// Run verifier to confirm the planner's findings
		e.addLog(m, stepNum, "VERIFIER", "starting", "Verifying planner findings...")
		verifier := &MigrationAgent{
			client:       e.aiClient,
			toolExecutor: toolExec,
			maxIterations: 10,
			currentStep:  stepNum,
			role:         RoleVerifier,
			addonName:    m.AddonName,
			clusterName:  m.ClusterName,
			steps:        m.Steps,
			stepContexts: m.StepContexts,
			logFn:        logFn,
		}
		verifierResult, err := verifier.ExecuteStepWithResult(stepCtx, stepNum)
		if err != nil {
			return fmt.Errorf("verifier error: %w", err)
		}
		if verifierResult.Status == "failed" {
			return fmt.Errorf("verification failed: %s", verifierResult.Summary)
		}

		SaveStepContext(m, stepNum, plannerResult, nil, verifierResult)
		return nil
	}

	// Gates mode: show plan to user, wait for approval
	if m.Mode == "gates" {
		e.addLog(m, stepNum, "PLANNER", "plan", plannerResult.Summary)
		for _, a := range pd.Actions {
			e.addLog(m, stepNum, "PLANNER", "action",
				fmt.Sprintf("[%s] %s (required: %v)", a.Repo, a.Description, a.Required))
		}
		// The gating happens at the outer level (RunSteps handles StatusGated)
	}

	// --- EXECUTOR ---
	e.addLog(m, stepNum, "EXECUTOR", "starting", "Performing planned actions...")

	// Build executor instruction that includes the planner's plan
	executor := &MigrationAgent{
		client:       e.aiClient,
		toolExecutor: toolExec,
		maxIterations: 20,
		currentStep:  stepNum,
		role:         RoleExecutor,
		addonName:    m.AddonName,
		clusterName:  m.ClusterName,
		steps:        m.Steps,
		stepContexts: m.StepContexts,
		logFn:        logFn,
	}

	// Inject planner output into executor's context
	planJSON, _ := json.Marshal(plannerResult)
	executor.plannerOutput = string(planJSON)

	executorResult, err := executor.ExecuteStepWithResult(stepCtx, stepNum)
	if err != nil {
		return fmt.Errorf("executor error: %w", err)
	}
	if executorResult.Status == "failed" {
		return fmt.Errorf("executor: %s", executorResult.Summary)
	}
	if executorResult.Status == "needs_user_action" {
		// Save context so far, return special error for the outer loop
		SaveStepContext(m, stepNum, plannerResult, executorResult, nil)
		return &needsUserError{message: executorResult.Summary}
	}

	// --- VERIFIER ---
	e.addLog(m, stepNum, "VERIFIER", "starting", "Verifying executed actions...")

	verifier := &MigrationAgent{
		client:       e.aiClient,
		toolExecutor: toolExec,
		maxIterations: 10,
		currentStep:  stepNum,
		role:         RoleVerifier,
		addonName:    m.AddonName,
		clusterName:  m.ClusterName,
		steps:        m.Steps,
		stepContexts: m.StepContexts,
		logFn:        logFn,
	}

	// Inject planner + executor output into verifier's context
	execJSON, _ := json.Marshal(executorResult)
	verifier.plannerOutput = string(planJSON)
	verifier.executorOutput = string(execJSON)

	verifierResult, err := verifier.ExecuteStepWithResult(stepCtx, stepNum)
	if err != nil {
		return fmt.Errorf("verifier error: %w", err)
	}
	if verifierResult.Status == "failed" {
		return fmt.Errorf("verification failed: %s", verifierResult.Summary)
	}

	// Save context for next steps
	SaveStepContext(m, stepNum, plannerResult, executorResult, verifierResult)

	e.addLog(m, stepNum, "VERIFIER", "result", "✓ "+verifierResult.Summary)
	return nil
}

// needsUserError signals that the executor needs user action (not a failure).
type needsUserError struct {
	message string
}

func (e *needsUserError) Error() string {
	return e.message
}
```

- [ ] **Step 2: Add plannerOutput and executorOutput fields to MigrationAgent**

In `migration_agent.go`, add to the struct:

```go
plannerOutput  string // JSON of planner's AgentResult (for executor/verifier)
executorOutput string // JSON of executor's AgentResult (for verifier)
```

And include them in `buildStepInstruction`:

```go
// After the existing step instruction, append cross-agent context
if a.plannerOutput != "" {
    instruction += "\n\n## Planner's Analysis:\n" + a.plannerOutput
}
if a.executorOutput != "" {
    instruction += "\n\n## Executor's Actions:\n" + a.executorOutput
}
```

- [ ] **Step 3: Update RunSteps to use runStepWithAgents**

Replace the AI execution path in `RunSteps`:

```go
// In the existing RunSteps method, replace the agent execution block:
if e.aiClient != nil && e.aiClient.IsEnabled() {
    stepErr = e.runStepWithAgents(stepCtx, m, m.CurrentStep)
    // Handle needsUserError specially
    if nue, ok := stepErr.(*needsUserError); ok {
        step.Status = StepWaiting
        step.Message = nue.message
        m.Status = StatusWaiting
        m.UpdatedAt = now()
        _ = e.store.SaveMigration(m)
        stepCancel()
        return
    }
} else {
    stepErr = e.executeStep(stepCtx, m, m.CurrentStep)
}
```

- [ ] **Step 4: Remove old single-agent caching (getOrCreateAgent)**

The multi-agent approach creates fresh agents per step per role. Remove the `agents` map and `getOrCreateAgent`/`GetAgent`/`CreateAgentForMigration` methods. Replace `CreateAgentForMigration` with a troubleshooter:

```go
// CreateTroubleshootAgent creates a read-only agent for troubleshooting chat.
func (e *Executor) CreateTroubleshootAgent(m *Migration) *MigrationAgent {
	logFn := func(step int, repo, action, detail string) {
		e.addLog(m, step, repo, action, detail)
	}
	toolExec := NewMigrationToolExecutor(e.newGP, e.oldGP, e.newArgoCD, e.oldArgoCD,
		func(repo, action, detail string) {
			logFn(m.CurrentStep, repo, action, detail)
		})

	return &MigrationAgent{
		client:       e.aiClient,
		toolExecutor: toolExec,
		maxIterations: 10,
		currentStep:  m.CurrentStep,
		role:         RoleTroubleshooter,
		addonName:    m.AddonName,
		clusterName:  m.ClusterName,
		steps:        m.Steps,
		stepContexts: m.StepContexts,
		logFn:        logFn,
	}
}
```

- [ ] **Step 5: Update the error diagnosis section in RunSteps**

Replace the agent-based diagnosis with a troubleshooter:

```go
if e.aiClient != nil && e.aiClient.IsEnabled() {
    troubleshooter := e.CreateTroubleshootAgent(m)
    diagCtx, diagCancel := context.WithTimeout(context.Background(), 30*time.Second)
    diagResult, diagErr := troubleshooter.Chat(diagCtx, fmt.Sprintf(
        "Step %d failed with this error:\n%s\n\nExplain what went wrong in 2-3 sentences.",
        m.CurrentStep, rawErr))
    diagCancel()
    if diagErr == nil && diagResult != "" {
        diagnosis = diagResult
        e.addLog(m, m.CurrentStep, "AGENT", "diagnosis", diagnosis)
    }
}
```

- [ ] **Step 6: Update the Executor struct — remove agents map**

```go
type Executor struct {
	store     *Store
	newGP     gitprovider.GitProvider
	oldGP     gitprovider.GitProvider
	newArgoCD *argocd.Client
	oldArgoCD *argocd.Client
	aiClient  *ai.Client

	mu      sync.Mutex
	running map[string]context.CancelFunc
}
```

Update `NewExecutor` to remove `agents: make(...)`.

- [ ] **Step 7: Add `encoding/json` import to executor.go**

- [ ] **Step 8: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`

- [ ] **Step 9: Commit**

```bash
git add internal/migration/executor.go internal/migration/migration_agent.go
git commit -m "Wire multi-agent Planner→Executor→Verifier pipeline into migration executor"
```

---

### Task 7: Update API handler for troubleshooting chat

**Files:**
- Modify: `internal/api/migration_handler.go` (or wherever the chat endpoint lives)

- [ ] **Step 1: Find the chat handler**

Search for the `/migration/{id}/chat` endpoint handler.

- [ ] **Step 2: Update to use CreateTroubleshootAgent instead of GetAgent**

The handler currently calls `executor.GetAgent(id)` or `executor.CreateAgentForMigration(m)`. Replace with:

```go
agent := executor.CreateTroubleshootAgent(m)
// Include step context in the troubleshooter's system prompt
response, err := agent.Chat(ctx, request.Message)
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/api/migration_handler.go
git commit -m "Update troubleshooting chat to use CreateTroubleshootAgent"
```

---

### Task 8: Update UI to show planner/executor/verifier phases

**Files:**
- Modify: `ui/src/views/MigrationDetail.tsx`

- [ ] **Step 1: Add agent role color mapping**

Add colors for each agent role in the log display:

```typescript
const agentRoleColors: Record<string, string> = {
  'PLANNER': 'text-blue-400',
  'EXECUTOR': 'text-yellow-400',
  'VERIFIER': 'text-green-400',
  'AGENT': 'text-violet-400',    // legacy/troubleshooter
  'SYSTEM': 'text-gray-400',
};
```

- [ ] **Step 2: Update log entry rendering**

In the log rendering section, use the role-based colors:

```typescript
const getLogColor = (repo: string) => {
  return agentRoleColors[repo] || 'text-gray-300';
};
```

- [ ] **Step 3: Add phase indicator in step header**

When a step is running, show which phase it's in (Planning/Executing/Verifying) based on the latest log entry's repo field:

```typescript
const getCurrentPhase = (step: MigrationStep, logs: LogEntry[]) => {
  const stepLogs = logs.filter(l => l.step === step.number);
  const lastLog = stepLogs[stepLogs.length - 1];
  if (!lastLog) return null;
  if (lastLog.repo === 'PLANNER') return '🔍 Planning';
  if (lastLog.repo === 'EXECUTOR') return '⚡ Executing';
  if (lastLog.repo === 'VERIFIER') return '✓ Verifying';
  return null;
};
```

- [ ] **Step 4: Verify dev build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform/ui && npm run build`

- [ ] **Step 5: Commit**

```bash
git add ui/src/views/MigrationDetail.tsx
git commit -m "Show planner/executor/verifier phases in migration UI with role colors"
```

---

### Task 9: Bump version, create PR, merge, deploy

**Files:**
- Modify: `charts/argocd-addons-platform/Chart.yaml`

- [ ] **Step 1: Bump chart version**

Increment the minor version (new feature).

- [ ] **Step 2: Create feature branch, commit all changes, push**

```bash
git checkout -b feature/migration-v4-multi-agent
git add -A
git push -u origin feature/migration-v4-multi-agent
```

- [ ] **Step 3: Create PR**

```bash
gh pr create --title "Migration v4: Multi-agent Planner→Executor→Verifier" --body "..."
```

- [ ] **Step 4: Merge PR**

```bash
gh pr merge --squash --delete-branch
```

- [ ] **Step 5: Wait for CI/CD and verify deployment**
