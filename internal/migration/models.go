package migration

import (
	"crypto/rand"
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
	PRNumber    int        `json:"pr_number,omitempty"`
	PRRepo      string     `json:"pr_repo,omitempty"` // "new" or "old" — which provider to merge on
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

// MigrationBatch represents a sequential queue of addon migrations for a cluster.
type MigrationBatch struct {
	ID          string   `json:"id"`
	ClusterName string   `json:"cluster_name"`
	Mode        string   `json:"mode"`
	Addons      []string `json:"addons"`       // all addons in order
	MigrationIDs []string `json:"migration_ids"` // corresponding migration IDs
	CurrentIndex int      `json:"current_index"` // which addon is currently running (0-based)
	Status      string   `json:"status"`        // "running", "completed", "failed", "cancelled"
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
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
	{"Verify addon in catalog", "Checks that the addon exists in the NEW repo's catalog and is marked for migration."},
	{"Compare values", "Reads and compares addon configuration values between the OLD and NEW repositories to identify any differences."},
	{"Enable addon in NEW repo", "Creates a pull request in the NEW repo to enable the addon on the target cluster."},
	{"Verify app in NEW ArgoCD", "Waits for ArgoCD to detect the new configuration and create the application."},
	{"Disable addon in OLD repo", "Creates a pull request in the OLD repo to disable the addon, preparing for handover."},
	{"Sync OLD ArgoCD", "Triggers a sync in the OLD ArgoCD so it processes the removal."},
	{"Verify removal from OLD ArgoCD", "Confirms the application has been removed from the OLD ArgoCD instance."},
	{"Refresh NEW ArgoCD", "Triggers a hard refresh in the NEW ArgoCD so it adopts any orphaned resources."},
	{"Verify healthy", "Confirms the application is Synced and Healthy in the NEW ArgoCD with no issues."},
	{"Finalize migration", "Creates a pull request to mark the migration as complete in the addon catalog."},
}

// NewMigration creates a new Migration with all 10 steps pre-populated in
// pending state.
func NewMigration(addonName, clusterName, mode string) *Migration {
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	id := fmt.Sprintf("mig-%d-%x", time.Now().Unix(), randBytes)
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
