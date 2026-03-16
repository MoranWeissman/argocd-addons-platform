package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

const systemPrompt = `You are an expert Kubernetes platform engineer assistant for the ArgoCD Addons Platform. You help users understand their addon deployments, cluster configurations, and upgrade impacts.

You have access to tools that let you query real data from the platform. Use them to provide accurate, data-driven answers. Always check the actual data before making claims.

Keep responses concise and actionable. Use bullet points for lists. When discussing versions or configs, be specific with numbers and paths.`

// Agent manages a multi-turn conversation with tool calling.
type Agent struct {
	client   *Client
	executor *ToolExecutor
	messages []ChatMessage
}

// NewAgent creates a new conversational agent backed by the given AI client and tool executor.
func NewAgent(client *Client, executor *ToolExecutor) *Agent {
	return &Agent{
		client:   client,
		executor: executor,
		messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// Chat sends a user message and returns the assistant's response.
// It handles tool calling loops internally (max 5 iterations).
func (a *Agent) Chat(ctx context.Context, userMessage string) (string, error) {
	if !a.client.IsEnabled() {
		return "", fmt.Errorf("AI not configured")
	}

	a.messages = append(a.messages, ChatMessage{Role: "user", Content: userMessage})

	for i := 0; i < 5; i++ {
		resp, err := a.callOllama(ctx)
		if err != nil {
			return "", err
		}

		// If no tool calls, we have the final response
		if len(resp.Message.ToolCalls) == 0 {
			a.messages = append(a.messages, resp.Message)
			return resp.Message.Content, nil
		}

		// Process tool calls
		a.messages = append(a.messages, resp.Message)

		for _, tc := range resp.Message.ToolCalls {
			result, err := a.executor.ExecuteTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error executing %s: %v", tc.Function.Name, err)
			}

			// Add tool response
			a.messages = append(a.messages, ChatMessage{
				Role:    "tool",
				Content: result,
			})
		}
	}

	return "I've used all my available tool calls. Here's what I found based on the data gathered so far.", nil
}

func (a *Agent) callOllama(ctx context.Context) (*ollamaChatResponse, error) {
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

	return &result, nil
}

// Reset clears conversation history (keeps system prompt).
func (a *Agent) Reset() {
	a.messages = []ChatMessage{
		{Role: "system", Content: systemPrompt},
	}
}
