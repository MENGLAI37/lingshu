package agent

import (
	"context"
	"time"

	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Core Types for Agent Loop
// ===========================================================================

// LoopState represents the current state of the agent loop.
type LoopState string

const (
	StateThinking    LoopState = "thinking"
	StateToolCall    LoopState = "tool_call"
	StateExecuting   LoopState = "executing"
	StateObserving   LoopState = "observing"
	StateResponding  LoopState = "responding"
	StateCompleted   LoopState = "completed"
	StateError       LoopState = "error"
	StateTimeout     LoopState = "timeout"
	StateCancelled   LoopState = "cancelled"
)

// LoopPhase represents a phase in the agent loop cycle.
type LoopPhase string

const (
	PhaseThink      LoopPhase = "think"
	PhaseAct        LoopPhase = "act"
	PhaseObserve    LoopPhase = "observe"
	PhaseReflect    LoopPhase = "reflect"
)

// AgentLoop represents the core reasoning loop.
type AgentLoop struct {
	state          LoopState
	currentPhase   LoopPhase
	iterationCount int
	maxIterations  int
	startTime      time.Time
	toolResults    []ToolExecutionResult
	thinkingHistory []ThinkingStep
}

// ThinkingStep represents a single thinking step in the loop.
type ThinkingStep struct {
	Phase       LoopPhase
	Thought     string
	ToolCalls   []ParsedToolCall
	Observation string
	Timestamp   time.Time
}

// ToolExecutionResult represents the result of executing a tool.
type ToolExecutionResult struct {
	ToolName  string
	Arguments map[string]any
	Result    *tools.ToolResult
	Error     error
	Duration  time.Duration
	Timestamp time.Time
}

// LoopConfig holds configuration for the agent loop.
type LoopConfig struct {
	MaxIterations        int           // Maximum number of loop iterations (default: 10)
	GlobalTimeout        time.Duration // Global timeout for the entire loop (default: 5min)
	ToolTimeout          time.Duration // Timeout for individual tool calls (default: 30s)
	MaxTokens            int64         // Maximum token budget for context
	EnableParallelTools  bool          // Enable parallel tool execution
	MaxParallelTools     int           // Maximum number of parallel tool calls (default: 5)
}

// DefaultLoopConfig returns the default loop configuration.
func DefaultLoopConfig() LoopConfig {
	return LoopConfig{
		MaxIterations:       10,
		GlobalTimeout:       5 * time.Minute,
		ToolTimeout:         30 * time.Second,
		MaxTokens:           100000,
		EnableParallelTools: true,
		MaxParallelTools:    5,
	}
}

// LoopResult represents the final result of the agent loop.
type LoopResult struct {
	State           LoopState
	FinalResponse   string
	ToolResults     []ToolExecutionResult
	ThinkingHistory []ThinkingStep
	TotalIterations int
	TotalDuration   time.Duration
	TokenUsage      llm.TokenUsage
	Error           error
}

// LoopEvent represents an event emitted during loop execution.
type LoopEvent struct {
	Type      string      // "state_change", "tool_call", "tool_result", "thinking", "error"
	State     LoopState   // Current loop state
	Phase     LoopPhase   // Current phase
	Data      interface{} // Event-specific data
	Timestamp time.Time
}

// LoopEventHandler handles loop events.
type LoopEventHandler func(event LoopEvent)

// ===========================================================================
// Interfaces
// ===========================================================================

// ToolRegistry manages available tools.
type ToolRegistry interface {
	GetTool(name string) (tools.Tool, error)
	ListTools() []tools.Tool
	RegisterTool(tool tools.Tool) error
}

// SecurityGateway checks operation safety.
type SecurityGateway interface {
	EvaluateRisk(ctx context.Context, toolName string, args map[string]any) (RiskEvaluation, error)
	IsAllowed(ctx context.Context, evaluation RiskEvaluation) (bool, string)
}

// RiskEvaluation represents a risk assessment result.
type RiskEvaluation struct {
	RiskLevel     tools.ToolRiskLevel
	Score         int // 0-100, higher = more risky
	Reason        string
	AffectedResources []string
	EnvironmentWeight int // Additional weight based on environment (prod=+2, kube-system=+3)
}

// ContextManager manages conversation context.
type ContextManager interface {
	AddMessage(role llm.MessageRole, content string)
	AddToolResult(toolName string, result string)
	GetMessages() []llm.Message
	GetTokenCount() int64
	TrimContext(maxTokens int64) error
	Reset()
}

// AgentExecutor executes the agent loop.
type AgentExecutor interface {
	Execute(ctx context.Context, input string, handler LoopEventHandler) (*LoopResult, error)
	ExecuteWithTools(ctx context.Context, input string, tools []tools.Tool, handler LoopEventHandler) (*LoopResult, error)
}

// ===========================================================================
// Errors
// ===========================================================================

// LoopError represents an error from the agent loop.
type LoopError struct {
	Code    string
	Message string
	Phase   LoopPhase
	Cause   error
}

func (e *LoopError) Error() string {
	if e.Cause != nil {
		return formatLoopError(e.Code, e.Message, e.Phase, e.Cause)
	}
	return formatLoopErrorSimple(e.Code, e.Message, e.Phase)
}

func (e *LoopError) Unwrap() error {
	return e.Cause
}

func formatLoopError(code, message string, phase LoopPhase, cause error) string {
	return "agent loop error [" + code + "] in phase " + string(phase) + ": " + message + ": " + cause.Error()
}

func formatLoopErrorSimple(code, message string, phase LoopPhase) string {
	return "agent loop error [" + code + "] in phase " + string(phase) + ": " + message
}

// Common error codes.
const (
	ErrCodeMaxIterations   = "MAX_ITERATIONS"
	ErrCodeGlobalTimeout   = "GLOBAL_TIMEOUT"
	ErrCodeToolTimeout     = "TOOL_TIMEOUT"
	ErrCodeToolNotFound    = "TOOL_NOT_FOUND"
	ErrCodeToolExecution   = "TOOL_EXECUTION"
	ErrCodeLLMError        = "LLM_ERROR"
	ErrCodeSecurityBlocked = "SECURITY_BLOCKED"
	ErrCodeContextOverflow = "CONTEXT_OVERFLOW"
	ErrCodeInvalidInput    = "INVALID_INPUT"
	ErrCodeLoopCancelled   = "LOOP_CANCELLED"
)

// NewLoopError creates a new LoopError.
func NewLoopError(code, message string, phase LoopPhase, cause error) *LoopError {
	return &LoopError{Code: code, Message: message, Phase: phase, Cause: cause}
}