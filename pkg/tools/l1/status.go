package l1

import (
	"context"
	"fmt"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type StatusTool struct {
	client *kubernetes.Clientset
}

func NewStatusTool(client *kubernetes.Clientset) *StatusTool {
	return &StatusTool{
		client: client,
	}
}

func (t *StatusTool) Name() string {
	return "k8s_status"
}

func (t *StatusTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL1
}

func (t *StatusTool) Description() string {
	return "Get cluster and resource status summary"
}

func (t *StatusTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	statusType, _ := params["status_type"].(string)
	namespace, _ := params["namespace"].(string)

	if statusType == "" {
		statusType = "cluster"
	}

	var result any
	var err error

	switch statusType {
	case "cluster":
		result, err = t.clusterStatus(ctx)
	case "namespace":
		result, err = t.namespaceStatus(ctx, namespace)
	case "deployments":
		result, err = t.deploymentsStatus(ctx, namespace)
	case "nodes":
		result, err = t.nodesStatus(ctx)
	default:
		return &tools.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("unsupported status type: %s", statusType),
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("unsupported status type: %s", statusType)
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
		Message:   fmt.Sprintf("Successfully retrieved %s status", statusType),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type ClusterStatus struct {
	NodeCount       int
	ReadyNodes      int
	NotReadyNodes   int
	PodCount        int
	RunningPods     int
	PendingPods     int
	FailedPods      int
	DeploymentCount int
	ServiceCount    int
	NamespaceCount  int
	KubernetesVersion string
}

func (t *StatusTool) clusterStatus(ctx context.Context) (*ClusterStatus, error) {
	nodes, err := t.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	readyNodes := 0
	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}

	pods, err := t.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	runningPods := 0
	pendingPods := 0
	failedPods := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
		case corev1.PodFailed:
			failedPods++
		}
	}

	deployments, err := t.client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	services, err := t.client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	namespaces, err := t.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	version, err := t.client.Discovery().ServerVersion()
	k8sVersion := ""
	if err == nil {
		k8sVersion = version.GitVersion
	}

	return &ClusterStatus{
		NodeCount:         len(nodes.Items),
		ReadyNodes:        readyNodes,
		NotReadyNodes:     len(nodes.Items) - readyNodes,
		PodCount:          len(pods.Items),
		RunningPods:       runningPods,
		PendingPods:       pendingPods,
		FailedPods:        failedPods,
		DeploymentCount:   len(deployments.Items),
		ServiceCount:      len(services.Items),
		NamespaceCount:    len(namespaces.Items),
		KubernetesVersion: k8sVersion,
	}, nil
}

type NamespaceStatus struct {
	Namespace       string
	PodCount        int
	RunningPods     int
	PendingPods     int
	FailedPods      int
	DeploymentCount int
	ServiceCount    int
	ConfigMapCount  int
}

func (t *StatusTool) namespaceStatus(ctx context.Context, namespace string) (*NamespaceStatus, error) {
	if namespace == "" {
		namespace = "default"
	}

	pods, err := t.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in %s: %w", namespace, err)
	}

	runningPods := 0
	pendingPods := 0
	failedPods := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
		case corev1.PodFailed:
			failedPods++
		}
	}

	deployments, err := t.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments in %s: %w", namespace, err)
	}

	services, err := t.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services in %s: %w", namespace, err)
	}

	configMaps, err := t.client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps in %s: %w", namespace, err)
	}

	return &NamespaceStatus{
		Namespace:       namespace,
		PodCount:        len(pods.Items),
		RunningPods:     runningPods,
		PendingPods:     pendingPods,
		FailedPods:      failedPods,
		DeploymentCount: len(deployments.Items),
		ServiceCount:    len(services.Items),
		ConfigMapCount:  len(configMaps.Items),
	}, nil
}

type DeploymentStatusInfo struct {
	Name              string
	Namespace         string
	Replicas          int32
	ReadyReplicas     int32
	AvailableReplicas int32
	UpdatedReplicas   int32
	Status            string
}

type DeploymentsStatus struct {
	Deployments      []DeploymentStatusInfo
	TotalCount       int
	HealthyCount     int
	UnhealthyCount   int
}

func (t *StatusTool) deploymentsStatus(ctx context.Context, namespace string) (*DeploymentsStatus, error) {
	ns := namespace
	if ns == "" {
		ns = ""
	}

	deployments, err := t.client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	var depInfos []DeploymentStatusInfo
	healthyCount := 0

	for _, deploy := range deployments.Items {
		status := "Healthy"
		if deploy.Status.ReadyReplicas < deploy.Status.Replicas {
			status = "Unhealthy"
		} else if deploy.Generation > deploy.Status.ObservedGeneration {
			status = "Progressing"
		} else {
			healthyCount++
		}

		depInfos = append(depInfos, DeploymentStatusInfo{
			Name:              deploy.Name,
			Namespace:         deploy.Namespace,
			Replicas:          deploy.Status.Replicas,
			ReadyReplicas:     deploy.Status.ReadyReplicas,
			AvailableReplicas: deploy.Status.AvailableReplicas,
			UpdatedReplicas:   deploy.Status.UpdatedReplicas,
			Status:            status,
		})
	}

	return &DeploymentsStatus{
		Deployments:    depInfos,
		TotalCount:     len(deployments.Items),
		HealthyCount:   healthyCount,
		UnhealthyCount: len(deployments.Items) - healthyCount,
	}, nil
}

type NodeStatusInfo struct {
	Name     string
	Status   string
	Role     string
	Version  string
	PodCIDR  string
	InternalIP string
}

type NodesStatus struct {
	Nodes         []NodeStatusInfo
	TotalCount    int
	ReadyCount    int
	NotReadyCount int
}

func (t *StatusTool) nodesStatus(ctx context.Context) (*NodesStatus, error) {
	nodes, err := t.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var nodeInfos []NodeStatusInfo
	readyCount := 0

	for _, node := range nodes.Items {
		status := "NotReady"
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				status = "Ready"
				readyCount++
				break
			}
		}

		role := "worker"
		for label := range node.Labels {
			if label == "node-role.kubernetes.io/control-plane" || label == "node-role.kubernetes.io/master" {
				role = "control-plane"
				break
			}
		}

		internalIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				internalIP = addr.Address
				break
			}
		}

		nodeInfos = append(nodeInfos, NodeStatusInfo{
			Name:       node.Name,
			Status:     status,
			Role:       role,
			Version:    node.Status.NodeInfo.KubeletVersion,
			PodCIDR:    node.Spec.PodCIDR,
			InternalIP: internalIP,
		})
	}

	return &NodesStatus{
		Nodes:         nodeInfos,
		TotalCount:    len(nodes.Items),
		ReadyCount:    readyCount,
		NotReadyCount: len(nodes.Items) - readyCount,
	}, nil
}
