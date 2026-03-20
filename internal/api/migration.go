package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

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

// handleDeleteMigration removes a migration session.
func (s *Server) handleDeleteMigration(w http.ResponseWriter, r *http.Request) {
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

	// Delete the source branch after merge
	if step.PRURL != "" {
		// Extract branch name from the step's message or use a pattern
		// The branch was created as aap/migration/... — try to delete it
		_ = gp.DeleteBranch(r.Context(), fmt.Sprintf("aap/migration/%s/%s", m.AddonName, m.ClusterName))
	}

	_ = s.migrationExecutor.GetStore().SaveMigration(m)

	writeJSON(w, http.StatusOK, map[string]string{"status": "merged"})
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
	}

	// Fall back to V1 structure
	if len(addons) == 0 {
		data, err = oldGP.GetFileContent(r.Context(), "values/addons-list.yaml", "main")
		if err == nil {
			addons = parseAddonNamesV1(data)
		}
	}

	if len(addons) == 0 {
		writeError(w, http.StatusNotFound, "no addons found in old repo")
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

// handleOldRepoClusterAddons fetches enabled addons for a specific cluster.
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
	writeJSON(w, http.StatusOK, enabledAddons)
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
