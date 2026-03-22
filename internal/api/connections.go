package api

import (
	"encoding/json"
	"log/slog"
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

	if err := s.connSvc.Create(req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "name": req.Name})
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "connection name is required")
		return
	}

	var req models.CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = name // ensure name matches URL

	if err := s.connSvc.Create(req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "name": name})
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

func (s *Server) handleTestCredentials(w http.ResponseWriter, r *http.Request) {
	var req models.CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conn := &models.Connection{
		Name:   req.Name,
		Git:    req.Git,
		Argocd: req.Argocd,
	}

	slog.Info("test-credentials request",
		"git_provider", conn.Git.Provider,
		"git_repo_url", conn.Git.RepoURL,
		"git_owner", conn.Git.Owner,
		"git_repo", conn.Git.Repo,
		"has_token", conn.Git.Token != "",
		"argocd_url", conn.Argocd.ServerURL,
	)

	gitErr, argocdErr := s.connSvc.TestCredentials(r.Context(), conn)

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
