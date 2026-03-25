package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
	"github.com/moran/argocd-addons-platform/internal/migration"
	"gopkg.in/yaml.v3"
)

// --- Migration Settings ---

func (s *Server) handleGetMigrationSettings(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	settings, err := s.migrationExecutor.GetStore().GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleSaveMigrationSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) { return }
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	var settings migration.MigrationSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	settings.Configured = true
	if err := s.migrationExecutor.GetStore().SaveSettings(&settings); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleTestMigrationConnection(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	settings, err := s.migrationExecutor.GetStore().GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !settings.Configured {
		writeError(w, http.StatusBadRequest, "migration settings not configured yet")
		return
	}

	result := map[string]interface{}{
		"git":          false,
		"git_error":    "",
		"argocd":       false,
		"argocd_error": "",
	}

	// Test old git connection
	oldGP, err := buildOldGitProvider(settings)
	if err != nil {
		result["git_error"] = err.Error()
	} else {
		if testErr := oldGP.TestConnection(r.Context()); testErr != nil {
			result["git_error"] = testErr.Error()
		} else {
			result["git"] = true
		}
	}

	// Test old ArgoCD connection
	oldAC := argocd.NewClient(
		settings.OldArgocd.ServerURL,
		settings.OldArgocd.Token,
		settings.OldArgocd.Insecure,
	)
	if testErr := oldAC.TestConnection(r.Context()); testErr != nil {
		result["argocd_error"] = testErr.Error()
	} else {
		result["argocd"] = true
	}

	writeJSON(w, http.StatusOK, result)
}

// --- Migration CRUD ---

func (s *Server) handleListMigrations(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	migrations, err := s.migrationExecutor.GetStore().ListMigrations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if migrations == nil {
		migrations = []*migration.Migration{}
	}
	writeJSON(w, http.StatusOK, migrations)
}

func (s *Server) handleGetMigration(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}
	m, err := s.migrationExecutor.GetStore().GetMigration(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleStartMigration(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	var req struct {
		AddonName   string `json:"addon_name"`
		ClusterName string `json:"cluster_name"`
		Mode        string `json:"mode"` // "yolo" or "gates"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AddonName == "" || req.ClusterName == "" {
		writeError(w, http.StatusBadRequest, "addon_name and cluster_name are required")
		return
	}
	if req.Mode == "" {
		req.Mode = "gates"
	}

	// Check for an existing active migration
	hasActive, activeID, err := s.migrationExecutor.GetStore().HasActiveMigration()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check active migrations: "+err.Error())
		return
	}
	if hasActive {
		writeError(w, http.StatusConflict, fmt.Sprintf("an active migration already exists: %s", activeID))
		return
	}

	// Resolve all providers before starting
	if err := s.resolveExecutorProviders(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Use background context — the migration runs independently of the HTTP request lifecycle
	m, err := s.migrationExecutor.StartMigration(context.Background(), req.AddonName, req.ClusterName, req.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// handleStartBatch starts a sequential batch migration for multiple addons.
func (s *Server) handleStartBatch(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	var req struct {
		Addons      []string `json:"addons"`
		ClusterName string   `json:"cluster_name"`
		Mode        string   `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Addons) == 0 || req.ClusterName == "" {
		writeError(w, http.StatusBadRequest, "addons and cluster_name are required")
		return
	}
	if req.Mode == "" {
		req.Mode = "gates"
	}

	hasActive, activeID, err := s.migrationExecutor.GetStore().HasActiveMigration()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if hasActive {
		writeError(w, http.StatusConflict, fmt.Sprintf("an active migration already exists: %s", activeID))
		return
	}

	if err := s.resolveExecutorProviders(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	batch, err := s.migrationExecutor.StartBatch(context.Background(), req.Addons, req.ClusterName, req.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

// handleGetBatch returns the active batch status.
func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id != "" {
		batch, err := s.migrationExecutor.GetBatch(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "batch not found")
			return
		}
		writeJSON(w, http.StatusOK, batch)
		return
	}
	// No ID — return active batch
	batch, _ := s.migrationExecutor.GetActiveBatch()
	if batch == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

// --- Migration actions ---

func (s *Server) handleContinueMigration(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}

	// Re-resolve providers (they may be nil after pod restart)
	if err := s.resolveExecutorProviders(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	if err := s.migrationExecutor.ContinueAfterPR(context.Background(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, _ := s.migrationExecutor.GetStore().GetMigration(id)
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handlePauseMigration(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}
	if err := s.migrationExecutor.PauseMigration(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, _ := s.migrationExecutor.GetStore().GetMigration(id)
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleRetryMigration(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}
	if err := s.resolveExecutorProviders(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err := s.migrationExecutor.RetryStep(context.Background(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, _ := s.migrationExecutor.GetStore().GetMigration(id)
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleCancelMigration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) { return }
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}
	if err := s.migrationExecutor.CancelMigration(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, _ := s.migrationExecutor.GetStore().GetMigration(id)
	writeJSON(w, http.StatusOK, m)
}

// handleRollbackMigration reverses completed PR steps of a failed migration.
func (s *Server) handleRollbackMigration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) { return }
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}
	if err := s.resolveExecutorProviders(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err := s.migrationExecutor.RollbackMigration(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, _ := s.migrationExecutor.GetStore().GetMigration(id)
	writeJSON(w, http.StatusOK, m)
}

// handleDeleteMigration removes a migration session.
func (s *Server) handleDeleteMigration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) { return }
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}
	// Cancel first if running
	_ = s.migrationExecutor.CancelMigration(id)
	if err := s.migrationExecutor.GetStore().DeleteMigration(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleMergePR attempts to merge a PR from a migration step.
func (s *Server) handleMergePR(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")

	var req struct {
		Step int `json:"step"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	m, err := s.migrationExecutor.GetStore().GetMigration(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Step < 1 || req.Step > len(m.Steps) {
		writeError(w, http.StatusBadRequest, "invalid step number")
		return
	}

	step := &m.Steps[req.Step-1]
	if step.PRNumber == 0 {
		writeError(w, http.StatusBadRequest, "this step has no PR")
		return
	}

	if err := s.resolveExecutorProviders(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Determine which provider to use
	var gp gitprovider.GitProvider
	if step.PRRepo == "old" {
		settings, _ := s.migrationExecutor.GetStore().GetSettings()
		gp, _ = buildOldGitProvider(settings)
	} else {
		gp, _ = s.connSvc.GetActiveGitProvider()
	}

	if gp == nil {
		writeError(w, http.StatusServiceUnavailable, "git provider not available")
		return
	}

	if mergeErr := gp.MergePullRequest(r.Context(), step.PRNumber); mergeErr != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to merge PR #%d: %s", step.PRNumber, mergeErr.Error()))
		return
	}

	step.PRStatus = "merged"
	step.Message = fmt.Sprintf("PR #%d merged successfully", step.PRNumber)

	// If step was waiting, mark it completed and advance
	if step.Status == "waiting" {
		step.Status = "completed"
		step.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		m.CurrentStep++
		// Resume the migration if it was waiting
		if m.Status == "waiting" {
			m.Status = "running"
			m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			_ = s.migrationExecutor.GetStore().SaveMigration(m)
			// Re-resolve providers and continue execution
			if err := s.resolveExecutorProviders(); err == nil {
				go s.migrationExecutor.ContinueAfterPR(context.Background(), id)
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "merged", "message": "PR merged, migration continuing..."})
			return
		}
	}

	_ = s.migrationExecutor.GetStore().SaveMigration(m)

	writeJSON(w, http.StatusOK, map[string]string{"status": "merged", "message": fmt.Sprintf("PR #%d merged", step.PRNumber)})
}

// --- Migration Chat (Troubleshooting) ---

func (s *Server) handleMigrationChat(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	m, mErr := s.migrationExecutor.GetStore().GetMigration(id)
	if mErr != nil || m == nil {
		writeError(w, http.StatusNotFound, "migration not found")
		return
	}
	if err := s.resolveExecutorProviders(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	agent := s.migrationExecutor.CreateTroubleshootAgent(m)

	response, err := agent.Chat(r.Context(), req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"response": response})
}

// --- Migration Explain (AI explains current state) ---

func (s *Server) handleMigrationExplain(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "migration id is required")
		return
	}

	m, err := s.migrationExecutor.GetStore().GetMigration(id)
	if err != nil || m == nil {
		writeError(w, http.StatusNotFound, "migration not found")
		return
	}

	// Build a concise state summary for the AI
	var stepsSummary string
	for _, s := range m.Steps {
		line := fmt.Sprintf("Step %d (%s): %s", s.Number, s.Title, string(s.Status))
		if s.Message != "" {
			line += " — " + s.Message
		}
		if s.Error != "" {
			line += " [error: " + s.Error + "]"
		}
		stepsSummary += line + "\n"
	}

	prompt := fmt.Sprintf(
		"Migration for addon %q on cluster %q. Status: %s. Current step: %d.\n\nStep details:\n%s\nExplain in 2-3 sentences what's happening and what the user should do next.",
		m.AddonName, m.ClusterName, string(m.Status), m.CurrentStep, stepsSummary)

	explanation, aiErr := s.migrationExecutor.AIExplain(r.Context(), prompt)
	if aiErr != nil {
		// Fallback: provide a non-AI explanation based on state
		explanation = buildStateExplanation(m)
	}

	writeJSON(w, http.StatusOK, map[string]string{"explanation": explanation})
}

// buildStateExplanation provides a deterministic explanation when AI is unavailable.
func buildStateExplanation(m *migration.Migration) string {
	switch m.Status {
	case migration.StatusWaiting:
		for _, s := range m.Steps {
			if s.Status == migration.StepWaiting && s.PRURL != "" {
				return fmt.Sprintf("Migration is waiting for you to review and merge the PR from step %d: %s. After merging, click Continue to proceed.", s.Number, s.PRURL)
			}
		}
		return "Migration is waiting for user action. Check the current step for details."
	case migration.StatusGated:
		return fmt.Sprintf("Step %d completed. Migration is paused in gates mode — click Approve to continue to the next step.", m.CurrentStep)
	case migration.StatusFailed:
		return fmt.Sprintf("Step %d failed: %s. You can click Retry to try again or check the logs for details.", m.CurrentStep, m.Error)
	case migration.StatusPaused:
		return fmt.Sprintf("Migration was paused at step %d. Click Resume to continue.", m.CurrentStep)
	case migration.StatusCompleted:
		return "Migration completed successfully."
	case migration.StatusRunning:
		return fmt.Sprintf("Step %d is currently running.", m.CurrentStep)
	default:
		return fmt.Sprintf("Migration status: %s", string(m.Status))
	}
}

// --- Azure DevOps Discovery ---

func (s *Server) handleAzureListProjects(w http.ResponseWriter, r *http.Request) {
	org := r.URL.Query().Get("org")
	pat := r.URL.Query().Get("pat")
	if org == "" || pat == "" {
		writeError(w, http.StatusBadRequest, "org and pat query parameters are required")
		return
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/_apis/projects?api-version=7.1", org)
	req, err := http.NewRequestWithContext(r.Context(), "GET", url, nil) // #nosec G704 -- org is user-provided config for their own Azure DevOps
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.SetBasicAuth("", pat)

	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- URL targets user-configured Azure DevOps instance
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to connect to Azure DevOps: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		writeError(w, resp.StatusCode, fmt.Sprintf("Azure DevOps returned %d", resp.StatusCode))
		return
	}

	var result struct {
		Value []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse Azure DevOps response")
		return
	}

	names := make([]string, 0, len(result.Value))
	for _, p := range result.Value {
		names = append(names, p.Name)
	}
	writeJSON(w, http.StatusOK, names)
}

func (s *Server) handleAzureListRepos(w http.ResponseWriter, r *http.Request) {
	org := r.URL.Query().Get("org")
	project := r.URL.Query().Get("project")
	pat := r.URL.Query().Get("pat")
	if org == "" || project == "" || pat == "" {
		writeError(w, http.StatusBadRequest, "org, project, and pat query parameters are required")
		return
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/repositories?api-version=7.1", org, project)
	req, err := http.NewRequestWithContext(r.Context(), "GET", url, nil) // #nosec G704 -- org/project are user-provided config
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.SetBasicAuth("", pat)

	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- URL targets user-configured Azure DevOps instance
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to connect to Azure DevOps: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		writeError(w, resp.StatusCode, fmt.Sprintf("Azure DevOps returned %d", resp.StatusCode))
		return
	}

	var result struct {
		Value []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse Azure DevOps response")
		return
	}

	names := make([]string, 0, len(result.Value))
	for _, r := range result.Value {
		names = append(names, r.Name)
	}
	writeJSON(w, http.StatusOK, names)
}

// --- OLD Repo Discovery (addons + clusters) ---

// handleOldRepoAddons fetches the addon list from the OLD repo.
// Tries V2 path (configuration/addons-catalog.yaml) then V1 (values/addons-list.yaml).
func (s *Server) handleOldRepoAddons(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	settings, err := s.migrationExecutor.GetStore().GetSettings()
	if err != nil || !settings.Configured {
		writeError(w, http.StatusBadRequest, "migration settings not configured")
		return
	}
	oldGP, err := buildOldGitProvider(settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var addons []string

	// Try V2 structure first
	data, err := oldGP.GetFileContent(r.Context(), "configuration/addons-catalog.yaml", "main")
	if err == nil {
		addons = parseAddonNames(data)
		slog.Info("old repo addons: V2 path", "count", len(addons))
	} else {
		slog.Info("old repo addons: V2 path not found", "error", err.Error())
	}

	// Fall back to V1 structure
	if len(addons) == 0 {
		data, err = oldGP.GetFileContent(r.Context(), "values/addons-list.yaml", "main")
		if err == nil {
			addons = parseAddonNamesV1(data)
			slog.Info("old repo addons: V1 path", "count", len(addons))
		} else {
			slog.Info("old repo addons: V1 path not found", "error", err.Error())
		}
	}

	if len(addons) == 0 {
		writeError(w, http.StatusNotFound, "no addons found in old repo (tried configuration/addons-catalog.yaml and values/addons-list.yaml)")
		return
	}
	writeJSON(w, http.StatusOK, addons)
}

// handleOldRepoClusters fetches the cluster list from the OLD repo.
// Tries V2 path (configuration/cluster-addons.yaml) then V1 (values/clusters.yaml).
func (s *Server) handleOldRepoClusters(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	settings, err := s.migrationExecutor.GetStore().GetSettings()
	if err != nil || !settings.Configured {
		writeError(w, http.StatusBadRequest, "migration settings not configured")
		return
	}
	oldGP, err := buildOldGitProvider(settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var clusters []string

	// Try V2 structure first
	data, err := oldGP.GetFileContent(r.Context(), "configuration/cluster-addons.yaml", "main")
	if err == nil {
		clusters = parseClusterNames(data)
	}

	// Fall back to V1 structure
	if len(clusters) == 0 {
		data, err = oldGP.GetFileContent(r.Context(), "values/clusters.yaml", "main")
		if err == nil {
			clusters = parseClusterNames(data)
		}
	}

	if len(clusters) == 0 {
		writeError(w, http.StatusNotFound, "no clusters found in old repo")
		return
	}
	writeJSON(w, http.StatusOK, clusters)
}

// clusterAddonInfo describes an addon's migration status for a specific cluster.
type clusterAddonInfo struct {
	Name            string `json:"name"`
	AlreadyMigrated bool   `json:"already_migrated"` // true if enabled in NEW repo for this cluster
}

// handleOldRepoClusterAddons fetches enabled addons for a specific cluster,
// cross-referencing with the NEW repo to flag already-migrated ones.
func (s *Server) handleOldRepoClusterAddons(w http.ResponseWriter, r *http.Request) {
	if s.migrationExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "migration service not configured")
		return
	}
	clusterName := r.URL.Query().Get("cluster")
	if clusterName == "" {
		writeError(w, http.StatusBadRequest, "cluster query parameter is required")
		return
	}
	settings, err := s.migrationExecutor.GetStore().GetSettings()
	if err != nil || !settings.Configured {
		writeError(w, http.StatusBadRequest, "migration settings not configured")
		return
	}
	oldGP, err := buildOldGitProvider(settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var enabledAddons []string

	// Try V2 then V1
	data, err := oldGP.GetFileContent(r.Context(), "configuration/cluster-addons.yaml", "main")
	if err != nil {
		data, err = oldGP.GetFileContent(r.Context(), "values/clusters.yaml", "main")
	}
	if err == nil {
		enabledAddons = parseEnabledAddonsForCluster(data, clusterName)
	}

	if enabledAddons == nil {
		enabledAddons = []string{}
	}

	// Cross-reference with NEW repo to find already-migrated addons
	newEnabledAddons := map[string]bool{}
	if err := s.resolveExecutorProviders(); err == nil {
		newGP := s.migrationExecutor.GetNewGP()
		if newGP != nil {
			newData, newErr := newGP.GetFileContent(r.Context(), "configuration/cluster-addons.yaml", "main")
			if newErr == nil {
				for _, a := range parseEnabledAddonsForCluster(newData, clusterName) {
					newEnabledAddons[a] = true
				}
			}
		}
	}

	result := make([]clusterAddonInfo, 0, len(enabledAddons))
	for _, addon := range enabledAddons {
		result = append(result, clusterAddonInfo{
			Name:            addon,
			AlreadyMigrated: newEnabledAddons[addon],
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// parseAddonNames extracts appName values from V2 addons-catalog.yaml
func parseAddonNames(data []byte) []string {
	var names []string
	var parsed struct {
		ApplicationSets []struct {
			AppName string `yaml:"appName"`
		} `yaml:"applicationsets"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	for _, a := range parsed.ApplicationSets {
		if a.AppName != "" {
			names = append(names, a.AppName)
		}
	}
	return names
}

// parseAddonNamesV1 extracts addon names from V1 addons-list.yaml
func parseAddonNamesV1(data []byte) []string {
	var parsed struct {
		ApplicationSets []struct {
			AppName string `yaml:"appName"`
		} `yaml:"applicationsets"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var names []string
	for _, a := range parsed.ApplicationSets {
		if a.AppName != "" && !seen[a.AppName] {
			seen[a.AppName] = true
			names = append(names, a.AppName)
		}
	}
	return names
}

// parseClusterNames extracts cluster names from cluster-addons.yaml or clusters.yaml
func parseClusterNames(data []byte) []string {
	var parsed struct {
		Clusters []struct {
			Name string `yaml:"name"`
		} `yaml:"clusters"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	var names []string
	for _, c := range parsed.Clusters {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	return names
}

// parseEnabledAddonsForCluster extracts enabled addon labels for a specific cluster
func parseEnabledAddonsForCluster(data []byte, clusterName string) []string {
	var parsed struct {
		Clusters []struct {
			Name   string            `yaml:"name"`
			Labels map[string]string `yaml:"labels"`
		} `yaml:"clusters"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	for _, c := range parsed.Clusters {
		if c.Name == clusterName {
			var addons []string
			for k, v := range c.Labels {
				if v == "enabled" && !strings.HasSuffix(k, "-version") {
					addons = append(addons, k)
				}
			}
			sort.Strings(addons)
			return addons
		}
	}
	return nil
}

// --- Helpers ---

// resolveExecutorProviders sets up both OLD and NEW providers on the executor.
// Called before starting, continuing, or retrying a migration.
func (s *Server) resolveExecutorProviders() error {
	settings, err := s.migrationExecutor.GetStore().GetSettings()
	if err != nil {
		return fmt.Errorf("reading migration settings: %w", err)
	}
	if !settings.Configured {
		return fmt.Errorf("migration settings not configured")
	}

	oldGP, err := buildOldGitProvider(settings)
	if err != nil {
		return fmt.Errorf("building old git provider: %w", err)
	}
	oldAC := argocd.NewClient(settings.OldArgocd.ServerURL, settings.OldArgocd.Token, settings.OldArgocd.Insecure)
	s.migrationExecutor.SetOldProviders(oldGP, oldAC)

	newGP, err := s.connSvc.GetActiveGitProvider()
	if err != nil {
		return fmt.Errorf("no active git connection: %w", err)
	}
	newAC, err := s.connSvc.GetActiveArgocdClient()
	if err != nil {
		return fmt.Errorf("no active ArgoCD connection: %w", err)
	}
	s.migrationExecutor.SetNewProviders(newGP, newAC)
	return nil
}

// buildOldGitProvider creates a GitProvider from the saved migration settings.
func buildOldGitProvider(settings *migration.MigrationSettings) (gitprovider.GitProvider, error) {
	switch settings.OldGit.Provider {
	case "github":
		return gitprovider.NewGitHubProvider(
			settings.OldGit.Owner,
			settings.OldGit.Repo,
			settings.OldGit.Token,
		), nil
	case "azuredevops":
		return gitprovider.NewAzureDevOpsProvider(
			settings.OldGit.Organization,
			settings.OldGit.Project,
			settings.OldGit.Repository,
			settings.OldGit.PAT,
		), nil
	default:
		return nil, fmt.Errorf("unsupported git provider: %q", settings.OldGit.Provider)
	}
}
