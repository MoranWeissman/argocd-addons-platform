package migration

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
)

// Executor orchestrates the 10-step migration workflow, advancing through
// steps, creating PRs, and polling ArgoCD as needed.
type Executor struct {
	store     *Store
	newGP     gitprovider.GitProvider
	oldGP     gitprovider.GitProvider
	newArgoCD *argocd.Client
	oldArgoCD *argocd.Client
	aiClient  *ai.Client

	mu      sync.Mutex
	running map[string]context.CancelFunc // active goroutines keyed by migration ID
}

// NewExecutor creates a migration executor wired to the given providers.
func NewExecutor(
	store *Store,
	newGP gitprovider.GitProvider,
	oldGP gitprovider.GitProvider,
	newAC, oldAC *argocd.Client,
	aiClient *ai.Client,
) *Executor {
	return &Executor{
		store:     store,
		newGP:     newGP,
		oldGP:     oldGP,
		newArgoCD: newAC,
		oldArgoCD: oldAC,
		aiClient:  aiClient,
		running:   make(map[string]context.CancelFunc),
	}
}

// SetOldProviders sets the old (source) git provider and ArgoCD client.
// These are resolved at runtime from the saved migration settings.
func (e *Executor) SetOldProviders(gp gitprovider.GitProvider, ac *argocd.Client) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.oldGP = gp
	e.oldArgoCD = ac
}

// SetNewProviders sets the new (target) git provider and ArgoCD client.
// These are resolved at runtime from the active connection.
func (e *Executor) SetNewProviders(gp gitprovider.GitProvider, ac *argocd.Client) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.newGP = gp
	e.newArgoCD = ac
}

// GetStore returns the underlying migration store.
func (e *Executor) GetStore() *Store {
	return e.store
}

// addLog appends a structured log entry to the migration and persists it.
func (e *Executor) addLog(m *Migration, step int, repo, action, detail string) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Step:      step,
		Repo:      repo,
		Action:    action,
		Detail:    detail,
	}
	m.Logs = append(m.Logs, entry)
	_ = e.store.SaveMigration(m)
	slog.Info("migration log", "id", m.ID, "step", step, "repo", repo, "action", action, "detail", detail)
}

// newRepoLabel returns a descriptive label for the new (target) repository.
func (e *Executor) newRepoLabel() string {
	return "NEW repo"
}

// oldRepoLabel returns a descriptive label for the old (source) repository.
func (e *Executor) oldRepoLabel() string {
	return "OLD repo"
}

// StartMigration creates a new migration and begins executing steps in the
// background. The migration object is returned immediately so the caller can
// track progress via the store.
func (e *Executor) StartMigration(ctx context.Context, addonName, clusterName, mode string) (*Migration, error) {
	m := NewMigration(addonName, clusterName, mode)
	m.Status = StatusRunning
	if err := e.store.SaveMigration(m); err != nil {
		return nil, fmt.Errorf("saving new migration: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx) // #nosec G118 -- cancel is stored in e.running and deferred in the goroutine
	e.mu.Lock()
	e.running[m.ID] = cancel
	e.mu.Unlock()

	go func() {
		defer cancel()
		defer func() {
			e.mu.Lock()
			delete(e.running, m.ID)
			e.mu.Unlock()
		}()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("migration: panic in step execution", "id", m.ID, "panic", r)
				if mg, err := e.store.GetMigration(m.ID); err == nil {
					mg.Status = StatusFailed
					mg.Error = fmt.Sprintf("internal error: %v", r)
					mg.UpdatedAt = now()
					_ = e.store.SaveMigration(mg)
				}
			}
		}()
		e.RunSteps(runCtx, m.ID)
	}()

	return m, nil
}

// RunSteps executes steps from current_step until completion, a step enters
// waiting state (PR created), or a step fails. It is safe to call from a
// goroutine.
func (e *Executor) RunSteps(ctx context.Context, migrationID string) {
	for {
		if ctx.Err() != nil {
			return
		}

		m, err := e.store.GetMigration(migrationID)
		if err != nil {
			slog.Error("migration: failed to read state", "id", migrationID, "error", err)
			return
		}
		if m.Status != StatusRunning {
			return
		}
		if m.CurrentStep > len(StepDefinitions) {
			m.Status = StatusCompleted
			m.CompletedAt = now()
			m.UpdatedAt = now()
			_ = e.store.SaveMigration(m)
			slog.Info("migration completed", "id", migrationID)
			return
		}

		step := &m.Steps[m.CurrentStep-1]
		step.Status = StepRunning
		step.StartedAt = now()
		m.UpdatedAt = now()
		_ = e.store.SaveMigration(m)

		slog.Info("migration: executing step", "id", migrationID, "step", m.CurrentStep, "title", step.Title)

		// Execute step with timeout (2 minutes per step)
		stepCtx, stepCancel := context.WithTimeout(ctx, 2*time.Minute)
		stepErr := e.executeStep(stepCtx, m, m.CurrentStep)
		stepCancel()

		// Re-read in case the step handler updated the migration.
		m, err = e.store.GetMigration(migrationID)
		if err != nil {
			slog.Error("migration: failed to re-read state", "id", migrationID, "error", err)
			return
		}

		step = &m.Steps[m.CurrentStep-1]

		if stepErr != nil {
			step.Status = StepFailed
			step.Error = stepErr.Error()
			m.Status = StatusFailed
			m.Error = stepErr.Error()
			m.UpdatedAt = now()
			_ = e.store.SaveMigration(m)
			slog.Error("migration: step failed", "id", migrationID, "step", m.CurrentStep, "error", stepErr)
			return
		}

		// If the step set itself to waiting (e.g. PR created), pause the loop.
		if step.Status == StepWaiting {
			m.Status = StatusWaiting
			m.UpdatedAt = now()
			_ = e.store.SaveMigration(m)
			slog.Info("migration: waiting for PR", "id", migrationID, "step", m.CurrentStep)
			return
		}

		// Step completed — advance.
		step.Status = StepCompleted
		step.CompletedAt = now()
		m.CurrentStep++
		m.UpdatedAt = now()
		_ = e.store.SaveMigration(m)

		// If gates mode, pause and wait for user approval before next step.
		if m.Mode == "gates" && m.CurrentStep <= len(StepDefinitions) {
			m.Status = StatusGated
			m.UpdatedAt = now()
			_ = e.store.SaveMigration(m)
			slog.Info("migration: gated — waiting for user approval", "id", migrationID, "step", m.CurrentStep)
			return
		}
	}
}

// ContinueAfterPR resumes execution after the user confirms a PR was merged.
func (e *Executor) ContinueAfterPR(ctx context.Context, migrationID string) error {
	m, err := e.store.GetMigration(migrationID)
	if err != nil {
		return err
	}
	if m.Status != StatusWaiting && m.Status != StatusGated {
		return fmt.Errorf("migration %s is not in waiting or gated state (current: %s)", migrationID, m.Status)
	}

	// For waiting migrations (PR created), mark the current step as completed.
	// For gated migrations, the step is already completed — just resume.
	if m.Status == StatusWaiting {
		step := &m.Steps[m.CurrentStep-1]
		step.Status = StepCompleted
		step.CompletedAt = now()
		step.PRStatus = "merged"
		m.CurrentStep++
	}
	m.Status = StatusRunning
	m.UpdatedAt = now()
	if err := e.store.SaveMigration(m); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx) // #nosec G118 -- cancel is stored in e.running and deferred in the goroutine
	e.mu.Lock()
	e.running[m.ID] = cancel
	e.mu.Unlock()

	go func() {
		defer cancel()
		defer func() {
			e.mu.Lock()
			delete(e.running, m.ID)
			e.mu.Unlock()
		}()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("migration: panic in step execution", "id", m.ID, "panic", r)
				if mg, err := e.store.GetMigration(m.ID); err == nil {
					mg.Status = StatusFailed
					mg.Error = fmt.Sprintf("internal error: %v", r)
					mg.UpdatedAt = now()
					_ = e.store.SaveMigration(mg)
				}
			}
		}()
		e.RunSteps(runCtx, m.ID)
	}()

	return nil
}

// PauseMigration stops execution at the current step.
func (e *Executor) PauseMigration(migrationID string) error {
	e.mu.Lock()
	if cancel, ok := e.running[migrationID]; ok {
		cancel()
		delete(e.running, migrationID)
	}
	e.mu.Unlock()

	m, err := e.store.GetMigration(migrationID)
	if err != nil {
		return err
	}
	m.Status = StatusPaused
	m.UpdatedAt = now()
	return e.store.SaveMigration(m)
}

// RetryStep retries the current failed step.
func (e *Executor) RetryStep(ctx context.Context, migrationID string) error {
	m, err := e.store.GetMigration(migrationID)
	if err != nil {
		return err
	}
	if m.Status != StatusFailed {
		return fmt.Errorf("migration %s is not in failed state (current: %s)", migrationID, m.Status)
	}

	step := &m.Steps[m.CurrentStep-1]
	step.Status = StepPending
	step.Error = ""
	m.Status = StatusRunning
	m.Error = ""
	m.UpdatedAt = now()
	if err := e.store.SaveMigration(m); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx) // #nosec G118 -- cancel is stored in e.running and deferred in the goroutine
	e.mu.Lock()
	e.running[m.ID] = cancel
	e.mu.Unlock()

	go func() {
		defer cancel()
		defer func() {
			e.mu.Lock()
			delete(e.running, m.ID)
			e.mu.Unlock()
		}()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("migration: panic in step execution", "id", m.ID, "panic", r)
				if mg, err := e.store.GetMigration(m.ID); err == nil {
					mg.Status = StatusFailed
					mg.Error = fmt.Sprintf("internal error: %v", r)
					mg.UpdatedAt = now()
					_ = e.store.SaveMigration(mg)
				}
			}
		}()
		e.RunSteps(runCtx, m.ID)
	}()

	return nil
}

// CancelMigration cancels the migration.
func (e *Executor) CancelMigration(migrationID string) error {
	e.mu.Lock()
	if cancel, ok := e.running[migrationID]; ok {
		cancel()
		delete(e.running, migrationID)
	}
	e.mu.Unlock()

	m, err := e.store.GetMigration(migrationID)
	if err != nil {
		return err
	}
	m.Status = StatusCancelled
	m.UpdatedAt = now()
	return e.store.SaveMigration(m)
}

// aiEvaluate asks the AI provider for a brief assessment of a migration step.
// If AI is not configured, it returns a neutral message.
func (e *Executor) aiEvaluate(ctx context.Context, stepTitle, prompt string) (string, error) {
	if e.aiClient == nil || !e.aiClient.IsEnabled() {
		return "AI evaluation skipped — no AI provider configured.", nil
	}
	fullPrompt := fmt.Sprintf(
		"You are evaluating migration step: %s\n\n%s\n\nProvide a brief assessment (2-3 sentences). Is this expected? Should the migration continue?",
		stepTitle, prompt,
	)
	return e.aiClient.Summarize(ctx, fullPrompt)
}

// now returns the current UTC time in RFC3339 format.
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
