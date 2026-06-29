package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Tool Registry - 工具注册器
// ===========================================================================

// DefaultToolRegistry implements ToolRegistry interface.
type DefaultToolRegistry struct {
	tools map[string]tools.Tool
	mu    sync.RWMutex
}

// NewDefaultToolRegistry creates a new tool registry.
func NewDefaultToolRegistry() *DefaultToolRegistry {
	return &DefaultToolRegistry{
		tools: map[string]tools.Tool{},
	}
}

// RegisterTool registers a tool in the registry.
func (tr *DefaultToolRegistry) RegisterTool(tool tools.Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	tr.mu.Lock()
	tr.tools[tool.Name()] = tool
	tr.mu.Unlock()

	return nil
}

// GetTool retrieves a tool by name.
func (tr *DefaultToolRegistry) GetTool(name string) (tools.Tool, error) {
	tr.mu.RLock()
	tool, ok := tr.tools[name]
	tr.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// ListTools returns all registered tools.
func (tr *DefaultToolRegistry) ListTools() []tools.Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	result := []tools.Tool{}
	for _, tool := range tr.tools {
		result = append(result, tool)
	}

	return result
}

// RemoveTool removes a tool from the registry.
func (tr *DefaultToolRegistry) RemoveTool(name string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if _, ok := tr.tools[name]; !ok {
		return fmt.Errorf("tool %s not found", name)
	}

	delete(tr.tools, name)
	return nil
}

// HasTool checks if a tool exists in the registry.
func (tr *DefaultToolRegistry) HasTool(name string) bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.tools[name] != nil
}

// GetToolRiskLevel returns the risk level for a tool.
func (tr *DefaultToolRegistry) GetToolRiskLevel(name string) (tools.ToolRiskLevel, error) {
	tool, err := tr.GetTool(name)
	if err != nil {
		return "", err
	}

	return tool.RiskLevel(), nil
}

// GetToolsByRiskLevel returns tools filtered by risk level.
func (tr *DefaultToolRegistry) GetToolsByRiskLevel(level tools.ToolRiskLevel) []tools.Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	result := []tools.Tool{}
	for _, tool := range tr.tools {
		if tool.RiskLevel() == level {
			result = append(result, tool)
		}
	}

	return result
}

// Clear removes all tools from the registry.
func (tr *DefaultToolRegistry) Clear() {
	tr.mu.Lock()
	tr.tools = map[string]tools.Tool{}
	tr.mu.Unlock()
}

// Count returns the number of registered tools.
func (tr *DefaultToolRegistry) Count() int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return len(tr.tools)
}

// ===========================================================================
// Tool Registry Builder
// ===========================================================================

// ToolRegistryBuilder helps build a tool registry with common tools.
type ToolRegistryBuilder struct {
	registry *DefaultToolRegistry
}

// NewToolRegistryBuilder creates a new builder.
func NewToolRegistryBuilder() *ToolRegistryBuilder {
	return &ToolRegistryBuilder{
		registry: NewDefaultToolRegistry(),
	}
}

// AddTool adds a tool to the registry.
func (b *ToolRegistryBuilder) AddTool(tool tools.Tool) *ToolRegistryBuilder {
	_ = b.registry.RegisterTool(tool)
	return b
}

// AddTools adds multiple tools to the registry.
func (b *ToolRegistryBuilder) AddTools(toolsToAdd ...tools.Tool) *ToolRegistryBuilder {
	for _, tool := range toolsToAdd {
		_ = b.registry.RegisterTool(tool)
	}
	return b
}

// Build returns the built registry.
func (b *ToolRegistryBuilder) Build() ToolRegistry {
	return b.registry
}

// ===========================================================================
// Tool Execution Context
// ===========================================================================

// ToolExecutionContext provides context for tool execution.
type ToolExecutionContext struct {
	Namespace   string
	ClusterName string
	UserID      string
	SessionID   string
	Environment string // "production", "staging", "development"
	Labels      map[string]string
}

// NewToolExecutionContext creates a new tool execution context.
func NewToolExecutionContext(namespace, clusterName string) *ToolExecutionContext {
	return &ToolExecutionContext{
		Namespace:   namespace,
		ClusterName: clusterName,
		Labels:      map[string]string{},
	}
}

// SetUserID sets the user ID.
func (tec *ToolExecutionContext) SetUserID(userID string) {
	tec.UserID = userID
}

// SetSessionID sets the session ID.
func (tec *ToolExecutionContext) SetSessionID(sessionID string) {
	tec.SessionID = sessionID
}

// SetEnvironment sets the environment.
func (tec *ToolExecutionContext) SetEnvironment(env string) {
	tec.Environment = env
}

// SetLabel sets a label.
func (tec *ToolExecutionContext) SetLabel(key, value string) {
	tec.Labels[key] = value
}

// GetLabel gets a label value.
func (tec *ToolExecutionContext) GetLabel(key string) string {
	return tec.Labels[key]
}

// IsProduction returns true if the environment is production.
func (tec *ToolExecutionContext) IsProduction() bool {
	return tec.Environment == "production" || tec.Environment == "prod"
}

// IsKubeSystem returns true if the namespace is kube-system.
func (tec *ToolExecutionContext) IsKubeSystem() bool {
	return tec.Namespace == "kube-system"
}

// ===========================================================================
// Tool Execution Wrapper
// ===========================================================================

// ToolExecutionWrapper wraps tool execution with additional checks.
type ToolExecutionWrapper struct {
	registry      ToolRegistry
	security      SecurityGateway
	context       *ToolExecutionContext
	beforeExecute func(ctx context.Context, toolName string, args map[string]any) error
	afterExecute  func(ctx context.Context, result *ToolExecutionResult)
}

// NewToolExecutionWrapper creates a new execution wrapper.
func NewToolExecutionWrapper(registry ToolRegistry, security SecurityGateway, context *ToolExecutionContext) *ToolExecutionWrapper {
	return &ToolExecutionWrapper{
		registry: registry,
		security: security,
		context:  context,
	}
}

// SetBeforeExecute sets a hook called before execution.
func (tew *ToolExecutionWrapper) SetBeforeExecute(hook func(ctx context.Context, toolName string, args map[string]any) error) {
	tew.beforeExecute = hook
}

// SetAfterExecute sets a hook called after execution.
func (tew *ToolExecutionWrapper) SetAfterExecute(hook func(ctx context.Context, result *ToolExecutionResult)) {
	tew.afterExecute = hook
}

// Execute executes a tool with checks and hooks.
func (tew *ToolExecutionWrapper) Execute(ctx context.Context, toolName string, args map[string]any) (*ToolExecutionResult, error) {
	// Get tool
	tool, err := tew.registry.GetTool(toolName)
	if err != nil {
		return nil, err
	}

	// Security check
	if tew.security != nil {
		evaluation, err := tew.security.EvaluateRisk(ctx, toolName, args)
		if err != nil {
			return nil, fmt.Errorf("security evaluation failed: %w", err)
		}

		allowed, reason := tew.security.IsAllowed(ctx, evaluation)
		if !allowed {
			return nil, fmt.Errorf("operation blocked by security gateway: %s", reason)
		}
	}

	// Add execution context to arguments
	enrichedArgs := tew.enrichArgs(args)

	// Before execute hook
	if tew.beforeExecute != nil {
		if err := tew.beforeExecute(ctx, toolName, enrichedArgs); err != nil {
			return nil, err
		}
	}

	// Execute tool
	result, err := tool.Execute(ctx, enrichedArgs)

	executionResult := &ToolExecutionResult{
		ToolName:  toolName,
		Arguments: args,
		Result:    result,
		Error:     err,
	}

	// After execute hook
	if tew.afterExecute != nil {
		tew.afterExecute(ctx, executionResult)
	}

	return executionResult, err
}

// enrichArgs enriches arguments with execution context.
func (tew *ToolExecutionWrapper) enrichArgs(args map[string]any) map[string]any {
	enriched := map[string]any{}
	for k, v := range args {
		enriched[k] = v
	}

	// Add context information
	if tew.context != nil {
		if enriched["namespace"] == nil && tew.context.Namespace != "" {
			enriched["namespace"] = tew.context.Namespace
		}
		if enriched["cluster"] == nil && tew.context.ClusterName != "" {
			enriched["cluster"] = tew.context.ClusterName
		}
		enriched["_context"] = tew.context
	}

	return enriched
}

// ===========================================================================
// Tool Group Registry
// ===========================================================================

// ToolGroup represents a group of related tools.
type ToolGroup struct {
	Name        string
	Description string
	Tools       []tools.Tool
}

// ToolGroupRegistry manages tool groups.
type ToolGroupRegistry struct {
	groups  map[string]*ToolGroup
	registry ToolRegistry
	mu      sync.RWMutex
}

// NewToolGroupRegistry creates a new group registry.
func NewToolGroupRegistry(baseRegistry ToolRegistry) *ToolGroupRegistry {
	return &ToolGroupRegistry{
		groups:   map[string]*ToolGroup{},
		registry: baseRegistry,
	}
}

// RegisterGroup registers a tool group.
func (tgr *ToolGroupRegistry) RegisterGroup(group *ToolGroup) error {
	tgr.mu.Lock()
	defer tgr.mu.Unlock()

	if group == nil || group.Name == "" {
		return fmt.Errorf("group must have a name")
	}

	// Register all tools in the group
	for _, tool := range group.Tools {
		if err := tgr.registry.RegisterTool(tool); err != nil {
			return err
		}
	}

	tgr.groups[group.Name] = group
	return nil
}

// GetGroup retrieves a tool group by name.
func (tgr *ToolGroupRegistry) GetGroup(name string) (*ToolGroup, error) {
	tgr.mu.RLock()
	group, ok := tgr.groups[name]
	tgr.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("group %s not found", name)
	}

	return group, nil
}

// ListGroups returns all registered groups.
func (tgr *ToolGroupRegistry) ListGroups() []string {
	tgr.mu.RLock()
	defer tgr.mu.RUnlock()

	names := []string{}
	for name := range tgr.groups {
		names = append(names, name)
	}
	return names
}

// GetTool delegates to the underlying registry.
func (tgr *ToolGroupRegistry) GetTool(name string) (tools.Tool, error) {
	return tgr.registry.GetTool(name)
}

// ListTools delegates to the underlying registry.
func (tgr *ToolGroupRegistry) ListTools() []tools.Tool {
	return tgr.registry.ListTools()
}

// RegisterTool delegates to the underlying registry.
func (tgr *ToolGroupRegistry) RegisterTool(tool tools.Tool) error {
	return tgr.registry.RegisterTool(tool)
}

// ===========================================================================
// Built-in Tool Groups
// ===========================================================================

// GetBuiltInToolGroups returns built-in tool groups.
func GetBuiltInToolGroups() []*ToolGroup {
	return []*ToolGroup{
		{
			Name:        "L0_read_only",
			Description: "Read-only operations - no risk",
			Tools:       []tools.Tool{}, // Will be populated with actual tools
		},
		{
			Name:        "L1_safe_write",
			Description: "Safe write operations - minimal risk",
			Tools:       []tools.Tool{},
		},
		{
			Name:        "L2_moderate_risk",
			Description: "Moderate risk operations - requires confirmation",
			Tools:       []tools.Tool{},
		},
		{
			Name:        "L3_high_risk",
			Description: "High risk operations - strong confirmation required",
			Tools:       []tools.Tool{},
		},
		{
			Name:        "L4_destructive",
			Description: "Destructive operations - maximum caution required",
			Tools:       []tools.Tool{},
		},
	}
}