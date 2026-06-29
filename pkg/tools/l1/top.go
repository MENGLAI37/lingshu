package l1

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

type TopTool struct {
	client        *kubernetes.Clientset
	metricsClient *versioned.Clientset
}

func NewTopTool(client *kubernetes.Clientset, metricsClient *versioned.Clientset) *TopTool {
	return &TopTool{
		client:        client,
		metricsClient: metricsClient,
	}
}

func (t *TopTool) Name() string {
	return "k8s_top"
}

func (t *TopTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL1
}

func (t *TopTool) Description() string {
	return "Display resource (CPU/memory) usage of pods or nodes"
}

func (t *TopTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	resourceType, _ := params["resource_type"].(string)
	namespace, _ := params["namespace"].(string)
	allNamespaces, _ := params["all_namespaces"].(bool)
	sortBy, _ := params["sort_by"].(string)
	limit, _ := params["limit"].(int)

	if resourceType == "" {
		resourceType = "pod"
	}

	var result any
	var err error

	switch resourceType {
	case "pod", "pods":
		result, err = t.topPods(ctx, namespace, allNamespaces, sortBy, limit)
	case "node", "nodes":
		result, err = t.topNodes(ctx, sortBy, limit)
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
		Message:   fmt.Sprintf("Successfully retrieved %s resource usage", resourceType),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type PodMetrics struct {
	Namespace     string
	PodName       string
	CPU           resource.Quantity
	Memory        resource.Quantity
	CPUPercent    float64
	MemoryPercent float64
}

type TopPodsResult struct {
	Pods []PodMetrics
	TotalCPU    resource.Quantity
	TotalMemory resource.Quantity
	Count       int
}

func (t *TopTool) topPods(ctx context.Context, namespace string, allNamespaces bool, sortBy string, limit int) (*TopPodsResult, error) {
	if t.metricsClient == nil {
		return nil, fmt.Errorf("metrics client is not available")
	}

	ns := namespace
	if allNamespaces {
		ns = ""
	}

	podMetrics, err := t.metricsClient.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	pods, err := t.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	podMap := make(map[string]*corev1.Pod)
	for i := range pods.Items {
		pod := &pods.Items[i]
		podMap[pod.Namespace+"/"+pod.Name] = pod
	}

	var podMetricsList []PodMetrics
	var totalCPU, totalMemory resource.Quantity

	for _, pm := range podMetrics.Items {
		key := pm.Namespace + "/" + pm.Name
		pod, ok := podMap[key]
		if !ok {
			continue
		}

		var cpuTotal, memoryTotal resource.Quantity
		for _, container := range pm.Containers {
			cpu := container.Usage[corev1.ResourceCPU]
			mem := container.Usage[corev1.ResourceMemory]
			cpuTotal.Add(cpu)
			memoryTotal.Add(mem)
		}

		var cpuPercent, memoryPercent float64
		for _, container := range pod.Spec.Containers {
			cpuLimit := container.Resources.Limits[corev1.ResourceCPU]
			if cpuLimit.MilliValue() > 0 {
				cpuPercent = float64(cpuTotal.MilliValue()) / float64(cpuLimit.MilliValue()) * 100
			}
			memLimit := container.Resources.Limits[corev1.ResourceMemory]
			if memLimit.Value() > 0 {
				memoryPercent = float64(memoryTotal.Value()) / float64(memLimit.Value()) * 100
			}
		}

		podMetricsList = append(podMetricsList, PodMetrics{
			Namespace:     pm.Namespace,
			PodName:       pm.Name,
			CPU:           cpuTotal,
			Memory:        memoryTotal,
			CPUPercent:    cpuPercent,
			MemoryPercent: memoryPercent,
		})

		totalCPU.Add(cpuTotal)
		totalMemory.Add(memoryTotal)
	}

	switch sortBy {
	case "cpu":
		sort.Slice(podMetricsList, func(i, j int) bool {
			return podMetricsList[i].CPU.MilliValue() > podMetricsList[j].CPU.MilliValue()
		})
	case "memory":
		sort.Slice(podMetricsList, func(i, j int) bool {
			return podMetricsList[i].Memory.Value() > podMetricsList[j].Memory.Value()
		})
	default:
		sort.Slice(podMetricsList, func(i, j int) bool {
			return podMetricsList[i].CPU.MilliValue() > podMetricsList[j].CPU.MilliValue()
		})
	}

	if limit > 0 && limit < len(podMetricsList) {
		podMetricsList = podMetricsList[:limit]
	}

	return &TopPodsResult{
		Pods:        podMetricsList,
		TotalCPU:    totalCPU,
		TotalMemory: totalMemory,
		Count:       len(podMetricsList),
	}, nil
}

type NodeMetrics struct {
	NodeName      string
	CPU           resource.Quantity
	Memory        resource.Quantity
	CPUPercent    float64
	MemoryPercent float64
	Status        string
}

type TopNodesResult struct {
	Nodes []NodeMetrics
	Count int
}

func (t *TopTool) topNodes(ctx context.Context, sortBy string, limit int) (*TopNodesResult, error) {
	if t.metricsClient == nil {
		return nil, fmt.Errorf("metrics client is not available")
	}

	nodeMetrics, err := t.metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node metrics: %w", err)
	}

	nodes, err := t.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodeMap := make(map[string]*corev1.Node)
	for i := range nodes.Items {
		node := &nodes.Items[i]
		nodeMap[node.Name] = node
	}

	var nodeMetricsList []NodeMetrics

	for _, nm := range nodeMetrics.Items {
		node, ok := nodeMap[nm.Name]
		if !ok {
			continue
		}

		cpuPercent := float64(0)
		memoryPercent := float64(0)

		cpuUsage := nm.Usage[corev1.ResourceCPU]
		memUsage := nm.Usage[corev1.ResourceMemory]
		cpuAllocatable := node.Status.Allocatable[corev1.ResourceCPU]
		memAllocatable := node.Status.Allocatable[corev1.ResourceMemory]

		if cpuAllocatable.MilliValue() > 0 {
			cpuPercent = float64(cpuUsage.MilliValue()) / float64(cpuAllocatable.MilliValue()) * 100
		}
		if memAllocatable.Value() > 0 {
			memoryPercent = float64(memUsage.Value()) / float64(memAllocatable.Value()) * 100
		}

		status := "Ready"
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
				status = "NotReady"
				break
			}
		}

		nodeMetricsList = append(nodeMetricsList, NodeMetrics{
			NodeName:      nm.Name,
			CPU:           nm.Usage[corev1.ResourceCPU],
			Memory:        nm.Usage[corev1.ResourceMemory],
			CPUPercent:    cpuPercent,
			MemoryPercent: memoryPercent,
			Status:        status,
		})
	}

	switch sortBy {
	case "cpu":
		sort.Slice(nodeMetricsList, func(i, j int) bool {
			return nodeMetricsList[i].CPU.MilliValue() > nodeMetricsList[j].CPU.MilliValue()
		})
	case "memory":
		sort.Slice(nodeMetricsList, func(i, j int) bool {
			return nodeMetricsList[i].Memory.Value() > nodeMetricsList[j].Memory.Value()
		})
	default:
		sort.Slice(nodeMetricsList, func(i, j int) bool {
			return nodeMetricsList[i].CPU.MilliValue() > nodeMetricsList[j].CPU.MilliValue()
		})
	}

	if limit > 0 && limit < len(nodeMetricsList) {
		nodeMetricsList = nodeMetricsList[:limit]
	}

	return &TopNodesResult{
		Nodes: nodeMetricsList,
		Count: len(nodeMetricsList),
	}, nil
}
