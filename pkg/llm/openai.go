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
// OpenAI Provider
// ===========================================================================

const openAIDefaultBaseURL = "https://api.openai.com/v1"

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	config     ProviderConfig
	httpClient *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(cfg ProviderConfig) *OpenAIProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = openAIDefaultBaseURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &OpenAIProvider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Type returns the provider type.
func (p *OpenAIProvider) Type() ProviderType {
	return ProviderOpenAI
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return p.config.Name
}

// Complete sends a non-streaming completion request.
func (p *OpenAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	body, err := p.buildRequestBody(req, false)
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to build request body", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to create HTTP request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewError(ErrCodeProviderUnavailable, "OpenAI request failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeProviderUnavailable, fmt.Sprintf("OpenAI returned status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	var result openAICompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to decode OpenAI response", err)
	}

	if len(result.Choices) == 0 {
		return nil, NewError(ErrCodeProviderUnavailable, "OpenAI returned no choices", nil)
	}

	choice := result.Choices[0]
	latency := time.Since(start)

	return &CompletionResponse{
		Content: choice.Message.Content,
		FunctionCall: convertOpenAIFunctionCall(choice.Message.ToolCalls),
		FinishReason: choice.FinishReason,
		Usage: TokenUsage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
			TotalTokens:  result.Usage.TotalTokens,
		},
		Provider: ProviderOpenAI,
		Model:    result.Model,
		Latency:  latency,
	}, nil
}

// Stream sends a streaming completion request.
func (p *OpenAIProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	body, err := p.buildRequestBody(req, true)
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to build request body", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to create HTTP request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewError(ErrCodeProviderUnavailable, "OpenAI streaming request failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeProviderUnavailable, fmt.Sprintf("OpenAI returned status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	ch := make(chan StreamChunk, 10)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()
		p.readSSEStream(ctx, resp.Body, ch, req.Model)
	}()

	return ch, nil
}

// HealthCheck checks if the provider is available.
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// ===========================================================================
// OpenAI Request / Response Types
// ===========================================================================

type openAIMessage struct {
	Role       string              `json:"role"`
	Content    string              `json:"content"`
	Name       string              `json:"name,omitempty"`
	ToolCalls  []openAIToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function openAIFunction   `json:"function"`
}

type openAIFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAICompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []openAIMessage   `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Tools       []openAITool      `json:"tools,omitempty"`
	Stream      bool              `json:"stream"`
}

type openAITool struct {
	Type     string           `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type openAICompletionResponse struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Choices []openAIChoice   `json:"choices"`
	Usage   openAIUsage      `json:"usage"`
}

type openAIChoice struct {
	Index        int          `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type openAIStreamResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Index        int          `json:"index"`
	Delta        openAIMessage `json:"delta"`
	FinishReason string       `json:"finish_reason"`
}

// ===========================================================================
// Helpers
// ===========================================================================

func (p *OpenAIProvider) buildRequestBody(req *CompletionRequest, stream bool) ([]byte, error) {
	messages := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openAIMessage{
			Role:    string(m.Role),
			Content: m.Content,
			Name:    m.Name,
		})
	}

	body := openAICompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      stream,
	}

	if len(req.Functions) > 0 {
		tools := make([]openAITool, len(req.Functions))
		for i, fn := range req.Functions {
			tools[i] = openAITool{Type: "function", Function: fn}
		}
		body.Tools = tools
	}

	return json.Marshal(body)
}

func (p *OpenAIProvider) readSSEStream(ctx context.Context, r io.Reader, ch chan<- StreamChunk, model string) {
	scanner := bufio.NewScanner(r)
	var buffer strings.Builder
	var currentToolCall *FunctionCall

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
		if data == "[DONE]" {
			ch <- StreamChunk{Done: true, Provider: ProviderOpenAI, Model: model}
			return
		}

		var streamResp openAIStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) == 0 {
			continue
		}

		delta := streamResp.Choices[0].Delta
		finishReason := streamResp.Choices[0].FinishReason

		if delta.Content != "" {
			buffer.WriteString(delta.Content)
			ch <- StreamChunk{
				Content:  delta.Content,
				Provider: ProviderOpenAI,
				Model:    model,
			}
		}

		if len(delta.ToolCalls) > 0 {
			tc := delta.ToolCalls[0]
			if currentToolCall == nil {
				currentToolCall = &FunctionCall{Name: tc.Function.Name}
			}
			currentToolCall.Arguments += tc.Function.Arguments
		}

		if finishReason != "" {
			chunk := StreamChunk{
				FinishReason: finishReason,
				Done:         true,
				Provider:     ProviderOpenAI,
				Model:        model,
			}
			if buffer.Len() > 0 {
				chunk.Content = buffer.String()
			}
			if currentToolCall != nil {
				chunk.FunctionCall = currentToolCall
			}
			ch <- chunk
			return
		}
	}

	// Stream ended without [DONE]
	if buffer.Len() > 0 || currentToolCall != nil {
		ch <- StreamChunk{
			Content:      buffer.String(),
			FunctionCall: currentToolCall,
			Done:         true,
			Provider:     ProviderOpenAI,
			Model:        model,
		}
	}
}

func convertOpenAIFunctionCall(toolCalls []openAIToolCall) *FunctionCall {
	if len(toolCalls) == 0 {
		return nil
	}
	return &FunctionCall{
		Name:      toolCalls[0].Function.Name,
		Arguments: toolCalls[0].Function.Arguments,
	}
}
