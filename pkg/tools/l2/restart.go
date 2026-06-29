package l2

import (
	"context"
	"fmt"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RestartTool struct {
	client *kubernetes.Clientset
}

func NewRestartTool(client *kubernetes.Clientset) *RestartTool {
	return &RestartTool{
		client: client,
	}
}

func (t *RestartTool) Name() string {
	return "k8s_restart"
}

func (t *RestartTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL2
}

func (t *RestartTool) Description() string {
	return "Restart Kubernetes resources (deployments, statefulsets, daemonsets)"
}

func (t *RestartTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	resourceType, _ := params["resource_type"].(string)
	namespace, _ := params["namespace"].(string)
	name, _ := params["name"].(string)

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

	var result any
	var err error

	switch resourceType {
	case "deployment", "deployments", "deploy":
		result, err = t.restartDeployment(ctx, namespace, name)
	case "statefulset", "statefulsets", "sts":
		result, err = t.restartStatefulSet(ctx, namespace, name)
	case "daemonset", "daemonsets", "ds":
		result, err = t.restartDaemonSet(ctx, namespace, name)
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
		Message:   fmt.Sprintf("Successfully triggered restart of %s/%s", resourceType, name),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type RestartResult struct {
	ResourceType string
	Name         string
	Namespace    string
	RestartedAt  time.Time
	Status       string
}

func (t *RestartTool) restartDeployment(ctx context.Context, namespace, name string) (*RestartResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = t.client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to restart deployment %s/%s: %w", namespace, name, err)
	}

	return &RestartResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		RestartedAt:  time.Now(),
		Status:       "Rolling restart triggered",
	}, nil
}

func (t *RestartTool) restartStatefulSet(ctx context.Context, namespace, name string) (*RestartResult, error) {
	sts, err := t.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get statefulset %s/%s: %w", namespace, name, err)
	}

	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = make(map[string]string)
	}
	sts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = t.client.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to restart statefulset %s/%s: %w", namespace, name, err)
	}

	return &RestartResult{
		ResourceType: "statefulset",
		Name:         name,
		Namespace:    namespace,
		RestartedAt:  time.Now(),
		Status:       "Rolling restart triggered",
	}, nil
}

func (t *RestartTool) restartDaemonSet(ctx context.Context, namespace, name string) (*RestartResult, error) {
	ds, err := t.client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get daemonset %s/%s: %w", namespace, name, err)
	}

	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = make(map[string]string)
	}
	ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = t.client.AppsV1().DaemonSets(namespace).Update(ctx, ds, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to restart daemonset %s/%s: %w", namespace, name, err)
	}

	return &RestartResult{
		ResourceType: "daemonset",
		Name:         name,
		Namespace:    namespace,
		RestartedAt:  time.Now(),
		Status:       "Rolling restart triggered",
	}, nil
}
