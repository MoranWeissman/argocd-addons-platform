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
	ProviderClaude Provider = "claude"
	ProviderOpenAI Provider = "openai"
	ProviderNone   Provider = "none"
)

// Config holds AI provider settings.
type Config struct {
	Provider    Provider `yaml:"provider"`
	OllamaURL   string   `yaml:"ollama_url"`
	OllamaModel string   `yaml:"ollama_model"`
	AgentModel  string   `yaml:"agent_model"`  // Separate model for agent (tool calling needs larger model)
	APIKey      string   `yaml:"api_key"`       // API key for Claude/OpenAI
	CloudModel  string   `yaml:"cloud_model"`   // Model name for cloud providers (e.g., "claude-sonnet-4-20250514", "gpt-4o")
}

// GetAgentModel returns the model to use for agent conversations.
// For cloud providers, falls back to CloudModel. For Ollama, falls back to OllamaModel.
func (c Config) GetAgentModel() string {
	if c.AgentModel != "" {
		return c.AgentModel
	}
	if c.Provider == ProviderClaude || c.Provider == ProviderOpenAI {
		return c.CloudModel
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
	case ProviderClaude:
		return c.claudeSummarize(ctx, prompt)
	case ProviderOpenAI:
		return c.openaiSummarize(ctx, prompt)
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

func (c *Client) claudeSummarize(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model":      c.config.CloudModel,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshaling claude request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("claude returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parsing claude response: %w", err)
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", fmt.Errorf("empty response from Claude")
}

func (c *Client) openaiSummarize(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model":      c.config.CloudModel,
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshaling openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parsing openai response: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("empty response from OpenAI")
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
