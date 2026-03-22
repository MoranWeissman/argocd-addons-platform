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
	// Create a step-aware log wrapper; currentStep is captured by reference via the closure.
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

	_ = currentStep // suppress unused-variable warning; used via closure in stepLogFn

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
