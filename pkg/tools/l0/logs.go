package l0

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type LogsTool struct {
	client *kubernetes.Clientset
}

func NewLogsTool(client *kubernetes.Clientset) *LogsTool {
	return &LogsTool{
		client: client,
	}
}

func (t *LogsTool) Name() string {
	return "k8s_logs"
}

func (t *LogsTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL0
}

func (t *LogsTool) Description() string {
	return "Get logs from Kubernetes pods"
}

func (t *LogsTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	namespace, _ := params["namespace"].(string)
	podName, _ := params["pod_name"].(string)
	containerName, _ := params["container_name"].(string)

	tailLines, _ := params["tail_lines"].(int)
	if tailLines <= 0 {
		tailLines = 100
	}

	timestamps, _ := params["timestamps"].(bool)

	if podName == "" {
		return &tools.ToolResult{
			Success:   false,
			Error:     "pod_name is required",
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("pod_name is required")
	}

	logs, err := t.getPodLogs(ctx, namespace, podName, containerName, tailLines, timestamps)
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
		Data:      logs,
		Message:   fmt.Sprintf("Successfully retrieved logs from pod %s/%s", namespace, podName),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type PodLogs struct {
	PodName       string
	Namespace     string
	ContainerName string
	Logs          string
	LogLines      []string
}

func (t *LogsTool) getPodLogs(ctx context.Context, namespace, podName, containerName string, tailLines int, timestamps bool) (*PodLogs, error) {
	pod, err := t.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	tail := int64(tailLines)
	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  &tail,
		Timestamps: timestamps,
	}

	req := t.client.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	logStream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get log stream for pod %s/%s: %w", namespace, podName, err)
	}
	defer func() { _ = logStream.Close() }()

	var logsBuilder strings.Builder
	var logLines []string

	reader := bufio.NewReader(logStream)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			break
		}
		if line != "" {
			logsBuilder.WriteString(line)
			logLines = append(logLines, line)
		}
		if err == io.EOF {
			break
		}
	}

	return &PodLogs{
		PodName:       podName,
		Namespace:     namespace,
		ContainerName: containerName,
		Logs:          logsBuilder.String(),
		LogLines:      logLines,
	}, nil
}

func (t *LogsTool) StreamLogs(ctx context.Context, namespace, podName, containerName string, since time.Duration) (<-chan string, <-chan error, error) {
	tail := int64(100)
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    true,
		TailLines: &tail,
	}

	if since > 0 {
		sinceSeconds := int64(since.Seconds())
		logOptions.SinceSeconds = &sinceSeconds
	}

	req := t.client.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	logStream, err := req.Stream(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get log stream: %w", err)
	}

	logCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(logCh)
		defer close(errCh)
		defer func() { _ = logStream.Close() }()

		reader := bufio.NewReader(logStream)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					errCh <- err
				}
				return
			}
			if line != "" {
				logCh <- line
			}
		}
	}()

	return logCh, errCh, nil
}
