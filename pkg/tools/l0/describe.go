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

type DescribeTool struct {
	client *kubernetes.Clientset
}

func NewDescribeTool(client *kubernetes.Clientset) *DescribeTool {
	return &DescribeTool{
		client: client,
	}
}

func (t *DescribeTool) Name() string {
	return "k8s_describe"
}

func (t *DescribeTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL0
}

func (t *DescribeTool) Description() string {
	return "Describe Kubernetes resources with detailed information"
}

func (t *DescribeTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
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
	case "pod", "pods":
		result, err = t.describePod(ctx, namespace, name)
	case "deployment", "deployments":
		result, err = t.describeDeployment(ctx, namespace, name)
	case "service", "services", "svc":
		result, err = t.describeService(ctx, namespace, name)
	case "ingress", "ingresses", "ing":
		result, err = t.describeIngress(ctx, namespace, name)
	case "configmap", "configmaps", "cm":
		result, err = t.describeConfigMap(ctx, namespace, name)
	case "node", "nodes":
		result, err = t.describeNode(ctx, name)
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
		Message:   fmt.Sprintf("Successfully described %s/%s", resourceType, name),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type PodDescription struct {
	Pod       *corev1.Pod
	Events    []corev1.Event
	Status    PodStatusInfo
}

type PodStatusInfo struct {
	Phase       string
	Ready       bool
	RestartCount int32
	PodIP       string
	HostIP      string
	StartTime   *metav1.Time
}

func (t *DescribeTool) describePod(ctx context.Context, namespace, name string) (*PodDescription, error) {
	pod, err := t.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
	}

	events, err := t.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", name),
	})
	if err != nil {
		events = &corev1.EventList{}
	}

	ready := false
	restartCount := int32(0)
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			ready = true
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		restartCount += cs.RestartCount
	}

	return &PodDescription{
		Pod:    pod,
		Events: events.Items,
		Status: PodStatusInfo{
			Phase:        string(pod.Status.Phase),
			Ready:        ready,
			RestartCount: restartCount,
			PodIP:        pod.Status.PodIP,
			HostIP:       pod.Status.HostIP,
			StartTime:    pod.Status.StartTime,
		},
	}, nil
}

type DeploymentDescription struct {
	Deployment  *appsv1.Deployment
	ReplicaSets []appsv1.ReplicaSet
	Events      []corev1.Event
}

func (t *DescribeTool) describeDeployment(ctx context.Context, namespace, name string) (*DeploymentDescription, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	rsList, err := t.client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deploy.Spec.Selector),
	})
	if err != nil {
		rsList = &appsv1.ReplicaSetList{}
	}

	events, err := t.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Deployment", name),
	})
	if err != nil {
		events = &corev1.EventList{}
	}

	return &DeploymentDescription{
		Deployment:  deploy,
		ReplicaSets: rsList.Items,
		Events:      events.Items,
	}, nil
}

type ServiceDescription struct {
	Service *corev1.Service
	Endpoints *corev1.Endpoints
	Events  []corev1.Event
}

func (t *DescribeTool) describeService(ctx context.Context, namespace, name string) (*ServiceDescription, error) {
	svc, err := t.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", namespace, name, err)
	}

	endpoints, err := t.client.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		endpoints = &corev1.Endpoints{}
	}

	events, err := t.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Service", name),
	})
	if err != nil {
		events = &corev1.EventList{}
	}

	return &ServiceDescription{
		Service:   svc,
		Endpoints: endpoints,
		Events:    events.Items,
	}, nil
}

type IngressDescription struct {
	Ingress *networkingv1.Ingress
	Events  []corev1.Event
}

func (t *DescribeTool) describeIngress(ctx context.Context, namespace, name string) (*IngressDescription, error) {
	ing, err := t.client.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ingress %s/%s: %w", namespace, name, err)
	}

	events, err := t.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Ingress", name),
	})
	if err != nil {
		events = &corev1.EventList{}
	}

	return &IngressDescription{
		Ingress: ing,
		Events:  events.Items,
	}, nil
}

type ConfigMapDescription struct {
	ConfigMap *corev1.ConfigMap
	Events    []corev1.Event
}

func (t *DescribeTool) describeConfigMap(ctx context.Context, namespace, name string) (*ConfigMapDescription, error) {
	cm, err := t.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s: %w", namespace, name, err)
	}

	events, err := t.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=ConfigMap", name),
	})
	if err != nil {
		events = &corev1.EventList{}
	}

	return &ConfigMapDescription{
		ConfigMap: cm,
		Events:    events.Items,
	}, nil
}

type NodeDescription struct {
	Node   *corev1.Node
	Events []corev1.Event
}

func (t *DescribeTool) describeNode(ctx context.Context, name string) (*NodeDescription, error) {
	node, err := t.client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", name, err)
	}

	events, err := t.client.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Node", name),
	})
	if err != nil {
		events = &corev1.EventList{}
	}

	return &NodeDescription{
		Node:   node,
		Events: events.Items,
	}, nil
}
