package api

import (
	"encoding/json"
	"net/http"

	"github.com/moran/argocd-addons-platform/internal/models"
)

func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	resp, err := s.connSvc.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	var req models.CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := s.connSvc.Create(req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "name": req.Name})
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "connection name is required")
		return
	}

	if err := s.connSvc.Delete(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}

func (s *Server) handleSetActiveConnection(w http.ResponseWriter, r *http.Request) {
	var req models.SetActiveConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.connSvc.SetActive(req.ConnectionName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "active", "connection": req.ConnectionName})
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	gitErr, argocdErr := s.connSvc.TestConnection(r.Context())

	result := map[string]interface{}{
		"git":    map[string]interface{}{"status": "ok"},
		"argocd": map[string]interface{}{"status": "ok"},
	}

	if gitErr != nil {
		result["git"] = map[string]interface{}{"status": "error", "message": gitErr.Error()}
	}
	if argocdErr != nil {
		result["argocd"] = map[string]interface{}{"status": "error", "message": argocdErr.Error()}
	}

	writeJSON(w, http.StatusOK, result)
}
