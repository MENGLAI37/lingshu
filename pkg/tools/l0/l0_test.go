package l0

import (
	"context"
	"testing"

	toolspkg "github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// L0 Tool Interface Tests — all tools must be L0
// ===========================================================================

func TestGetTool_Interface(t *testing.T) {
	tool := NewGetTool(nil)

	if tool.Name() != "k8s_get" {
		t.Errorf("expected name 'k8s_get', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL0 {
		t.Errorf("expected L0, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	schema := tool.ParameterSchema()
	if _, ok := schema["resource_type"]; !ok {
		t.Errorf("expected 'resource_type' in parameter schema")
	}
	if _, ok := schema["namespace"]; !ok {
		t.Errorf("expected 'namespace' in parameter schema")
	}
}

func TestDescribeTool_Interface(t *testing.T) {
	tool := NewDescribeTool(nil)

	if tool.Name() != "k8s_describe" {
		t.Errorf("expected name 'k8s_describe', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL0 {
		t.Errorf("expected L0, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

func TestLogsTool_Interface(t *testing.T) {
	tool := NewLogsTool(nil)

	if tool.Name() != "k8s_logs" {
		t.Errorf("expected name 'k8s_logs', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL0 {
		t.Errorf("expected L0, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

func TestEventsTool_Interface(t *testing.T) {
	tool := NewEventsTool(nil)

	if tool.Name() != "k8s_events" {
		t.Errorf("expected name 'k8s_events', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL0 {
		t.Errorf("expected L0, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

// ===========================================================================
// GetTool Execute Tests (nil client — error handling only)
// ===========================================================================

func TestGetTool_Execute_MissingResourceType(t *testing.T) {
	tool := NewGetTool(nil)

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Errorf("expected error for missing resource_type")
	}
	if result != nil && result.Success {
		t.Errorf("expected unsuccessful result for missing resource_type")
	}
}

func TestGetTool_Execute_UnsupportedResourceType(t *testing.T) {
	tool := NewGetTool(nil)

	result, err := tool.Execute(context.Background(), map[string]any{
		"resource_type": "nonexistent",
	})
	if err == nil {
		t.Errorf("expected error for unsupported resource_type")
	}
	if result != nil && result.Success {
		t.Errorf("expected unsuccessful result")
	}
}


// ===========================================================================
// Parameter Schema Completeness
// ===========================================================================

func TestAllL0Tools_ParameterSchemas(t *testing.T) {
	allTools := []toolspkg.Tool{
		NewGetTool(nil),
		NewDescribeTool(nil),
		NewLogsTool(nil),
		NewEventsTool(nil),
	}

	for _, tool := range allTools {
		t.Run(tool.Name(), func(t *testing.T) {
			schema := tool.ParameterSchema()
			if schema == nil {
				t.Errorf("%s: expected non-nil parameter schema", tool.Name())
			}
		})
	}
}

func TestAllL0Tools_RiskLevelIsL0(t *testing.T) {
	allTools := []toolspkg.Tool{
		NewGetTool(nil),
		NewDescribeTool(nil),
		NewLogsTool(nil),
		NewEventsTool(nil),
	}

	for _, tool := range allTools {
		t.Run(tool.Name(), func(t *testing.T) {
			if tool.RiskLevel() != toolspkg.RiskLevelL0 {
				t.Errorf("%s: expected L0, got %s", tool.Name(), tool.RiskLevel())
			}
		})
	}
}
