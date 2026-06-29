package l0

import (
	"context"
	"fmt"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type GetTool struct {
	client *kubernetes.Clientset
}

func NewGetTool(client *kubernetes.Clientset) *GetTool {
	return &GetTool{
		client: client,
	}
}

func (t *GetTool) Name() string {
	return "k8s_get"
}

func (t *GetTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL0
}

func (t *GetTool) Description() string {
	return "Get Kubernetes resources (pods, deployments, services, events, ingresses, configmaps)"
}

func (t *GetTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	resourceType, _ := params["resource_type"].(string)
	namespace, _ := params["namespace"].(string)
	name, _ := params["name"].(string)
	allNamespaces, _ := params["all_namespaces"].(bool)

	if resourceType == "" {
		return &tools.ToolResult{
			Success:   false,
			Error:     "resource_type is required",
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("resource_type is required")
	}

	var result any
	var err error

	switch resourceType {
	case "pod", "pods":
		result, err = t.getPods(ctx, namespace, name, allNamespaces)
	case "deployment", "deployments":
		result, err = t.getDeployments(ctx, namespace, name, allNamespaces)
	case "service", "services", "svc":
		result, err = t.getServices(ctx, namespace, name, allNamespaces)
	case "event", "events":
		result, err = t.getEvents(ctx, namespace, allNamespaces)
	case "ingress", "ingresses", "ing":
		result, err = t.getIngresses(ctx, namespace, name, allNamespaces)
	case "configmap", "configmaps", "cm":
		result, err = t.getConfigMaps(ctx, namespace, name, allNamespaces)
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
		Message:   fmt.Sprintf("Successfully retrieved %s", resourceType),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

func (t *GetTool) getPods(ctx context.Context, namespace, name string, allNamespaces bool) (*corev1.PodList, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}

	if name != "" {
		pod, err := t.client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s/%s: %w", ns, name, err)
		}
		return &corev1.PodList{Items: []corev1.Pod{*pod}}, nil
	}

	pods, err := t.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in %s: %w", ns, err)
	}
	return pods, nil
}

func (t *GetTool) getDeployments(ctx context.Context, namespace, name string, allNamespaces bool) (*appsv1.DeploymentList, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}

	if name != "" {
		deploy, err := t.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get deployment %s/%s: %w", ns, name, err)
		}
		return &appsv1.DeploymentList{Items: []appsv1.Deployment{*deploy}}, nil
	}

	deploys, err := t.client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments in %s: %w", ns, err)
	}
	return deploys, nil
}

func (t *GetTool) getServices(ctx context.Context, namespace, name string, allNamespaces bool) (*corev1.ServiceList, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}

	if name != "" {
		svc, err := t.client.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get service %s/%s: %w", ns, name, err)
		}
		return &corev1.ServiceList{Items: []corev1.Service{*svc}}, nil
	}

	svcs, err := t.client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services in %s: %w", ns, err)
	}
	return svcs, nil
}

func (t *GetTool) getEvents(ctx context.Context, namespace string, allNamespaces bool) (*corev1.EventList, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}

	events, err := t.client.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list events in %s: %w", ns, err)
	}
	return events, nil
}

func (t *GetTool) getIngresses(ctx context.Context, namespace, name string, allNamespaces bool) (*networkingv1.IngressList, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}

	if name != "" {
		ing, err := t.client.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get ingress %s/%s: %w", ns, name, err)
		}
		return &networkingv1.IngressList{Items: []networkingv1.Ingress{*ing}}, nil
	}

	ings, err := t.client.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses in %s: %w", ns, err)
	}
	return ings, nil
}

func (t *GetTool) getConfigMaps(ctx context.Context, namespace, name string, allNamespaces bool) (*corev1.ConfigMapList, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}

	if name != "" {
		cm, err := t.client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get configmap %s/%s: %w", ns, name, err)
		}
		return &corev1.ConfigMapList{Items: []corev1.ConfigMap{*cm}}, nil
	}

	cms, err := t.client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps in %s: %w", ns, err)
	}
	return cms, nil
}
