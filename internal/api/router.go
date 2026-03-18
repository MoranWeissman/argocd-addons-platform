package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/auth"
	"github.com/moran/argocd-addons-platform/internal/datadog"
	"github.com/moran/argocd-addons-platform/internal/service"
	"golang.org/x/crypto/bcrypt"
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
	ddClient         *datadog.Client
	agentMemory      *ai.MemoryStore
	authStore        *auth.Store
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
	ddClient *datadog.Client,
) *Server {
	// Initialize agent memory — store in /tmp for containers (writable), or local dir for dev
	memoryPath := "/tmp/aap-agent-memory.json"
	agentMemory := ai.NewMemoryStore(memoryPath)

	// Initialize auth store (auto-detects K8s vs local mode)
	authStore := auth.NewStore()

	return &Server{
		connSvc:          connSvc,
		clusterSvc:       clusterSvc,
		addonSvc:         addonSvc,
		dashboardSvc:     dashboardSvc,
		observabilitySvc: observabilitySvc,
		upgradeSvc:       upgradeSvc,
		aiClient:         aiClient,
		ddClient:         ddClient,
		agentMemory:      agentMemory,
		authStore:        authStore,
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

	// Datadog Metrics
	mux.HandleFunc("GET /api/v1/datadog/status", srv.handleDatadogStatus)
	mux.HandleFunc("GET /api/v1/datadog/metrics/{namespace}", srv.handleDatadogNamespaceMetrics)
	mux.HandleFunc("GET /api/v1/datadog/cluster-metrics/{clusterName}", srv.handleDatadogClusterMetrics)

	// AI Agent
	mux.HandleFunc("POST /api/v1/agent/chat", srv.handleAgentChat)
	mux.HandleFunc("POST /api/v1/agent/reset", srv.handleAgentReset)

	// Cluster info
	mux.HandleFunc("GET /api/v1/cluster/nodes", srv.handleGetNodeInfo)

	// Auth
	mux.HandleFunc("POST /api/v1/auth/login", srv.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/update-password", srv.handleUpdatePassword)
	mux.HandleFunc("POST /api/v1/auth/hash", srv.handleHashPassword)

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
	handler = srv.basicAuthMiddleware(handler)
	handler = corsMiddleware(handler)
	handler = loggingMiddleware(handler)

	return handler
}

// --- Session token auth ---

var (
	activeSessions   = make(map[string]time.Time) // token -> expiry
	sessionsMu       sync.RWMutex
	sessionLifetime  = 24 * time.Hour
)

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func isValidSession(token string) bool {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	expiry, ok := activeSessions[token]
	return ok && time.Now().Before(expiry)
}

// handleLogin validates credentials and returns a session token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// If no auth configured, allow any login
	if !s.authStore.HasUsers() {
		token := generateToken()
		sessionsMu.Lock()
		activeSessions[token] = time.Now().Add(sessionLifetime)
		sessionsMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
		return
	}

	if !s.authStore.ValidateCredentials(req.Username, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token := generateToken()
	sessionsMu.Lock()
	activeSessions[token] = time.Now().Add(sessionLifetime)
	sessionsMu.Unlock()

	slog.Info("user logged in", "username", req.Username)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// basicAuthMiddleware enforces token-based auth on all API routes.
// Accepts: Authorization: Bearer <token>
// Skips: health checks, login endpoint, and static files.
func (s *Server) basicAuthMiddleware(next http.Handler) http.Handler {
	// If no users configured, skip auth entirely
	if !s.authStore.HasUsers() {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth for: health, login, static files
		if path == "/api/v1/health" || path == "/api/v1/auth/login" || !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Check Bearer token
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if isValidSession(token) {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

// handleUpdatePassword allows changing the password. Verifies current password first.
func (s *Server) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	if !s.authStore.HasUsers() {
		writeError(w, http.StatusBadRequest, "no password configured")
		return
	}

	var req struct {
		Username        string `json:"username"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.NewPassword == "" || len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	// Default to "admin" if no username provided (backward compat)
	username := req.Username
	if username == "" {
		username = "admin"
	}

	if err := s.authStore.UpdatePassword(username, req.CurrentPassword, req.NewPassword); err != nil {
		if strings.Contains(err.Error(), "incorrect") {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if strings.Contains(err.Error(), "at least") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

// handleHashPassword generates a bcrypt hash from a plaintext password.
// Only available when auth is disabled (no users configured) for initial setup.
func (s *Server) handleHashPassword(w http.ResponseWriter, r *http.Request) {
	if s.authStore.HasUsers() {
		writeError(w, http.StatusForbidden, "hash endpoint is only available when auth is disabled")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate hash")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"hash": string(hash)})
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

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs each request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(sr, r)
		slog.Info("request completed", "method", r.Method, "path", r.URL.Path, "status", sr.statusCode, "duration", time.Since(start))
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
