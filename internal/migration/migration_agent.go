package migration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
)

// StepResult is the outcome of an agent step execution.
type StepResult string

const (
	StepResultSuccess   StepResult = "SUCCESS"
	StepResultFailed    StepResult = "FAILED"
	StepResultNeedsUser StepResult = "NEEDS_USER_ACTION"
)

// MigrationAgent executes migration steps using an LLM with tools.
type MigrationAgent struct {
	client        *ai.Client
	toolExecutor  *MigrationToolExecutor
	messages      []ai.ChatMessage
	maxIterations int
	currentStep   int // tracks which step the agent is executing (for log routing)

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
	agent := &MigrationAgent{
		client:        client,
		maxIterations: 20,
		addonName:     addonName,
		clusterName:   clusterName,
		steps:         steps,
		logFn:         logFn,
	}

	// Create tool executor with a closure that reads the current step from the agent
	stepLogFn := func(repo, action, detail string) {
		logFn(agent.currentStep, repo, action, detail)
	}
	agent.toolExecutor = NewMigrationToolExecutor(newGP, oldGP, newArgoCD, oldArgoCD, stepLogFn)

	// Allow override from config
	if client != nil && client.GetConfig().MaxIterations > 0 {
		agent.maxIterations = client.GetConfig().MaxIterations
	}

	return agent
}

// ExecuteStep runs the agent for a specific migration step.
func (a *MigrationAgent) ExecuteStep(ctx context.Context, stepNum int) (StepResult, string, error) {
	a.currentStep = stepNum
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

		// Log the agent's reasoning text (if any) before processing tool calls
		if resp.Content != "" {
			a.logFn(stepNum, "AGENT", "thinking", resp.Content)
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
	// Read core knowledge files
	guideContent := readKnowledgeFile("docs/agent/migration-guide.md")
	promptContent := readKnowledgeFile("docs/agent/migration-agent-prompt.md")

	// Load relevant skills based on what the step needs
	skills := a.getSkillsForStep(stepNum)
	var skillsContent strings.Builder
	for _, skill := range skills {
		content := readKnowledgeFile("docs/agent/skills/" + skill + ".md")
		if content != "" && !strings.HasPrefix(content, "(Knowledge file") {
			skillsContent.WriteString(fmt.Sprintf("\n---\n## Skill: %s\n%s\n", skill, content))
		}
	}

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
%s
---

## Current Migration Context

Addon: %s
Cluster: %s
Current Step: %d

## All Steps Status:
%s`,
		promptContent, guideContent, skillsContent.String(),
		a.addonName, a.clusterName, stepNum,
		stepSummary.String())

	return context, nil
}

// getSkillsForStep returns which skill files are relevant for a given step.
// Keep it minimal — only 1-2 most relevant skills to avoid overloading the prompt.
func (a *MigrationAgent) getSkillsForStep(stepNum int) []string {
	switch stepNum {
	case 1: // Verify catalog
		return []string{"yaml"}
	case 2: // Compare values
		return []string{"helm"}
	case 3: // Enable addon in NEW repo (PR)
		return []string{"github"}
	case 4: // Verify app in NEW ArgoCD
		return []string{"argocd"}
	case 5: // Disable addon in OLD repo (PR)
		return []string{"azure-devops"}
	case 6: // Sync OLD ArgoCD
		return []string{"argocd"}
	case 7: // Verify removal
		return []string{"argocd"}
	case 8: // Refresh NEW ArgoCD
		return []string{"argocd"}
	case 9: // Verify healthy
		return []string{"kubernetes"}
	case 10: // Finalize
		return []string{"gitops"}
	default:
		return nil
	}
}

func (a *MigrationAgent) buildStepInstruction(stepNum int) string {
	if stepNum < 1 || stepNum > len(a.steps) {
		return fmt.Sprintf("Execute migration step %d", stepNum)
	}

	step := a.steps[stepNum-1]

	// Add step-specific instructions for critical steps
	specific := ""
	switch stepNum {
	case 1:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS:
1. Use git_read_file to read "configuration/addons-catalog.yaml" from the "new" repo
2. Check that addon "%s" exists with inMigration: true
3. If found, respond SUCCESS. If not found, respond FAILED.`, a.addonName)
	case 2:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS:
1. Read global values for "%s" from both "new" and "old" repos
2. Read cluster values for "%s" from both repos
3. Use the log tool to write your comparison findings (do NOT put everything in the final response)
4. Keep your final SUCCESS response SHORT (1-2 sentences). The details should already be in the logs.
5. This step is advisory — differences don't block migration.`, a.addonName, a.clusterName)
	case 3:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS — YOU MUST CREATE A PULL REQUEST:
1. Use git_read_file to read "configuration/cluster-addons.yaml" from the "new" repo
2. Find the exact YAML block for cluster "%s" that contains the addon label
3. Use git_create_pr with "find" and "replace" parameters to make a SURGICAL edit:
   - "find": the exact text snippet containing "%s: disabled" (or missing) for cluster "%s"
   - "replace": the same snippet with "%s: enabled"
   CRITICAL: Do NOT pass the entire file as "content". Use find/replace to safely edit without losing data.
4. Do NOT merge the PR. Respond NEEDS_USER_ACTION asking the user to review and merge.
5. You MUST use the git_create_pr tool. Do NOT respond SUCCESS without creating a PR.
6. After creating the PR, respond NEEDS_USER_ACTION with the PR URL and ask user to review and merge it.`, a.clusterName, a.addonName, a.clusterName, a.addonName)
	case 5:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS — YOU MUST CREATE A PULL REQUEST:
1. Use git_read_file to read the cluster config from the "old" repo (try "configuration/cluster-addons.yaml" first, then "values/clusters.yaml")
2. Find the exact YAML block for cluster "%s" that contains the addon label
3. Use git_create_pr with "find" and "replace" parameters to make a SURGICAL edit:
   - "find": the exact text snippet containing "%s: enabled" for cluster "%s"
   - "replace": the same snippet with "%s: disabled"
   CRITICAL: Do NOT pass the entire file as "content". Use find/replace to safely edit without losing data.
4. Do NOT merge the PR. Respond NEEDS_USER_ACTION asking the user to review and merge.
5. You MUST use the git_create_pr tool. Do NOT respond SUCCESS without creating a PR.
6. After creating the PR, respond NEEDS_USER_ACTION with the PR URL and ask user to review and merge it.`, a.clusterName, a.addonName, a.clusterName, a.addonName)
	case 4:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS:
1. Use argocd_get_app to check if application "%s-%s" exists in the "new" ArgoCD
2. If not found, wait 10 seconds and try again (up to 3 attempts)
3. If found, respond SUCCESS with the sync and health status
4. If not found after 3 attempts, respond FAILED`, a.addonName, a.clusterName)
	case 6:
		specific = `
REQUIRED ACTIONS:
1. Use argocd_list_apps on the "old" ArgoCD to find the clusters/bootstrap application
2. Use argocd_sync_app to trigger a sync on that application
3. Respond SUCCESS after sync is triggered`
	case 7:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS:
1. Use argocd_get_app to check if "%s-%s" still exists in the "old" ArgoCD
2. If NOT found (error/404), that's the desired state — respond SUCCESS
3. If still found, wait and retry (up to 3 attempts)
4. If still present after retries, respond FAILED`, a.addonName, a.clusterName)
	case 8:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS:
1. Use argocd_refresh_app on "%s-%s" in the "new" ArgoCD
2. Respond SUCCESS with the resulting sync and health status`, a.addonName, a.clusterName)
	case 9:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS:
1. Use argocd_get_app to check "%s-%s" health in the "new" ArgoCD
2. If HealthStatus is "Healthy", respond SUCCESS (OutOfSync is OK)
3. If not healthy, respond FAILED with the status`, a.addonName, a.clusterName)
	case 10:
		specific = fmt.Sprintf(`
REQUIRED ACTIONS:
1. First check if ALL clusters have been migrated for addon "%s":
   - Read cluster config from "old" repo to find which clusters had this addon
   - Read cluster config from "new" repo to find which clusters have it enabled
2. If clusters remain unmigrated in the old repo, respond NEEDS_USER_ACTION listing them
3. If all migrated, use git_create_pr with find/replace to set inMigration: false in addons-catalog.yaml:
   - "find": "inMigration: true" (for this addon's block)
   - "replace": "inMigration: false"
   CRITICAL: Do NOT pass the entire file as "content". Use find/replace to safely edit.
4. Do NOT merge the PR. Respond NEEDS_USER_ACTION with the PR URL and ask user to review and merge it.`, a.addonName)
	}

	return fmt.Sprintf(`Execute migration step %d: "%s"

Description: %s
%s

IMPORTANT RULES:
- Use the log tool to explain what you are doing BEFORE each action
- Use tools to verify state before and after each action
- Do NOT claim success without actually performing the required actions
- Do NOT skip tool calls — every action must go through the tools

When done, you MUST respond with EXACTLY one of these prefixes:
- SUCCESS: <brief summary of what was accomplished>
- FAILED: <what went wrong and your diagnosis>
- NEEDS_USER_ACTION: <what the user needs to do>`, stepNum, step.Title, step.Description, specific)
}

func (a *MigrationAgent) parseResult(content string) (StepResult, string, error) {
	content = strings.TrimSpace(content)

	// Check for prefixes at the start
	if strings.HasPrefix(content, "SUCCESS:") {
		return StepResultSuccess, strings.TrimSpace(strings.TrimPrefix(content, "SUCCESS:")), nil
	}
	if strings.HasPrefix(content, "FAILED:") {
		return StepResultFailed, strings.TrimSpace(strings.TrimPrefix(content, "FAILED:")), nil
	}
	if strings.HasPrefix(content, "NEEDS_USER_ACTION:") {
		return StepResultNeedsUser, strings.TrimSpace(strings.TrimPrefix(content, "NEEDS_USER_ACTION:")), nil
	}

	// Agent may put text before the prefix — search for it anywhere
	for _, prefix := range []string{"SUCCESS:", "FAILED:", "NEEDS_USER_ACTION:"} {
		if idx := strings.Index(content, prefix); idx >= 0 {
			after := strings.TrimSpace(content[idx+len(prefix):])
			switch prefix {
			case "SUCCESS:":
				return StepResultSuccess, after, nil
			case "FAILED:":
				return StepResultFailed, after, nil
			case "NEEDS_USER_ACTION:":
				return StepResultNeedsUser, after, nil
			}
		}
	}

	// Last resort: check for clear success/failure signals
	lower := strings.ToLower(content)
	if strings.Contains(lower, "successfully") || strings.Contains(lower, "verified") || strings.Contains(lower, "confirmed") || strings.Contains(lower, "found") {
		return StepResultSuccess, content, nil
	}

	return StepResultFailed, "Agent did not return a clear result. Response: " + content, nil
}

func readKnowledgeFile(relativePath string) string {
	// Try common locations
	paths := []string{
		relativePath,
		filepath.Join("/app", relativePath),
	}

	// Also try relative to the binary source location
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
		"git_create_pr":      true,
		"git_merge_pr":       true,
		"argocd_sync_app":    true,
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
