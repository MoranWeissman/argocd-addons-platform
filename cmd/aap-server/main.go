package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/api"
	"github.com/moran/argocd-addons-platform/internal/config"
	"github.com/moran/argocd-addons-platform/internal/platform"
	"github.com/moran/argocd-addons-platform/internal/service"
)

func getEnvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
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
		Provider:    ai.Provider(os.Getenv("AI_PROVIDER")),
		OllamaURL:  getEnvDefault("AI_OLLAMA_URL", "http://localhost:11434"),
		OllamaModel: getEnvDefault("AI_OLLAMA_MODEL", "llama3.2"),
		AgentModel:  os.Getenv("AI_AGENT_MODEL"), // defaults to OllamaModel if empty
	}
	aiClient := ai.NewClient(aiCfg)
	if aiClient.IsEnabled() {
		log.Printf("AI provider enabled: %s (model: %s)", aiCfg.Provider, aiCfg.OllamaModel)
	}

	// Wire up services
	connSvc := service.NewConnectionService(store)
	clusterSvc := service.NewClusterService()
	addonSvc := service.NewAddonService()
	dashboardSvc := service.NewDashboardService(connSvc)
	observabilitySvc := service.NewObservabilityService()
	upgradeSvc := service.NewUpgradeService(aiClient)

	// Build server
	srv := api.NewServer(connSvc, clusterSvc, addonSvc, dashboardSvc, observabilitySvc, upgradeSvc, aiClient)

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
