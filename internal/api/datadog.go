package api

import (
	"net/http"
	"time"
)

func (s *Server) handleDatadogStatus(w http.ResponseWriter, r *http.Request) {
	enabled := s.ddClient != nil && s.ddClient.IsEnabled()
	site := ""
	if s.ddClient != nil {
		site = s.ddClient.Site()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": enabled,
		"site":    site,
	})
}

func (s *Server) handleDatadogNamespaceMetrics(w http.ResponseWriter, r *http.Request) {
	if s.ddClient == nil || !s.ddClient.IsEnabled() {
		writeError(w, http.StatusServiceUnavailable, "Datadog is not configured")
		return
	}

	namespace := r.PathValue("namespace")
	if namespace == "" {
		writeError(w, http.StatusBadRequest, "namespace is required")
		return
	}

	metrics, err := s.ddClient.GetNamespaceMetrics(r.Context(), namespace, 15*time.Minute)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch metrics: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}
