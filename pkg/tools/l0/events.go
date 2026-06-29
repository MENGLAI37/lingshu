package l0

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type EventsTool struct {
	client *kubernetes.Clientset
}

func NewEventsTool(client *kubernetes.Clientset) *EventsTool {
	return &EventsTool{
		client: client,
	}
}

func (t *EventsTool) Name() string {
	return "k8s_events"
}

func (t *EventsTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL0
}

func (t *EventsTool) Description() string {
	return "Get Kubernetes events with filtering options"
}

func (t *EventsTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	namespace, _ := params["namespace"].(string)
	allNamespaces, _ := params["all_namespaces"].(bool)
	fieldSelector, _ := params["field_selector"].(string)
	limit, _ := params["limit"].(int)
	if limit <= 0 {
		limit = 100
	}

	events, err := t.getEvents(ctx, namespace, allNamespaces, fieldSelector, limit)
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
		Data:      events,
		Message:   fmt.Sprintf("Successfully retrieved %d events", len(events.Events)),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type EventsResult struct {
	Events    []corev1.Event
	WarningCount int
	NormalCount  int
	TotalCount   int
}

func (t *EventsTool) getEvents(ctx context.Context, namespace string, allNamespaces bool, fieldSelector string, limit int) (*EventsResult, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}

	listOptions := metav1.ListOptions{
		FieldSelector: fieldSelector,
		Limit:         int64(limit),
	}

	eventList, err := t.client.CoreV1().Events(ns).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list events in %s: %w", ns, err)
	}

	sort.Slice(eventList.Items, func(i, j int) bool {
		return eventList.Items[i].LastTimestamp.After(eventList.Items[j].LastTimestamp.Time)
	})

	warningCount := 0
	normalCount := 0
	for _, event := range eventList.Items {
		switch event.Type {
		case corev1.EventTypeWarning:
			warningCount++
		case corev1.EventTypeNormal:
			normalCount++
		}
	}

	return &EventsResult{
		Events:       eventList.Items,
		WarningCount: warningCount,
		NormalCount:  normalCount,
		TotalCount:   len(eventList.Items),
	}, nil
}

func (t *EventsTool) GetEventsForPod(ctx context.Context, namespace, podName string, limit int) (*EventsResult, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)
	return t.getEvents(ctx, namespace, false, fieldSelector, limit)
}

func (t *EventsTool) GetEventsForDeployment(ctx context.Context, namespace, deployName string, limit int) (*EventsResult, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Deployment", deployName)
	return t.getEvents(ctx, namespace, false, fieldSelector, limit)
}

func (t *EventsTool) GetWarningEvents(ctx context.Context, namespace string, allNamespaces bool, limit int) (*EventsResult, error) {
	fieldSelector := "type=Warning"
	return t.getEvents(ctx, namespace, allNamespaces, fieldSelector, limit)
}

func (t *EventsTool) GetRecentEvents(ctx context.Context, namespace string, allNamespaces bool, since time.Duration, limit int) (*EventsResult, error) {
	result, err := t.getEvents(ctx, namespace, allNamespaces, "", limit)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-since)
	filtered := make([]corev1.Event, 0)
	warningCount := 0
	normalCount := 0

	for _, event := range result.Events {
		if event.LastTimestamp.Time.After(cutoff) {
			filtered = append(filtered, event)
			switch event.Type {
			case corev1.EventTypeWarning:
				warningCount++
			case corev1.EventTypeNormal:
				normalCount++
			}
		}
	}

	return &EventsResult{
		Events:       filtered,
		WarningCount: warningCount,
		NormalCount:  normalCount,
		TotalCount:   len(filtered),
	}, nil
}
