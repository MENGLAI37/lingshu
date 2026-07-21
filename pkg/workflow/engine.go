package workflow

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lingshu/lingshu/pkg/agent"
	"github.com/lingshu/lingshu/pkg/tools"
)

type DefaultWorkFlowEngine struct {
	workflows map[string]*WorkFlow
	toolRegistry agent.ToolRegistry
	mu         sync.RWMutex
}

func NewDefaultWorkFlowEngine(toolRegistry agent.ToolRegistry) *DefaultWorkFlowEngine {
	return &DefaultWorkFlowEngine{
		workflows:    map[string]*WorkFlow{},
		toolRegistry: toolRegistry,
	}
}

func (e *DefaultWorkFlowEngine) RegisterWorkFlow(wf *WorkFlow) error {
	if wf == nil || wf.ID == "" {
		return fmt.Errorf("workflow must have an ID")
	}
	if _, ok := wf.Steps[wf.StartStep]; !ok {
		return fmt.Errorf("start step %s not found in workflow", wf.StartStep)
	}

	e.mu.Lock()
	e.workflows[wf.ID] = wf
	e.mu.Unlock()
	return nil
}

func (e *DefaultWorkFlowEngine) GetWorkFlow(id string) (*WorkFlow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	wf, ok := e.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow %s not found", id)
	}
	return wf, nil
}

func (e *DefaultWorkFlowEngine) ListWorkFlows() []*WorkFlow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := []*WorkFlow{}
	for _, wf := range e.workflows {
		result = append(result, wf)
	}
	return result
}

func (e *DefaultWorkFlowEngine) Execute(ctx context.Context, wfID string, initialVars map[string]interface{}, handler agent.LoopEventHandler) (*WorkFlowInstance, error) {
	wf, err := e.GetWorkFlow(wfID)
	if err != nil {
		return nil, err
	}

	instance := &WorkFlowInstance{
		ID:         fmt.Sprintf("inst_%d", time.Now().UnixNano()),
		WorkFlowID: wfID,
		Status:     StatusRunning,
		CurrentStep: wf.StartStep,
		Variables:  map[string]interface{}{},
		Results:    map[string]*tools.ToolResult{},
	}

	for k, v := range wf.Variables {
		instance.Variables[k] = v
	}
	for k, v := range initialVars {
		instance.Variables[k] = v
	}

	for {
		if ctx.Err() != nil {
			instance.Status = StatusCancelled
			instance.Error = ctx.Err()
			return instance, nil
		}

		step, ok := wf.Steps[instance.CurrentStep]
		if !ok {
			break
		}

		if step.Condition != nil && !e.evaluateCondition(step.Condition, instance.Variables) {
			instance.CurrentStep = step.NextStep
			continue
		}

		for _, action := range step.Actions {
			if ctx.Err() != nil {
				instance.Status = StatusCancelled
				instance.Error = ctx.Err()
				return instance, nil
			}

			err := e.executeAction(ctx, action, instance, handler)
			if err != nil {
				instance.Status = StatusFailed
				instance.Error = err
				return instance, nil
			}

			if action.NextStep != "" {
				instance.CurrentStep = action.NextStep
				break
			}
		}

		if instance.CurrentStep == step.ID {
			instance.CurrentStep = step.NextStep
		}

		if instance.CurrentStep == "" {
			break
		}
	}

	instance.Status = StatusCompleted
	return instance, nil
}

func (e *DefaultWorkFlowEngine) evaluateCondition(cond *Condition, vars map[string]interface{}) bool {
	if cond.Type == ConditionAlways {
		return true
	}

	value, ok := vars[cond.Variable]
	if !ok {
		return cond.Invert
	}

	var result bool
	switch cond.Type {
	case ConditionEquals:
		result = fmt.Sprintf("%v", value) == fmt.Sprintf("%v", cond.Expected)
	case ConditionNotEquals:
		result = fmt.Sprintf("%v", value) != fmt.Sprintf("%v", cond.Expected)
	case ConditionContains:
		result = strings.Contains(fmt.Sprintf("%v", value), fmt.Sprintf("%v", cond.Expected))
	case ConditionMatches:
		re, err := regexp.Compile(fmt.Sprintf("%v", cond.Expected))
		result = err == nil && re.MatchString(fmt.Sprintf("%v", value))
	case ConditionGreater:
		result = e.compareNumeric(value, cond.Expected) > 0
	case ConditionLess:
		result = e.compareNumeric(value, cond.Expected) < 0
	default:
		result = false
	}

	if cond.Invert {
		return !result
	}
	return result
}

func (e *DefaultWorkFlowEngine) compareNumeric(a, b interface{}) int {
	af := e.toFloat(a)
	bf := e.toFloat(b)
	if af > bf {
		return 1
	}
	if af < bf {
		return -1
	}
	return 0
}

func (e *DefaultWorkFlowEngine) toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

func (e *DefaultWorkFlowEngine) executeAction(ctx context.Context, action Action, instance *WorkFlowInstance, handler agent.LoopEventHandler) error {
	switch action.Type {
	case ActionToolCall:
		return e.executeToolCall(ctx, action, instance, handler)
	case ActionDelay:
		time.Sleep(time.Duration(action.DelaySeconds) * time.Second)
		return nil
	case ActionLog:
		if handler != nil {
			handler(agent.LoopEvent{
				Type: "thinking",
				Data: action.Message,
			})
		}
		return nil
	case ActionSetVariable:
		instance.Variables[action.VariableName] = action.VariableValue
		return nil
	case ActionExit:
		instance.CurrentStep = ""
		return nil
	default:
		return fmt.Errorf("unknown action type %s", action.Type)
	}
}

func (e *DefaultWorkFlowEngine) executeToolCall(ctx context.Context, action Action, instance *WorkFlowInstance, handler agent.LoopEventHandler) error {
	tool, err := e.toolRegistry.GetTool(action.ToolName)
	if err != nil {
		return fmt.Errorf("tool %s not found: %w", action.ToolName, err)
	}

	params := map[string]any{}
	for k, v := range action.ToolParams {
		if str, ok := v.(string); ok {
			params[k] = e.replaceVariables(str, instance.Variables)
		} else {
			params[k] = v
		}
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		return err
	}

	instance.Results[action.ToolName] = result

	if result.Data != nil {
		if statusMap, ok := result.Data.(map[string]interface{}); ok {
			for k, v := range statusMap {
				instance.Variables[k] = v
			}
		}
	}

	if handler != nil {
		handler(agent.LoopEvent{
			Type: "tool_result",
			Data: agent.ToolExecutionResult{
				ToolName:  action.ToolName,
				Arguments: params,
				Result:    result,
				Duration:  0,
			},
		})
	}

	return nil
}

func (e *DefaultWorkFlowEngine) replaceVariables(str string, vars map[string]interface{}) string {
	for k, v := range vars {
		str = strings.ReplaceAll(str, fmt.Sprintf("{{%s}}", k), fmt.Sprintf("%v", v))
	}
	return str
}
