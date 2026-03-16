package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/service"
)

// Server holds the HTTP handlers and their dependencies.
type Server struct {
	connSvc          *service.ConnectionService
	clusterSvc       *service.ClusterService
	addonSvc         *service.AddonService
	dashboardSvc     *service.DashboardService
	observabilitySvc *service.ObservabilityService
	upgradeSvc       *service.UpgradeService
	aiClient         *ai.Client
}

// NewServer creates a new API server.
func NewServer(
	connSvc *service.ConnectionService,
	clusterSvc *service.ClusterService,
	addonSvc *service.AddonService,
	dashboardSvc *service.DashboardService,
	observabilitySvc *service.ObservabilityService,
	upgradeSvc *service.UpgradeService,
	aiClient *ai.Client,
) *Server {
	return &Server{
		connSvc:          connSvc,
		clusterSvc:       clusterSvc,
		addonSvc:         addonSvc,
		dashboardSvc:     dashboardSvc,
		observabilitySvc: observabilitySvc,
		upgradeSvc:       upgradeSvc,
		aiClient:         aiClient,
	}
}

// NewRouter builds the HTTP router with all API routes and static file serving.
// staticFS can be nil if no static files are available (e.g., dev mode).
func NewRouter(srv *Server, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/v1/health", srv.handleHealth)

	// Connections
	mux.HandleFunc("GET /api/v1/connections/", srv.handleListConnections)
	mux.HandleFunc("POST /api/v1/connections/", srv.handleCreateConnection)
	mux.HandleFunc("DELETE /api/v1/connections/{name}", srv.handleDeleteConnection)
	mux.HandleFunc("POST /api/v1/connections/active", srv.handleSetActiveConnection)
	mux.HandleFunc("POST /api/v1/connections/test", srv.handleTestConnection)

	// Clusters
	mux.HandleFunc("GET /api/v1/clusters", srv.handleListClusters)
	mux.HandleFunc("GET /api/v1/clusters/{name}/values", srv.handleGetClusterValues)
	mux.HandleFunc("GET /api/v1/clusters/{name}/config-diff", srv.handleGetConfigDiff)
	mux.HandleFunc("GET /api/v1/clusters/{name}/comparison", srv.handleGetClusterComparison)
	mux.HandleFunc("GET /api/v1/clusters/{name}", srv.handleGetCluster)

	// Addons
	mux.HandleFunc("GET /api/v1/addons/list", srv.handleListAddons)
	mux.HandleFunc("GET /api/v1/addons/catalog", srv.handleGetAddonCatalog)
	mux.HandleFunc("GET /api/v1/addons/version-matrix", srv.handleGetVersionMatrix)
	mux.HandleFunc("GET /api/v1/addons/{name}/values", srv.handleGetAddonValues)
	mux.HandleFunc("GET /api/v1/addons/{name}", srv.handleGetAddonDetail)

	// Dashboard
	mux.HandleFunc("GET /api/v1/dashboard/stats", srv.handleGetDashboardStats)
	mux.HandleFunc("GET /api/v1/dashboard/pull-requests", srv.handleGetPullRequests)

	// Upgrade Impact Checker
	mux.HandleFunc("GET /api/v1/upgrade/{addonName}/versions", srv.handleListUpgradeVersions)
	mux.HandleFunc("POST /api/v1/upgrade/check", srv.handleCheckUpgrade)
	mux.HandleFunc("POST /api/v1/upgrade/ai-summary", srv.handleGetAISummary)
	mux.HandleFunc("GET /api/v1/upgrade/ai-status", srv.handleGetAIStatus)

	// AI Configuration
	mux.HandleFunc("GET /api/v1/ai/config", srv.handleGetAIConfig)
	mux.HandleFunc("POST /api/v1/ai/provider", srv.handleSetAIProvider)
	mux.HandleFunc("POST /api/v1/ai/test", srv.handleTestAI)

	// Observability
	mux.HandleFunc("GET /api/v1/observability/overview", srv.handleGetObservabilityOverview)

	// AI Agent
	mux.HandleFunc("POST /api/v1/agent/chat", srv.handleAgentChat)
	mux.HandleFunc("POST /api/v1/agent/reset", srv.handleAgentReset)

	// Static files (SPA)
	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the file; if not found, serve index.html for SPA routing
			path := r.URL.Path
			if path == "/" {
				path = "index.html"
			}
			if _, err := fs.Stat(staticFS, path[1:]); err != nil {
				// File not found — serve index.html for client-side routing
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	// Wrap with middleware
	var handler http.Handler = mux
	handler = corsMiddleware(handler)
	handler = loggingMiddleware(handler)

	return handler
}

// corsMiddleware adds CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-AAP-Connection")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
