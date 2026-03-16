package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/config"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
	"github.com/moran/argocd-addons-platform/internal/helm"
	"gopkg.in/yaml.v3"
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
				Name:        "find_addon_deployments",
				Description: "Find which clusters have a specific addon installed and its status on each. Use this when asked 'where is addon X deployed' or 'which clusters have addon X'",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"addon_name":{"type":"string","description":"Name of the addon to search for, e.g. datadog, istio-base, keda"}},"required":["addon_name"]}`),
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
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_addon_on_cluster",
				Description: "Get detailed information about a specific addon on a specific cluster, including version, health status, and whether it's enabled. Use this when asked about a specific addon on a specific cluster.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"addon_name":{"type":"string","description":"Name of the addon (e.g. datadog, istio-base)"},"cluster_name":{"type":"string","description":"Name of the cluster"}},"required":["addon_name","cluster_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_unhealthy_addons",
				Description: "Find all addons that are not healthy across all clusters. Returns addons with Degraded, Progressing, or Unknown health status.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_cluster_status",
				Description: "Get the connection status of all clusters (Connected, Failed, Unknown). Use this to find disconnected or problematic clusters.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_recent_syncs",
				Description: "Get recent sync/deployment activity across all ArgoCD applications. Shows what was deployed recently.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"limit":{"type":"string","description":"Maximum number of results (default 10)"}}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_platform_info",
				Description: "Get platform information including ArgoCD version, total clusters, total applications, and overall health summary.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_addon_config_on_cluster",
				Description: "Get the specific configuration values for an addon on a particular cluster, including both global defaults and cluster-specific overrides.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"addon_name":{"type":"string","description":"Name of the addon"},"cluster_name":{"type":"string","description":"Name of the cluster"}},"required":["addon_name","cluster_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "search_addons",
				Description: "Search for addons by name (partial match). Use this when the user mentions an addon name that might not be exact.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search term to match against addon names"}},"required":["query"]}`),
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
		// Accept both cluster_name and name as parameter (LLMs sometimes use wrong key)
		cn := params["cluster_name"]
		if cn == "" { cn = params["name"] }
		if cn == "" { cn = params["cluster"] }
		return e.getClusterAddons(ctx, cn)
	case "list_addons":
		return e.listAddons(ctx)
	case "get_addon_values":
		return e.getAddonValues(ctx, params["addon_name"])
	case "get_cluster_values":
		return e.getClusterValues(ctx, params["cluster_name"])
	case "find_addon_deployments":
		an := params["addon_name"]
		if an == "" { an = params["name"] }
		if an == "" { an = params["addon"] }
		return e.findAddonDeployments(ctx, an)
	case "get_argocd_app_health":
		return e.getArgocdHealth(ctx, params["cluster_name"])
	case "compare_chart_versions":
		return e.compareVersions(ctx, params["addon_name"], params["current_version"], params["target_version"])
	case "list_chart_versions":
		return e.listChartVersions(ctx, params["addon_name"])
	case "get_addon_on_cluster":
		return e.getAddonOnCluster(ctx, params["addon_name"], params["cluster_name"])
	case "get_unhealthy_addons":
		return e.getUnhealthyAddons(ctx)
	case "get_cluster_status":
		return e.getClusterStatus(ctx)
	case "get_recent_syncs":
		limit := 10
		if l, ok := params["limit"]; ok && l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		return e.getRecentSyncs(ctx, limit)
	case "get_platform_info":
		return e.getPlatformInfo(ctx)
	case "get_addon_config_on_cluster":
		return e.getAddonConfigOnCluster(ctx, params["addon_name"], params["cluster_name"])
	case "search_addons":
		return e.searchAddons(ctx, params["query"])
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

func (e *ToolExecutor) findAddonDeployments(ctx context.Context, addonName string) (string, error) {
	if addonName == "" {
		return "Please specify an addon name.", nil
	}

	clusterData, err := e.gp.GetFileContent(ctx, "configuration/cluster-addons.yaml", "main")
	if err != nil {
		return "", err
	}
	clusters, err := e.parser.ParseClusterAddons(clusterData)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	found := 0
	for _, c := range clusters {
		labelVal, has := c.Labels[addonName]
		if has {
			found++
			versionKey := addonName + "-version"
			version := ""
			if v, ok := c.Labels[versionKey]; ok {
				version = " (version override: " + v + ")"
			}
			fmt.Fprintf(&sb, "- %s: %s%s\n", c.Name, labelVal, version)
		}
	}

	if found == 0 {
		return fmt.Sprintf("Addon '%s' is not configured on any cluster.", addonName), nil
	}
	return fmt.Sprintf("Addon '%s' found on %d clusters:\n%s", addonName, found, sb.String()), nil
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

func (e *ToolExecutor) getAddonOnCluster(ctx context.Context, addonName, clusterName string) (string, error) {
	if addonName == "" {
		return "Please specify an addon name.", nil
	}
	if clusterName == "" {
		return "Please specify a cluster name.", nil
	}

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

	// Find the cluster
	var found bool
	for _, cluster := range repoCfg.Clusters {
		if cluster.Name != clusterName {
			continue
		}
		found = true

		labelVal, hasLabel := cluster.Labels[addonName]
		if !hasLabel {
			return fmt.Sprintf("Addon '%s' is not configured on cluster '%s'.", addonName, clusterName), nil
		}

		enabled := strings.EqualFold(labelVal, "enabled")

		// Get catalog version
		catalogVersion := "unknown"
		for _, a := range repoCfg.Addons {
			if a.AppName == addonName {
				catalogVersion = a.Version
				break
			}
		}

		// Check for version override
		versionKey := addonName + "-version"
		effectiveVersion := catalogVersion
		overrideNote := ""
		if v, ok := cluster.Labels[versionKey]; ok && v != "" {
			effectiveVersion = v
			overrideNote = fmt.Sprintf(" (override from catalog %s)", catalogVersion)
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Addon: %s\n", addonName)
		fmt.Fprintf(&sb, "Cluster: %s\n", clusterName)
		fmt.Fprintf(&sb, "Enabled: %v\n", enabled)
		fmt.Fprintf(&sb, "Version: %s%s\n", effectiveVersion, overrideNote)

		// Try to get ArgoCD health
		if enabled {
			appName := addonName + "-" + clusterName
			app, appErr := e.ac.GetApplication(ctx, appName)
			if appErr == nil && app != nil {
				fmt.Fprintf(&sb, "ArgoCD Health: %s\n", app.HealthStatus)
				fmt.Fprintf(&sb, "ArgoCD Sync: %s\n", app.SyncStatus)
				if app.ReconciledAt != "" {
					fmt.Fprintf(&sb, "Last Reconciled: %s\n", app.ReconciledAt)
				}
			} else {
				fmt.Fprintf(&sb, "ArgoCD Health: unable to fetch (app name tried: %s)\n", appName)
			}
		}

		return sb.String(), nil
	}

	if !found {
		return fmt.Sprintf("Cluster '%s' not found.", clusterName), nil
	}
	return "", nil
}

func (e *ToolExecutor) getUnhealthyAddons(ctx context.Context) (string, error) {
	apps, err := e.ac.ListApplications(ctx)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	count := 0
	for _, app := range apps {
		if app.HealthStatus != "Healthy" && app.HealthStatus != "" {
			count++
			fmt.Fprintf(&sb, "- %s: health=%s, sync=%s, cluster=%s\n",
				app.Name, app.HealthStatus, app.SyncStatus, app.DestinationName)
		}
	}

	if count == 0 {
		return "All addons are healthy.", nil
	}
	return fmt.Sprintf("Found %d unhealthy addons:\n%s", count, sb.String()), nil
}

func (e *ToolExecutor) getClusterStatus(ctx context.Context) (string, error) {
	clusters, err := e.ac.ListClusters(ctx)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, c := range clusters {
		state := c.ConnectionState
		if state == "" {
			state = "Unknown"
		}
		fmt.Fprintf(&sb, "- %s: %s (server: %s)\n", c.Name, state, c.Server)
	}

	if sb.Len() == 0 {
		return "No clusters found.", nil
	}
	return fmt.Sprintf("Cluster connection status (%d clusters):\n%s", len(clusters), sb.String()), nil
}

func (e *ToolExecutor) getRecentSyncs(ctx context.Context, limit int) (string, error) {
	apps, err := e.ac.ListApplications(ctx)
	if err != nil {
		return "", err
	}

	// Sort apps by ReconciledAt descending (most recent first)
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].ReconciledAt > apps[j].ReconciledAt
	})

	if limit <= 0 {
		limit = 10
	}
	if limit > len(apps) {
		limit = len(apps)
	}

	var sb strings.Builder
	for i := 0; i < limit; i++ {
		app := apps[i]
		syncTime := app.ReconciledAt
		if syncTime == "" {
			syncTime = "N/A"
		}
		fmt.Fprintf(&sb, "- %s: health=%s, sync=%s, cluster=%s, reconciledAt=%s\n",
			app.Name, app.HealthStatus, app.SyncStatus, app.DestinationName, syncTime)
	}

	if sb.Len() == 0 {
		return "No applications found.", nil
	}
	return fmt.Sprintf("Recent sync activity (showing %d of %d apps):\n%s", limit, len(apps), sb.String()), nil
}

func (e *ToolExecutor) getPlatformInfo(ctx context.Context) (string, error) {
	var sb strings.Builder

	// ArgoCD version
	version, err := e.ac.GetVersion(ctx)
	if err != nil {
		fmt.Fprintf(&sb, "ArgoCD Version: unable to fetch (%v)\n", err)
	} else {
		v := version["Version"]
		if v == "" {
			v = "unknown"
		}
		fmt.Fprintf(&sb, "ArgoCD Version: %s\n", v)
	}

	// Clusters
	clusters, err := e.ac.ListClusters(ctx)
	if err != nil {
		fmt.Fprintf(&sb, "Clusters: unable to fetch (%v)\n", err)
	} else {
		connected := 0
		for _, c := range clusters {
			if strings.EqualFold(c.ConnectionState, "Successful") {
				connected++
			}
		}
		fmt.Fprintf(&sb, "Total Clusters: %d (%d connected)\n", len(clusters), connected)
	}

	// Applications
	apps, err := e.ac.ListApplications(ctx)
	if err != nil {
		fmt.Fprintf(&sb, "Applications: unable to fetch (%v)\n", err)
	} else {
		healthy, degraded, progressing, unknown := 0, 0, 0, 0
		synced, outOfSync := 0, 0
		for _, app := range apps {
			switch app.HealthStatus {
			case "Healthy":
				healthy++
			case "Degraded":
				degraded++
			case "Progressing":
				progressing++
			default:
				unknown++
			}
			switch app.SyncStatus {
			case "Synced":
				synced++
			default:
				outOfSync++
			}
		}
		fmt.Fprintf(&sb, "Total Applications: %d\n", len(apps))
		fmt.Fprintf(&sb, "Health: %d healthy, %d degraded, %d progressing, %d unknown\n",
			healthy, degraded, progressing, unknown)
		fmt.Fprintf(&sb, "Sync: %d synced, %d out-of-sync\n", synced, outOfSync)
	}

	return sb.String(), nil
}

func (e *ToolExecutor) getAddonConfigOnCluster(ctx context.Context, addonName, clusterName string) (string, error) {
	if addonName == "" {
		return "Please specify an addon name.", nil
	}
	if clusterName == "" {
		return "Please specify a cluster name.", nil
	}

	var sb strings.Builder

	// Fetch global values
	globalData, err := e.gp.GetFileContent(ctx, fmt.Sprintf("configuration/addons-global-values/%s.yaml", addonName), "main")
	if err != nil {
		fmt.Fprintf(&sb, "Global values: not found for addon '%s'\n", addonName)
	} else {
		s := string(globalData)
		if len(s) > 1500 {
			s = s[:1500] + "\n... (truncated)"
		}
		fmt.Fprintf(&sb, "=== Global values for %s ===\n%s\n\n", addonName, s)
	}

	// Fetch cluster values and extract addon section
	clusterData, err := e.gp.GetFileContent(ctx, fmt.Sprintf("configuration/addons-clusters-values/%s.yaml", clusterName), "main")
	if err != nil {
		fmt.Fprintf(&sb, "Cluster values: not found for cluster '%s'\n", clusterName)
	} else {
		// Parse the YAML to extract the addon-specific section
		var raw map[string]interface{}
		if yamlErr := yaml.Unmarshal(clusterData, &raw); yamlErr != nil {
			// Fall back to showing raw content
			s := string(clusterData)
			if len(s) > 1500 {
				s = s[:1500] + "\n... (truncated)"
			}
			fmt.Fprintf(&sb, "=== Cluster values for %s (raw) ===\n%s\n", clusterName, s)
		} else {
			// Look for addon section in the cluster values
			addonSection, found := raw[addonName]
			if !found {
				// Also check under clusterGlobalValues
				if cgv, ok := raw["clusterGlobalValues"]; ok {
					if cgvMap, ok := cgv.(map[string]interface{}); ok {
						addonSection, found = cgvMap[addonName]
					}
				}
			}

			if found {
				addonYAML, marshalErr := yaml.Marshal(addonSection)
				if marshalErr != nil {
					fmt.Fprintf(&sb, "=== Cluster overrides for %s on %s ===\n(could not serialize)\n", addonName, clusterName)
				} else {
					s := string(addonYAML)
					if len(s) > 1500 {
						s = s[:1500] + "\n... (truncated)"
					}
					fmt.Fprintf(&sb, "=== Cluster overrides for %s on %s ===\n%s\n", addonName, clusterName, s)
				}
			} else {
				fmt.Fprintf(&sb, "=== Cluster overrides for %s on %s ===\nNo addon-specific overrides found.\n", addonName, clusterName)
			}
		}
	}

	return sb.String(), nil
}

func (e *ToolExecutor) searchAddons(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "Please specify a search query.", nil
	}

	data, err := e.gp.GetFileContent(ctx, "configuration/addons-catalog.yaml", "main")
	if err != nil {
		return "", err
	}
	addons, err := e.parser.ParseAddonsCatalog(data)
	if err != nil {
		return "", err
	}

	queryLower := strings.ToLower(query)
	var sb strings.Builder
	count := 0
	for _, a := range addons {
		if strings.Contains(strings.ToLower(a.AppName), queryLower) {
			count++
			fmt.Fprintf(&sb, "- %s: chart=%s, version=%s, repo=%s\n", a.AppName, a.Chart, a.Version, a.RepoURL)
		}
	}

	if count == 0 {
		return fmt.Sprintf("No addons matching '%s' found.", query), nil
	}
	return fmt.Sprintf("Found %d addons matching '%s':\n%s", count, query, sb.String()), nil
}
