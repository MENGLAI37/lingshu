package l2

import (
	"context"
	"fmt"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type PatchTool struct {
	client *kubernetes.Clientset
}

func NewPatchTool(client *kubernetes.Clientset) *PatchTool {
	return &PatchTool{
		client: client,
	}
}

func (t *PatchTool) Name() string {
	return "k8s_patch"
}

func (t *PatchTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL2
}

func (t *PatchTool) Description() string {
	return "Patch Kubernetes resources (update images, annotations, labels, etc.)"
}

func (t *PatchTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	resourceType, _ := params["resource_type"].(string)
	namespace, _ := params["namespace"].(string)
	name, _ := params["name"].(string)
	patchType, _ := params["patch_type"].(string)
	patchData, _ := params["patch_data"].(string)

	if resourceType == "" || name == "" || patchData == "" {
		return &tools.ToolResult{
			Success:   false,
			Error:     "resource_type, name, and patch_data are required",
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("resource_type, name, and patch_data are required")
	}

	if patchType == "" {
		patchType = "strategic"
	}

	var patchTypeEnum types.PatchType
	switch patchType {
	case "json":
		patchTypeEnum = types.JSONPatchType
	case "merge":
		patchTypeEnum = types.MergePatchType
	case "strategic":
		patchTypeEnum = types.StrategicMergePatchType
	case "apply":
		patchTypeEnum = types.ApplyPatchType
	default:
		return &tools.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("unsupported patch type: %s", patchType),
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("unsupported patch type: %s", patchType)
	}

	var result any
	var err error

	switch resourceType {
	case "deployment", "deployments", "deploy":
		result, err = t.patchDeployment(ctx, namespace, name, patchTypeEnum, []byte(patchData))
	case "pod", "pods":
		result, err = t.patchPod(ctx, namespace, name, patchTypeEnum, []byte(patchData))
	case "service", "services", "svc":
		result, err = t.patchService(ctx, namespace, name, patchTypeEnum, []byte(patchData))
	case "configmap", "configmaps", "cm":
		result, err = t.patchConfigMap(ctx, namespace, name, patchTypeEnum, []byte(patchData))
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
		Message:   fmt.Sprintf("Successfully patched %s/%s", resourceType, name),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type PatchResult struct {
	ResourceType string
	Name         string
	Namespace    string
	PatchType    string
	Status       string
}

func (t *PatchTool) patchDeployment(ctx context.Context, namespace, name string, patchType types.PatchType, data []byte) (*PatchResult, error) {
	_, err := t.client.AppsV1().Deployments(namespace).Patch(ctx, name, patchType, data, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch deployment %s/%s: %w", namespace, name, err)
	}

	return &PatchResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		PatchType:    string(patchType),
		Status:       "Patched successfully",
	}, nil
}

func (t *PatchTool) patchPod(ctx context.Context, namespace, name string, patchType types.PatchType, data []byte) (*PatchResult, error) {
	_, err := t.client.CoreV1().Pods(namespace).Patch(ctx, name, patchType, data, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch pod %s/%s: %w", namespace, name, err)
	}

	return &PatchResult{
		ResourceType: "pod",
		Name:         name,
		Namespace:    namespace,
		PatchType:    string(patchType),
		Status:       "Patched successfully",
	}, nil
}

func (t *PatchTool) patchService(ctx context.Context, namespace, name string, patchType types.PatchType, data []byte) (*PatchResult, error) {
	_, err := t.client.CoreV1().Services(namespace).Patch(ctx, name, patchType, data, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch service %s/%s: %w", namespace, name, err)
	}

	return &PatchResult{
		ResourceType: "service",
		Name:         name,
		Namespace:    namespace,
		PatchType:    string(patchType),
		Status:       "Patched successfully",
	}, nil
}

func (t *PatchTool) patchConfigMap(ctx context.Context, namespace, name string, patchType types.PatchType, data []byte) (*PatchResult, error) {
	_, err := t.client.CoreV1().ConfigMaps(namespace).Patch(ctx, name, patchType, data, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch configmap %s/%s: %w", namespace, name, err)
	}

	return &PatchResult{
		ResourceType: "configmap",
		Name:         name,
		Namespace:    namespace,
		PatchType:    string(patchType),
		Status:       "Patched successfully",
	}, nil
}

type ImageUpdateResult struct {
	ResourceType string
	Name         string
	Namespace    string
	Container    string
	OldImage     string
	NewImage     string
	Status       string
}

func (t *PatchTool) UpdateDeploymentImage(ctx context.Context, namespace, name, container, image string) (*ImageUpdateResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	oldImage := ""
	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == container {
			oldImage = c.Image
			deploy.Spec.Template.Spec.Containers[i].Image = image
			break
		}
	}

	if oldImage == "" {
		return nil, fmt.Errorf("container %s not found in deployment %s/%s", container, namespace, name)
	}

	_, err = t.client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment image %s/%s: %w", namespace, name, err)
	}

	return &ImageUpdateResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		Container:    container,
		OldImage:     oldImage,
		NewImage:     image,
		Status:       fmt.Sprintf("Image updated from %s to %s", oldImage, image),
	}, nil
}
