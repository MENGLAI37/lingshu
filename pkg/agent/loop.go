package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lingshu/lingshu/pkg/audit"
	"github.com/lingshu/lingshu/pkg/gitops"
	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/logger"
	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Core Agent Loop Implementation
// ===========================================================================

// RAGRetriever provides semantic search over runbooks and operational knowledge.
// Defined as interface to avoid circular imports with the rag package.
type RAGRetriever interface {
	Search(ctx context.Context, query string, collection string, topK int) ([]RAGDocument, error)
}

// RAGDocument represents a retrieved document from the knowledge base.
type RAGDocument struct {
	Content  string
	Score    float32
	Metadata map[string]string
}

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
	gitopsDetector  *gitops.Detector
	ragRetriever    RAGRetriever
	snapshotter     Snapshotter
	sessionMgr      SessionManager
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

// SetConfirmationHandler sets a custom confirmation handler on the loop config.
func (al *DefaultAgentLoop) SetConfirmationHandler(handler func(ConfirmationRequest) bool) {
	al.config.ConfirmationHandler = handler
}

// SetGitOpsDetector wires the GitOps detector for conflict warnings.
func (al *DefaultAgentLoop) SetGitOpsDetector(detector *gitops.Detector) {
	al.gitopsDetector = detector
}

// SetRAGRetriever wires a RAG retriever for runbook-augmented diagnosis.
func (al *DefaultAgentLoop) SetRAGRetriever(retriever RAGRetriever) {
	al.ragRetriever = retriever
}

// SetSnapshotter wires a snapshotter for pre-mutation snapshots and auto-rollback.
func (al *DefaultAgentLoop) SetSnapshotter(s Snapshotter) {
	al.snapshotter = s
}

// SetSessionManager wires a session manager for session lifecycle tracking.
func (al *DefaultAgentLoop) SetSessionManager(mgr SessionManager) {
	al.sessionMgr = mgr
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

	// Inject relevant runbook content via RAG (if available)
	if al.ragRetriever != nil {
		docs, err := al.ragRetriever.Search(ctx, input, "runbooks", 3)
		if err == nil && len(docs) > 0 {
			ragContext := "相关的运维知识库 (Runbook) 内容，供参考:\n\n"
			for i, doc := range docs {
				if doc.Content != "" {
					ragContext += fmt.Sprintf("## Runbook %d (相关度: %.0f%%)\n%s\n\n",
						i+1, doc.Score*100, doc.Content)
				}
			}
			ragContext += "---\n请结合以上 Runbook 知识，处理用户的问题。\n"
			al.contextManager.AddMessage(llm.RoleSystem, ragContext)
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

	// Session tracking: create session if manager is wired
	if al.sessionMgr != nil {
		ns, _ := ctx.Value("namespace").(string)
		cluster, _ := ctx.Value("cluster").(string)
		if sid, err := al.sessionMgr.Create(ctx, cluster, ns); err != nil {
			logger.Warn("Failed to create session", "error", err)
		} else {
			result.SessionID = sid
		}
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

		// Check context overflow: use current context token estimate
		// (the size of the next request), not cumulative usage.
		// Reserve room for the model's max output tokens.
		currentTokens := al.contextManager.GetTokenCount()
		estimatedNextRequest := currentTokens + resp.Usage.OutputTokens
		if estimatedNextRequest > al.config.MaxTokens {
			targetTokens := al.config.MaxTokens - resp.Usage.OutputTokens
			if targetTokens < al.config.MaxTokens/4 {
				targetTokens = al.config.MaxTokens / 4 // keep at least 25% of window
			}
			logger.Warn("Context window near limit, trimming",
				"current_tokens", currentTokens,
				"max_tokens", al.config.MaxTokens,
				"target_after_trim", targetTokens,
			)
			err := al.contextManager.TrimContext(targetTokens)
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

				// Session tracking: record tool call
				if al.sessionMgr != nil && result.SessionID != "" {
					riskLevel := "L0"
					if execResult.Result != nil {
						riskLevel = string(execResult.Result.RiskLevel)
					}
					if recErr := al.sessionMgr.AddToolCall(ctx, result.SessionID,
						execResult.ToolName, riskLevel, execResult.Arguments, resultStr); recErr != nil {
						logger.Warn("Failed to record tool call in session", "error", recErr)
					}
				}

		}

		// Record iteration for dead-loop detection
		toolNames := make([]string, len(execResults))
		for i, r := range execResults {
			toolNames[i] = r.ToolName
		}
		al.timeoutChecker.RecordIteration(IterationRecord{
			IterationNumber: state.iterationCount,
			StartTime:       time.Now(),
			EndTime:         time.Now(),
			Phase:           PhaseAct,
			ToolCalls:       toolNames,
			ResultSummary:   "iteration",
		})

		// Dead-loop detection every 3 iterations
		if state.iterationCount > 0 && state.iterationCount%3 == 0 {
			analysis := al.timeoutChecker.DetectDeadLoop()
			if analysis.HasDeadLoop {
				logger.Warn("Dead loop detected",
					"iteration", state.iterationCount,
					"suggestion", analysis.Suggestion,
				)
				al.emitEvent(handler, "thinking", state.state, state.currentPhase,
					"DEAD LOOP WARNING: "+analysis.Suggestion)
			}
		}

		// Increment iteration count
		state.iterationCount++
		result.TotalIterations = state.iterationCount
	}

	result.TotalDuration = time.Since(state.startTime)

	// Session tracking: record final costs and complete
	if al.sessionMgr != nil && result.SessionID != "" {
		if tcErr := al.sessionMgr.AddCost(ctx, result.SessionID,
			result.TokenUsage.InputTokens, result.TokenUsage.OutputTokens); tcErr != nil {
			logger.Warn("Failed to record session cost", "error", tcErr)
		}
		if compErr := al.sessionMgr.Complete(ctx, result.SessionID, result.FinalResponse); compErr != nil {
			logger.Warn("Failed to complete session", "error", compErr)
		}
	}

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
			// GitOps conflict detection for L2+ operations
			if al.gitopsDetector != nil && eval.RiskLevel != tools.RiskLevelL0 && eval.RiskLevel != tools.RiskLevelL1 {
				ns, _ := tc.Arguments["namespace"].(string)
				name, _ := tc.Arguments["name"].(string)
				rt, _ := tc.Arguments["resource_type"].(string)
				if ns != "" && name != "" && rt != "" {
					if own, err := al.gitopsDetector.DetectOwnership(ctx, ns, rt, name); err == nil && own.IsManaged {
						msg = msg + "\n\nGITOPS CONFLICT WARNING:\n" + own.GetWarningMessage()
					}
				}
			}

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

		// Take pre-mutation snapshot for L2+ operations
		tool, _ := al.toolRegistry.GetTool(tc.Name)
		var snapMeta *SnapshotMeta
		if al.config.AutoSnapshot && al.snapshotter != nil && tool != nil &&
			(tool.RiskLevel() == tools.RiskLevelL2 || tool.RiskLevel() == tools.RiskLevelL3) {
			ns, _ := tc.Arguments["namespace"].(string)
			name, _ := tc.Arguments["name"].(string)
			rt, _ := tc.Arguments["resource_type"].(string)
			if ns != "" && name != "" && rt != "" {
				snap, snapErr := al.snapshotter.Snapshot(ctx, rt, ns, name)
				if snapErr == nil {
					snapMeta = &snap
					logger.Info("Pre-mutation snapshot captured",
						"snapshot_id", snapMeta.ID,
						"resource", fmt.Sprintf("%s/%s/%s", ns, rt, name),
					)
				}
			}
		}

		result := al.executeSingleTool(ctx, tc)
		results = append(results, result)
		al.emitEvent(handler, "tool_result", state.state, state.currentPhase, result)

		// Auto-rollback: if L2+ operation failed with a snapshot available, restore
		if result.Error != nil && al.config.AutoRollback && snapMeta != nil {
			logger.Warn("L2+ operation failed, attempting auto-rollback",
				"tool", tc.Name,
				"snapshot_id", snapMeta.ID,
				"error", result.Error,
			)
			if rollbackErr := al.snapshotter.Restore(ctx, snapMeta.ID); rollbackErr != nil {
				logger.Error("Auto-rollback failed",
					"snapshot_id", snapMeta.ID,
					"error", rollbackErr,
				)
			} else {
				logger.Info("Auto-rollback succeeded via snapshot",
					"snapshot_id", snapMeta.ID,
				)
				al.emitEvent(handler, "thinking", state.state, state.currentPhase,
					fmt.Sprintf("操作失败，已自动回滚到快照 %s (资源: %s/%s/%s)",
						snapMeta.ID, snapMeta.Namespace, snapMeta.ResourceType, snapMeta.Name))
			}
		}
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