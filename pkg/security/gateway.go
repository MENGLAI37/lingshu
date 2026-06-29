package security

import (
	"context"
	"fmt"
	"sync"

	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Security Gateway - 安全网关接口定义 (L0-L4)
// ===========================================================================

// RiskLevel represents the risk classification of an operation.
type RiskLevel string

const (
	LevelL0 RiskLevel = "L0" // Read-only operations, no risk
	LevelL1 RiskLevel = "L1" // Safe write operations, minimal risk
	LevelL2 RiskLevel = "L2" // Moderate risk, requires confirmation
	LevelL3 RiskLevel = "L3" // High risk, strong confirmation required
	LevelL4 RiskLevel = "L4" // Destructive operations, maximum caution
)

// OperationCategory represents the category of an operation.
type OperationCategory string

const (
	CategoryRead       OperationCategory = "read"
	CategoryWrite      OperationCategory = "write"
	CategoryModify     OperationCategory = "modify"
	CategoryDelete     OperationCategory = "delete"
	CategoryDangerous  OperationCategory = "dangerous"
)

// SecurityGateway is the main interface for security checks.
type SecurityGateway interface {
	// EvaluateRisk evaluates the risk level of an operation
	EvaluateRisk(ctx context.Context, toolName string, args map[string]any) (RiskEvaluation, error)

	// IsAllowed determines if an operation is allowed
	IsAllowed(ctx context.Context, evaluation RiskEvaluation) (bool, string)

	// RequiresConfirmation checks if user confirmation is required
	RequiresConfirmation(ctx context.Context, evaluation RiskEvaluation) bool

	// GetConfirmationMessage returns the confirmation message for an operation
	GetConfirmationMessage(ctx context.Context, evaluation RiskEvaluation) string
}

// RiskEvaluation represents a risk assessment result.
type RiskEvaluation struct {
	RiskLevel         RiskLevel
	Category          OperationCategory
	Score             int             // 0-100, higher = more risky
	Reason            string
	AffectedResources []string
	EnvironmentWeight int             // Additional weight based on environment
	ToolRiskLevel     tools.ToolRiskLevel
	RequiresDoubleConfirm bool        // Whether two-person confirmation is needed
	BlockingRules     []BlockingRule  // Rules that blocked the operation
	Warnings          []string        // Warning messages for user
	Suggestions       []string        // Suggestions for safer alternatives
}

// BlockingRule represents a rule that blocks an operation.
type BlockingRule struct {
	RuleName    string
	Description string
	Severity    string
}

// ===========================================================================
// Gateway Configuration
// ===========================================================================

// GatewayConfig holds configuration for the security gateway.
type GatewayConfig struct {
	StrictMode            bool             // If true, apply stricter rules
	AllowL2InProduction   bool             // Allow L2 operations in production
	AllowL3InProduction   bool             // Allow L3 operations in production (rarely true)
	RequireDoubleConfirm  map[RiskLevel]bool // Risk levels requiring double confirmation
	BlockedOperations     []string         // Operations always blocked
	BlockedNamespaces     []string         // Namespaces where L2+ are blocked
	ProtectedResources    []string         // Resources that require extra protection
	CustomRules           []SecurityRule   // Custom security rules
}

// DefaultGatewayConfig returns the default configuration.
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		StrictMode:           true,
		AllowL2InProduction:  true,
		AllowL3InProduction:  false,
		RequireDoubleConfirm: map[RiskLevel]bool{
			LevelL3: true,
			LevelL4: true,
		},
		BlockedOperations: []string{},
		BlockedNamespaces: []string{
			"kube-system",
			"kube-public",
		},
		ProtectedResources: []string{},
		CustomRules:        []SecurityRule{},
	}
}

// ===========================================================================
// Security Rules
// ===========================================================================

// SecurityRule defines a security rule.
type SecurityRule interface {
	// GetName returns the rule name
	GetName() string

	// Evaluate evaluates the rule against an operation
	Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (RuleResult, error)
}

// RuleResult represents the result of rule evaluation.
type RuleResult struct {
	Passed    bool
	Blocked   bool
	Warning   string
	Suggestion string
}

// ExecutionContext provides context for security evaluation.
type ExecutionContext struct {
	Namespace      string
	Environment    string // "production", "staging", "development"
	ClusterName    string
	UserID         string
	UserRole       string
	OnCall         bool   // Whether user is on-call
	MaintenanceMode bool  // Whether maintenance mode is active
	ChangeWindow   bool   // Whether within change window
}

// ===========================================================================
// Default Security Gateway Implementation
// ===========================================================================

// DefaultSecurityGateway implements the SecurityGateway interface.
type DefaultSecurityGateway struct {
	config       GatewayConfig
	rules        []SecurityRule
 evaluators   []RiskEvaluator
	mu           sync.RWMutex
}

// NewDefaultSecurityGateway creates a new security gateway.
func NewDefaultSecurityGateway(config GatewayConfig) *DefaultSecurityGateway {
	gateway := &DefaultSecurityGateway{
		config:     config,
		rules:      []SecurityRule{},
		evaluators: []RiskEvaluator{},
	}

	// Add default evaluators
	gateway.AddEvaluator(NewToolRiskEvaluator())
	gateway.AddEvaluator(NewEnvironmentRiskEvaluator())
	gateway.AddEvaluator(NewResourceRiskEvaluator())

	// Add default rules
	gateway.AddRule(NewProductionBlockRule())
	gateway.AddRule(NewNamespaceBlockRule())
	gateway.AddRule(NewClusterLevelBlockRule())

	// Add custom rules from config
	for _, rule := range config.CustomRules {
		gateway.AddRule(rule)
	}

	return gateway
}

// AddRule adds a security rule.
func (g *DefaultSecurityGateway) AddRule(rule SecurityRule) {
	g.mu.Lock()
	g.rules = append(g.rules, rule)
	g.mu.Unlock()
}

// AddEvaluator adds a risk evaluator.
func (g *DefaultSecurityGateway) AddEvaluator(evaluator RiskEvaluator) {
	g.mu.Lock()
	g.evaluators = append(g.evaluators, evaluator)
	g.mu.Unlock()
}

// EvaluateRisk evaluates the risk level of an operation.
func (g *DefaultSecurityGateway) EvaluateRisk(ctx context.Context, toolName string, args map[string]any) (RiskEvaluation, error) {
	evaluation := RiskEvaluation{
		AffectedResources: []string{},
		BlockingRules:     []BlockingRule{},
		Warnings:          []string{},
		Suggestions:       []string{},
	}

	// Get execution context
	execContext := extractExecutionContext(ctx)

	// Run evaluators
	totalScore := 0
	for _, evaluator := range g.getEvaluators() {
		score, reason, err := evaluator.Evaluate(ctx, toolName, args, execContext)
		if err != nil {
			return evaluation, err
		}
		totalScore += score
		if reason != "" {
			evaluation.Reason = reason
		}
	}

	evaluation.Score = totalScore

	// Determine risk level based on score
	evaluation.RiskLevel = scoreToRiskLevel(totalScore)

	// Run security rules
	for _, rule := range g.getRules() {
		result, err := rule.Evaluate(ctx, toolName, args, execContext)
		if err != nil {
			return evaluation, err
		}

		if result.Blocked {
			evaluation.BlockingRules = append(evaluation.BlockingRules, BlockingRule{
				RuleName:    rule.GetName(),
				Description: result.Warning,
				Severity:    "high",
			})
		}

		if !result.Passed && result.Warning != "" {
			evaluation.Warnings = append(evaluation.Warnings, result.Warning)
		}

		if result.Suggestion != "" {
			evaluation.Suggestions = append(evaluation.Suggestions, result.Suggestion)
		}
	}

	// Check if double confirmation is required
	evaluation.RequiresDoubleConfirm = g.config.RequireDoubleConfirm[evaluation.RiskLevel]

	// Identify affected resources
	evaluation.AffectedResources = extractAffectedResources(toolName, args)

	return evaluation, nil
}

// IsAllowed determines if an operation is allowed.
func (g *DefaultSecurityGateway) IsAllowed(ctx context.Context, evaluation RiskEvaluation) (bool, string) {
	// Check blocking rules first
	if len(evaluation.BlockingRules) > 0 {
		for _, rule := range evaluation.BlockingRules {
			return false, fmt.Sprintf("Blocked by rule %s: %s", rule.RuleName, rule.Description)
		}
	}

	// Check against configuration
	execContext := extractExecutionContext(ctx)

	// Check L3/L4 in production
	if execContext.Environment == "production" {
		if evaluation.RiskLevel == LevelL3 && !g.config.AllowL3InProduction {
			return false, "L3 operations not allowed in production"
		}
		if evaluation.RiskLevel == LevelL4 {
			return false, "L4 operations always blocked in production"
		}
	}

	// Check on-call requirement
	if evaluation.RequiresDoubleConfirm && !execContext.OnCall {
		return false, "High-risk operation requires on-call personnel present"
	}

	// Check change window
	if evaluation.RiskLevel == LevelL3 || evaluation.RiskLevel == LevelL4 {
		if !execContext.ChangeWindow && !execContext.MaintenanceMode {
			return false, "High-risk operation must be within change window or maintenance mode"
		}
	}

	// If passed all checks
	return true, ""
}

// RequiresConfirmation checks if user confirmation is required.
func (g *DefaultSecurityGateway) RequiresConfirmation(ctx context.Context, evaluation RiskEvaluation) bool {
	return evaluation.RiskLevel == LevelL2 || evaluation.RiskLevel == LevelL3 || evaluation.RiskLevel == LevelL4
}

// GetConfirmationMessage returns the confirmation message.
func (g *DefaultSecurityGateway) GetConfirmationMessage(ctx context.Context, evaluation RiskEvaluation) string {
	message := fmt.Sprintf("Operation requires confirmation (Risk: %s)\n", evaluation.RiskLevel)
	message += fmt.Sprintf("Reason: %s\n", evaluation.Reason)

	if len(evaluation.AffectedResources) > 0 {
		message += "Affected resources:\n"
		for _, resource := range evaluation.AffectedResources {
			message += fmt.Sprintf("  - %s\n", resource)
		}
	}

	if len(evaluation.Warnings) > 0 {
		message += "Warnings:\n"
		for _, warning := range evaluation.Warnings {
			message += fmt.Sprintf("  - %s\n", warning)
		}
	}

	if evaluation.RequiresDoubleConfirm {
		message += "\n⚠️ This operation requires TWO-person confirmation (on-call + operator)\n"
	}

	return message
}

func (g *DefaultSecurityGateway) getRules() []SecurityRule {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.rules
}

func (g *DefaultSecurityGateway) getEvaluators() []RiskEvaluator {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.evaluators
}

// ===========================================================================
// Risk Evaluators
// ===========================================================================

// RiskEvaluator evaluates risk factors.
type RiskEvaluator interface {
	Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (int, string, error)
}

// ToolRiskEvaluator evaluates risk based on tool type.
type ToolRiskEvaluator struct{}

func NewToolRiskEvaluator() *ToolRiskEvaluator {
	return &ToolRiskEvaluator{}
}

func (e *ToolRiskEvaluator) Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (int, string, error) {
	// Base score from tool type
	baseScore := 0
	category := CategoryRead

	switch toolName {
	case "k8s_get", "k8s_describe", "k8s_logs", "k8s_events", "k8s_status", "k8s_top":
		baseScore = 0
		category = CategoryRead
	case "k8s_restart":
		baseScore = 30
		category = CategoryModify
	case "k8s_scale":
		baseScore = 25
		category = CategoryModify
	case "k8s_patch":
		baseScore = 40
		category = CategoryModify
	case "k8s_rollout":
		baseScore = 35
		category = CategoryModify
	case "k8s_delete":
		baseScore = 80
		category = CategoryDelete
	default:
		baseScore = 20 // Default moderate risk
	}

	reason := fmt.Sprintf("Tool %s is classified as %s operation", toolName, category)
	return baseScore, reason, nil
}

// EnvironmentRiskEvaluator evaluates risk based on environment.
type EnvironmentRiskEvaluator struct{}

func NewEnvironmentRiskEvaluator() *EnvironmentRiskEvaluator {
	return &EnvironmentRiskEvaluator{}
}

func (e *EnvironmentRiskEvaluator) Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (int, string, error) {
	// Additional risk based on environment
	envScore := 0
	envReason := ""

	switch context.Environment {
	case "production", "prod":
		envScore = 20
		envReason = "Production environment (+20 risk)"
	case "staging", "stage":
		envScore = 10
		envReason = "Staging environment (+10 risk)"
	case "development", "dev":
		envScore = 0
		envReason = "Development environment (no additional risk)"
	default:
		envScore = 5
		envReason = "Unknown environment (+5 risk)"
	}

	// Check for sensitive namespaces
	namespace, _ := args["namespace"].(string)
	if namespace == "kube-system" {
		envScore += 30
		envReason += ", kube-system namespace (+30 risk)"
	}
	if namespace == "kube-public" {
		envScore += 20
		envReason += ", kube-public namespace (+20 risk)"
	}

	return envScore, envReason, nil
}

// ResourceRiskEvaluator evaluates risk based on resource type.
type ResourceRiskEvaluator struct{}

func NewResourceRiskEvaluator() *ResourceRiskEvaluator {
	return &ResourceRiskEvaluator{}
}

func (e *ResourceRiskEvaluator) Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (int, string, error) {
	resourceScore := 0
	reason := ""

	// Check resource type
	resourceType, _ := args["resource_type"].(string)

	switch resourceType {
	case "clusterrolebinding", "clusterrole":
		resourceScore = 50
		reason = "Cluster-level RBAC resource (+50 risk)"
	case "namespace":
		resourceScore = 40
		reason = "Namespace resource (+40 risk)"
	case "persistentvolume", "pv":
		resourceScore = 35
		reason = "Persistent volume (+35 risk)"
	case "secret":
		resourceScore = 30
		reason = "Secret resource (+30 risk)"
	case "deployment", "statefulset":
		resourceScore = 20
		reason = "Workload resource (+20 risk)"
	default:
		resourceScore = 10
	}

	return resourceScore, reason, nil
}

// ===========================================================================
// Default Security Rules
// ===========================================================================

// ProductionBlockRule blocks dangerous operations in production.
type ProductionBlockRule struct{}

func NewProductionBlockRule() *ProductionBlockRule {
	return &ProductionBlockRule{}
}

func (r *ProductionBlockRule) GetName() string {
	return "production_block"
}

func (r *ProductionBlockRule) Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (RuleResult, error) {
	result := RuleResult{Passed: true}

	if context.Environment == "production" {
		// Block delete operations in production
		if toolName == "k8s_delete" {
			result.Passed = false
			result.Blocked = true
			result.Warning = "Delete operations blocked in production"
			result.Suggestion = "Use scale to 0 or consider manual intervention"
		}
	}

	return result, nil
}

// NamespaceBlockRule blocks operations in sensitive namespaces.
type NamespaceBlockRule struct{}

func NewNamespaceBlockRule() *NamespaceBlockRule {
	return &NamespaceBlockRule{}
}

func (r *NamespaceBlockRule) GetName() string {
	return "namespace_block"
}

func (r *NamespaceBlockRule) Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (RuleResult, error) {
	result := RuleResult{Passed: true}

	namespace, _ := args["namespace"].(string)

	if namespace == "kube-system" && toolName != "k8s_get" && toolName != "k8s_describe" {
		result.Passed = false
		result.Blocked = true
		result.Warning = "Write operations blocked in kube-system namespace"
		result.Suggestion = "Only read operations allowed in kube-system"
	}

	return result, nil
}

// ClusterLevelBlockRule blocks cluster-level bindings.
type ClusterLevelBlockRule struct{}

func NewClusterLevelBlockRule() *ClusterLevelBlockRule {
	return &ClusterLevelBlockRule{}
}

func (r *ClusterLevelBlockRule) GetName() string {
	return "cluster_level_block"
}

func (r *ClusterLevelBlockRule) Evaluate(ctx context.Context, toolName string, args map[string]any, context *ExecutionContext) (RuleResult, error) {
	result := RuleResult{Passed: true}

	resourceType, _ := args["resource_type"].(string)

	if resourceType == "clusterrolebinding" || resourceType == "clusterrole" {
		result.Passed = false
		result.Blocked = true
		result.Warning = "Cluster-level RBAC modifications blocked"
		result.Suggestion = "Contact cluster admin for RBAC changes"
	}

	return result, nil
}

// ===========================================================================
// Helper Functions
// ===========================================================================

func scoreToRiskLevel(score int) RiskLevel {
	if score >= 70 {
		return LevelL4
	}
	if score >= 50 {
		return LevelL3
	}
	if score >= 30 {
		return LevelL2
	}
	if score >= 10 {
		return LevelL1
	}
	return LevelL0
}

func extractExecutionContext(ctx context.Context) *ExecutionContext {
	// Extract from context values
	execCtx := &ExecutionContext{}

	// Try to get from context value
	if v := ctx.Value("namespace"); v != nil {
		execCtx.Namespace = v.(string)
	}
	if v := ctx.Value("environment"); v != nil {
		execCtx.Environment = v.(string)
	}
	if v := ctx.Value("cluster"); v != nil {
		execCtx.ClusterName = v.(string)
	}
	if v := ctx.Value("user_id"); v != nil {
		execCtx.UserID = v.(string)
	}
	if v := ctx.Value("on_call"); v != nil {
		execCtx.OnCall = v.(bool)
	}
	if v := ctx.Value("change_window"); v != nil {
		execCtx.ChangeWindow = v.(bool)
	}
	if v := ctx.Value("maintenance_mode"); v != nil {
		execCtx.MaintenanceMode = v.(bool)
	}

	return execCtx
}

func extractAffectedResources(toolName string, args map[string]any) []string {
	resources := []string{}

	namespace, _ := args["namespace"].(string)
	name, _ := args["name"].(string)
	resourceType, _ := args["resource_type"].(string)

	if namespace != "" && name != "" {
		resources = append(resources, fmt.Sprintf("%s/%s/%s", namespace, resourceType, name))
	} else if namespace != "" && resourceType != "" {
		resources = append(resources, fmt.Sprintf("%s/%s/*", namespace, resourceType))
	} else if name != "" {
		resources = append(resources, fmt.Sprintf("%s/%s", resourceType, name))
	}

	return resources
}