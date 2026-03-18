package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/api"
	"github.com/moran/argocd-addons-platform/internal/config"
	"github.com/moran/argocd-addons-platform/internal/datadog"
	"github.com/moran/argocd-addons-platform/internal/platform"
	"github.com/moran/argocd-addons-platform/internal/service"
)

func getEnvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// loadSecretsEnv loads KEY=VALUE pairs from secrets.env into the environment.
// Lines starting with # and empty lines are skipped. Does not override existing env vars.
func loadSecretsEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file doesn't exist, that's fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Remove surrounding quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
			count++
		}
	}
	if count > 0 {
		log.Printf("Loaded %d secrets from %s", count, path)
	}
}

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	configPath := flag.String("config", "config.yaml", "Path to config file (local mode)")
	staticDir := flag.String("static", "", "Path to static files directory (UI)")
	flag.Parse()

	// Override port from env
	if envPort := os.Getenv("AAP_PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", port)
	}
	if envConfig := os.Getenv("AAP_CONFIG"); envConfig != "" {
		*configPath = envConfig
	}
	if envStatic := os.Getenv("AAP_STATIC_DIR"); envStatic != "" {
		*staticDir = envStatic
	}

	// Load secrets from secrets.env for local development
	loadSecretsEnv("secrets.env")

	// Detect runtime mode
	mode := platform.Detect()
	log.Printf("ArgoCD Addons Platform starting in %s mode", mode)

	// Initialize config store
	var store config.Store
	switch mode {
	case platform.ModeKubernetes:
		// In-cluster: still use file store but from a mounted path
		// K8s Secrets store can be added later
		mountedConfig := "/etc/aap/config.yaml"
		if _, err := os.Stat(mountedConfig); err == nil {
			*configPath = mountedConfig
		}
		store = config.NewFileStore(*configPath)
	default:
		store = config.NewFileStore(*configPath)
	}

	// AI configuration
	aiCfg := ai.Config{
		Provider:      ai.Provider(os.Getenv("AI_PROVIDER")),
		OllamaURL:    getEnvDefault("AI_OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:  getEnvDefault("AI_OLLAMA_MODEL", "llama3.2"),
		AgentModel:   os.Getenv("AI_AGENT_MODEL"),
		APIKey:       os.Getenv("AI_API_KEY"),
		CloudModel:   os.Getenv("AI_CLOUD_MODEL"),
		BaseURL:      os.Getenv("AI_BASE_URL"),
		AuthHeader:   os.Getenv("AI_AUTH_HEADER"),
		GitOpsEnabled: os.Getenv("GITOPS_ACTIONS_ENABLED") == "true",
	}
	if v := os.Getenv("AI_MAX_ITERATIONS"); v != "" {
		fmt.Sscanf(v, "%d", &aiCfg.MaxIterations)
	}
	aiClient := ai.NewClient(aiCfg)
	if aiClient.IsEnabled() {
		model := aiCfg.OllamaModel
		if aiCfg.Provider == ai.ProviderClaude || aiCfg.Provider == ai.ProviderOpenAI || aiCfg.Provider == ai.ProviderGemini {
			model = aiCfg.CloudModel
		}
		log.Printf("AI provider enabled: %s (model: %s)", aiCfg.Provider, model)
	}

	// Datadog configuration
	ddCfg := datadog.Config{
		APIKey: os.Getenv("DATADOG_API_KEY"),
		AppKey: os.Getenv("DATADOG_APP_KEY"),
		Site:   getEnvDefault("DATADOG_SITE", "datadoghq.com"),
	}
	ddClient := datadog.NewClient(ddCfg)
	if ddClient.IsEnabled() {
		log.Printf("Datadog metrics enabled (site: %s)", ddCfg.Site)
	}

	// Wire up services
	connSvc := service.NewConnectionService(store)
	clusterSvc := service.NewClusterService()
	addonSvc := service.NewAddonService()
	dashboardSvc := service.NewDashboardService(connSvc)
	observabilitySvc := service.NewObservabilityService()
	upgradeSvc := service.NewUpgradeService(aiClient)

	// Build server
	srv := api.NewServer(connSvc, clusterSvc, addonSvc, dashboardSvc, observabilitySvc, upgradeSvc, aiClient, ddClient)

	// Static files
	var staticFS fs.FS
	if *staticDir != "" {
		if info, err := os.Stat(*staticDir); err == nil && info.IsDir() {
			staticFS = os.DirFS(*staticDir)
			log.Printf("Serving static files from %s", *staticDir)
		}
	}

	router := api.NewRouter(srv, staticFS)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
