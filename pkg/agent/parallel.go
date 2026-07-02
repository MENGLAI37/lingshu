package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ===========================================================================
// Parallel Executor - 并行工具调用
// ===========================================================================

// ParallelExecutor executes multiple tools concurrently.
type ParallelExecutor struct {
	maxParallel  int
	toolTimeout  time.Duration
	errorHandler ErrorHandler
}

// ErrorHandler handles errors during parallel execution.
type ErrorHandler func(toolName string, err error) error

// NewParallelExecutor creates a new parallel executor.
func NewParallelExecutor(maxParallel int, toolTimeout time.Duration) *ParallelExecutor {
	if maxParallel <= 0 {
		maxParallel = 5
	}
	if toolTimeout <= 0 {
		toolTimeout = 30 * time.Second
	}

	return &ParallelExecutor{
		maxParallel:  maxParallel,
		toolTimeout:  toolTimeout,
		errorHandler: defaultErrorHandler,
	}
}

// SetErrorHandler sets a custom error handler.
func (pe *ParallelExecutor) SetErrorHandler(handler ErrorHandler) {
	pe.errorHandler = handler
}

// ExecuteParallel executes multiple tool calls concurrently.
func (pe *ParallelExecutor) ExecuteParallel(ctx context.Context, toolCalls []ParsedToolCall, registry ToolRegistry) []ToolExecutionResult {
	// Limit to maxParallel concurrent executions
	if len(toolCalls) > pe.maxParallel {
		// Execute in batches
		return pe.executeInBatches(ctx, toolCalls, registry)
	}

	return pe.executeConcurrent(ctx, toolCalls, registry)
}

// executeConcurrent executes all tool calls concurrently.
func (pe *ParallelExecutor) executeConcurrent(ctx context.Context, toolCalls []ParsedToolCall, registry ToolRegistry) []ToolExecutionResult {
	results := make([]ToolExecutionResult, len(toolCalls))
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, toolCall ParsedToolCall) {
			defer wg.Done()
			results[idx] = pe.executeSingle(ctx, toolCall, registry)
		}(i, tc)
	}

	wg.Wait()
	return results
}

// executeInBatches executes tool calls in batches to limit concurrency.
func (pe *ParallelExecutor) executeInBatches(ctx context.Context, toolCalls []ParsedToolCall, registry ToolRegistry) []ToolExecutionResult {
	results := []ToolExecutionResult{}
	batchSize := pe.maxParallel

	for i := 0; i < len(toolCalls); i += batchSize {
		end := i + batchSize
		if end > len(toolCalls) {
			end = len(toolCalls)
		}

		batch := toolCalls[i:end]
		batchResults := pe.executeConcurrent(ctx, batch, registry)
		results = append(results, batchResults...)

		// Check context cancellation between batches
		if ctx.Err() != nil {
			break
		}
	}

	return results
}

// executeSingle executes a single tool call with timeout.
func (pe *ParallelExecutor) executeSingle(ctx context.Context, tc ParsedToolCall, registry ToolRegistry) ToolExecutionResult {
	start := time.Now()

	// Get tool from registry
	tool, err := registry.GetTool(tc.Name)
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

	// Create tool context with timeout
	toolCtx, cancel := context.WithTimeout(ctx, pe.toolTimeout)
	defer cancel()

	// Execute tool
	result, err := tool.Execute(toolCtx, tc.Arguments)

	// Handle error if any
	if err != nil && pe.errorHandler != nil {
		err = pe.errorHandler(tc.Name, err)
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

// defaultErrorHandler is the default error handler.
func defaultErrorHandler(toolName string, err error) error {
	return fmt.Errorf("tool %s execution failed: %w", toolName, err)
}

// ===========================================================================
// Execution Strategy
// ===========================================================================

// ExecutionStrategy defines how tools should be executed.
type ExecutionStrategy string

const (
	StrategyParallel   ExecutionStrategy = "parallel"   // Execute all at once
	StrategySequential ExecutionStrategy = "sequential" // Execute one at a time
	StrategyBatch      ExecutionStrategy = "batch"      // Execute in batches
	StrategyDependent  ExecutionStrategy = "dependent"  // Execute respecting dependencies
)

// StrategyExecutor executes tools based on a strategy.
type StrategyExecutor struct {
	parallelExec  *ParallelExecutor
	toolTimeout   time.Duration
	registry      ToolRegistry
}

// NewStrategyExecutor creates a new strategy executor.
func NewStrategyExecutor(maxParallel int, toolTimeout time.Duration, registry ToolRegistry) *StrategyExecutor {
	return &StrategyExecutor{
		parallelExec: NewParallelExecutor(maxParallel, toolTimeout),
		toolTimeout:  toolTimeout,
		registry:     registry,
	}
}

// Execute executes tool calls with the specified strategy.
func (se *StrategyExecutor) Execute(ctx context.Context, strategy ExecutionStrategy, toolCalls []ParsedToolCall) []ToolExecutionResult {
	switch strategy {
	case StrategyParallel:
		return se.parallelExec.ExecuteParallel(ctx, toolCalls, se.registry)
	case StrategySequential:
		return se.executeSequential(ctx, toolCalls)
	case StrategyBatch:
		return se.parallelExec.executeInBatches(ctx, toolCalls, se.registry)
	case StrategyDependent:
		return se.executeDependent(ctx, toolCalls)
	default:
		return se.parallelExec.ExecuteParallel(ctx, toolCalls, se.registry)
	}
}

// executeSequential executes tool calls one at a time.
func (se *StrategyExecutor) executeSequential(ctx context.Context, toolCalls []ParsedToolCall) []ToolExecutionResult {
	results := []ToolExecutionResult{}

	for _, tc := range toolCalls {
		result := se.parallelExec.executeSingle(ctx, tc, se.registry)
		results = append(results, result)

		// Check context cancellation
		if ctx.Err() != nil {
			break
		}
	}

	return results
}

// executeDependent executes tools respecting dependencies.
func (se *StrategyExecutor) executeDependent(ctx context.Context, toolCalls []ParsedToolCall) []ToolExecutionResult {
	// Build dependency graph
	graph := buildDependencyGraph(toolCalls)
	results := []ToolExecutionResult{}

	// Execute in topological order
	ordered := topologicalSort(graph)

	for _, tc := range ordered {
		// Wait for dependencies if needed
		result := se.parallelExec.executeSingle(ctx, tc, se.registry)
		results = append(results, result)

		if ctx.Err() != nil {
			break
		}
	}

	return results
}

// ===========================================================================
// Dependency Graph
// ===========================================================================

// ToolDependency represents a dependency between tools.
type ToolDependency struct {
	Tool         ParsedToolCall
	DependsOn    []string // Tool names this tool depends on
	Dependencies []ToolDependency
}

// buildDependencyGraph builds a dependency graph from tool calls.
func buildDependencyGraph(toolCalls []ParsedToolCall) []ToolDependency {
	graph := []ToolDependency{}

	for _, tc := range toolCalls {
		deps := inferDependencies(tc, toolCalls)
		graph = append(graph, ToolDependency{
			Tool:      tc,
			DependsOn: deps,
		})
	}

	return graph
}

// inferDependencies infers dependencies for a tool call.
func inferDependencies(tc ParsedToolCall, allCalls []ParsedToolCall) []string {
	deps := []string{}

	// Infer dependencies based on common patterns
	// Example: k8s_describe might depend on k8s_get if getting the same resource
	// Example: k8s_scale might depend on k8s_status to know current state

	for _, other := range allCalls {
		if other.Name == tc.Name {
			continue // Skip self
		}

		// Check for inferred dependency
		if hasDependency(tc, other) {
			deps = append(deps, other.Name)
		}
	}

	return deps
}

// hasDependency checks if tc depends on other.
func hasDependency(tc, other ParsedToolCall) bool {
	// Pattern 1: Scale/Restart after Get/Describe
	if (tc.Name == "k8s_scale" || tc.Name == "k8s_restart" || tc.Name == "k8s_patch" || tc.Name == "k8s_rollout") &&
		(other.Name == "k8s_get" || other.Name == "k8s_describe" || other.Name == "k8s_status") {
		// Check if they refer to the same resource
		tcName, _ := tc.Arguments["name"].(string)
		otherName, _ := other.Arguments["name"].(string)
		if tcName == otherName && tcName != "" {
			return true
		}
	}

	// Pattern 2: Logs after Events
	if tc.Name == "k8s_logs" && other.Name == "k8s_events" {
		tcPod, _ := tc.Arguments["pod_name"].(string)
		otherPod, _ := other.Arguments["name"].(string)
		if tcPod == otherPod && tcPod != "" {
			return true
		}
	}

	return false
}

// topologicalSort sorts tools by dependencies.
func topologicalSort(graph []ToolDependency) []ParsedToolCall {
	// Simple topological sort using Kahn's algorithm
	// Build adjacency list
	adj := map[string][]string{}
	inDegree := map[string]int{}
	toolMap := map[string]ParsedToolCall{}

	for _, dep := range graph {
		toolMap[dep.Tool.Name] = dep.Tool
		inDegree[dep.Tool.Name] = len(dep.DependsOn)
		for _, dependsOn := range dep.DependsOn {
			adj[dependsOn] = append(adj[dependsOn], dep.Tool.Name)
		}
	}

	// Find all nodes with no incoming edges
	queue := []string{}
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	// Process queue
	result := []ParsedToolCall{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		result = append(result, toolMap[current])

		for _, neighbor := range adj[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	return result
}

// ===========================================================================
// Execution Result Aggregator
// ===========================================================================

// ResultAggregator aggregates results from multiple tool executions.
type ResultAggregator struct {
	results []ToolExecutionResult
	mu      sync.Mutex
}

// NewResultAggregator creates a new result aggregator.
func NewResultAggregator() *ResultAggregator {
	return &ResultAggregator{
		results: []ToolExecutionResult{},
	}
}

// AddResult adds a result to the aggregator.
func (ra *ResultAggregator) AddResult(result ToolExecutionResult) {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	ra.results = append(ra.results, result)
}

// GetResults returns all results.
func (ra *ResultAggregator) GetResults() []ToolExecutionResult {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	return ra.results
}

// GetSuccessful returns only successful results.
func (ra *ResultAggregator) GetSuccessful() []ToolExecutionResult {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	successful := []ToolExecutionResult{}
	for _, r := range ra.results {
		if r.Error == nil && r.Result != nil && r.Result.Success {
			successful = append(successful, r)
		}
	}
	return successful
}

// GetFailed returns only failed results.
func (ra *ResultAggregator) GetFailed() []ToolExecutionResult {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	failed := []ToolExecutionResult{}
	for _, r := range ra.results {
		if r.Error != nil || (r.Result != nil && !r.Result.Success) {
			failed = append(failed, r)
		}
	}
	return failed
}

// AggregateSummary returns a summary of all results.
func (ra *ResultAggregator) AggregateSummary() ExecutionSummary {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	total := len(ra.results)
	success := 0
	failed := 0
	totalDuration := time.Duration(0)

	for _, r := range ra.results {
		totalDuration += r.Duration
		if r.Error != nil || (r.Result != nil && !r.Result.Success) {
			failed++
		} else {
			success++
		}
	}

	return ExecutionSummary{
		TotalExecutions: total,
		Successful:      success,
		Failed:          failed,
		TotalDuration:   totalDuration,
		AverageDuration: totalDuration / time.Duration(total),
		SuccessRate:     float64(success) / float64(total) * 100,
	}
}

// ExecutionSummary provides a summary of execution results.
type ExecutionSummary struct {
	TotalExecutions int
	Successful      int
	Failed          int
	TotalDuration   time.Duration
	AverageDuration time.Duration
	SuccessRate     float64
}

// ===========================================================================
// Execution Monitor
// ===========================================================================

// ExecutionMonitor monitors parallel execution progress.
type ExecutionMonitor struct {
	aggregator   *ResultAggregator
	progress     map[string]ExecutionStatus
	startTime    time.Time
	mu           sync.RWMutex
	onProgress   func(toolName string, status ExecutionStatus)
}

// ExecutionStatus represents the status of a tool execution.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusTimeout   ExecutionStatus = "timeout"
)

// NewExecutionMonitor creates a new execution monitor.
func NewExecutionMonitor() *ExecutionMonitor {
	return &ExecutionMonitor{
		aggregator: NewResultAggregator(),
		progress:   map[string]ExecutionStatus{},
		startTime:  time.Now(),
	}
}

// SetProgressHandler sets a handler for progress updates.
func (em *ExecutionMonitor) SetProgressHandler(handler func(toolName string, status ExecutionStatus)) {
	em.onProgress = handler
}

// MarkPending marks a tool as pending.
func (em *ExecutionMonitor) MarkPending(toolName string) {
	em.mu.Lock()
	em.progress[toolName] = StatusPending
	em.mu.Unlock()

	em.notifyProgress(toolName, StatusPending)
}

// MarkRunning marks a tool as running.
func (em *ExecutionMonitor) MarkRunning(toolName string) {
	em.mu.Lock()
	em.progress[toolName] = StatusRunning
	em.mu.Unlock()

	em.notifyProgress(toolName, StatusRunning)
}

// MarkCompleted marks a tool as completed.
func (em *ExecutionMonitor) MarkCompleted(toolName string, result ToolExecutionResult) {
	em.mu.Lock()
	em.progress[toolName] = StatusCompleted
	em.aggregator.AddResult(result)
	em.mu.Unlock()

	em.notifyProgress(toolName, StatusCompleted)
}

// MarkFailed marks a tool as failed.
func (em *ExecutionMonitor) MarkFailed(toolName string, result ToolExecutionResult) {
	em.mu.Lock()
	em.progress[toolName] = StatusFailed
	em.aggregator.AddResult(result)
	em.mu.Unlock()

	em.notifyProgress(toolName, StatusFailed)
}

// GetProgress returns the current progress status.
func (em *ExecutionMonitor) GetProgress() map[string]ExecutionStatus {
	em.mu.RLock()
	defer em.mu.RUnlock()

	result := map[string]ExecutionStatus{}
	for k, v := range em.progress {
		result[k] = v
	}
	return result
}

// GetSummary returns the execution summary.
func (em *ExecutionMonitor) GetSummary() ExecutionSummary {
	return em.aggregator.AggregateSummary()
}

// notifyProgress notifies the progress handler.
func (em *ExecutionMonitor) notifyProgress(toolName string, status ExecutionStatus) {
	if em.onProgress != nil {
		em.onProgress(toolName, status)
	}
}

// WaitForCompletion waits for all executions to complete.
func (em *ExecutionMonitor) WaitForCompletion(ctx context.Context, expectedCount int) bool {
	for {
		em.mu.RLock()
		completed := len(em.progress)
		em.mu.RUnlock()

		if completed >= expectedCount {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}