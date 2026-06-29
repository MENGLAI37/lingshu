package security

import (
	"context"
	"fmt"
	"testing"
)

// ===========================================================================
// Security Gateway Tests
// ===========================================================================

func TestDefaultSecurityGateway_EvaluateRisk(t *testing.T) {
	gateway := NewDefaultSecurityGateway(DefaultGatewayConfig())

	tests := []struct {
		name         string
		toolName     string
		args         map[string]any
		expectedRisk RiskLevel
	}{
		{
			name:         "read operation",
			toolName:     "k8s_get",
			args:         map[string]any{"resource_type": "pod", "namespace": "default"},
			expectedRisk: LevelL0,
		},
		{
			name:         "scale operation",
			toolName:     "k8s_scale",
			args:         map[string]any{"resource_type": "deployment", "name": "nginx", "namespace": "default"},
			expectedRisk: LevelL2,
		},
		{
			name:         "delete operation",
			toolName:     "k8s_delete",
			args:         map[string]any{"resource_type": "pod", "name": "nginx", "namespace": "default"},
			expectedRisk: LevelL4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = context.WithValue(ctx, "namespace", tt.args["namespace"])
			ctx = context.WithValue(ctx, "environment", "development")

			evaluation, err := gateway.EvaluateRisk(ctx, tt.toolName, tt.args)
			if err != nil {
				t.Errorf("evaluate risk error: %v", err)
				return
			}

			t.Logf("Tool: %s, Risk Level: %s, Score: %d", tt.toolName, evaluation.RiskLevel, evaluation.Score)
		})
	}
}

func TestDefaultSecurityGateway_IsAllowed(t *testing.T) {
	gateway := NewDefaultSecurityGateway(DefaultGatewayConfig())

	tests := []struct {
		name       string
		env        string
		riskLevel  RiskLevel
		expected   bool
	}{
		{
			name:      "L0 in production",
			env:       "production",
			riskLevel: LevelL0,
			expected:  true,
		},
		{
			name:      "L2 in production",
			env:       "production",
			riskLevel: LevelL2,
			expected:  true, // Default config allows L2 in production
		},
		{
			name:      "L3 in production",
			env:       "production",
			riskLevel: LevelL3,
			expected:  false, // Default config blocks L3 in production
		},
		{
			name:      "L4 in production",
			env:       "production",
			riskLevel: LevelL4,
			expected:  false, // L4 always blocked in production
		},
		{
			name:      "L3 in development",
			env:       "development",
			riskLevel: LevelL3,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = context.WithValue(ctx, "environment", tt.env)
			ctx = context.WithValue(ctx, "on_call", true)
			ctx = context.WithValue(ctx, "change_window", true) // Enable change window for L3/L4

			evaluation := RiskEvaluation{
				RiskLevel:         tt.riskLevel,
				RequiresDoubleConfirm: tt.riskLevel == LevelL3 || tt.riskLevel == LevelL4,
				BlockingRules:     []BlockingRule{},
			}

			allowed, reason := gateway.IsAllowed(ctx, evaluation)
			if allowed != tt.expected {
				t.Errorf("expected %v, got %v, reason: %s", tt.expected, allowed, reason)
			}

			if !allowed {
				t.Logf("blocked reason: %s", reason)
			}
		})
	}
}

func TestDefaultSecurityGateway_RequiresConfirmation(t *testing.T) {
	gateway := NewDefaultSecurityGateway(DefaultGatewayConfig())

	tests := []struct {
		riskLevel RiskLevel
		expected  bool
	}{
		{LevelL0, false},
		{LevelL1, false},
		{LevelL2, true},
		{LevelL3, true},
		{LevelL4, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.riskLevel), func(t *testing.T) {
			ctx := context.Background()
			evaluation := RiskEvaluation{RiskLevel: tt.riskLevel}

			result := gateway.RequiresConfirmation(ctx, evaluation)
			if result != tt.expected {
				t.Errorf("risk level %s: expected %v, got %v", tt.riskLevel, tt.expected, result)
			}
		})
	}
}

func TestDefaultSecurityGateway_GetConfirmationMessage(t *testing.T) {
	gateway := NewDefaultSecurityGateway(DefaultGatewayConfig())

	ctx := context.Background()
	evaluation := RiskEvaluation{
		RiskLevel:         LevelL3,
		Reason:            "High risk operation on critical resource",
		AffectedResources: []string{"default/deployment/nginx"},
		Warnings:          []string{"Resource is part of production workload"},
		RequiresDoubleConfirm: true,
	}

	message := gateway.GetConfirmationMessage(ctx, evaluation)

	if message == "" {
		t.Errorf("expected non-empty confirmation message")
	}

	t.Logf("confirmation message:\n%s", message)
}

// ===========================================================================
// Risk Evaluator Tests
// ===========================================================================

func TestToolRiskEvaluator_Evaluate(t *testing.T) {
	evaluator := NewToolRiskEvaluator()

	tests := []struct {
		toolName     string
		expectedMin  int
		expectedMax  int
	}{
		{"k8s_get", 0, 10},
		{"k8s_scale", 20, 35},
		{"k8s_restart", 25, 40},
		{"k8s_delete", 70, 100},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			ctx := context.Background()
			score, reason, err := evaluator.Evaluate(ctx, tt.toolName, map[string]any{}, nil)
			if err != nil {
				t.Errorf("evaluate error: %v", err)
				return
			}

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("score %d not in expected range [%d, %d]", score, tt.expectedMin, tt.expectedMax)
			}

			t.Logf("tool: %s, score: %d, reason: %s", tt.toolName, score, reason)
		})
	}
}

func TestEnvironmentRiskEvaluator_Evaluate(t *testing.T) {
	evaluator := NewEnvironmentRiskEvaluator()

	tests := []struct {
		env       string
		namespace string
		expected  int
	}{
		{"production", "default", 20},
		{"staging", "default", 10},
		{"development", "default", 0},
		{"production", "kube-system", 50}, // 20 + 30
	}

	for _, tt := range tests {
		t.Run(tt.env+"_"+tt.namespace, func(t *testing.T) {
			ctx := context.Background()
			execCtx := &ExecutionContext{
				Environment: tt.env,
			}
			args := map[string]any{"namespace": tt.namespace}

			score, reason, err := evaluator.Evaluate(ctx, "k8s_get", args, execCtx)
			if err != nil {
				t.Errorf("evaluate error: %v", err)
				return
			}

			t.Logf("env: %s, namespace: %s, score: %d, reason: %s", tt.env, tt.namespace, score, reason)
		})
	}
}

func TestResourceRiskEvaluator_Evaluate(t *testing.T) {
	evaluator := NewResourceRiskEvaluator()

	tests := []struct {
		resourceType string
		expected     int
	}{
		{"clusterrolebinding", 50},
		{"namespace", 40},
		{"persistentvolume", 35},
		{"secret", 30},
		{"deployment", 20},
		{"pod", 10},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			ctx := context.Background()
			args := map[string]any{"resource_type": tt.resourceType}

			score, reason, err := evaluator.Evaluate(ctx, "k8s_get", args, nil)
			if err != nil {
				t.Errorf("evaluate error: %v", err)
				return
			}

			t.Logf("resource type: %s, score: %d, reason: %s", tt.resourceType, score, reason)
		})
	}
}

// ===========================================================================
// Security Rule Tests
// ===========================================================================

func TestProductionBlockRule_Evaluate(t *testing.T) {
	rule := NewProductionBlockRule()

	tests := []struct {
		name      string
		env       string
		toolName  string
		expected  bool // blocked?
	}{
		{"delete in production", "production", "k8s_delete", true},
		{"delete in development", "development", "k8s_delete", false},
		{"get in production", "production", "k8s_get", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			execCtx := &ExecutionContext{
				Environment: tt.env,
			}

			result, err := rule.Evaluate(ctx, tt.toolName, map[string]any{}, execCtx)
			if err != nil {
				t.Errorf("evaluate error: %v", err)
				return
			}

			if result.Blocked != tt.expected {
				t.Errorf("expected blocked=%v, got blocked=%v", tt.expected, result.Blocked)
			}

			if result.Warning != "" {
				t.Logf("warning: %s", result.Warning)
			}
		})
	}
}

func TestNamespaceBlockRule_Evaluate(t *testing.T) {
	rule := NewNamespaceBlockRule()

	tests := []struct {
		name      string
		namespace string
		toolName  string
		expected  bool // blocked?
	}{
		{"scale in kube-system", "kube-system", "k8s_scale", true},
		{"get in kube-system", "kube-system", "k8s_get", false},
		{"scale in default", "default", "k8s_scale", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			args := map[string]any{"namespace": tt.namespace}

			result, err := rule.Evaluate(ctx, tt.toolName, args, nil)
			if err != nil {
				t.Errorf("evaluate error: %v", err)
				return
			}

			if result.Blocked != tt.expected {
				t.Errorf("expected blocked=%v, got blocked=%v", tt.expected, result.Blocked)
			}
		})
	}
}

func TestClusterLevelBlockRule_Evaluate(t *testing.T) {
	rule := NewClusterLevelBlockRule()

	tests := []struct {
		name         string
		resourceType string
		expected     bool // blocked?
	}{
		{"clusterrolebinding", "clusterrolebinding", true},
		{"clusterrole", "clusterrole", true},
		{"rolebinding", "rolebinding", false},
		{"deployment", "deployment", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			args := map[string]any{"resource_type": tt.resourceType}

			result, err := rule.Evaluate(ctx, "k8s_create", args, nil)
			if err != nil {
				t.Errorf("evaluate error: %v", err)
				return
			}

			if result.Blocked != tt.expected {
				t.Errorf("expected blocked=%v, got blocked=%v", tt.expected, result.Blocked)
			}
		})
	}
}

// ===========================================================================
// Helper Function Tests
// ===========================================================================

func TestScoreToRiskLevel(t *testing.T) {
	tests := []struct {
		score    int
		expected RiskLevel
	}{
		{0, LevelL0},
		{15, LevelL1},
		{35, LevelL2},
		{55, LevelL3},
		{85, LevelL4},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%d", tt.score), func(t *testing.T) {
			result := scoreToRiskLevel(tt.score)
			if result != tt.expected {
				t.Errorf("score %d: expected %s, got %s", tt.score, tt.expected, result)
			}
		})
	}
}

func TestExtractAffectedResources(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]any
		expected int
	}{
		{
			name:     "full args",
			toolName: "k8s_scale",
			args:     map[string]any{"namespace": "default", "resource_type": "deployment", "name": "nginx"},
			expected: 1,
		},
		{
			name:     "partial args",
			toolName: "k8s_get",
			args:     map[string]any{"namespace": "default", "resource_type": "pod"},
			expected: 1,
		},
		{
			name:     "minimal args",
			toolName: "k8s_get",
			args:     map[string]any{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAffectedResources(tt.toolName, tt.args)
			if len(result) != tt.expected {
				t.Errorf("expected %d resources, got %d", tt.expected, len(result))
			}

			for _, r := range result {
				t.Logf("affected resource: %s", r)
			}
		})
	}
}