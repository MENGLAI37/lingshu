package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ===========================================================================
// Timeout Checker - 全局超时 & 死循环检测
// ===========================================================================

// TimeoutChecker manages global timeout and iteration limits.
type TimeoutChecker struct {
	globalTimeout    time.Duration
	maxIterations    int
	loopStartTime    time.Time
	iterationHistory []IterationRecord
	mu               sync.Mutex
}

// IterationRecord records iteration details for loop detection.
type IterationRecord struct {
	IterationNumber int
	StartTime       time.Time
	EndTime         time.Time
	Phase           LoopPhase
	ToolCalls       []string
	ResultSummary   string
}

// NewTimeoutChecker creates a new timeout checker.
func NewTimeoutChecker(globalTimeout time.Duration, maxIterations int) *TimeoutChecker {
	if globalTimeout <= 0 {
		globalTimeout = 5 * time.Minute
	}
	if maxIterations <= 0 {
		maxIterations = 10
	}

	return &TimeoutChecker{
		globalTimeout:    globalTimeout,
		maxIterations:    maxIterations,
		iterationHistory: []IterationRecord{},
	}
}

// CreateLoopContext creates a context with global timeout.
func (tc *TimeoutChecker) CreateLoopContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	tc.mu.Lock()
	tc.loopStartTime = time.Now()
	tc.mu.Unlock()

	return context.WithTimeout(parentCtx, tc.globalTimeout)
}

// IsTimedOut checks if the global timeout has been exceeded.
func (tc *TimeoutChecker) IsTimedOut(startTime time.Time) bool {
	tc.mu.Lock()
	globalTimeout := tc.globalTimeout
	tc.mu.Unlock()

	return time.Since(startTime) > globalTimeout
}

// IsMaxIterations checks if the maximum iterations have been reached.
func (tc *TimeoutChecker) IsMaxIterations(currentIteration int) bool {
	tc.mu.Lock()
	maxIterations := tc.maxIterations
	tc.mu.Unlock()

	return currentIteration >= maxIterations
}

// RecordIteration records an iteration for loop detection analysis.
func (tc *TimeoutChecker) RecordIteration(record IterationRecord) {
	tc.mu.Lock()
	tc.iterationHistory = append(tc.iterationHistory, record)
	tc.mu.Unlock()
}

// GetIterationHistory returns the iteration history.
func (tc *TimeoutChecker) GetIterationHistory() []IterationRecord {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	result := []IterationRecord{}
	result = append(result, tc.iterationHistory...)
	return result
}

// DetectDeadLoop analyzes iteration history to detect potential dead loops.
func (tc *TimeoutChecker) DetectDeadLoop() DeadLoopAnalysis {
	tc.mu.Lock()
	history := tc.iterationHistory
	tc.mu.Unlock()

	analysis := DeadLoopAnalysis{
		HasDeadLoop:      false,
		LoopPattern:      "",
		RepeatedPatterns: []RepeatedPattern{},
		Suggestion:       "",
	}

	if len(history) < 3 {
		return analysis
	}

	// Detect repeating tool call patterns
	toolPatterns := analyzeToolPatterns(history)
	if len(toolPatterns) > 0 {
		analysis.HasDeadLoop = true
		analysis.RepeatedPatterns = toolPatterns
		analysis.Suggestion = "Detected repeated tool call pattern. Consider changing approach."
	}

	// Detect same error repeating
	errorPatterns := analyzeErrorPatterns(history)
	if len(errorPatterns) > 0 {
		analysis.HasDeadLoop = true
		analysis.RepeatedPatterns = append(analysis.RepeatedPatterns, errorPatterns...)
		analysis.Suggestion = "Detected repeated errors. Tool may be unavailable or arguments incorrect."
	}

	// Detect no progress (same state for multiple iterations)
	if detectNoProgress(history) {
		analysis.HasDeadLoop = true
		analysis.LoopPattern = "no_progress"
		analysis.Suggestion = "No progress detected. Agent may be stuck in same state."
	}

	return analysis
}

// analyzeToolPatterns analyzes repeating tool call patterns.
func analyzeToolPatterns(history []IterationRecord) []RepeatedPattern {
	patterns := []RepeatedPattern{}
	toolCallSequences := []string{}

	// Extract tool call sequences
	for _, record := range history {
		seq := formatToolSequence(record.ToolCalls)
		toolCallSequences = append(toolCallSequences, seq)
	}

	// Find repeating sequences
	for i := 0; i < len(toolCallSequences)-2; i++ {
		seq := toolCallSequences[i]
		repeatCount := 0

		for j := i + 1; j < len(toolCallSequences); j++ {
			if toolCallSequences[j] == seq && seq != "" {
				repeatCount++
			}
		}

		if repeatCount >= 2 {
			patterns = append(patterns, RepeatedPattern{
				Type:        "tool_sequence",
				Pattern:     seq,
				RepeatCount: repeatCount + 1,
				StartIndex:  i,
			})
		}
	}

	return patterns
}

// analyzeErrorPatterns analyzes repeating error patterns.
func analyzeErrorPatterns(history []IterationRecord) []RepeatedPattern {
	patterns := []RepeatedPattern{}

	// Track error summaries
	errorCounts := map[string]int{}
	for _, record := range history {
		if containsError(record.ResultSummary) {
			errorCounts[record.ResultSummary]++
		}
	}

	// Find repeating errors
	for errorMsg, count := range errorCounts {
		if count >= 2 {
			patterns = append(patterns, RepeatedPattern{
				Type:        "error",
				Pattern:     errorMsg,
				RepeatCount: count,
			})
		}
	}

	return patterns
}

// detectNoProgress detects if there is no progress across iterations.
func detectNoProgress(history []IterationRecord) bool {
	if len(history) < 3 {
		return false
	}

	// Compare recent iterations
	recent := history[len(history)-3:]
	sameState := true

	for i := 1; i < len(recent); i++ {
		if recent[i].ResultSummary != recent[i-1].ResultSummary {
			sameState = false
			break
		}
		if formatToolSequence(recent[i].ToolCalls) != formatToolSequence(recent[i-1].ToolCalls) {
			sameState = false
			break
		}
	}

	return sameState
}

// containsError checks if a string contains error indication.
func containsError(s string) bool {
	errorKeywords := []string{"error", "failed", "timeout", "unavailable", "not found"}
	for _, keyword := range errorKeywords {
		if contains(s, keyword) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func formatToolSequence(toolCalls []string) string {
	result := ""
	for i, call := range toolCalls {
		if i > 0 {
			result += ","
		}
		result += call
	}
	return result
}

// DeadLoopAnalysis represents analysis results for dead loop detection.
type DeadLoopAnalysis struct {
	HasDeadLoop      bool
	LoopPattern      string
	RepeatedPatterns []RepeatedPattern
	Suggestion       string
}

// RepeatedPattern represents a detected repeated pattern.
type RepeatedPattern struct {
	Type        string // "tool_sequence", "error", "phase"
	Pattern     string
	RepeatCount int
	StartIndex  int
}

// ===========================================================================
// Timeout Manager
// ===========================================================================

// TimeoutManager manages multiple timeout levels.
type TimeoutManager struct {
	globalTimeout  time.Duration
	phaseTimeouts  map[LoopPhase]time.Duration
	toolTimeout    time.Duration
	startTime      time.Time
	mu             sync.Mutex
}

// NewTimeoutManager creates a new timeout manager.
func NewTimeoutManager(globalTimeout time.Duration) *TimeoutManager {
	return &TimeoutManager{
		globalTimeout: globalTimeout,
		phaseTimeouts: map[LoopPhase]time.Duration{
			PhaseThink:   60 * time.Second,
			PhaseAct:     30 * time.Second,
			PhaseObserve: 10 * time.Second,
			PhaseReflect: 30 * time.Second,
		},
		toolTimeout: 30 * time.Second,
		startTime:   time.Now(),
	}
}

// SetPhaseTimeout sets timeout for a specific phase.
func (tm *TimeoutManager) SetPhaseTimeout(phase LoopPhase, timeout time.Duration) {
	tm.mu.Lock()
	tm.phaseTimeouts[phase] = timeout
	tm.mu.Unlock()
}

// SetToolTimeout sets the global tool timeout.
func (tm *TimeoutManager) SetToolTimeout(timeout time.Duration) {
	tm.mu.Lock()
	tm.toolTimeout = timeout
	tm.mu.Unlock()
}

// CreatePhaseContext creates a context with phase-specific timeout.
func (tm *TimeoutManager) CreatePhaseContext(parentCtx context.Context, phase LoopPhase) (context.Context, context.CancelFunc) {
	tm.mu.Lock()
	timeout, ok := tm.phaseTimeouts[phase]
	if !ok {
		timeout = 30 * time.Second
	}
	tm.mu.Unlock()

	return context.WithTimeout(parentCtx, timeout)
}

// CreateToolContext creates a context with tool timeout.
func (tm *TimeoutManager) CreateToolContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	tm.mu.Lock()
	timeout := tm.toolTimeout
	tm.mu.Unlock()

	return context.WithTimeout(parentCtx, timeout)
}

// GetRemainingTime returns the remaining time before global timeout.
func (tm *TimeoutManager) GetRemainingTime() time.Duration {
	tm.mu.Lock()
	startTime := tm.startTime
	globalTimeout := tm.globalTimeout
	tm.mu.Unlock()

	elapsed := time.Since(startTime)
	remaining := globalTimeout - elapsed

	if remaining < 0 {
		return 0
	}
	return remaining
}

// CheckTimeout checks if timeout has been exceeded and returns error if so.
func (tm *TimeoutManager) CheckTimeout(phase LoopPhase) error {
	remaining := tm.GetRemainingTime()
	if remaining <= 0 {
		return NewLoopError(ErrCodeGlobalTimeout, "global timeout exceeded", phase, nil)
	}

	tm.mu.Lock()
	phaseTimeout, ok := tm.phaseTimeouts[phase]
	if !ok {
		phaseTimeout = 30 * time.Second
	}
	tm.mu.Unlock()

	if remaining < phaseTimeout {
		return fmt.Errorf("insufficient time for phase %s: %v remaining, need %v",
			phase, remaining, phaseTimeout)
	}

	return nil
}

// Reset resets the timeout manager for a new execution.
func (tm *TimeoutManager) Reset() {
	tm.mu.Lock()
	tm.startTime = time.Now()
	tm.mu.Unlock()
}

// ===========================================================================
// Timeout Warning System
// ===========================================================================

// TimeoutWarning represents a timeout warning.
type TimeoutWarning struct {
	Level          WarningLevel
	RemainingTime  time.Duration
	CurrentPhase   LoopPhase
	SuggestedAction string
}

// WarningLevel represents the severity level of a warning.
type WarningLevel string

const (
	WarningInfo     WarningLevel = "info"
	WarningWarning  WarningLevel = "warning"
	WarningCritical WarningLevel = "critical"
)

// TimeoutWarner provides timeout warnings.
type TimeoutWarner struct {
	manager          *TimeoutManager
	warningThresholds map[WarningLevel]time.Duration
	onWarning        func(warning TimeoutWarning)
	mu               sync.Mutex
}

// NewTimeoutWarner creates a new timeout warner.
func NewTimeoutWarner(manager *TimeoutManager) *TimeoutWarner {
	return &TimeoutWarner{
		manager: manager,
		warningThresholds: map[WarningLevel]time.Duration{
			WarningInfo:     2 * time.Minute,
			WarningWarning:  1 * time.Minute,
			WarningCritical: 30 * time.Second,
		},
	}
}

// SetWarningThresholds sets custom warning thresholds.
func (tw *TimeoutWarner) SetWarningThresholds(thresholds map[WarningLevel]time.Duration) {
	tw.mu.Lock()
	tw.warningThresholds = thresholds
	tw.mu.Unlock()
}

// SetWarningHandler sets a handler for timeout warnings.
func (tw *TimeoutWarner) SetWarningHandler(handler func(warning TimeoutWarning)) {
	tw.mu.Lock()
	tw.onWarning = handler
	tw.mu.Unlock()
}

// CheckAndWarn checks remaining time and emits warnings if needed.
func (tw *TimeoutWarner) CheckAndWarn(phase LoopPhase) TimeoutWarning {
	remaining := tw.manager.GetRemainingTime()

	tw.mu.Lock()
	thresholds := tw.warningThresholds
	tw.mu.Unlock()

	var level WarningLevel
	var suggestedAction string

	if remaining <= thresholds[WarningCritical] {
		level = WarningCritical
		suggestedAction = "Immediately terminate current operation and return partial result"
	} else if remaining <= thresholds[WarningWarning] {
		level = WarningWarning
		suggestedAction = "Consider simplifying remaining operations"
	} else if remaining <= thresholds[WarningInfo] {
		level = WarningInfo
		suggestedAction = "Proceed with caution"
	} else {
		return TimeoutWarning{Level: WarningInfo, RemainingTime: remaining, CurrentPhase: phase}
	}

	warning := TimeoutWarning{
		Level:          level,
		RemainingTime:  remaining,
		CurrentPhase:   phase,
		SuggestedAction: suggestedAction,
	}

	tw.emitWarning(warning)

	return warning
}

// emitWarning emits a warning to the handler.
func (tw *TimeoutWarner) emitWarning(warning TimeoutWarning) {
	tw.mu.Lock()
	handler := tw.onWarning
	tw.mu.Unlock()

	if handler != nil {
		handler(warning)
	}
}

// ===========================================================================
// Timeout Metrics
// ===========================================================================

// TimeoutMetrics tracks timeout-related metrics.
type TimeoutMetrics struct {
	GlobalTimeoutCount   int64
	PhaseTimeoutCounts   map[LoopPhase]int64
	ToolTimeoutCount     int64
	TotalExecutions      int64
	TimeoutRate          float64
	AverageExecutionTime time.Duration
	MaxExecutionTime     time.Duration
	mu                   sync.Mutex
}

// NewTimeoutMetrics creates new timeout metrics tracker.
func NewTimeoutMetrics() *TimeoutMetrics {
	return &TimeoutMetrics{
		PhaseTimeoutCounts: map[LoopPhase]int64{},
	}
}

// RecordGlobalTimeout records a global timeout event.
func (tm *TimeoutMetrics) RecordGlobalTimeout() {
	tm.mu.Lock()
	tm.GlobalTimeoutCount++
	tm.TotalExecutions++
	tm.TimeoutRate = float64(tm.GlobalTimeoutCount + tm.ToolTimeoutCount) / float64(tm.TotalExecutions)
	tm.mu.Unlock()
}

// RecordPhaseTimeout records a phase-specific timeout event.
func (tm *TimeoutMetrics) RecordPhaseTimeout(phase LoopPhase) {
	tm.mu.Lock()
	tm.PhaseTimeoutCounts[phase]++
	tm.mu.Unlock()
}

// RecordToolTimeout records a tool timeout event.
func (tm *TimeoutMetrics) RecordToolTimeout() {
	tm.mu.Lock()
	tm.ToolTimeoutCount++
	tm.TotalExecutions++
	tm.TimeoutRate = float64(tm.GlobalTimeoutCount + tm.ToolTimeoutCount) / float64(tm.TotalExecutions)
	tm.mu.Unlock()
}

// RecordSuccessfulExecution records a successful execution.
func (tm *TimeoutMetrics) RecordSuccessfulExecution(duration time.Duration) {
	tm.mu.Lock()
	tm.TotalExecutions++
	tm.TimeoutRate = float64(tm.GlobalTimeoutCount + tm.ToolTimeoutCount) / float64(tm.TotalExecutions)
	if tm.TotalExecutions > 1 {
		tm.AverageExecutionTime = (tm.AverageExecutionTime*time.Duration(tm.TotalExecutions-1) + duration) / time.Duration(tm.TotalExecutions)
	} else {
		tm.AverageExecutionTime = duration
	}
	if duration > tm.MaxExecutionTime {
		tm.MaxExecutionTime = duration
	}
	tm.mu.Unlock()
}

// GetMetrics returns current metrics.
func (tm *TimeoutMetrics) GetMetrics() TimeoutMetricsSnapshot {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return TimeoutMetricsSnapshot{
		GlobalTimeoutCount:   tm.GlobalTimeoutCount,
		PhaseTimeoutCounts:   copyPhaseTimeoutCounts(tm.PhaseTimeoutCounts),
		ToolTimeoutCount:     tm.ToolTimeoutCount,
		TotalExecutions:      tm.TotalExecutions,
		TimeoutRate:          tm.TimeoutRate,
		AverageExecutionTime: tm.AverageExecutionTime,
		MaxExecutionTime:     tm.MaxExecutionTime,
	}
}

func copyPhaseTimeoutCounts(original map[LoopPhase]int64) map[LoopPhase]int64 {
	result := map[LoopPhase]int64{}
	for k, v := range original {
		result[k] = v
	}
	return result
}

// TimeoutMetricsSnapshot represents a snapshot of timeout metrics.
type TimeoutMetricsSnapshot struct {
	GlobalTimeoutCount   int64
	PhaseTimeoutCounts   map[LoopPhase]int64
	ToolTimeoutCount     int64
	TotalExecutions      int64
	TimeoutRate          float64
	AverageExecutionTime time.Duration
	MaxExecutionTime     time.Duration
}

// ===========================================================================
// Graceful Timeout Handling
// ===========================================================================

// GracefulTimeoutHandler handles timeouts gracefully.
type GracefulTimeoutHandler struct {
	manager          *TimeoutManager
	onGlobalTimeout  func() (string, error)
	onPhaseTimeout   func(phase LoopPhase) (string, error)
	onToolTimeout    func(toolName string) (string, error)
	mu               sync.Mutex
}

// NewGracefulTimeoutHandler creates a new graceful timeout handler.
func NewGracefulTimeoutHandler(manager *TimeoutManager) *GracefulTimeoutHandler {
	return &GracefulTimeoutHandler{
		manager: manager,
	}
}

// SetGlobalTimeoutHandler sets handler for global timeout.
func (gth *GracefulTimeoutHandler) SetGlobalTimeoutHandler(handler func() (string, error)) {
	gth.mu.Lock()
	gth.onGlobalTimeout = handler
	gth.mu.Unlock()
}

// SetPhaseTimeoutHandler sets handler for phase timeout.
func (gth *GracefulTimeoutHandler) SetPhaseTimeoutHandler(handler func(phase LoopPhase) (string, error)) {
	gth.mu.Lock()
	gth.onPhaseTimeout = handler
	gth.mu.Unlock()
}

// SetToolTimeoutHandler sets handler for tool timeout.
func (gth *GracefulTimeoutHandler) SetToolTimeoutHandler(handler func(toolName string) (string, error)) {
	gth.mu.Lock()
	gth.onToolTimeout = handler
	gth.mu.Unlock()
}

// HandleGlobalTimeout handles global timeout gracefully.
func (gth *GracefulTimeoutHandler) HandleGlobalTimeout() (string, error) {
	gth.mu.Lock()
	handler := gth.onGlobalTimeout
	gth.mu.Unlock()

	if handler != nil {
		return handler()
	}
	return "Execution timed out. Returning partial results.", nil
}

// HandlePhaseTimeout handles phase timeout gracefully.
func (gth *GracefulTimeoutHandler) HandlePhaseTimeout(phase LoopPhase) (string, error) {
	gth.mu.Lock()
	handler := gth.onPhaseTimeout
	gth.mu.Unlock()

	if handler != nil {
		return handler(phase)
	}
	return fmt.Sprintf("Phase %s timed out. Proceeding with available data.", phase), nil
}

// HandleToolTimeout handles tool timeout gracefully.
func (gth *GracefulTimeoutHandler) HandleToolTimeout(toolName string) (string, error) {
	gth.mu.Lock()
	handler := gth.onToolTimeout
	gth.mu.Unlock()

	if handler != nil {
		return handler(toolName)
	}
	return fmt.Sprintf("Tool %s timed out. Skipping and continuing.", toolName), nil
}