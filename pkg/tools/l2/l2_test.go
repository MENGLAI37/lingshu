package l2

import (
	"testing"

	toolspkg "github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// L2 Tool Interface Tests
// ===========================================================================

func TestScaleTool_Interface(t *testing.T) {
	tool := NewScaleTool(nil)

	if tool.Name() != "k8s_scale" {
		t.Errorf("expected name 'k8s_scale', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL2 {
		t.Errorf("expected L2, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

func TestRestartTool_Interface(t *testing.T) {
	tool := NewRestartTool(nil)

	if tool.Name() != "k8s_restart" {
		t.Errorf("expected name 'k8s_restart', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL2 {
		t.Errorf("expected L2, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

func TestRolloutTool_Interface(t *testing.T) {
	tool := NewRolloutTool(nil)

	if tool.Name() != "k8s_rollout" {
		t.Errorf("expected name 'k8s_rollout', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL2 {
		t.Errorf("expected L2, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

func TestPatchTool_Interface(t *testing.T) {
	tool := NewPatchTool(nil)

	if tool.Name() != "k8s_patch" {
		t.Errorf("expected name 'k8s_patch', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != toolspkg.RiskLevelL2 {
		t.Errorf("expected L2, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

// ===========================================================================
// All L2 Tools — Consistency Checks
// ===========================================================================

func TestAllL2Tools_RiskLevelIsL2(t *testing.T) {
	entries := []struct {
		name string
		tool toolspkg.Tool
	}{
		{"ScaleTool", NewScaleTool(nil)},
		{"RestartTool", NewRestartTool(nil)},
		{"RolloutTool", NewRolloutTool(nil)},
		{"PatchTool", NewPatchTool(nil)},
	}

	for _, e := range entries {
		t.Run(e.name, func(t *testing.T) {
			if e.tool.RiskLevel() != toolspkg.RiskLevelL2 {
				t.Errorf("%s: expected L2, got %s", e.name, e.tool.RiskLevel())
			}
		})
	}
}

func TestAllL2Tools_ParameterSchemas(t *testing.T) {
	allTools := []toolspkg.Tool{
		NewScaleTool(nil),
		NewRestartTool(nil),
		NewRolloutTool(nil),
		NewPatchTool(nil),
	}

	for _, tool := range allTools {
		t.Run(tool.Name(), func(t *testing.T) {
			if tool.ParameterSchema() == nil {
				t.Errorf("%s: expected non-nil parameter schema", tool.Name())
			}
		})
	}
}

func TestAllL2Tools_NonEmptyDescription(t *testing.T) {
	allTools := []toolspkg.Tool{
		NewScaleTool(nil),
		NewRestartTool(nil),
		NewRolloutTool(nil),
		NewPatchTool(nil),
	}

	for _, tool := range allTools {
		t.Run(tool.Name(), func(t *testing.T) {
			if tool.Description() == "" {
				t.Errorf("%s: expected non-empty description", tool.Name())
			}
		})
	}
}
