package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/moran/argocd-addons-platform/internal/ai"
)

// agentSessions stores per-session agents (in-memory, simple approach).
var (
	agentSessions = make(map[string]*ai.Agent)
	agentMu       sync.Mutex
)

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Get or create agent session
	agentMu.Lock()
	agent, exists := agentSessions[req.SessionID]
	if !exists || req.SessionID == "" {
		// Create new agent with active connection's providers
		gp, err := s.connSvc.GetActiveGitProvider()
		if err != nil {
			agentMu.Unlock()
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		ac, err := s.connSvc.GetActiveArgocdClient()
		if err != nil {
			agentMu.Unlock()
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}

		executor := ai.NewToolExecutor(gp, ac)
		agent = ai.NewAgent(s.aiClient, executor)

		if req.SessionID == "" {
			req.SessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
		}
		agentSessions[req.SessionID] = agent
	}
	agentMu.Unlock()

	response, err := agent.Chat(r.Context(), req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": req.SessionID,
		"response":   response,
	})
}

func (s *Server) handleAgentReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	agentMu.Lock()
	if agent, exists := agentSessions[req.SessionID]; exists {
		agent.Reset()
	}
	agentMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
