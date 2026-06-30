package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Types Tests
// ===========================================================================

func TestNewError(t *testing.T) {
	err := NewError(ErrCodeProviderUnavailable, "test message", nil)
	assert.Equal(t, ErrCodeProviderUnavailable, err.Code)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, "llm error [PROVIDER_UNAVAILABLE]: test message", err.Error())

	inner := fmt.Errorf("inner cause")
	err2 := NewError(ErrCodeRateLimited, "rate limited", inner)
	assert.ErrorIs(t, err2, inner)
	assert.Contains(t, err2.Error(), "rate limited")
}

func TestTokenUsage(t *testing.T) {
	u := TokenUsage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30}
	assert.Equal(t, int64(10), u.InputTokens)
	assert.Equal(t, int64(20), u.OutputTokens)
	assert.Equal(t, int64(30), u.TotalTokens)
}

// ===========================================================================
// OpenAI Provider Tests
// ===========================================================================

func TestOpenAIProvider_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var req openAICompletionRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "gpt-4o", req.Model)
		assert.False(t, req.Stream)

		resp := openAICompletionResponse{
			ID:    "test-id",
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIMessage{
						Role:    "assistant",
						Content: "Hello from OpenAI",
					},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		Name:    "openai",
		Model:   "gpt-4o",
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
		Model: "gpt-4o",
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello from OpenAI", resp.Content)
	assert.Equal(t, ProviderOpenAI, resp.Provider)
	assert.Equal(t, "gpt-4o", resp.Model)
	assert.Equal(t, int64(10), resp.Usage.InputTokens)
	assert.Equal(t, int64(5), resp.Usage.OutputTokens)
	assert.Equal(t, "stop", resp.FinishReason)
}

func TestOpenAIProvider_Complete_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		Name:    "openai",
		APIKey:  "bad-key",
		BaseURL: server.URL + "/v1",
	})

	_, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	require.Error(t, err)
	llmErr, ok := err.(*LLMError)
	require.True(t, ok)
	assert.Equal(t, ErrCodeProviderUnavailable, llmErr.Code)
}

func TestOpenAIProvider_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"id":"1","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":""}]}` + "\n\n",
			`data: {"id":"1","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":""}]}` + "\n\n",
			`data: {"id":"1","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, c := range chunks {
			_, _ = w.Write([]byte(c))
		}
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		Name:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	ch, err := provider.Stream(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
		Model:    "gpt-4o",
	})
	require.NoError(t, err)

	var content strings.Builder
	for chunk := range ch {
		if chunk.Content != "" && !chunk.Done {
			content.WriteString(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}

	assert.Equal(t, "Hello world", content.String())
}

func TestOpenAIProvider_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		Name:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	err := provider.HealthCheck(context.Background())
	require.NoError(t, err)
}

func TestOpenAIProvider_FunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAICompletionResponse{
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role: "assistant",
						ToolCalls: []openAIToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: openAIFunction{
									Name:      "get_pods",
									Arguments: `{"namespace": "default"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		Name:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "get pods"}},
		Functions: []FunctionDefinition{
			{Name: "get_pods", Description: "get pods", Parameters: map[string]interface{}{}},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp.FunctionCall)
	assert.Equal(t, "get_pods", resp.FunctionCall.Name)
	assert.Equal(t, `{"namespace": "default"}`, resp.FunctionCall.Arguments)
}

// ===========================================================================
// Claude Provider Tests
// ===========================================================================

func TestClaudeProvider_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))

		resp := claudeCompletionResponse{
			ID:    "msg_01",
			Model: "claude-3-opus",
			Content: []claudeContentBlock{
				{Type: "text", Text: "Hello from Claude"},
			},
			StopReason: "end_turn",
			Usage: claudeUsage{
				InputTokens:  20,
				OutputTokens: 10,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewClaudeProvider(ProviderConfig{
		Name:    "claude",
		Model:   "claude-3-opus",
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
		Model:    "claude-3-opus",
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello from Claude", resp.Content)
	assert.Equal(t, ProviderClaude, resp.Provider)
	assert.Equal(t, int64(20), resp.Usage.InputTokens)
	assert.Equal(t, int64(10), resp.Usage.OutputTokens)
}

func TestClaudeProvider_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}` + "\n\n",
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" there"}}` + "\n\n",
			`data: {"type":"message_stop"}` + "\n\n",
		}
		for _, c := range chunks {
			_, _ = w.Write([]byte(c))
		}
	}))
	defer server.Close()

	provider := NewClaudeProvider(ProviderConfig{
		Name:    "claude",
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	ch, err := provider.Stream(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
		Model:    "claude-3-opus",
	})
	require.NoError(t, err)

	var content strings.Builder
	for chunk := range ch {
		if chunk.Content != "" && !chunk.Done {
			content.WriteString(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}

	assert.Equal(t, "Hi there", content.String())
}

func TestClaudeProvider_ToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := claudeCompletionResponse{
			Model: "claude-3-opus",
			Content: []claudeContentBlock{
				{Type: "tool_use", Name: "scale_deployment", Input: json.RawMessage(`{"name":"nginx","replicas":5}`)},
			},
			StopReason: "tool_use",
			Usage:      claudeUsage{InputTokens: 50, OutputTokens: 25},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewClaudeProvider(ProviderConfig{
		Name:    "claude",
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "scale nginx to 5"}},
	})

	require.NoError(t, err)
	require.NotNil(t, resp.FunctionCall)
	assert.Equal(t, "scale_deployment", resp.FunctionCall.Name)
	assert.Equal(t, `{"name":"nginx","replicas":5}`, resp.FunctionCall.Arguments)
}

// ===========================================================================
// Ollama Provider Tests
// ===========================================================================

func TestOllamaProvider_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)

		var req ollamaCompletionRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "qwen2.5-coder:7b", req.Model)
		assert.False(t, req.Stream)

		resp := ollamaCompletionResponse{
			Model: "qwen2.5-coder:7b",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "Hello from Ollama",
			},
			Done:            true,
			DoneReason:      "stop",
			PromptEvalCount: 15,
			EvalCount:       8,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(ProviderConfig{
		Name:    "ollama",
		Model:   "qwen2.5-coder:7b",
		BaseURL: server.URL,
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
		Model:    "qwen2.5-coder:7b",
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello from Ollama", resp.Content)
	assert.Equal(t, ProviderOllama, resp.Provider)
	assert.Equal(t, int64(15), resp.Usage.InputTokens)
	assert.Equal(t, int64(8), resp.Usage.OutputTokens)
}

func TestOllamaProvider_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		lines := []string{
			`{"model":"qwen2.5-coder:7b","message":{"role":"assistant","content":"Hi"},"done":false}` + "\n",
			`{"model":"qwen2.5-coder:7b","message":{"role":"assistant","content":"!"},"done":true,"done_reason":"stop"}` + "\n",
		}
		for _, line := range lines {
			_, _ = w.Write([]byte(line))
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(ProviderConfig{
		Name:    "ollama",
		BaseURL: server.URL,
	})

	ch, err := provider.Stream(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)

	var content strings.Builder
	for chunk := range ch {
		if chunk.Content != "" && !chunk.Done {
			content.WriteString(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}

	assert.Equal(t, "Hi!", content.String())
}

func TestOllamaProvider_DetectLocal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewOllamaProvider(ProviderConfig{
		BaseURL: server.URL,
	})

	ok := provider.DetectLocal(context.Background())
	assert.True(t, ok)
}

// ===========================================================================
// Router Tests
// ===========================================================================

func TestRouter_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAICompletionResponse{
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Message:      openAIMessage{Role: "assistant", Content: "Routed!"},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	router := NewRouter([]ProviderConfig{
		{Name: "openai", APIKey: "test", BaseURL: server.URL + "/v1", Priority: 1},
	})

	resp, err := router.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "Routed!", resp.Content)
}

func TestRouter_Complete_Failover(t *testing.T) {
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failServer.Close()

	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := claudeCompletionResponse{
			ID:    "msg_01",
			Model: "claude-3-opus",
			Content: []claudeContentBlock{
				{Type: "text", Text: "Fallback!"},
			},
			StopReason: "end_turn",
			Usage:      claudeUsage{InputTokens: 5, OutputTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer okServer.Close()

	router := NewRouter([]ProviderConfig{
		{Name: "openai", APIKey: "test", BaseURL: failServer.URL + "/v1", Priority: 1},
		{Name: "claude", APIKey: "test", BaseURL: okServer.URL + "/v1", Priority: 2},
	})

	resp, err := router.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "Fallback!", resp.Content)
}

func TestRouter_Complete_NoProviders(t *testing.T) {
	router := NewRouter([]ProviderConfig{})
	_, err := router.Complete(context.Background(), &CompletionRequest{})
	require.Error(t, err)
	assert.Equal(t, ErrCodeNoProviderAvailable, err.(*LLMError).Code)
}

func TestRouter_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	router := NewRouter([]ProviderConfig{
		{Name: "openai", APIKey: "test", BaseURL: server.URL + "/v1"},
	})

	results := router.HealthCheck(context.Background())
	assert.Len(t, results, 1)
	assert.NoError(t, results["openai"])
}

func TestRouter_UpdateProviders(t *testing.T) {
	router := NewRouter([]ProviderConfig{
		{Name: "openai", APIKey: "test", Priority: 1},
	})

	assert.Len(t, router.GetProviders(), 1)

	router.UpdateProviders([]ProviderConfig{
		{Name: "claude", APIKey: "test", Priority: 1},
		{Name: "ollama", APIKey: "", IsLocal: true, Priority: 2},
	})

	providers := router.GetProviders()
	assert.Len(t, providers, 2)
}

func TestCreateRouterFromConfig_Error(t *testing.T) {
	_, err := CreateRouterFromConfig([]ProviderConfig{})
	require.Error(t, err)
}

// ===========================================================================
// Prompt Engine Tests
// ===========================================================================

func TestPromptEngine_RegisterAndRender(t *testing.T) {
	engine := NewPromptEngine()

	err := engine.Register("test", "Hello {{name}}!")
	require.NoError(t, err)

	result, err := engine.Render("test", map[string]string{"name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", result)
}

func TestPromptEngine_VersionBump(t *testing.T) {
	engine := NewPromptEngine()

	_ = engine.Register("test", "Hello {{name}}!")
	v1 := engine.GetVersion("test")
	assert.Equal(t, "v1.0.0", v1)

	_ = engine.Register("test", "Hi {{name}}!")
	v2 := engine.GetVersion("test")
	assert.Equal(t, "v1.0.1", v2)

	// Same template, version unchanged
	_ = engine.Register("test", "Hi {{name}}!")
	v3 := engine.GetVersion("test")
	assert.Equal(t, "v1.0.1", v3)
}

func TestPromptEngine_Render_NotFound(t *testing.T) {
	engine := NewPromptEngine()
	_, err := engine.Render("missing", nil)
	require.Error(t, err)
}

func TestPromptEngine_GetTemplate(t *testing.T) {
	engine := NewPromptEngine()
	_ = engine.Register("test", "Hello {{name}}!")

	tpl, err := engine.GetTemplate("test")
	require.NoError(t, err)
	assert.Equal(t, "test", tpl.Name)
	assert.Equal(t, "Hello {{name}}!", tpl.Template)
	assert.Contains(t, tpl.Vars, "name")
}

func TestPromptEngine_ListTemplates(t *testing.T) {
	engine := NewPromptEngine()
	_ = engine.Register("a", "A")
	_ = engine.Register("b", "B")

	names := engine.ListTemplates()
	assert.Len(t, names, 2)
}

func TestPromptEngine_RegisterBuiltinTemplates(t *testing.T) {
	engine := NewPromptEngine()
	engine.RegisterBuiltinTemplates()

	names := engine.ListTemplates()
	assert.GreaterOrEqual(t, len(names), 4)

	result, err := engine.Render("diagnose", map[string]string{
		"cluster":   "prod",
		"namespace": "default",
		"issue":     "pod crash",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "prod")
	assert.Contains(t, result, "pod crash")
}

func TestPromptEngine_Register_EmptyName(t *testing.T) {
	engine := NewPromptEngine()
	err := engine.Register("", "test")
	require.Error(t, err)
}

// ===========================================================================
// Metrics Tests
// ===========================================================================

func TestNoopMetricsCollector(t *testing.T) {
	m := NewNoopMetricsCollector()
	// Should not panic
	m.RecordUsage(context.Background(), ProviderOpenAI, "gpt-4o", TokenUsage{InputTokens: 10, OutputTokens: 5}, time.Second, nil)
}

func TestDefaultMetricsCollector_RecordUsage(t *testing.T) {
	m := NewMetricsCollector(nil)
	defer func() { _ = m.Close() }()

	ctx := context.WithValue(context.Background(), "session_id", "sess-123")
	ctx = context.WithValue(ctx, "user_id", "user-456")

	m.RecordUsage(ctx, ProviderOpenAI, "gpt-4o", TokenUsage{InputTokens: 1000, OutputTokens: 500}, time.Second, nil)

	// Since db is nil, flush should return error but not panic
	err := m.Flush()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database not available")
}

func TestDefaultMetricsCollector_Flush_Empty(t *testing.T) {
	m := NewMetricsCollector(nil)
	defer func() { _ = m.Close() }()

	err := m.Flush()
	require.NoError(t, err)
}

func TestDefaultMetricsCollector_GetUsageStats_NoDB(t *testing.T) {
	m := NewMetricsCollector(nil)
	defer func() { _ = m.Close() }()

	_, err := m.GetUsageStats(context.Background(), "sess-123")
	require.Error(t, err)
}

// ===========================================================================
// Provider Interface Compliance
// ===========================================================================

func TestProviderInterface(t *testing.T) {
	// Ensure all providers implement the interface
	var _ Provider = (*OpenAIProvider)(nil)
	var _ Provider = (*ClaudeProvider)(nil)
	var _ Provider = (*OllamaProvider)(nil)
}

// ===========================================================================
// Integration-style test with mock server
// ===========================================================================

func TestRouter_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"streamed\"},\"finish_reason\":\"\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	router := NewRouter([]ProviderConfig{
		{Name: "openai", APIKey: "test", BaseURL: server.URL + "/v1", Priority: 1},
	})

	ch, err := router.Stream(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)

	var done bool
	for chunk := range ch {
		if chunk.Done {
			done = true
			break
		}
	}
	assert.True(t, done)
}

func TestRouter_Stream_AllFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	router := NewRouter([]ProviderConfig{
		{Name: "openai", APIKey: "test", BaseURL: server.URL + "/v1"},
	})

	_, err := router.Stream(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.Error(t, err)
	assert.Equal(t, ErrCodeNoProviderAvailable, err.(*LLMError).Code)
}
