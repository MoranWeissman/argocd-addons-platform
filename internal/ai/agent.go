package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role       string     `json:"role"`                  // system, user, assistant, tool
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // for tool responses
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"` // always "function"
	Function ToolCallFunc `json:"function"`
}

// ToolCallFunc describes the function name and arguments of a tool call.
type ToolCallFunc struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ChatResponse is a unified response type for all LLM providers.
type ChatResponse struct {
	Content   string     // text response
	ToolCalls []ToolCall // tool calls to execute
}

type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ChatMessage          `json:"messages"`
	Tools    []ToolDefinition       `json:"tools,omitempty"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaChatResponse struct {
	Message ChatMessage `json:"message"`
	Done    bool        `json:"done"`
}

const systemPrompt = `You are an expert Kubernetes platform engineer assistant for the ArgoCD Addons Platform.

=== PROJECT ARCHITECTURE ===
This platform manages software addons (Datadog, Istio, Kyverno, External Secrets, etc.) across 50+ EKS clusters using GitOps.

HOW IT WORKS:
1. All configuration lives in a Git repo with these key files:
   - configuration/cluster-addons.yaml — lists all clusters and which addons are enabled via labels
   - configuration/addons-catalog.yaml — defines available addons, their Helm charts, repos, and default versions
   - configuration/addons-global-values/<addon>.yaml — default Helm values for each addon
   - configuration/addons-clusters-values/<cluster>.yaml — per-cluster value overrides

2. ArgoCD ApplicationSets watch the Git repo and automatically:
   - Detect clusters with "addon-name: enabled" labels
   - Create an ArgoCD Application for each addon-cluster combination
   - Deploy the Helm chart with merged values (global + cluster overrides)

3. ArgoCD Application naming pattern: {addon-name}-{cluster-name}
   Example: "datadog-devops-argocd-addons-dev-eks" is the datadog addon on the devops-argocd-addons-dev-eks cluster

4. Only "enabled" labels matter. "disabled" means the addon is NOT deployed on that cluster.
   When asked "where is addon X installed/deployed", only report clusters where it is ENABLED.

5. Each addon is a Helm chart. The addon's resources (Pods, Services, ConfigMaps, CRDs, etc.) are managed by ArgoCD and visible in the app's resource tree.

6. Version overrides: a cluster can pin an addon to a specific version with a "{addon}-version" label (e.g. "datadog-version: 3.70.7"). Otherwise the catalog version is used.

=== RULES ===
1. NEVER guess or assume data. ALWAYS use tools first.
2. If a tool returns no data or an error, say so — do NOT invent answers.
3. Only state facts from tool results.
4. Keep responses concise with bullet points.
5. NEVER ask the user for information you can look up with tools. Call the tool yourself.
6. ALWAYS remember the full conversation context. If the user mentioned an addon earlier, use it.
7. When in doubt, call a tool. It is ALWAYS better to call a tool than to ask the user.

=== TOOL SELECTION GUIDE ===
CRITICAL DISTINCTION:
- "What addons are DEPLOYED/RUNNING?" → get_argocd_app_health (shows ACTUAL running apps in ArgoCD)
- "What addons EXIST/are available?" → list_addons (shows the catalog — what COULD be deployed)
These are DIFFERENT. "deployed" = get_argocd_app_health. "available"/"catalog" = list_addons.

- "Where is addon X deployed/installed?" → find_addon_deployments (only shows ENABLED clusters)
- "What addons are on cluster X?" → get_cluster_addons
- "What version of addon X on cluster Y?" → get_addon_on_cluster
- "Show me the pods/resources for addon X" → get_app_resources with app_name={addon}-{cluster}
- "What's happening with addon X?" → get_app_events with app_name={addon}-{cluster}
- "Show details of app X" → get_app_details with app_name={addon}-{cluster}
- "Is everything healthy?" → get_unhealthy_addons
- "What clusters are connected?" → get_cluster_status
- "Compare versions / upgrade" → list_chart_versions then compare_chart_versions
- "Config for addon X on cluster Y?" → get_addon_config_on_cluster
- "How many versions behind?" → list_chart_versions, then count
- "Search for info about addon X" → web_search for docs, CVEs, best practices
- "Platform info / ArgoCD version" → get_platform_info

=== CRITICAL: CONVERSATION CONTEXT ===
- ALWAYS use context from earlier messages. If the user said "datadog" earlier and now asks "how many pods", they mean datadog.
- NEVER respond with "which addon?" if it was already discussed.
- To get resources/events for an addon on a cluster, the ArgoCD app name is: {addon}-{cluster}

=== LEARNING ===
You can save important observations and learnings for future conversations using save_memory.
Save things like:
- User preferences ("user prefers short answers")
- Platform observations ("datadog is deployed only on the addons dev cluster")
- Useful patterns you discover
- Frequently asked questions and their answers
Do NOT save trivial or transient information. Only save genuinely useful learnings.
Your LEARNED MEMORIES above (if any) are from previous conversations — use them.

=== CRITICAL: CLUSTER NAME MATCHING ===
Match partial names against the KNOWN CLUSTERS list:
- "addons cluster" → cluster containing "addons" in its name
- "automation cluster" → cluster containing "automation"
- NEVER say "cluster not found" if a partial match exists.`

// Agent manages a multi-turn conversation with tool calling.
type Agent struct {
	client   *Client
	executor *ToolExecutor
	memory   *MemoryStore
	messages []ChatMessage
}

// NewAgent creates a new conversational agent backed by the given AI client and tool executor.
// It pre-loads cluster and addon context so the LLM always knows what exists.
func NewAgent(client *Client, executor *ToolExecutor, memory *MemoryStore) *Agent {
	a := &Agent{
		client:   client,
		executor: executor,
		memory:   memory,
	}
	a.initContext()
	return a
}

// initContext pre-loads cluster names and addon catalog into the system prompt.
func (a *Agent) initContext() {
	ctx := context.Background()

	contextInfo := ""

	// Pre-load cluster list
	clusterList, err := a.executor.listClusters(ctx)
	if err == nil && clusterList != "" {
		contextInfo += "\n\nKNOWN CLUSTERS:\n" + clusterList
	}

	// Pre-load addon catalog
	addonList, err := a.executor.listAddons(ctx)
	if err == nil && addonList != "" {
		contextInfo += "\nKNOWN ADDONS:\n" + addonList
	}

	// Inject learned memories
	if a.memory != nil {
		memories := a.memory.GetAll()
		if memories != "" {
			contextInfo += "\nLEARNED MEMORIES (from previous conversations):\n" + memories
		}
	}

	a.messages = []ChatMessage{
		{Role: "system", Content: systemPrompt + contextInfo},
	}
	slog.Info("agent session initialized")
}

// Chat sends a user message and returns the assistant's response.
// It handles tool calling loops internally (max 5 iterations).
func (a *Agent) Chat(ctx context.Context, userMessage string) (string, error) {
	if !a.client.IsEnabled() {
		return "", fmt.Errorf("AI not configured")
	}

	a.messages = append(a.messages, ChatMessage{Role: "user", Content: userMessage})

	chatStart := time.Now()
	for i := 0; i < 8; i++ {
		resp, err := a.callLLM(ctx)
		if err != nil {
			return "", err
		}

		// If no tool calls, we have the final response
		if len(resp.ToolCalls) == 0 {
			a.messages = append(a.messages, ChatMessage{Role: "assistant", Content: resp.Content})
			slog.Info("agent chat completed", "iterations", i+1, "duration", time.Since(chatStart))
			return resp.Content, nil
		}

		// Process tool calls
		a.messages = append(a.messages, ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			slog.Info("agent tool call", "tool", tc.Function.Name)
			result, err := a.executor.ExecuteTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error executing %s: %v", tc.Function.Name, err)
			}

			// Add tool response — format depends on provider
			switch a.client.config.Provider {
			case ProviderClaude:
				// Claude expects tool_result in a user message with content blocks
				a.messages = append(a.messages, ChatMessage{
					Role:       "user",
					Content:    result,
					ToolCallID: tc.ID,
				})
			case ProviderGemini:
				// Gemini expects functionResponse in a user message
				a.messages = append(a.messages, ChatMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.Function.Name, // Gemini uses function name, not an ID
				})
			default:
				// Ollama and OpenAI use role=tool with tool_call_id
				a.messages = append(a.messages, ChatMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				})
			}
		}
	}

	return "I've used all my available tool calls. Here's what I found based on the data gathered so far.", nil
}

// callLLM dispatches to the appropriate provider's chat endpoint.
func (a *Agent) callLLM(ctx context.Context) (*ChatResponse, error) {
	switch a.client.config.Provider {
	case ProviderOllama:
		return a.callOllamaChat(ctx)
	case ProviderClaude:
		return a.callClaudeChat(ctx)
	case ProviderOpenAI:
		return a.callOpenAIChat(ctx)
	case ProviderGemini:
		return a.callGeminiChat(ctx)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", a.client.config.Provider)
	}
}

func (a *Agent) callOllamaChat(ctx context.Context) (*ChatResponse, error) {
	reqBody := ollamaChatRequest{
		Model:    a.client.config.GetAgentModel(),
		Messages: a.messages,
		Tools:    GetToolDefinitions(),
		Stream:   false,
		Options: map[string]interface{}{
			"temperature": 0.3,
			"num_predict": 600,
		},
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", a.client.config.OllamaURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing ollama chat response: %w", err)
	}

	return &ChatResponse{
		Content:   result.Message.Content,
		ToolCalls: result.Message.ToolCalls,
	}, nil
}

// callClaudeChat sends messages to the Claude API with tool calling support.
func (a *Agent) callClaudeChat(ctx context.Context) (*ChatResponse, error) {
	// Convert tool definitions to Claude format
	claudeTools := convertToolsToClaude(GetToolDefinitions())

	// Convert messages to Claude format (separate system from messages)
	claudeMessages := convertMessagesToClaude(a.messages)

	reqBody := map[string]interface{}{
		"model":      a.client.config.GetAgentModel(),
		"max_tokens": 1024,
		"system":     systemPrompt,
		"messages":   claudeMessages,
		"tools":      claudeTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling claude chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.client.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text,omitempty"`
			ID    string          `json:"id,omitempty"`
			Name  string          `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing claude chat response: %w", err)
	}

	chatResp := &ChatResponse{}
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			chatResp.Content += block.Text
		case "tool_use":
			chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunc{
					Name:      block.Name,
					Arguments: block.Input,
				},
			})
		}
	}

	return chatResp, nil
}

// callOpenAIChat sends messages to the OpenAI API with tool calling support.
func (a *Agent) callOpenAIChat(ctx context.Context) (*ChatResponse, error) {
	// Convert tool definitions to OpenAI format
	openaiTools := convertToolsToOpenAI(GetToolDefinitions())

	// Convert messages to OpenAI format
	openaiMessages := convertMessagesToOpenAI(a.messages)

	reqBody := map[string]interface{}{
		"model":    a.client.config.GetAgentModel(),
		"messages": openaiMessages,
		"tools":    openaiTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling openai chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.client.config.APIKey)

	resp, err := a.client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing openai chat response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	choice := result.Choices[0]
	chatResp := &ChatResponse{
		Content: choice.Message.Content,
	}

	for _, tc := range choice.Message.ToolCalls {
		chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: ToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			},
		})
	}

	return chatResp, nil
}

// convertToolsToClaude converts tool definitions to Claude's format.
func convertToolsToClaude(tools []ToolDefinition) []map[string]interface{} {
	var claudeTools []map[string]interface{}
	for _, t := range tools {
		claudeTools = append(claudeTools, map[string]interface{}{
			"name":         t.Function.Name,
			"description":  t.Function.Description,
			"input_schema": json.RawMessage(t.Function.Parameters),
		})
	}
	return claudeTools
}

// convertToolsToOpenAI converts tool definitions to OpenAI's format.
func convertToolsToOpenAI(tools []ToolDefinition) []map[string]interface{} {
	var openaiTools []map[string]interface{}
	for _, t := range tools {
		openaiTools = append(openaiTools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  json.RawMessage(t.Function.Parameters),
			},
		})
	}
	return openaiTools
}

// convertMessagesToClaude converts internal messages to Claude's format.
// Claude does not use a "system" role in messages — it's a top-level parameter.
// Claude uses structured content blocks for tool results.
func convertMessagesToClaude(messages []ChatMessage) []map[string]interface{} {
	var claudeMessages []map[string]interface{}
	for _, m := range messages {
		// Skip system messages (handled as top-level parameter)
		if m.Role == "system" {
			continue
		}

		// Tool result messages: Claude expects these as user messages with tool_result content blocks
		if m.Role == "user" && m.ToolCallID != "" {
			claudeMessages = append(claudeMessages, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
			continue
		}

		// Tool responses from Ollama format — also convert to Claude tool_result
		if m.Role == "tool" {
			claudeMessages = append(claudeMessages, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
			continue
		}

		// Assistant messages with tool calls: convert to Claude content blocks
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var contentBlocks []map[string]interface{}
			if m.Content != "" {
				contentBlocks = append(contentBlocks, map[string]interface{}{
					"type": "text",
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				contentBlocks = append(contentBlocks, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": json.RawMessage(tc.Function.Arguments),
				})
			}
			claudeMessages = append(claudeMessages, map[string]interface{}{
				"role":    "assistant",
				"content": contentBlocks,
			})
			continue
		}

		// Regular user/assistant messages
		claudeMessages = append(claudeMessages, map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	return claudeMessages
}

// convertMessagesToOpenAI converts internal messages to OpenAI's format.
func convertMessagesToOpenAI(messages []ChatMessage) []map[string]interface{} {
	var openaiMessages []map[string]interface{}
	for _, m := range messages {
		msg := map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		}

		// Assistant messages with tool calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var toolCalls []map[string]interface{}
			for _, tc := range m.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": string(tc.Function.Arguments),
					},
				})
			}
			msg["tool_calls"] = toolCalls
		}

		// Tool response messages
		if m.Role == "tool" {
			msg["tool_call_id"] = m.ToolCallID
		}

		openaiMessages = append(openaiMessages, msg)
	}
	return openaiMessages
}

// callGeminiChat sends messages to the Gemini API with tool calling support.
func (a *Agent) callGeminiChat(ctx context.Context) (*ChatResponse, error) {
	// Convert tool definitions to Gemini format
	geminiTools := convertToolsToGemini(GetToolDefinitions())

	// Convert messages to Gemini format (separate system from messages)
	geminiContents := convertMessagesToGemini(a.messages)

	model := a.client.config.GetAgentModel()
	if model == "" {
		model = "gemini-2.5-flash"
	}

	reqBody := map[string]interface{}{
		"contents": geminiContents,
		"tools": []map[string]interface{}{
			{"function_declarations": geminiTools},
		},
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{
				{"text": systemPrompt},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":    0.3,
			"maxOutputTokens": 1024,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling gemini chat request: %w", err)
	}

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", a.client.config.APIKey)

	resp, err := a.client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text,omitempty"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall,omitempty"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing gemini chat response: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	chatResp := &ChatResponse{}
	for _, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
				ID:   part.FunctionCall.Name, // Gemini doesn't use separate IDs
				Type: "function",
				Function: ToolCallFunc{
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				},
			})
		} else if part.Text != "" {
			chatResp.Content += part.Text
		}
	}

	return chatResp, nil
}

// convertToolsToGemini converts tool definitions to Gemini's function_declarations format.
func convertToolsToGemini(tools []ToolDefinition) []map[string]interface{} {
	var geminiTools []map[string]interface{}
	for _, t := range tools {
		geminiTools = append(geminiTools, map[string]interface{}{
			"name":        t.Function.Name,
			"description": t.Function.Description,
			"parameters":  json.RawMessage(t.Function.Parameters),
		})
	}
	return geminiTools
}

// convertMessagesToGemini converts internal messages to Gemini's format.
// Gemini uses "model" instead of "assistant" and does not include system messages in contents.
// Tool results are sent as functionResponse parts in user messages.
func convertMessagesToGemini(messages []ChatMessage) []map[string]interface{} {
	var geminiContents []map[string]interface{}
	for _, m := range messages {
		// Skip system messages (handled as system_instruction)
		if m.Role == "system" {
			continue
		}

		// Tool result messages: Gemini expects functionResponse in a user message
		if m.Role == "tool" {
			geminiContents = append(geminiContents, map[string]interface{}{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"functionResponse": map[string]interface{}{
							"name": m.ToolCallID,
							"response": map[string]interface{}{
								"result": m.Content,
							},
						},
					},
				},
			})
			continue
		}

		// Claude-style tool results (user role with ToolCallID)
		if m.Role == "user" && m.ToolCallID != "" {
			geminiContents = append(geminiContents, map[string]interface{}{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"functionResponse": map[string]interface{}{
							"name": m.ToolCallID,
							"response": map[string]interface{}{
								"result": m.Content,
							},
						},
					},
				},
			})
			continue
		}

		// Assistant messages with tool calls: convert to model role with functionCall parts
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var parts []map[string]interface{}
			if m.Content != "" {
				parts = append(parts, map[string]interface{}{
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, map[string]interface{}{
					"functionCall": map[string]interface{}{
						"name": tc.Function.Name,
						"args": json.RawMessage(tc.Function.Arguments),
					},
				})
			}
			geminiContents = append(geminiContents, map[string]interface{}{
				"role":  "model",
				"parts": parts,
			})
			continue
		}

		// Regular messages: map assistant -> model
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		geminiContents = append(geminiContents, map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{"text": m.Content},
			},
		})
	}
	return geminiContents
}

// Reset clears conversation history (keeps system prompt).
func (a *Agent) Reset() {
	slog.Info("agent session reset")
	a.initContext()
}
