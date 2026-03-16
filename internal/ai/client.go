package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Provider identifies which AI backend to use.
type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderNone   Provider = "none"
)

// Config holds AI provider settings.
type Config struct {
	Provider    Provider `yaml:"provider"`
	OllamaURL   string   `yaml:"ollama_url"`
	OllamaModel string   `yaml:"ollama_model"`
	AgentModel  string   `yaml:"agent_model"` // Separate model for agent (tool calling needs larger model)
}

// GetAgentModel returns the model to use for agent conversations.
// Falls back to OllamaModel if not set.
func (c Config) GetAgentModel() string {
	if c.AgentModel != "" {
		return c.AgentModel
	}
	return c.OllamaModel
}

// Client wraps communication with an AI provider.
type Client struct {
	config Config
	http   *http.Client
}

// NewClient creates a new AI client with the given configuration.
func NewClient(cfg Config) *Client {
	return &Client{config: cfg, http: &http.Client{}}
}

// IsEnabled returns true when a valid AI provider is configured.
func (c *Client) IsEnabled() bool {
	return c.config.Provider != ProviderNone && c.config.Provider != ""
}

// Summarize sends a prompt to the LLM and returns the response text.
func (c *Client) Summarize(ctx context.Context, prompt string) (string, error) {
	if !c.IsEnabled() {
		return "", fmt.Errorf("AI provider not configured")
	}

	switch c.config.Provider {
	case ProviderOllama:
		return c.ollamaSummarize(ctx, prompt)
	default:
		return "", fmt.Errorf("unsupported AI provider: %s", c.config.Provider)
	}
}

func (c *Client) ollamaSummarize(ctx context.Context, prompt string) (string, error) {
	url := c.config.OllamaURL + "/api/generate"

	body, _ := json.Marshal(map[string]interface{}{
		"model":  c.config.OllamaModel,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.3,
			"num_predict": 400,
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parsing ollama response: %w", err)
	}

	return result.Response, nil
}

// BuildUpgradePrompt creates a concise prompt for analyzing an upgrade.
// Keeps prompt small for faster LLM response.
func BuildUpgradePrompt(addonName, currentVersion, targetVersion string, added, removed, changed int, changedDetails string, conflicts string) string {
	// Limit changed details to top 15 entries to keep prompt short
	lines := strings.Split(changedDetails, "\n")
	if len(lines) > 15 {
		lines = append(lines[:15], fmt.Sprintf("... and %d more changes", len(lines)-15))
	}
	trimmedChanges := strings.Join(lines, "\n")

	prompt := fmt.Sprintf(`Helm chart upgrade analysis. Be brief.

%s: v%s → v%s
+%d added, -%d removed, ~%d changed

Key changes:
%s`, addonName, currentVersion, targetVersion, added, removed, changed, trimmedChanges)

	if conflicts != "" {
		prompt += "\n\nConflicts with our config:\n" + conflicts
	}

	prompt += `

Respond in this exact format (keep each section to 1-2 lines):
**Summary:** [what changed]
**Risk:** [low/medium/high] - [one line why]
**Action items:** [bullet list, max 3 items]`

	return prompt
}
