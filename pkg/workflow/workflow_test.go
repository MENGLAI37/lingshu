package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Mock Tool for Workflow Testing
// ===========================================================================

type mockWorkflowTool struct {
	name      string
	riskLevel tools.ToolRiskLevel
	execCount int
	lastArgs  map[string]any
}

func (t *mockWorkflowTool) Name() string                            { return t.name }
func (t *mockWorkflowTool) RiskLevel() tools.ToolRiskLevel           { return t.riskLevel }
func (t *mockWorkflowTool) Description() string                      { return "mock for testing" }
func (t *mockWorkflowTool) ParameterSchema() map[string]interface{}   { return map[string]interface{}{} }
func (t *mockWorkflowTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	t.execCount++
	t.lastArgs = params
	return &tools.ToolResult{
		Success:   true,
		Message:   "mock execution ok",
		Data:      params,
		Timestamp: time.Now(),
		Duration:  "1ms",
		ToolName:  t.name,
		RiskLevel: t.riskLevel,
	}, nil
}

// ===========================================================================
// Workflow Engine Tests
// ===========================================================================

func TestRegisterWorkFlow_Success(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	wf := &WorkFlow{
		ID:        "test-wf",
		Name:      "Test Workflow",
		StartStep: "step1",
		Steps: map[string]*Step{
			"step1": {ID: "step1", Actions: []Action{{Type: ActionExit}}},
		},
	}
	err := engine.RegisterWorkFlow(wf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	got, err := engine.GetWorkFlow("test-wf")
	if err != nil {
		t.Errorf("unexpected error getting workflow: %v", err)
	}
	if got.Name != "Test Workflow" {
		t.Errorf("expected name 'Test Workflow', got '%s'", got.Name)
	}
}

func TestRegisterWorkFlow_NilWorkflow(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	err := engine.RegisterWorkFlow(nil)
	if err == nil {
		t.Errorf("expected error for nil workflow")
	}
}

func TestRegisterWorkFlow_EmptyID(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	err := engine.RegisterWorkFlow(&WorkFlow{ID: ""})
	if err == nil {
		t.Errorf("expected error for empty ID")
	}
}

func TestRegisterWorkFlow_MissingStartStep(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	err := engine.RegisterWorkFlow(&WorkFlow{
		ID:        "bad-wf",
		StartStep: "nonexistent",
		Steps:     map[string]*Step{},
	})
	if err == nil {
		t.Errorf("expected error for missing start step")
	}
}

func TestGetWorkFlow_NotFound(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	_, err := engine.GetWorkFlow("nonexistent")
	if err == nil {
		t.Errorf("expected error for missing workflow")
	}
}

func TestListWorkFlows(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	_ = engine.RegisterWorkFlow(&WorkFlow{
		ID: "wf1", StartStep: "s1",
		Steps: map[string]*Step{"s1": {ID: "s1", Actions: []Action{{Type: ActionExit}}}},
	})
	_ = engine.RegisterWorkFlow(&WorkFlow{
		ID: "wf2", StartStep: "s2",
		Steps: map[string]*Step{"s2": {ID: "s2", Actions: []Action{{Type: ActionExit}}}},
	})

	list := engine.ListWorkFlows()
	if len(list) != 2 {
		t.Errorf("expected 2 workflows, got %d", len(list))
	}
}

// ===========================================================================
// Workflow Execution Tests
// ===========================================================================

func TestExecute_SimpleToolCall(t *testing.T) {
	mockTool := &mockWorkflowTool{name: "k8s_get", riskLevel: tools.RiskLevelL0}
	registry := newMockRegistryWith(mockTool)
	engine := NewDefaultWorkFlowEngine(registry)

	wf := &WorkFlow{
		ID:        "tool-wf",
		StartStep: "step1",
		Steps: map[string]*Step{
			"step1": {
				ID: "step1",
				Actions: []Action{
					{
						Type:     ActionToolCall,
						ToolName: "k8s_get",
						ToolParams: map[string]interface{}{
							"resource_type": "pod",
							"namespace":     "default",
						},
					},
					{Type: ActionExit},
				},
			},
		},
	}
	_ = engine.RegisterWorkFlow(wf)

	instance, err := engine.Execute(context.Background(), "tool-wf", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instance.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", instance.Status)
	}
	if mockTool.execCount != 1 {
		t.Errorf("expected 1 tool execution, got %d", mockTool.execCount)
	}
}

func TestExecute_WithVariables(t *testing.T) {
	mockTool := &mockWorkflowTool{name: "k8s_get", riskLevel: tools.RiskLevelL0}
	registry := newMockRegistryWith(mockTool)
	engine := NewDefaultWorkFlowEngine(registry)

	wf := &WorkFlow{
		ID:        "var-wf",
		StartStep: "step1",
		Variables: map[string]interface{}{"ns": "production"},
		Steps: map[string]*Step{
			"step1": {
				ID: "step1",
				Actions: []Action{
					{
						Type:     ActionToolCall,
						ToolName: "k8s_get",
						ToolParams: map[string]interface{}{
							"namespace": "{{ns}}",
						},
					},
					{Type: ActionExit},
				},
			},
		},
	}
	_ = engine.RegisterWorkFlow(wf)

	instance, err := engine.Execute(context.Background(), "var-wf", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instance.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", instance.Status)
	}
	if mockTool.lastArgs["namespace"] != "production" {
		t.Errorf("expected namespace 'production', got '%v'", mockTool.lastArgs["namespace"])
	}
}

func TestExecute_SetVariable(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	wf := &WorkFlow{
		ID:        "setvar-wf",
		StartStep: "step1",
		Steps: map[string]*Step{
			"step1": {
				ID: "step1",
				Actions: []Action{
					{
						Type:         ActionSetVariable,
						VariableName: "result",
						VariableValue: "success",
					},
					{Type: ActionExit},
				},
			},
		},
	}
	_ = engine.RegisterWorkFlow(wf)

	instance, err := engine.Execute(context.Background(), "setvar-wf", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instance.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", instance.Status)
	}
	if instance.Variables["result"] != "success" {
		t.Errorf("expected 'success', got '%v'", instance.Variables["result"])
	}
}

func TestExecute_CancelledContext(t *testing.T) {
	mockTool := &mockWorkflowTool{name: "slow_tool", riskLevel: tools.RiskLevelL0}
	registry := newMockRegistryWith(mockTool)
	engine := NewDefaultWorkFlowEngine(registry)

	wf := &WorkFlow{
		ID:        "cancel-wf",
		StartStep: "step1",
		Steps: map[string]*Step{
			"step1": {
				ID: "step1",
				Actions: []Action{
					{Type: ActionDelay, DelaySeconds: 10},
				},
			},
		},
	}
	_ = engine.RegisterWorkFlow(wf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	instance, _ := engine.Execute(ctx, "cancel-wf", nil, nil)
	if instance.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", instance.Status)
	}
}

// ===========================================================================
// Condition Evaluation Tests
// ===========================================================================

func TestEvaluateCondition_Always(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionAlways}
	if !engine.evaluateCondition(cond, nil) {
		t.Errorf("always condition should be true")
	}
}

func TestEvaluateCondition_Equals(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionEquals, Variable: "x", Expected: "hello"}
	vars := map[string]interface{}{"x": "hello"}

	if !engine.evaluateCondition(cond, vars) {
		t.Errorf("equals should be true")
	}

	vars["x"] = "world"
	if engine.evaluateCondition(cond, vars) {
		t.Errorf("equals should be false for different values")
	}
}

func TestEvaluateCondition_Contains(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionContains, Variable: "msg", Expected: "error"}
	vars := map[string]interface{}{"msg": "an error occurred"}

	if !engine.evaluateCondition(cond, vars) {
		t.Errorf("contains should be true")
	}
}

func TestEvaluateCondition_Greater(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionGreater, Variable: "count", Expected: int(5)}
	vars := map[string]interface{}{"count": 10}

	if !engine.evaluateCondition(cond, vars) {
		t.Errorf("10 > 5 should be true")
	}

	vars["count"] = 3
	if engine.evaluateCondition(cond, vars) {
		t.Errorf("3 > 5 should be false")
	}
}

func TestEvaluateCondition_Less(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionLess, Variable: "count", Expected: int(10)}
	vars := map[string]interface{}{"count": 5}

	if !engine.evaluateCondition(cond, vars) {
		t.Errorf("5 < 10 should be true")
	}
}

func TestEvaluateCondition_Matches(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionMatches, Variable: "text", Expected: `^\d+$`}
	vars := map[string]interface{}{"text": "12345"}

	if !engine.evaluateCondition(cond, vars) {
		t.Errorf("regex match should be true")
	}

	vars["text"] = "abc"
	if engine.evaluateCondition(cond, vars) {
		t.Errorf("regex match should be false for non-digits")
	}
}

func TestEvaluateCondition_Invert(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionEquals, Variable: "x", Expected: "hello", Invert: true}
	vars := map[string]interface{}{"x": "hello"}

	if engine.evaluateCondition(cond, vars) {
		t.Errorf("inverted equals should be false when matching")
	}

	vars["x"] = "world"
	if !engine.evaluateCondition(cond, vars) {
		t.Errorf("inverted equals should be true when not matching")
	}
}

func TestEvaluateCondition_MissingVariable(t *testing.T) {
	registry := newMockRegistry()
	engine := NewDefaultWorkFlowEngine(registry)

	cond := &Condition{Type: ConditionEquals, Variable: "missing", Expected: "any"}
	vars := map[string]interface{}{}

	if engine.evaluateCondition(cond, vars) {
		t.Errorf("missing variable without invert should be false")
	}
}

// ===========================================================================
// Helpers
// ===========================================================================

func newMockRegistry() *mockToolRegistry {
	return &mockToolRegistry{tools: map[string]tools.Tool{}}
}

func newMockRegistryWith(t tools.Tool) *mockToolRegistry {
	r := newMockRegistry()
	r.tools[t.Name()] = t
	return r
}

type mockToolRegistry struct {
	tools map[string]tools.Tool
}

func (r *mockToolRegistry) GetTool(name string) (tools.Tool, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, &toolNotFoundError{name: name}
	}
	return t, nil
}

func (r *mockToolRegistry) ListTools() []tools.Tool {
	result := make([]tools.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *mockToolRegistry) RegisterTool(t tools.Tool) error {
	r.tools[t.Name()] = t
	return nil
}

type toolNotFoundError struct{ name string }

func (e *toolNotFoundError) Error() string { return "tool not found: " + e.name }
