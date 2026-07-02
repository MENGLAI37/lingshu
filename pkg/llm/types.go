package llm

import (
	"context"
	"fmt"
	"time"
)

// ===========================================================================
// Core Types
// ===========================================================================

// ProviderType identifies the LLM provider.
type ProviderType string

const (
	ProviderOpenAI  ProviderType = "openai"
	ProviderClaude  ProviderType = "claude"
	ProviderOllama  ProviderType = "ollama"
)

// MessageRole represents the role of a message in the conversation.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Message represents a single message in the conversation.
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool call requested by the LLM (OpenAI tool calling protocol).
type ToolCall struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function FunctionCall  `json:"function"`
}

// FunctionCall represents a function call requested by the LLM.
type FunctionCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// CompletionResponse is the unified response from any LLM provider.
type CompletionResponse struct {
	Content      string         `json:"content"`
	FunctionCall *FunctionCall  `json:"function_call,omitempty"`
	ToolCalls    []ToolCall     `json:"tool_calls,omitempty"`
	FinishReason string         `json:"finish_reason"`
	Usage        TokenUsage     `json:"usage"`
	Provider     ProviderType   `json:"provider"`
	Model        string         `json:"model"`
	Latency      time.Duration  `json:"latency"`
}

// TokenUsage tracks input and output token counts.
type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	Content      string        `json:"content"`
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
	Usage        *TokenUsage   `json:"usage,omitempty"`
	Provider     ProviderType  `json:"provider"`
	Model        string        `json:"model"`
	Done         bool          `json:"done"`
}

// CompletionRequest is the unified request for any LLM provider.
type CompletionRequest struct {
	Messages     []Message
	Model        string
	Temperature  float64
	MaxTokens    int
	Functions    []FunctionDefinition
	Stream       bool
	SystemPrompt string
}

// FunctionDefinition defines a callable function for the LLM.
type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ProviderConfig holds configuration for an LLM provider.
type ProviderConfig struct {
	Name       string       `json:"name"`
	Model      string       `json:"model"`
	APIKey     string       `json:"api_key"`
	BaseURL    string       `json:"base_url"`
	Priority   int          `json:"priority"`
	Timeout    time.Duration `json:"timeout"`
	IsLocal    bool         `json:"is_local"`
	MaxRetries int          `json:"max_retries"`
}

// ===========================================================================
// Interfaces
// ===========================================================================

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	Type() ProviderType
	Name() string
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
	HealthCheck(ctx context.Context) error
}

// MetricsCollector tracks LLM usage metrics.
type MetricsCollector interface {
	RecordUsage(ctx context.Context, provider ProviderType, model string, usage TokenUsage, latency time.Duration, err error)
}

// PromptTemplateEngine manages prompt templates.
type PromptTemplateEngine interface {
	Render(name string, vars map[string]string) (string, error)
	Register(name, template string) error
	GetVersion(name string) string
}

// ===========================================================================
// Errors
// ===========================================================================

// LLMError represents an error from the LLM layer.
type LLMError struct {
	Code    string
	Message string
	Cause   error
}

func (e *LLMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("llm error [%s]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("llm error [%s]: %s", e.Code, e.Message)
}

func (e *LLMError) Unwrap() error {
	return e.Cause
}

// Common error codes.
const (
	ErrCodeProviderUnavailable = "PROVIDER_UNAVAILABLE"
	ErrCodeRateLimited         = "RATE_LIMITED"
	ErrCodeInvalidRequest      = "INVALID_REQUEST"
	ErrCodeStreamingFailed     = "STREAMING_FAILED"
	ErrCodeNoProviderAvailable = "NO_PROVIDER_AVAILABLE"
	ErrCodeTimeout             = "TIMEOUT"
	ErrCodeCancelled           = "CANCELLED"
)

// NewError creates a new LLMError.
func NewError(code, message string, cause error) *LLMError {
	return &LLMError{Code: code, Message: message, Cause: cause}
}
