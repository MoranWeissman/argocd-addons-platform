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
		{"The addon was enabled successfully", StepResultFailed}, // no prefix = default to FAILED (safe)
		{"An error occurred while reading", StepResultFailed},
	}

	for _, tt := range tests {
		result, _, _ := agent.parseResult(tt.input)
		if result != tt.expected {
			t.Errorf("parseResult(%q) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}
