package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
	"github.com/moran/argocd-addons-platform/internal/migration"
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

	result := map[string]bool{"git": false, "argocd": false}

	// Test old git connection
	oldGP, err := buildOldGitProvider(settings)
	if err == nil {
		if testErr := oldGP.TestConnection(r.Context()); testErr == nil {
			result["git"] = true
		}
	}

	// Test old ArgoCD connection
	oldAC := argocd.NewClient(
		settings.OldArgocd.ServerURL,
		settings.OldArgocd.Token,
		settings.OldArgocd.Insecure,
	)
	if testErr := oldAC.TestConnection(r.Context()); testErr == nil {
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AddonName == "" || req.ClusterName == "" {
		writeError(w, http.StatusBadRequest, "addon_name and cluster_name are required")
		return
	}

	// Resolve old providers from saved settings before starting
	settings, err := s.migrationExecutor.GetStore().GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read migration settings: "+err.Error())
		return
	}
	if !settings.Configured {
		writeError(w, http.StatusBadRequest, "migration settings must be configured before starting a migration")
		return
	}
	oldGP, err := buildOldGitProvider(settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to build old git provider: "+err.Error())
		return
	}
	oldAC := argocd.NewClient(
		settings.OldArgocd.ServerURL,
		settings.OldArgocd.Token,
		settings.OldArgocd.Insecure,
	)
	s.migrationExecutor.SetOldProviders(oldGP, oldAC)

	m, err := s.migrationExecutor.StartMigration(r.Context(), req.AddonName, req.ClusterName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
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
	if err := s.migrationExecutor.ContinueAfterPR(r.Context(), id); err != nil {
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
	if err := s.migrationExecutor.RetryStep(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, _ := s.migrationExecutor.GetStore().GetMigration(id)
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleCancelMigration(w http.ResponseWriter, r *http.Request) {
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

// --- Helpers ---

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
