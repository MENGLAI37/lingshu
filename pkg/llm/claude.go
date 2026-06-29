package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ===========================================================================
// Claude Provider
// ===========================================================================

const claudeDefaultBaseURL = "https://api.anthropic.com/v1"

// ClaudeProvider implements the Provider interface for Anthropic Claude.
type ClaudeProvider struct {
	config     ProviderConfig
	httpClient *http.Client
}

// NewClaudeProvider creates a new Claude provider.
func NewClaudeProvider(cfg ProviderConfig) *ClaudeProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = claudeDefaultBaseURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &ClaudeProvider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Type returns the provider type.
func (p *ClaudeProvider) Type() ProviderType {
	return ProviderClaude
}

// Name returns the provider name.
func (p *ClaudeProvider) Name() string {
	return p.config.Name
}

// Complete sends a non-streaming completion request.
func (p *ClaudeProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	body, err := p.buildRequestBody(req, false)
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to build Claude request body", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to create HTTP request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewError(ErrCodeProviderUnavailable, "Claude request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeProviderUnavailable, fmt.Sprintf("Claude returned status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	var result claudeCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to decode Claude response", err)
	}

	content, toolUse := extractClaudeContent(result.Content)
	latency := time.Since(start)

	var fnCall *FunctionCall
	if toolUse != nil {
		fnCall = &FunctionCall{
			Name:      toolUse.Name,
			Arguments: string(toolUse.Input),
		}
	}

	return &CompletionResponse{
		Content:      content,
		FunctionCall: fnCall,
		FinishReason: result.StopReason,
		Usage: TokenUsage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			TotalTokens:  result.Usage.InputTokens + result.Usage.OutputTokens,
		},
		Provider: ProviderClaude,
		Model:    result.Model,
		Latency:  latency,
	}, nil
}

// Stream sends a streaming completion request.
func (p *ClaudeProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	body, err := p.buildRequestBody(req, true)
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to build Claude request body", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to create HTTP request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewError(ErrCodeProviderUnavailable, "Claude streaming request failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeProviderUnavailable, fmt.Sprintf("Claude returned status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	ch := make(chan StreamChunk, 10)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		p.readSSEStream(ctx, resp.Body, ch, req.Model)
	}()

	return ch, nil
}

// HealthCheck checks if the provider is available.
func (p *ClaudeProvider) HealthCheck(ctx context.Context) error {
	// Claude doesn't have a dedicated models endpoint in the same way,
	// so we do a lightweight request.
	body, _ := json.Marshal(map[string]interface{}{
		"model":      p.config.Model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Even a 400 is fine for health check - it means the API is reachable.
	if resp.StatusCode >= 500 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// ===========================================================================
// Claude Request / Response Types
// ===========================================================================

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type claudeCompletionRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []claudeMessage `json:"messages"`
	System      string          `json:"system,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []claudeTool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type claudeContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type claudeCompletionResponse struct {
	ID         string               `json:"id"`
	Model      string               `json:"model"`
	Content    []claudeContentBlock `json:"content"`
	StopReason string               `json:"stop_reason"`
	Usage      claudeUsage          `json:"usage"`
}

type claudeUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// Stream event types
type claudeStreamEvent struct {
	Type    string               `json:"type"`
	Index   int                  `json:"index,omitempty"`
	Content []claudeContentBlock `json:"content,omitempty"`
	Delta   *claudeStreamDelta   `json:"delta,omitempty"`
	Model   string               `json:"model,omitempty"`
}

type claudeStreamDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ===========================================================================
// Helpers
// ===========================================================================

func (p *ClaudeProvider) buildRequestBody(req *CompletionRequest, stream bool) ([]byte, error) {
	messages := make([]claudeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			continue // system prompt handled separately
		}
		messages = append(messages, claudeMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body := claudeCompletionRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Messages:    messages,
		Temperature: req.Temperature,
		Stream:      stream,
	}
	if req.SystemPrompt != "" {
		body.System = req.SystemPrompt
	} else {
		// Extract system messages
		for _, m := range req.Messages {
			if m.Role == RoleSystem {
				body.System = m.Content
				break
			}
		}
	}

	if len(req.Functions) > 0 {
		tools := make([]claudeTool, len(req.Functions))
		for i, fn := range req.Functions {
			tools[i] = claudeTool{
				Name:        fn.Name,
				Description: fn.Description,
				InputSchema: fn.Parameters,
			}
		}
		body.Tools = tools
	}

	return json.Marshal(body)
}

func (p *ClaudeProvider) readSSEStream(ctx context.Context, r io.Reader, ch chan<- StreamChunk, model string) {
	scanner := bufio.NewScanner(r)
	var buffer strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event claudeStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Text != "" {
				buffer.WriteString(event.Delta.Text)
				ch <- StreamChunk{
					Content:  event.Delta.Text,
					Provider: ProviderClaude,
					Model:    model,
				}
			}
		case "message_stop":
			ch <- StreamChunk{
				Content:  buffer.String(),
				Done:     true,
				Provider: ProviderClaude,
				Model:    model,
			}
			return
		}
	}

	if buffer.Len() > 0 {
		ch <- StreamChunk{
			Content:  buffer.String(),
			Done:     true,
			Provider: ProviderClaude,
			Model:    model,
		}
	}
}

func extractClaudeContent(blocks []claudeContentBlock) (string, *claudeContentBlock) {
	var textParts []string
	var toolUse *claudeContentBlock
	for _, block := range blocks {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			toolUse = &block
		}
	}
	return strings.Join(textParts, ""), toolUse
}
