package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moran/argocd-addons-platform/internal/ai"
)

type aiProviderInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Model      string `json:"model"`
}

type aiConfigResponse struct {
	CurrentProvider    string           `json:"current_provider"`
	AvailableProviders []aiProviderInfo `json:"available_providers"`
}

func (s *Server) handleGetAIConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.aiClient.GetConfig()

	providers := []aiProviderInfo{
		{
			ID:         "ollama",
			Name:       "Ollama (Local)",
			Configured: cfg.OllamaURL != "",
			Model:      cfg.OllamaModel,
		},
		{
			ID:         "claude",
			Name:       "Claude (Anthropic)",
			Configured: cfg.APIKey != "" && cfg.Provider == ai.ProviderClaude,
			Model:      cloudModelForProvider(cfg, ai.ProviderClaude),
		},
		{
			ID:         "openai",
			Name:       "OpenAI",
			Configured: cfg.APIKey != "" && cfg.Provider == ai.ProviderOpenAI,
			Model:      cloudModelForProvider(cfg, ai.ProviderOpenAI),
		},
		{
			ID:         "gemini",
			Name:       "Gemini (Google)",
			Configured: cfg.APIKey != "" && cfg.Provider == ai.ProviderGemini,
			Model:      cloudModelForProvider(cfg, ai.ProviderGemini),
		},
	}

	// If an API key is set, mark all cloud providers that could use it as configured
	if cfg.APIKey != "" {
		for i := range providers {
			if providers[i].ID != "ollama" {
				providers[i].Configured = true
				if providers[i].Model == "" {
					providers[i].Model = cfg.CloudModel
				}
			}
		}
	}

	resp := aiConfigResponse{
		CurrentProvider:    string(cfg.Provider),
		AvailableProviders: providers,
	}

	writeJSON(w, http.StatusOK, resp)
}

func cloudModelForProvider(cfg ai.Config, p ai.Provider) string {
	if cfg.Provider == p {
		return cfg.CloudModel
	}
	return cfg.CloudModel
}

func (s *Server) handleSetAIProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate provider
	switch ai.Provider(req.Provider) {
	case ai.ProviderOllama, ai.ProviderClaude, ai.ProviderOpenAI, ai.ProviderGemini, ai.ProviderNone:
		// valid
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported provider: %s", req.Provider))
		return
	}

	s.aiClient.SetProvider(ai.Provider(req.Provider))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "provider": req.Provider})
}

func (s *Server) handleTestAI(w http.ResponseWriter, r *http.Request) {
	if !s.aiClient.IsEnabled() {
		writeError(w, http.StatusServiceUnavailable, "AI not configured")
		return
	}

	result, err := s.aiClient.Summarize(r.Context(), "Say 'AI connection successful' in one short sentence.")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "response": result})
}
