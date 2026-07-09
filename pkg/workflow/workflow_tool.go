package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingshu/lingshu/pkg/tools"
)

type WorkFlowToolImpl struct {
	engine *DefaultWorkFlowEngine
}

func NewWorkFlowTool(engine *DefaultWorkFlowEngine) *WorkFlowToolImpl {
	return &WorkFlowToolImpl{engine: engine}
}

func (t *WorkFlowToolImpl) Name() string {
	return "k8s_workflow"
}

func (t *WorkFlowToolImpl) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL1
}

func (t *WorkFlowToolImpl) Description() string {
	return "Execute predefined workflows for automated diagnosis and remediation"
}

func (t *WorkFlowToolImpl) ParameterSchema() map[string]interface{} {
	return map[string]interface{}{
		"workflow_id": map[string]interface{}{
			"type":        "string",
			"description": "ID of the workflow to execute",
		},
		"params": map[string]interface{}{
			"type":        "object",
			"description": "Initial parameters/variables for the workflow",
		},
	}
}

func (t *WorkFlowToolImpl) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	wfID, ok := params["workflow_id"].(string)
	if !ok {
		return nil, fmt.Errorf("workflow_id is required")
	}

	initialVars := map[string]interface{}{}
	if p, ok := params["params"].(map[string]interface{}); ok {
		initialVars = p
	}

	instance, err := t.engine.Execute(ctx, wfID, initialVars, nil)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(map[string]interface{}{
		"id":         instance.ID,
		"workflow_id": instance.WorkFlowID,
		"status":     string(instance.Status),
		"variables":  instance.Variables,
		"results":    instance.Results,
	})
	if err != nil {
		return nil, err
	}

	msg := fmt.Sprintf("Workflow %s executed successfully. Status: %s", wfID, instance.Status)
	if instance.Error != nil {
		msg = fmt.Sprintf("Workflow %s failed: %v", wfID, instance.Error)
	}

	return &tools.ToolResult{
		Data:    data,
		Message: msg,
	}, nil
}

func (t *WorkFlowToolImpl) GetWorkFlowEngine() WorkFlowEngine {
	return t.engine
}
