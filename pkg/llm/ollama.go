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
// Ollama Provider
// ===========================================================================

const ollamaDefaultBaseURL = "http://localhost:11434"

// OllamaProvider implements the Provider interface for local Ollama models.
type OllamaProvider struct {
	config     ProviderConfig
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(cfg ProviderConfig) *OllamaProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = ollamaDefaultBaseURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second // Local models may be slower
	}
	return &OllamaProvider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Type returns the provider type.
func (p *OllamaProvider) Type() ProviderType {
	return ProviderOllama
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return p.config.Name
}

// Complete sends a non-streaming completion request.
func (p *OllamaProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	body, err := p.buildRequestBody(req, false)
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to build Ollama request body", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to create HTTP request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewError(ErrCodeProviderUnavailable, "Ollama request failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeProviderUnavailable, fmt.Sprintf("Ollama returned status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	var result ollamaCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to decode Ollama response", err)
	}

	latency := time.Since(start)
	return &CompletionResponse{
		Content:      result.Message.Content,
		FinishReason: result.DoneReason,
		Usage: TokenUsage{
			InputTokens:  result.PromptEvalCount,
			OutputTokens: result.EvalCount,
			TotalTokens:  result.PromptEvalCount + result.EvalCount,
		},
		Provider: ProviderOllama,
		Model:    result.Model,
		Latency:  latency,
	}, nil
}

// Stream sends a streaming completion request.
func (p *OllamaProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	body, err := p.buildRequestBody(req, true)
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to build Ollama request body", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, NewError(ErrCodeInvalidRequest, "failed to create HTTP request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewError(ErrCodeProviderUnavailable, "Ollama streaming request failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, NewError(ErrCodeProviderUnavailable, fmt.Sprintf("Ollama returned status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	ch := make(chan StreamChunk, 10)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()
		p.readNDJSONStream(ctx, resp.Body, ch, req.Model)
	}()

	return ch, nil
}

// HealthCheck checks if the Ollama service is available.
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.BaseURL+"/api/tags", nil)
	if err != nil {
		return err
	}

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

// DetectLocal checks if a local Ollama instance is running.
func (p *OllamaProvider) DetectLocal(ctx context.Context) bool {
	return p.HealthCheck(ctx) == nil
}

// ===========================================================================
// Ollama Request / Response Types
// ===========================================================================

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaTool struct {
	Type     string           `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type ollamaCompletionRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Tools    []ollamaTool     `json:"tools,omitempty"`
}

type ollamaCompletionResponse struct {
	Model            string         `json:"model"`
	Message          ollamaMessage  `json:"message"`
	Done             bool           `json:"done"`
	DoneReason       string         `json:"done_reason,omitempty"`
	PromptEvalCount  int64          `json:"prompt_eval_count"`
	EvalCount        int64          `json:"eval_count"`
}

// ===========================================================================
// Helpers
// ===========================================================================

func (p *OllamaProvider) buildRequestBody(req *CompletionRequest, stream bool) ([]byte, error) {
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body := ollamaCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
	}

	if req.Temperature > 0 {
		body.Options = map[string]interface{}{
			"temperature": req.Temperature,
		}
	}
	if req.MaxTokens > 0 {
		if body.Options == nil {
			body.Options = map[string]interface{}{}
		}
		body.Options["num_predict"] = req.MaxTokens
	}

	if len(req.Functions) > 0 {
		tools := make([]ollamaTool, len(req.Functions))
		for i, fn := range req.Functions {
			tools[i] = ollamaTool{Type: "function", Function: fn}
		}
		body.Tools = tools
	}

	return json.Marshal(body)
}

func (p *OllamaProvider) readNDJSONStream(ctx context.Context, r io.Reader, ch chan<- StreamChunk, model string) {
	scanner := bufio.NewScanner(r)
	var buffer strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var result ollamaCompletionResponse
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue
		}

		if result.Message.Content != "" {
			buffer.WriteString(result.Message.Content)
			ch <- StreamChunk{
				Content:  result.Message.Content,
				Provider: ProviderOllama,
				Model:    model,
			}
		}

		if result.Done {
			ch <- StreamChunk{
				Content:      buffer.String(),
				FinishReason: result.DoneReason,
				Done:         true,
				Provider:     ProviderOllama,
				Model:        model,
			}
			return
		}
	}

	if buffer.Len() > 0 {
		ch <- StreamChunk{
			Content:  buffer.String(),
			Done:     true,
			Provider: ProviderOllama,
			Model:    model,
		}
	}
}
