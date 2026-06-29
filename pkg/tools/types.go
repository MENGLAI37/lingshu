package tools

import (
	"context"
	"time"
)

type ToolRiskLevel string

const (
	RiskLevelL0 ToolRiskLevel = "L0"
	RiskLevelL1 ToolRiskLevel = "L1"
	RiskLevelL2 ToolRiskLevel = "L2"
	RiskLevelL3 ToolRiskLevel = "L3"
	RiskLevelL4 ToolRiskLevel = "L4"
)

type ToolResult struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	Data      any       `json:"data,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`
	ToolName  string    `json:"tool_name"`
	RiskLevel ToolRiskLevel `json:"risk_level"`
}

type Tool interface {
	Name() string
	RiskLevel() ToolRiskLevel
	Description() string
	Execute(ctx context.Context, params map[string]any) (*ToolResult, error)
}

type ToolContext struct {
	Context     string
	Namespace   string
	ClusterName string
}

type GetOptions struct {
	Namespace string
	Labels    map[string]string
	AllNamespaces bool
	OutputFormat string
}

type DescribeOptions struct {
	Namespace string
	Name      string
}

type LogOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	TailLines     int
	Follow        bool
	Since         time.Duration
	Timestamps    bool
}

type EventOptions struct {
	Namespace     string
	FieldSelector string
	AllNamespaces bool
	Limit         int
}

type TopOptions struct {
	Namespace string
	AllNamespaces bool
	SortBy    string
	Limit     int
}

type ScaleOptions struct {
	Namespace string
	Name      string
	Replicas  int
	ResourceType string
}

type RestartOptions struct {
	Namespace string
	Name      string
	ResourceType string
}

type PatchOptions struct {
	Namespace string
	Name      string
	ResourceType string
	PatchType string
	PatchData string
}

type RolloutOptions struct {
	Namespace string
	Name      string
	Action    string
	ResourceType string
}
