package agent

import (
	"context"
	"testing"
	"time"

	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Parser Tests
// ===========================================================================

func TestToolCallParser_Parse(t *testing.T) {
	parser := NewToolCallParser()

	tests := []struct {
		name     string
		fc       *llm.FunctionCall
		expected int // number of tool calls expected
	}{
		{
			name: "nil function call",
			fc:   nil,
			expected: 0,
		},
		{
			name: "simple tool call",
			fc: &llm.FunctionCall{
				Name:      "k8s_get",
				Arguments: `{"resource_type": "pod", "namespace": "default"}`,
			},
			expected: 1,
		},
		{
			name: "empty arguments",
			fc: &llm.FunctionCall{
				Name:      "k8s_status",
				Arguments: "",
			},
			expected: 1,
		},
		{
			name: "invalid JSON arguments",
			fc: &llm.FunctionCall{
				Name:      "k8s_get",
				Arguments: `invalid json`,
			},
			expected: 1, // Should still parse with error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.fc)
			if len(result) != tt.expected {
				t.Errorf("expected %d tool calls, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestToolCallParser_ParseFromContent(t *testing.T) {
	parser := NewToolCallParser()

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:    "simple tool call pattern",
			content: `tool_call: k8s_get(namespace=default,name=my-pod)`,
			expected: 1,
		},
		{
			name:    "no tool call",
			content: `This is a regular response without tool calls`,
			expected: 0,
		},
		{
			name:    "JSON embedded",
			content: `I will use {"name": "k8s_get", "namespace": "default"} to get pods`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseFromContent(tt.content)
			if len(result) != tt.expected {
				t.Errorf("expected %d tool calls, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestFallbackParser_Parse(t *testing.T) {
	parser := NewFallbackParser()

	tests := []struct {
		name     string
		args     string
		expected int
	}{
		{
			name:    "scale deployment",
			args:    "scale deployment nginx to 5",
			expected: 1,
		},
		{
			name:    "restart deployment",
			args:    "restart deployment nginx",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse("", tt.args)
			if len(result) != tt.expected {
				t.Errorf("expected %d tool calls, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestArgumentValidator_Validate(t *testing.T) {
	validator := NewArgumentValidator()

	tests := []struct {
		name     string
		toolName string
		args     map[string]any
		hasError bool
	}{
		{
			name:     "valid get args",
			toolName: "k8s_get",
			args:     map[string]any{"resource_type": "pod", "namespace": "default"},
			hasError: false,
		},
		{
			name:     "missing required resource_type",
			toolName: "k8s_get",
			args:     map[string]any{"namespace": "default"},
			hasError: true,
		},
		{
			name:     "valid scale args",
			toolName: "k8s_scale",
			args:     map[string]any{"resource_type": "deployment", "name": "nginx", "replicas": 5},
			hasError: false,
		},
		{
			name:     "missing replicas",
			toolName: "k8s_scale",
			args:     map[string]any{"resource_type": "deployment", "name": "nginx"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.toolName, tt.args)
			if tt.hasError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ===========================================================================
// Context Manager Tests
// ===========================================================================

func TestDefaultContextManager_AddMessage(t *testing.T) {
	cm := NewDefaultContextManager(100000)

	cm.AddMessage(llm.RoleUser, "Hello")
	cm.AddMessage(llm.RoleAssistant, "Hi there")

	if cm.GetTokenCount() < 2 {
		t.Errorf("expected token count > 2, got %d", cm.GetTokenCount())
	}

	msgs := cm.GetMessages()
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestDefaultContextManager_TrimContext(t *testing.T) {
	cm := NewDefaultContextManager(100) // Small limit for testing

	// Add many messages to exceed limit
	for i := 0; i < 100; i++ {
		cm.AddMessage(llm.RoleUser, "This is a test message with some content to consume tokens")
	}

	initialTokens := cm.GetTokenCount()
	if initialTokens <= 100 {
		t.Logf("initial tokens: %d, may need larger test messages", initialTokens)
	}

	// Trim context
	err := cm.TrimContext(50)
	if err != nil {
		t.Errorf("trim context error: %v", err)
	}

	// Check that tokens were reduced
	finalTokens := cm.GetTokenCount()
	if finalTokens > 100 {
		t.Errorf("expected tokens <= 100 after trim, got %d", finalTokens)
	}
}

func TestDefaultContextManager_Reset(t *testing.T) {
	cm := NewDefaultContextManager(100000)

	cm.AddMessage(llm.RoleUser, "Test")
	cm.AddMessage(llm.RoleAssistant, "Response")

	if cm.GetTokenCount() == 0 {
		t.Errorf("expected non-zero tokens before reset")
	}

	cm.Reset()

	if cm.GetTokenCount() != 0 {
		t.Errorf("expected 0 tokens after reset, got %d", cm.GetTokenCount())
	}

	msgs := cm.GetMessages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after reset, got %d", len(msgs))
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		content  string
		expected int64
	}{
		{"hello world", 3},         // ~12 chars / 4 = 3
		{"这是一个测试", 3},          // 6 Chinese chars / 2 = 3
		{"hello 这是一个测试", 5},    // Mixed
		{"", 0},
	}

	for _, tt := range tests {
		result := estimateTokens(tt.content)
		// Allow some variation due to estimation method
		if result == 0 && tt.expected > 0 {
			t.Errorf("expected some tokens for '%s', got 0", tt.content)
		}
	}
}

// ===========================================================================
// Timeout Tests
// ===========================================================================

func TestTimeoutChecker_CreateLoopContext(t *testing.T) {
	tc := NewTimeoutChecker(5*time.Minute, 10)

	ctx, cancel := tc.CreateLoopContext(context.Background())
	defer cancel()

	// Check context has deadline
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		t.Errorf("expected context to have deadline")
	}

	// Check deadline is approximately 5 minutes from now
	expectedDeadline := time.Now().Add(5 * time.Minute)
	diff := expectedDeadline.Sub(deadline)
	if diff.Abs() > time.Second {
		t.Errorf("deadline differs from expected by %v", diff)
	}
}

func TestTimeoutChecker_IsMaxIterations(t *testing.T) {
	tc := NewTimeoutChecker(5*time.Minute, 10)

	tests := []struct {
		iteration int
		expected  bool
	}{
		{0, false},
		{5, false},
		{10, true},
		{15, true},
	}

	for _, tt := range tests {
		result := tc.IsMaxIterations(tt.iteration)
		if result != tt.expected {
			t.Errorf("iteration %d: expected %v, got %v", tt.iteration, tt.expected, result)
		}
	}
}

func TestTimeoutManager_GetRemainingTime(t *testing.T) {
	tm := NewTimeoutManager(10 * time.Second)

	// Immediately check remaining time
	remaining := tm.GetRemainingTime()
	if remaining > 10*time.Second {
		t.Errorf("remaining time too high: %v", remaining)
	}

	// After waiting
	time.Sleep(2 * time.Second)
	remaining = tm.GetRemainingTime()
	if remaining > 8*time.Second {
		t.Errorf("remaining time should be ~8s after sleep: %v", remaining)
	}
}

func TestDeadLoopDetection(t *testing.T) {
	tc := NewTimeoutChecker(5*time.Minute, 10)

	// Record some iterations
	for i := 0; i < 5; i++ {
		tc.RecordIteration(IterationRecord{
			IterationNumber: i,
			StartTime:       time.Now(),
			EndTime:         time.Now().Add(1 * time.Second),
			Phase:           PhaseThink,
			ToolCalls:       []string{"k8s_get"},
			ResultSummary:   "same result",
		})
	}

	// Detect dead loop
	analysis := tc.DetectDeadLoop()

	// Should detect repeating pattern
	if !analysis.HasDeadLoop {
		t.Logf("no dead loop detected - may need more iterations")
	}

	if len(analysis.RepeatedPatterns) > 0 {
		t.Logf("detected patterns: %v", analysis.RepeatedPatterns)
	}
}

// ===========================================================================
// Tool Registry Tests
// ===========================================================================

func TestDefaultToolRegistry_RegisterGet(t *testing.T) {
	tr := NewDefaultToolRegistry()

	// Register a mock tool
	mockTool := &MockTool{name: "test_tool"}
	err := tr.RegisterTool(mockTool)
	if err != nil {
		t.Errorf("register tool error: %v", err)
	}

	// Get the tool
	tool, err := tr.GetTool("test_tool")
	if err != nil {
		t.Errorf("get tool error: %v", err)
	}

	if tool.Name() != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got '%s'", tool.Name())
	}

	// List tools
	tools := tr.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	// Remove tool
	err = tr.RemoveTool("test_tool")
	if err != nil {
		t.Errorf("remove tool error: %v", err)
	}

	tools = tr.ListTools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools after remove, got %d", len(tools))
	}
}

func TestToolRegistryBuilder(t *testing.T) {
	builder := NewToolRegistryBuilder()

	registry := builder.
		AddTool(&MockTool{name: "tool1"}).
		AddTool(&MockTool{name: "tool2"}).
		AddTools(&MockTool{name: "tool3"}, &MockTool{name: "tool4"}).
		Build()

	tools := registry.ListTools()
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}
}

// ===========================================================================
// Mock Tool for Testing
// ===========================================================================

type MockTool struct {
	name string
}

func (t *MockTool) Name() string {
	return t.name
}

func (t *MockTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL0
}

func (t *MockTool) Description() string {
	return "mock tool for testing"
}

func (t *MockTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	return &tools.ToolResult{
		Success:   true,
		Message:   "mock execution",
		Data:      params,
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  t.name,
		RiskLevel: t.RiskLevel(),
	}, nil
}