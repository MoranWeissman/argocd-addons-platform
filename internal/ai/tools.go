package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/config"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
	"github.com/moran/argocd-addons-platform/internal/helm"
)

// ToolDefinition describes a tool for the Ollama API.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function for the LLM.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolExecutor can execute tools against the platform's data sources.
type ToolExecutor struct {
	parser  *config.Parser
	fetcher *helm.Fetcher
	gp      gitprovider.GitProvider
	ac      *argocd.Client
}

// NewToolExecutor creates a new ToolExecutor with the given providers.
func NewToolExecutor(gp gitprovider.GitProvider, ac *argocd.Client) *ToolExecutor {
	return &ToolExecutor{
		parser:  config.NewParser(),
		fetcher: helm.NewFetcher(),
		gp:      gp,
		ac:      ac,
	}
}

// GetToolDefinitions returns all available tool definitions for Ollama.
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_clusters",
				Description: "List all Kubernetes clusters managed by this platform with their connection status",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_cluster_addons",
				Description: "Get the list of addons enabled on a specific cluster with their health status",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"cluster_name":{"type":"string","description":"Name of the cluster"}},"required":["cluster_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_addons",
				Description: "List all available addons in the catalog with their versions",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_addon_values",
				Description: "Get the global default values configuration for a specific addon",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"addon_name":{"type":"string","description":"Name of the addon"}},"required":["addon_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_cluster_values",
				Description: "Get the per-cluster configuration overrides for a specific cluster",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"cluster_name":{"type":"string","description":"Name of the cluster"}},"required":["cluster_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_argocd_app_health",
				Description: "Get the health and sync status of ArgoCD applications, optionally filtered by cluster",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"cluster_name":{"type":"string","description":"Optional cluster name to filter by"}}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "compare_chart_versions",
				Description: "Compare values.yaml between two versions of a Helm chart to find changes",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"addon_name":{"type":"string","description":"Name of the addon"},"current_version":{"type":"string","description":"Current version"},"target_version":{"type":"string","description":"Target version to compare"}},"required":["addon_name","current_version","target_version"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_chart_versions",
				Description: "List available versions for a Helm chart",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"addon_name":{"type":"string","description":"Name of the addon"}},"required":["addon_name"]}`),
			},
		},
	}
}

// ExecuteTool runs a tool and returns the result as a string.
func (e *ToolExecutor) ExecuteTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	var params map[string]string
	json.Unmarshal(args, &params) //nolint:errcheck // best-effort parse

	switch name {
	case "list_clusters":
		return e.listClusters(ctx)
	case "get_cluster_addons":
		return e.getClusterAddons(ctx, params["cluster_name"])
	case "list_addons":
		return e.listAddons(ctx)
	case "get_addon_values":
		return e.getAddonValues(ctx, params["addon_name"])
	case "get_cluster_values":
		return e.getClusterValues(ctx, params["cluster_name"])
	case "get_argocd_app_health":
		return e.getArgocdHealth(ctx, params["cluster_name"])
	case "compare_chart_versions":
		return e.compareVersions(ctx, params["addon_name"], params["current_version"], params["target_version"])
	case "list_chart_versions":
		return e.listChartVersions(ctx, params["addon_name"])
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (e *ToolExecutor) listClusters(ctx context.Context) (string, error) {
	data, err := e.gp.GetFileContent(ctx, "configuration/cluster-addons.yaml", "main")
	if err != nil {
		return "", err
	}
	clusters, err := e.parser.ParseClusterAddons(data)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, c := range clusters {
		enabledCount := 0
		for _, v := range c.Labels {
			if v == "enabled" {
				enabledCount++
			}
		}
		fmt.Fprintf(&sb, "- %s: %d addons enabled\n", c.Name, enabledCount)
	}
	return sb.String(), nil
}

func (e *ToolExecutor) getClusterAddons(ctx context.Context, clusterName string) (string, error) {
	clusterData, err := e.gp.GetFileContent(ctx, "configuration/cluster-addons.yaml", "main")
	if err != nil {
		return "", err
	}
	catalogData, err := e.gp.GetFileContent(ctx, "configuration/addons-catalog.yaml", "main")
	if err != nil {
		return "", err
	}

	repoCfg, err := e.parser.ParseAll(clusterData, catalogData)
	if err != nil {
		return "", err
	}

	for _, cluster := range repoCfg.Clusters {
		if cluster.Name == clusterName {
			addons := e.parser.GetEnabledAddons(cluster, repoCfg.Addons)
			var sb strings.Builder
			for _, a := range addons {
				status := "enabled"
				if !a.Enabled {
					status = "disabled"
				}
				fmt.Fprintf(&sb, "- %s: version=%s, %s\n", a.AddonName, a.CurrentVersion, status)
			}
			if sb.Len() == 0 {
				return "No addons configured on this cluster.", nil
			}
			return sb.String(), nil
		}
	}
	return "Cluster not found.", nil
}

func (e *ToolExecutor) listAddons(ctx context.Context) (string, error) {
	data, err := e.gp.GetFileContent(ctx, "configuration/addons-catalog.yaml", "main")
	if err != nil {
		return "", err
	}
	addons, err := e.parser.ParseAddonsCatalog(data)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, a := range addons {
		fmt.Fprintf(&sb, "- %s: chart=%s, version=%s, repo=%s\n", a.AppName, a.Chart, a.Version, a.RepoURL)
	}
	return sb.String(), nil
}

func (e *ToolExecutor) getAddonValues(ctx context.Context, addonName string) (string, error) {
	data, err := e.gp.GetFileContent(ctx, fmt.Sprintf("configuration/addons-global-values/%s.yaml", addonName), "main")
	if err != nil {
		return fmt.Sprintf("No global values file found for %s", addonName), nil
	}
	// Truncate to 2000 chars to keep context manageable
	s := string(data)
	if len(s) > 2000 {
		s = s[:2000] + "\n... (truncated)"
	}
	return s, nil
}

func (e *ToolExecutor) getClusterValues(ctx context.Context, clusterName string) (string, error) {
	data, err := e.gp.GetFileContent(ctx, fmt.Sprintf("configuration/addons-clusters-values/%s.yaml", clusterName), "main")
	if err != nil {
		return fmt.Sprintf("No values file found for cluster %s", clusterName), nil
	}
	s := string(data)
	if len(s) > 2000 {
		s = s[:2000] + "\n... (truncated)"
	}
	return s, nil
}

func (e *ToolExecutor) getArgocdHealth(ctx context.Context, clusterName string) (string, error) {
	apps, err := e.ac.ListApplications(ctx)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, app := range apps {
		if clusterName != "" && app.DestinationName != clusterName {
			continue
		}
		fmt.Fprintf(&sb, "- %s: health=%s, sync=%s, dest=%s\n", app.Name, app.HealthStatus, app.SyncStatus, app.DestinationName)
	}
	if sb.Len() == 0 {
		return "No applications found.", nil
	}
	return sb.String(), nil
}

func (e *ToolExecutor) compareVersions(ctx context.Context, addonName, currentVersion, targetVersion string) (string, error) {
	catalogData, err := e.gp.GetFileContent(ctx, "configuration/addons-catalog.yaml", "main")
	if err != nil {
		return "", err
	}
	addons, err := e.parser.ParseAddonsCatalog(catalogData)
	if err != nil {
		return "", err
	}

	var repoURL, chart string
	for _, a := range addons {
		if a.AppName == addonName {
			repoURL = a.RepoURL
			chart = a.Chart
			break
		}
	}
	if repoURL == "" {
		return "Addon not found in catalog.", nil
	}

	oldValues, err := e.fetcher.FetchValues(ctx, repoURL, chart, currentVersion)
	if err != nil {
		return fmt.Sprintf("Could not fetch version %s: %v", currentVersion, err), nil
	}
	newValues, err := e.fetcher.FetchValues(ctx, repoURL, chart, targetVersion)
	if err != nil {
		return fmt.Sprintf("Could not fetch version %s: %v", targetVersion, err), nil
	}

	added, removed, changed, err := helm.DiffValues(oldValues, newValues)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Comparing %s v%s -> v%s:\n", addonName, currentVersion, targetVersion)
	fmt.Fprintf(&sb, "%d added, %d removed, %d changed\n\n", len(added), len(removed), len(changed))

	if len(changed) > 0 {
		sb.WriteString("Key changes:\n")
		for i, c := range changed {
			if i >= 10 {
				fmt.Fprintf(&sb, "... and %d more\n", len(changed)-10)
				break
			}
			fmt.Fprintf(&sb, "  %s: %s -> %s\n", c.Path, c.OldValue, c.NewValue)
		}
	}
	return sb.String(), nil
}

func (e *ToolExecutor) listChartVersions(ctx context.Context, addonName string) (string, error) {
	catalogData, err := e.gp.GetFileContent(ctx, "configuration/addons-catalog.yaml", "main")
	if err != nil {
		return "", err
	}
	addons, err := e.parser.ParseAddonsCatalog(catalogData)
	if err != nil {
		return "", err
	}

	var repoURL, chart string
	for _, a := range addons {
		if a.AppName == addonName {
			repoURL = a.RepoURL
			chart = a.Chart
			break
		}
	}
	if repoURL == "" {
		return "Addon not found.", nil
	}

	versions, err := e.fetcher.ListVersions(ctx, repoURL, chart)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	limit := 15
	if len(versions) < limit {
		limit = len(versions)
	}
	for i := 0; i < limit; i++ {
		fmt.Fprintf(&sb, "- %s\n", versions[i].Version)
	}
	if len(versions) > limit {
		fmt.Fprintf(&sb, "... and %d more versions\n", len(versions)-limit)
	}
	return sb.String(), nil
}
