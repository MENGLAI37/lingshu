package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lingshu/lingshu/pkg/audit"
	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/logger"
	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Core Agent Loop Implementation
// ===========================================================================

// DefaultAgentLoop implements the core reasoning loop.
type DefaultAgentLoop struct {
	config          LoopConfig
	llmRouter       *llm.Router
	promptEngine    *llm.DefaultPromptEngine
	toolRegistry    ToolRegistry
	securityGateway SecurityGateway
	contextManager  ContextManager
	parser          *ToolCallParser
	parallelExec    *ParallelExecutor
	timeoutChecker  *TimeoutChecker
	circuitBreaker  *CircuitBreaker
	auditMgr        *audit.Manager
	mu              sync.Mutex //nolint:unused
}

// NewDefaultAgentLoop creates a new agent loop instance.
func NewDefaultAgentLoop(
	config LoopConfig,
	llmRouter *llm.Router,
	toolRegistry ToolRegistry,
	securityGateway SecurityGateway,
) *DefaultAgentLoop {
	if config.MaxIterations == 0 {
		config = DefaultLoopConfig()
	}

	promptEngine := llm.NewPromptEngine()
	promptEngine.RegisterBuiltinTemplates()
	_ = promptEngine.Register("agent_system", agentSystemPrompt)

	return &DefaultAgentLoop{
		config:          config,
		llmRouter:       llmRouter,
		promptEngine:    promptEngine,
		toolRegistry:    toolRegistry,
		securityGateway: securityGateway,
		contextManager:  NewDefaultContextManager(config.MaxTokens),
		parser:          NewToolCallParser(),
		parallelExec:    NewParallelExecutor(config.MaxParallelTools, config.ToolTimeout),
		timeoutChecker:  NewTimeoutChecker(config.GlobalTimeout, config.MaxIterations),
		circuitBreaker:  NewCircuitBreaker(config.MaxL2Operations, config.MaxConsecutiveWrite),
	}
}

// SetAuditManager wires the audit manager for tool call logging.
func (al *DefaultAgentLoop) SetAuditManager(mgr *audit.Manager) {
	al.auditMgr = mgr
}

// Execute runs the agent loop with the given input.
func (al *DefaultAgentLoop) Execute(ctx context.Context, input string, handler LoopEventHandler) (*LoopResult, error) {
	return al.ExecuteWithTools(ctx, input, nil, handler)
}

// ExecuteWithTools runs the agent loop with optional additional tools.
func (al *DefaultAgentLoop) ExecuteWithTools(ctx context.Context, input string, extraTools []tools.Tool, handler LoopEventHandler) (*LoopResult, error) {
	// Panic recovery: catch panics and build partial result
	var panicErr error
	defer func() {
		if r := recover(); r != nil {
			panicErr = fmt.Errorf("agent loop panic recovered: %v", r)
			logger.Error("Agent loop panic", "error", panicErr, "input", input)
			al.emitEvent(handler, "error", StateError, PhaseThink, panicErr)
		}
	}()

	// Reset context for new execution
	al.contextManager.Reset()

	// Register extra tools if provided
	if extraTools != nil {
		for _, tool := range extraTools {
			_ = al.toolRegistry.RegisterTool(tool)
		}
	}

	// Create loop context with timeout
	loopCtx, cancel := al.timeoutChecker.CreateLoopContext(ctx)
	defer cancel()

	// Initialize loop state
	state := &LoopStateTracker{
		state:          StateThinking,
		currentPhase:   PhaseThink,
		iterationCount: 0,
		startTime:      time.Now(),
	}

	// Add initial user message
	al.contextManager.AddMessage(llm.RoleUser, input)

	// Build initial tool definitions
	toolDefs := al.buildToolDefinitions()

	result := &LoopResult{
		ToolResults:     []ToolExecutionResult{},
		ThinkingHistory: []ThinkingStep{},
	}

	// Main agent loop
	for {
		// Check for cancellation
		if loopCtx.Err() != nil {
			result.State = StateCancelled
			result.Error = NewLoopError(ErrCodeLoopCancelled, "loop cancelled", state.currentPhase, loopCtx.Err())
			al.emitEvent(handler, "error", state.state, state.currentPhase, result.Error)
			return result, result.Error
		}

		// Check for timeout
		if al.timeoutChecker.IsTimedOut(state.startTime) {
			result.State = StateTimeout
			result.Error = NewLoopError(ErrCodeGlobalTimeout, "global timeout exceeded", state.currentPhase, nil)
			al.emitEvent(handler, "error", state.state, state.currentPhase, result.Error)
			return result, result.Error
		}

		// Check for max iterations
		if al.timeoutChecker.IsMaxIterations(state.iterationCount) {
			result.State = StateCompleted
			result.FinalResponse = "Reached maximum iterations. Task may not be fully completed."
			result.TotalIterations = state.iterationCount
			al.emitEvent(handler, "state_change", StateCompleted, state.currentPhase, nil)
			break
		}

		// Phase: Think
		state.setState(StateThinking)
		state.setPhase(PhaseThink)
		al.emitEvent(handler, "state_change", state.state, state.currentPhase, nil)

		// Build LLM request
		req := al.buildCompletionRequest(toolDefs)

		// Call LLM
		resp, err := al.llmRouter.Complete(loopCtx, req)
		if err != nil {
			result.State = StateError
			result.Error = NewLoopError(ErrCodeLLMError, "LLM completion failed", PhaseThink, err)
			al.emitEvent(handler, "error", state.state, state.currentPhase, result.Error)
			return result, result.Error
		}

		// Track token usage
		result.TokenUsage.InputTokens += resp.Usage.InputTokens
		result.TokenUsage.OutputTokens += resp.Usage.OutputTokens
		result.TokenUsage.TotalTokens += resp.Usage.TotalTokens

		// Check context overflow
		if result.TokenUsage.TotalTokens > al.config.MaxTokens {
			err := al.contextManager.TrimContext(al.config.MaxTokens - resp.Usage.OutputTokens)
			if err != nil {
				result.State = StateError
				result.Error = NewLoopError(ErrCodeContextOverflow, "context overflow", PhaseThink, err)
				return result, result.Error
			}
		}

		// Parse response
		thought := resp.Content

		// Parse tool calls from ToolCalls (standard) or FunctionCall (legacy)
		var toolCalls []ParsedToolCall
		if len(resp.ToolCalls) > 0 {
			toolCalls = al.parser.ParseToolCalls(resp.ToolCalls)
		} else if resp.FunctionCall != nil {
			toolCalls = al.parser.Parse(resp.FunctionCall)
		}

		// Generate synthetic IDs for fallback-parsed tool calls (no ID from LLM)
		for i := range toolCalls {
			if toolCalls[i].ToolCallID == "" {
				toolCalls[i].ToolCallID = generateToolCallID()
			}
		}

		// Record thinking step
		thinkingStep := ThinkingStep{
			Phase:     PhaseThink,
			Thought:   thought,
			ToolCalls: toolCalls,
			Timestamp: time.Now(),
		}
		result.ThinkingHistory = append(result.ThinkingHistory, thinkingStep)

		// If no tool calls, we're done
		// Do NOT emit thinking event here; the final response will be sent
		// by the caller (runAgentLoop) to avoid duplication.
		if len(toolCalls) == 0 {
			state.setState(StateResponding)
			al.emitEvent(handler, "state_change", state.state, state.currentPhase, nil)

			al.contextManager.AddMessage(llm.RoleAssistant, thought)
			result.State = StateCompleted
			result.FinalResponse = thought
			result.TotalIterations = state.iterationCount
			break
		}

		// Emit thinking event for intermediate analysis (only when there are tool calls)
		al.emitEvent(handler, "thinking", state.state, state.currentPhase, thought)

		// Add assistant message with tool calls to context (required by OpenAI tool calling protocol)
		assistantToolCalls := make([]llm.ToolCall, len(toolCalls))
		for i, tc := range toolCalls {
			argsJSON, _ := json.Marshal(tc.Arguments)
			assistantToolCalls[i] = llm.ToolCall{
				ID:   tc.ToolCallID,
				Type: "function",
				Function: llm.Function{
					Name:      tc.Name,
					Arguments: string(argsJSON),
				},
			}
		}
		al.contextManager.AddAssistantWithToolCalls(thought, assistantToolCalls, resp.ReasoningContent)

		// Phase: Act (Execute tools)
		state.setState(StateExecuting)
		state.setPhase(PhaseAct)
		al.emitEvent(handler, "state_change", state.state, state.currentPhase, toolCalls)

		// Execute tools
		execResults := al.executeTools(loopCtx, toolCalls, handler, state)
		result.ToolResults = append(result.ToolResults, execResults...)

		// Phase: Observe
		state.setState(StateObserving)
		state.setPhase(PhaseObserve)
		al.emitEvent(handler, "state_change", state.state, state.currentPhase, nil)

		// Add tool results to context with proper tool_call_id
		for _, execResult := range execResults {
			resultStr := al.formatToolResult(execResult)
			al.contextManager.AddToolResult(execResult.ToolName, resultStr, execResult.ToolCallID)
		}

		// Increment iteration count
		state.iterationCount++
		result.TotalIterations = state.iterationCount
	}

	result.TotalDuration = time.Since(state.startTime)

	if panicErr != nil {
		result.State = StateError
		result.Error = panicErr
		return result, panicErr
	}
	return result, nil
}

type ConfirmationEventData struct {
	ToolName      string
	Arguments     map[string]any
	Message       string
	RiskLevel     tools.ToolRiskLevel
	Token         string
	Confirmed     bool
}

// executeTools executes the parsed tool calls.
func (al *DefaultAgentLoop) executeTools(ctx context.Context, toolCalls []ParsedToolCall, handler LoopEventHandler, state *LoopStateTracker) []ToolExecutionResult {
	results := []ToolExecutionResult{}

	for _, tc := range toolCalls {
		// Circuit breaker check: prevent runaway L2+ operations
		if al.circuitBreaker != nil {
			tool, _ := al.toolRegistry.GetTool(tc.Name)
			isWrite := tool != nil && tool.RiskLevel() != tools.RiskLevelL0
			isL2Plus := tool != nil && (tool.RiskLevel() == tools.RiskLevelL2 ||
				tool.RiskLevel() == tools.RiskLevelL3 ||
				tool.RiskLevel() == tools.RiskLevelL4)

			if err := al.circuitBreaker.Allow(isWrite, isL2Plus); err != nil {
				results = append(results, ToolExecutionResult{
					ToolName:   tc.Name,
					Arguments:  tc.Arguments,
					ToolCallID: tc.ToolCallID,
					Error:      fmt.Errorf("circuit breaker: %w", err),
					Timestamp:  time.Now(),
				})
				al.emitEvent(handler, "error", state.state, state.currentPhase, err)
				continue
			}
		}

		if al.securityGateway != nil {
			eval, err := al.securityGateway.EvaluateRisk(ctx, tc.Name, tc.Arguments)
			if err != nil {
				results = append(results, ToolExecutionResult{
					ToolName:  tc.Name,
					Arguments: tc.Arguments,
					Error:     fmt.Errorf("security evaluation failed: %w", err),
					Timestamp: time.Now(),
				})
				continue
			}

			allowed, reason := al.securityGateway.IsAllowed(ctx, eval)
			if !allowed {
				results = append(results, ToolExecutionResult{
					ToolName:   tc.Name,
					Arguments:  tc.Arguments,
					ToolCallID: tc.ToolCallID,
					Error:      fmt.Errorf("security blocked: %s", reason),
					Timestamp:  time.Now(),
				})
				al.emitEvent(handler, "error", state.state, state.currentPhase, fmt.Errorf("security blocked: %s", reason))
				continue
			}

			if al.securityGateway.RequiresConfirmation(ctx, eval) {
				msg := al.securityGateway.GetConfirmationMessage(ctx, eval)
				impactSummary := buildImpactSummary(eval)
				req := ConfirmationRequest{
					ToolName:          tc.Name,
					Arguments:         tc.Arguments,
					Message:           msg,
					RiskLevel:         eval.ToolRiskLevel,
					Token:             tc.ToolCallID,
					AffectedResources: eval.AffectedResources,
					ImpactSummary:     impactSummary,
				}

				al.emitEvent(handler, "confirmation_request", state.state, state.currentPhase, req)

				var confirmed bool
				if al.config.ConfirmationHandler != nil {
					confirmed = al.config.ConfirmationHandler(req)
				}

				al.emitEvent(handler, "confirmation_response", state.state, state.currentPhase, ConfirmationEventData{
					ToolName:  tc.Name,
					Arguments: tc.Arguments,
					Message:   msg,
					RiskLevel: eval.ToolRiskLevel,
					Token:     tc.ToolCallID,
					Confirmed: confirmed,
				})

				if !confirmed {
					results = append(results, ToolExecutionResult{
						ToolName:   tc.Name,
						Arguments:  tc.Arguments,
						ToolCallID: tc.ToolCallID,
						Error:      fmt.Errorf("operation cancelled by user"),
						Timestamp:  time.Now(),
					})
					continue
				}
			}
		}

		result := al.executeSingleTool(ctx, tc)
		results = append(results, result)
		al.emitEvent(handler, "tool_result", state.state, state.currentPhase, result)
	}

	return results
}

// executeSingleTool executes a single tool call.
func (al *DefaultAgentLoop) executeSingleTool(ctx context.Context, tc ParsedToolCall) ToolExecutionResult {
	start := time.Now()

	tool, err := al.toolRegistry.GetTool(tc.Name)
	if err != nil {
		return ToolExecutionResult{
			ToolName:   tc.Name,
			Arguments:  tc.Arguments,
			ToolCallID: tc.ToolCallID,
			Error:      fmt.Errorf("tool not found: %w", err),
			Duration:   time.Since(start),
			Timestamp:  start,
		}
	}

	// Dry-run: skip L1+ write operations, return preview result
	if al.config.DryRun && tool.RiskLevel() != tools.RiskLevelL0 {
		return ToolExecutionResult{
			ToolName:   tc.Name,
			Arguments:  tc.Arguments,
			ToolCallID: tc.ToolCallID,
			Result: &tools.ToolResult{
				Success:   true,
				Message:   fmt.Sprintf("[DRY RUN] Would execute %s with args %v (risk: %s)", tc.Name, tc.Arguments, tool.RiskLevel()),
				Timestamp: start,
				ToolName:  tc.Name,
				RiskLevel: tool.RiskLevel(),
			},
			Duration:  time.Since(start),
			Timestamp: start,
		}
	}

	// Create tool context with timeout
	toolCtx, cancel := context.WithTimeout(ctx, al.config.ToolTimeout)
	defer cancel()

	result, err := tool.Execute(toolCtx, tc.Arguments)

	// Audit log: record every tool execution (L1+)
	if al.auditMgr != nil && tool.RiskLevel() != tools.RiskLevelL0 {
		namespace, _ := tc.Arguments["namespace"].(string)
		toolName := tc.Name
		riskLevel := audit.RiskLevel(string(tool.RiskLevel()))
		auditResult := map[string]interface{}{"success": err == nil}
		if result != nil {
			auditResult["message"] = result.Message
		}
		if err != nil {
			auditResult["error"] = err.Error()
		}
		_ = al.auditMgr.Log(ctx, &audit.CreateAuditEventRequest{
			Action:    audit.ActionToolCall,
			ToolName:  &toolName,
			RiskLevel: riskLevel,
			Cluster:   getCtxString(ctx, "cluster"),
			Namespace: namespace,
			Result:    auditResult,
		})
	}

	return ToolExecutionResult{
		ToolName:   tc.Name,
		Arguments:  tc.Arguments,
		ToolCallID: tc.ToolCallID,
		Result:     result,
		Error:      err,
		Duration:   time.Since(start),
		Timestamp:  start,
	}
}

// buildCompletionRequest creates an LLM completion request.
func (al *DefaultAgentLoop) buildCompletionRequest(toolDefs []llm.FunctionDefinition) *llm.CompletionRequest {
	systemPrompt, _ := al.promptEngine.Render("agent_system", map[string]string{
		"tools": formatToolDefinitions(toolDefs),
	})

	return &llm.CompletionRequest{
		Messages:     al.contextManager.GetMessages(),
		Model:        "", // Use default from router
		Temperature:  0.7,
		MaxTokens:    4000,
		Functions:    toolDefs,
		ToolChoice:   "auto",
		Stream:       false,
		SystemPrompt: systemPrompt,
	}
}

// buildToolDefinitions builds function definitions from registered tools.
func (al *DefaultAgentLoop) buildToolDefinitions() []llm.FunctionDefinition {
	tools := al.toolRegistry.ListTools()
	defs := make([]llm.FunctionDefinition, len(tools))

	for i, tool := range tools {
		defs[i] = llm.FunctionDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": buildToolParameters(tool),
				"required":   getRequiredParameters(tool),
			},
		}
	}

	return defs
}

// formatToolResult formats a tool execution result for context.
func (al *DefaultAgentLoop) formatToolResult(result ToolExecutionResult) string {
	if result.Error != nil {
		return fmt.Sprintf("Tool %s failed: %s", result.ToolName, result.Error.Error())
	}

	if result.Result == nil {
		return fmt.Sprintf("Tool %s executed successfully (no result)", result.ToolName)
	}

	// Format based on result type
	if result.Result.Data != nil {
		dataJSON, err := json.Marshal(result.Result.Data)
		if err != nil {
			return fmt.Sprintf("Tool %s: %s", result.ToolName, result.Result.Message)
		}
		return fmt.Sprintf("Tool %s result: %s", result.ToolName, string(dataJSON))
	}

	return fmt.Sprintf("Tool %s: %s", result.ToolName, result.Result.Message)
}

// emitEvent emits a loop event to the handler.
func (al *DefaultAgentLoop) emitEvent(handler LoopEventHandler, eventType string, state LoopState, phase LoopPhase, data interface{}) {
	if handler != nil {
		handler(LoopEvent{
			Type:      eventType,
			State:     state,
			Phase:     phase,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}

// generateToolCallID generates a synthetic tool call ID for fallback-parsed tool calls.
func generateToolCallID() string {
	b := make([]byte, 4)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()>>uint(i*8)&0xf]
	}
	return fmt.Sprintf("call_%s", string(b))
}

// ===========================================================================
// Loop State Tracker
// ===========================================================================

// LoopStateTracker tracks the current state of the loop.
type LoopStateTracker struct {
	state          LoopState
	currentPhase   LoopPhase
	iterationCount int
	startTime      time.Time
	mu             sync.Mutex
}

func (t *LoopStateTracker) setState(state LoopState) {
	t.mu.Lock()
	t.state = state
	t.mu.Unlock()
}

func (t *LoopStateTracker) setPhase(phase LoopPhase) {
	t.mu.Lock()
	t.currentPhase = phase
	t.mu.Unlock()
}

func (t *LoopStateTracker) GetState() LoopState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

func (t *LoopStateTracker) GetPhase() LoopPhase {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentPhase
}

func (t *LoopStateTracker) GetIterationCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.iterationCount
}

// ===========================================================================
// Helper Functions
// ===========================================================================

func formatToolDefinitions(defs []llm.FunctionDefinition) string {
	result := ""
	for _, def := range defs {
		result += fmt.Sprintf("- %s: %s\n", def.Name, def.Description)
	}
	return result
}

func buildToolParameters(tool tools.Tool) map[string]interface{} {
	return tool.ParameterSchema()
}

func getRequiredParameters(tool tools.Tool) []string {
	// Basic required parameters based on tool risk level
	switch tool.RiskLevel() {
	case tools.RiskLevelL0:
		return []string{"namespace"}
	case tools.RiskLevelL1, tools.RiskLevelL2:
		return []string{"namespace", "name"}
	case tools.RiskLevelL3, tools.RiskLevelL4:
		return []string{"namespace", "name", "confirm"}
	default:
		return []string{}
	}
}

// buildImpactSummary builds a human-readable impact summary from a risk evaluation.
// getCtxString extracts a string value from context.
func getCtxString(ctx context.Context, key string) string {
	if v := ctx.Value(key); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func buildImpactSummary(eval RiskEvaluation) string {
	summary := "Risk Level: " + string(eval.RiskLevel)
	if eval.Score > 0 {
		summary += fmt.Sprintf(" (Score: %d)", eval.Score)
	}
	if eval.Reason != "" {
		summary += "\nReason: " + eval.Reason
	}
	if len(eval.AffectedResources) > 0 {
		summary += "\nAffected Resources:"
		for _, r := range eval.AffectedResources {
			summary += "\n  - " + r
		}
	}
	return summary
}

// Agent system prompt template
const agentSystemPrompt = `You are a Kubernetes SRE assistant with access to tools for cluster operations.

CRITICAL RULES:
1. You MUST use the available tools to execute operations. NEVER output kubectl commands for the user to run manually.
2. ALWAYS call tools to gather real cluster data before making conclusions.
3. Do NOT ask the user to execute commands. You have direct access to the cluster via tools.
4. For diagnosis tasks, proactively use read-only tools (k8s_get, k8s_describe, k8s_logs, k8s_events) to investigate.
5. After gathering data with tools, analyze the results and provide a clear diagnosis.

Available tools:
{{tools}}

Guidelines:
- Always verify the current state before making changes
- Use read-only tools (L0) to gather information first
- For risky operations (L2+), explain the impact before executing
- If you encounter errors, try alternative approaches
- Keep responses concise and focused on the user's request
- NEVER say "please run this command" or "execute the following". Use tools directly.`