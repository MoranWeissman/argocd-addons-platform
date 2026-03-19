package migration

import (
	"fmt"
	"time"
)

// MigrationStatus represents the overall status of a migration.
type MigrationStatus string

const (
	StatusPending   MigrationStatus = "pending"
	StatusRunning   MigrationStatus = "running"
	StatusWaiting   MigrationStatus = "waiting"   // waiting for PR merge
	StatusPaused    MigrationStatus = "paused"
	StatusCompleted MigrationStatus = "completed"
	StatusFailed    MigrationStatus = "failed"
	StatusGated     MigrationStatus = "gated" // paused at gate, waiting for user approval in gates mode
	StatusCancelled MigrationStatus = "cancelled"
)

// StepStatus represents the status of an individual migration step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepWaiting   StepStatus = "waiting"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// LogEntry represents a single log entry emitted during migration execution.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Step      int    `json:"step"`
	Repo      string `json:"repo"`   // e.g., "NEW (github.com/org/repo)" or "OLD (dev.azure.com/org/project/repo)"
	Action    string `json:"action"` // "reading", "creating", "comparing", etc.
	Detail    string `json:"detail"`
}

// MigrationStep represents a single step in the migration process.
type MigrationStep struct {
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      StepStatus `json:"status"`
	Message     string     `json:"message"`
	PRURL       string     `json:"pr_url,omitempty"`
	PRStatus    string     `json:"pr_status,omitempty"`
	StartedAt   string     `json:"started_at,omitempty"`
	CompletedAt string     `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// Migration represents a full addon migration from an old ArgoCD instance to
// the new platform-managed instance.
type Migration struct {
	ID          string          `json:"id"`
	AddonName   string          `json:"addon_name"`
	ClusterName string          `json:"cluster_name"`
	Status      MigrationStatus `json:"status"`
	CurrentStep int             `json:"current_step"`
	Mode        string          `json:"mode"` // "yolo" or "gates"
	Steps       []MigrationStep `json:"steps"`
	Logs        []LogEntry      `json:"logs"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	CompletedAt string          `json:"completed_at,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// MigrationSettings holds the configuration for connecting to the old
// (source) Git repository and ArgoCD instance during migration.
type MigrationSettings struct {
	OldGit     OldGitConfig    `json:"old_git" yaml:"old_git"`
	OldArgocd  OldArgocdConfig `json:"old_argocd" yaml:"old_argocd"`
	Configured bool            `json:"configured"`
}

// OldGitConfig describes the old Git repository that addons are being
// migrated away from.
type OldGitConfig struct {
	Provider string `json:"provider" yaml:"provider"` // "github" or "azuredevops"
	// GitHub fields
	Owner string `json:"owner,omitempty" yaml:"owner,omitempty"`
	Repo  string `json:"repo,omitempty" yaml:"repo,omitempty"`
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
	// Azure DevOps fields
	Organization string `json:"organization,omitempty" yaml:"organization,omitempty"`
	Project      string `json:"project,omitempty" yaml:"project,omitempty"`
	Repository   string `json:"repository,omitempty" yaml:"repository,omitempty"`
	PAT          string `json:"pat,omitempty" yaml:"pat,omitempty"`
}

// OldArgocdConfig describes the old ArgoCD instance that addons are being
// migrated away from.
type OldArgocdConfig struct {
	ServerURL string `json:"server_url" yaml:"server_url"`
	Token     string `json:"token" yaml:"token"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Insecure  bool   `json:"insecure,omitempty" yaml:"insecure,omitempty"`
}

// StepDefinitions describes the 10 migration steps in order.
var StepDefinitions = []struct {
	Title       string
	Description string
}{
	{"Verify addon in NEW catalog", "Check that the addon exists in addons-catalog.yaml with inMigration: true"},
	{"Configure values in NEW repo", "Verify global and cluster values match the OLD repo configuration"},
	{"Enable addon on cluster", "Create PR to set addon label to 'enabled' in cluster-addons.yaml"},
	{"Verify app created in NEW ArgoCD", "Check that ArgoCD created the application (may show OutOfSync)"},
	{"Disable addon in OLD repo", "Create PR to disable the addon label in the OLD repository"},
	{"Sync clusters app in OLD ArgoCD", "Trigger sync so OLD ArgoCD removes the application"},
	{"Verify app removed from OLD ArgoCD", "Confirm the application no longer exists in OLD ArgoCD"},
	{"Hard refresh in NEW ArgoCD", "Trigger hard refresh so NEW ArgoCD adopts orphaned resources"},
	{"Verify healthy in NEW ArgoCD", "Confirm application is Synced + Healthy with no pod restarts"},
	{"Disable migration mode", "Create PR to set inMigration: false in addons-catalog.yaml"},
}

// NewMigration creates a new Migration with all 10 steps pre-populated in
// pending state.
func NewMigration(addonName, clusterName, mode string) *Migration {
	id := fmt.Sprintf("mig-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)

	if mode == "" {
		mode = "gates"
	}

	steps := make([]MigrationStep, len(StepDefinitions))
	for i, def := range StepDefinitions {
		steps[i] = MigrationStep{
			Number:      i + 1,
			Title:       def.Title,
			Description: def.Description,
			Status:      StepPending,
		}
	}

	return &Migration{
		ID:          id,
		AddonName:   addonName,
		ClusterName: clusterName,
		Status:      StatusPending,
		CurrentStep: 1,
		Mode:        mode,
		Steps:       steps,
		Logs:        []LogEntry{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
