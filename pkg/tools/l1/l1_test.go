package l1

import (
	"testing"

	toolspkg "github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// L1 Tool Interface Tests
// ===========================================================================

func TestTopTool_Interface(t *testing.T) {
	tool := NewTopTool(nil, nil)

	if tool.Name() != "k8s_top" {
		t.Errorf("expected name 'k8s_top', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL1 {
		t.Errorf("expected L1, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

func TestStatusTool_Interface(t *testing.T) {
	tool := NewStatusTool(nil)

	if tool.Name() != "k8s_status" {
		t.Errorf("expected name 'k8s_status', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL1 {
		t.Errorf("expected L1, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

// ===========================================================================
// Parameter Schema Tests
// ===========================================================================

func TestAllL1Tools_ParameterSchemas(t *testing.T) {
	allTools := []toolspkg.Tool{
		NewTopTool(nil, nil),
		NewStatusTool(nil),
	}

	for _, tool := range allTools {
		t.Run(tool.Name(), func(t *testing.T) {
			if tool.ParameterSchema() == nil {
				t.Errorf("%s: expected non-nil parameter schema", tool.Name())
			}
		})
	}
}

func TestAllL1Tools_RiskLevelIsL1(t *testing.T) {
	allTools := []toolspkg.Tool{
		NewTopTool(nil, nil),
		NewStatusTool(nil),
	}

	for _, tool := range allTools {
		t.Run(tool.Name(), func(t *testing.T) {
			if tool.RiskLevel() != toolspkg.RiskLevelL1 {
				t.Errorf("%s: expected L1, got %s", tool.Name(), tool.RiskLevel())
			}
		})
	}
}
