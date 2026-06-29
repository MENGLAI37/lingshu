package l2

import (
	"context"
	"fmt"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ScaleTool struct {
	client *kubernetes.Clientset
}

func NewScaleTool(client *kubernetes.Clientset) *ScaleTool {
	return &ScaleTool{
		client: client,
	}
}

func (t *ScaleTool) Name() string {
	return "k8s_scale"
}

func (t *ScaleTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL2
}

func (t *ScaleTool) Description() string {
	return "Scale Kubernetes resources (deployments, replicasets, statefulsets, HPA)"
}

func (t *ScaleTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	resourceType, _ := params["resource_type"].(string)
	namespace, _ := params["namespace"].(string)
	name, _ := params["name"].(string)
	replicas, _ := params["replicas"].(int)

	if resourceType == "" || name == "" {
		return &tools.ToolResult{
			Success:   false,
			Error:     "resource_type and name are required",
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("resource_type and name are required")
	}

	if replicas < 0 {
		return &tools.ToolResult{
			Success:   false,
			Error:     "replicas must be >= 0",
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("replicas must be >= 0")
	}

	var result any
	var err error

	switch resourceType {
	case "deployment", "deployments", "deploy":
		result, err = t.scaleDeployment(ctx, namespace, name, int32(replicas))
	case "statefulset", "statefulsets", "sts":
		result, err = t.scaleStatefulSet(ctx, namespace, name, int32(replicas))
	case "replicaset", "replicasets", "rs":
		result, err = t.scaleReplicaSet(ctx, namespace, name, int32(replicas))
	case "hpa", "horizontalpodautoscaler":
		minReplicas, _ := params["min_replicas"].(int)
		maxReplicas, _ := params["max_replicas"].(int)
		result, err = t.updateHPA(ctx, namespace, name, int32(minReplicas), int32(maxReplicas))
	default:
		return &tools.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("unsupported resource type: %s", resourceType),
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	if err != nil {
		return &tools.ToolResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, err
	}

	return &tools.ToolResult{
		Success:   true,
		Data:      result,
		Message:   fmt.Sprintf("Successfully scaled %s/%s to %d replicas", resourceType, name, replicas),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type ScaleResult struct {
	ResourceType string
	Name         string
	Namespace    string
	OldReplicas  int32
	NewReplicas  int32
	Status       string
}

func (t *ScaleTool) scaleDeployment(ctx context.Context, namespace, name string, replicas int32) (*ScaleResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	oldReplicas := int32(0)
	if deploy.Spec.Replicas != nil {
		oldReplicas = *deploy.Spec.Replicas
	}

	deploy.Spec.Replicas = &replicas

	_, err = t.client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to scale deployment %s/%s: %w", namespace, name, err)
	}

	return &ScaleResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		OldReplicas:  oldReplicas,
		NewReplicas:  replicas,
		Status:       fmt.Sprintf("Scaling from %d to %d replicas", oldReplicas, replicas),
	}, nil
}

func (t *ScaleTool) scaleStatefulSet(ctx context.Context, namespace, name string, replicas int32) (*ScaleResult, error) {
	sts, err := t.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get statefulset %s/%s: %w", namespace, name, err)
	}

	oldReplicas := int32(0)
	if sts.Spec.Replicas != nil {
		oldReplicas = *sts.Spec.Replicas
	}

	sts.Spec.Replicas = &replicas

	_, err = t.client.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to scale statefulset %s/%s: %w", namespace, name, err)
	}

	return &ScaleResult{
		ResourceType: "statefulset",
		Name:         name,
		Namespace:    namespace,
		OldReplicas:  oldReplicas,
		NewReplicas:  replicas,
		Status:       fmt.Sprintf("Scaling from %d to %d replicas", oldReplicas, replicas),
	}, nil
}

func (t *ScaleTool) scaleReplicaSet(ctx context.Context, namespace, name string, replicas int32) (*ScaleResult, error) {
	rs, err := t.client.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get replicaset %s/%s: %w", namespace, name, err)
	}

	oldReplicas := int32(0)
	if rs.Spec.Replicas != nil {
		oldReplicas = *rs.Spec.Replicas
	}

	rs.Spec.Replicas = &replicas

	_, err = t.client.AppsV1().ReplicaSets(namespace).Update(ctx, rs, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to scale replicaset %s/%s: %w", namespace, name, err)
	}

	return &ScaleResult{
		ResourceType: "replicaset",
		Name:         name,
		Namespace:    namespace,
		OldReplicas:  oldReplicas,
		NewReplicas:  replicas,
		Status:       fmt.Sprintf("Scaling from %d to %d replicas", oldReplicas, replicas),
	}, nil
}

type HPAResult struct {
	Name        string
	Namespace   string
	MinReplicas int32
	MaxReplicas int32
	CurrentReplicas int32
	Status      string
}

func (t *ScaleTool) updateHPA(ctx context.Context, namespace, name string, minReplicas, maxReplicas int32) (*HPAResult, error) {
	hpa, err := t.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get HPA %s/%s: %w", namespace, name, err)
	}

	if minReplicas > 0 {
		hpa.Spec.MinReplicas = &minReplicas
	}
	if maxReplicas > 0 {
		hpa.Spec.MaxReplicas = maxReplicas
	}

	updated, err := t.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(ctx, hpa, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update HPA %s/%s: %w", namespace, name, err)
	}

	return &HPAResult{
		Name:             name,
		Namespace:        namespace,
		MinReplicas:      *updated.Spec.MinReplicas,
		MaxReplicas:      updated.Spec.MaxReplicas,
		CurrentReplicas:  updated.Status.CurrentReplicas,
		Status:           fmt.Sprintf("HPA updated: min=%d, max=%d", *updated.Spec.MinReplicas, updated.Spec.MaxReplicas),
	}, nil
}
