package workflow

import (
	"context"

	"github.com/lingshu/lingshu/pkg/agent"
	"github.com/lingshu/lingshu/pkg/tools"
)

type ConditionType string

const (
	ConditionAlways    ConditionType = "always"
	ConditionEquals    ConditionType = "equals"
	ConditionNotEquals ConditionType = "not_equals"
	ConditionContains  ConditionType = "contains"
	ConditionMatches   ConditionType = "matches"
	ConditionGreater   ConditionType = "greater_than"
	ConditionLess      ConditionType = "less_than"
)

type ActionType string

const (
	ActionToolCall    ActionType = "tool_call"
	ActionDelay       ActionType = "delay"
	ActionLog         ActionType = "log"
	ActionSetVariable ActionType = "set_variable"
	ActionExit        ActionType = "exit"
)

type WorkFlowStatus string

const (
	StatusPending   WorkFlowStatus = "pending"
	StatusRunning   WorkFlowStatus = "running"
	StatusCompleted WorkFlowStatus = "completed"
	StatusFailed    WorkFlowStatus = "failed"
	StatusCancelled WorkFlowStatus = "cancelled"
)

type Condition struct {
	Type      ConditionType
	Variable  string
	Expected  interface{}
	Invert    bool
}

type Action struct {
	Type         ActionType
	ToolName     string
	ToolParams   map[string]interface{}
	DelaySeconds int
	Message      string
	VariableName string
	VariableValue interface{}
	NextStep     string
}

type Step struct {
	ID          string
	Name        string
	Description string
	Condition   *Condition
	Actions     []Action
	NextStep    string
	OnFailure   string
}

type WorkFlow struct {
	ID          string
	Name        string
	Description string
	Version     string
	Steps       map[string]*Step
	StartStep   string
	Variables   map[string]interface{}
}

type WorkFlowInstance struct {
	ID         string
	WorkFlowID string
	Status     WorkFlowStatus
	CurrentStep string
	Variables  map[string]interface{}
	Results    map[string]*tools.ToolResult
	Error      error
}

type WorkFlowEngine interface {
	RegisterWorkFlow(wf *WorkFlow) error
	GetWorkFlow(id string) (*WorkFlow, error)
	ListWorkFlows() []*WorkFlow
	Execute(ctx context.Context, wfID string, initialVars map[string]interface{}, handler agent.LoopEventHandler) (*WorkFlowInstance, error)
}

type WorkFlowToolInterface interface {
	tools.Tool
	GetWorkFlowEngine() WorkFlowEngine
}
