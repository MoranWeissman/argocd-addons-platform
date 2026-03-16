package api

import (
	"net/http"
)

func (s *Server) handleGetObservabilityOverview(w http.ResponseWriter, r *http.Request) {
	ac, err := s.connSvc.GetActiveArgocdClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	resp, err := s.observabilitySvc.GetOverview(r.Context(), ac)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
