package l2

import (
	"context"
	"fmt"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RolloutTool struct {
	client *kubernetes.Clientset
}

func NewRolloutTool(client *kubernetes.Clientset) *RolloutTool {
	return &RolloutTool{
		client: client,
	}
}

func (t *RolloutTool) Name() string {
	return "k8s_rollout"
}

func (t *RolloutTool) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL2
}

func (t *RolloutTool) Description() string {
	return "Manage rollouts of Kubernetes resources (status, history, undo, pause, resume)"
}

func (t *RolloutTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	start := time.Now()

	action, _ := params["action"].(string)
	resourceType, _ := params["resource_type"].(string)
	namespace, _ := params["namespace"].(string)
	name, _ := params["name"].(string)

	if action == "" || resourceType == "" || name == "" {
		return &tools.ToolResult{
			Success:   false,
			Error:     "action, resource_type, and name are required",
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("action, resource_type, and name are required")
	}

	var result any
	var err error

	switch action {
	case "status":
		result, err = t.rolloutStatus(ctx, resourceType, namespace, name)
	case "history":
		result, err = t.rolloutHistory(ctx, resourceType, namespace, name)
	case "undo":
		revision, _ := params["revision"].(int)
		result, err = t.rolloutUndo(ctx, resourceType, namespace, name, int64(revision))
	case "pause":
		result, err = t.rolloutPause(ctx, resourceType, namespace, name)
	case "resume":
		result, err = t.rolloutResume(ctx, resourceType, namespace, name)
	case "restart":
		result, err = t.rolloutRestart(ctx, resourceType, namespace, name)
	default:
		return &tools.ToolResult{
			Success:   false,
			Error:     fmt.Sprintf("unsupported rollout action: %s", action),
			Timestamp: start,
			Duration:  time.Since(start).String(),
			ToolName:  t.Name(),
			RiskLevel: t.RiskLevel(),
		}, fmt.Errorf("unsupported rollout action: %s", action)
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
		Message:   fmt.Sprintf("Rollout %s of %s/%s completed", action, resourceType, name),
		Timestamp: start,
		Duration:  time.Since(start).String(),
		ToolName:  t.Name(),
		RiskLevel: t.RiskLevel(),
	}, nil
}

type RolloutStatusResult struct {
	ResourceType string
	Name         string
	Namespace    string
	Status       string
	ReadyReplicas int32
	TotalReplicas int32
	UpdatedReplicas int32
	AvailableReplicas int32
	ObservedGeneration int64
}

func (t *RolloutTool) rolloutStatus(ctx context.Context, resourceType, namespace, name string) (*RolloutStatusResult, error) {
	switch resourceType {
	case "deployment", "deployments", "deploy":
		return t.deploymentRolloutStatus(ctx, namespace, name)
	default:
		return nil, fmt.Errorf("unsupported resource type for rollout status: %s", resourceType)
	}
}

func (t *RolloutTool) deploymentRolloutStatus(ctx context.Context, namespace, name string) (*RolloutStatusResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	status := "Progressing"
	if deploy.Status.ObservedGeneration == deploy.Generation &&
		deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas &&
		deploy.Status.AvailableReplicas == *deploy.Spec.Replicas {
		status = "Complete"
	}

	return &RolloutStatusResult{
		ResourceType:       "deployment",
		Name:               name,
		Namespace:          namespace,
		Status:             status,
		ReadyReplicas:      deploy.Status.ReadyReplicas,
		TotalReplicas:      deploy.Status.Replicas,
		UpdatedReplicas:    deploy.Status.UpdatedReplicas,
		AvailableReplicas:  deploy.Status.AvailableReplicas,
		ObservedGeneration: deploy.Status.ObservedGeneration,
	}, nil
}

type RolloutHistoryResult struct {
	ResourceType string
	Name         string
	Namespace    string
	Revisions    []RevisionInfo
	Count        int
}

type RevisionInfo struct {
	Revision    int64
	Image       string
	Replicas    int32
	Annotations map[string]string
	CreatedAt   time.Time
}

func (t *RolloutTool) rolloutHistory(ctx context.Context, resourceType, namespace, name string) (*RolloutHistoryResult, error) {
	switch resourceType {
	case "deployment", "deployments", "deploy":
		return t.deploymentRolloutHistory(ctx, namespace, name)
	default:
		return nil, fmt.Errorf("unsupported resource type for rollout history: %s", resourceType)
	}
}

func (t *RolloutTool) deploymentRolloutHistory(ctx context.Context, namespace, name string) (*RolloutHistoryResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	rsList, err := t.client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deploy.Spec.Selector),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list replicasets for deployment %s/%s: %w", namespace, name, err)
	}

	var revisions []RevisionInfo
	for _, rs := range rsList.Items {
		if revision, ok := rs.Annotations["deployment.kubernetes.io/revision"]; ok {
			image := ""
			if len(rs.Spec.Template.Spec.Containers) > 0 {
				image = rs.Spec.Template.Spec.Containers[0].Image
			}

			var revNum int64
			_, _ = fmt.Sscanf(revision, "%d", &revNum)

			replicas := int32(0)
			if rs.Spec.Replicas != nil {
				replicas = *rs.Spec.Replicas
			}

			revisions = append(revisions, RevisionInfo{
				Revision:    revNum,
				Image:       image,
				Replicas:    replicas,
				Annotations: rs.Annotations,
				CreatedAt:   rs.CreationTimestamp.Time,
			})
		}
	}

	return &RolloutHistoryResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		Revisions:    revisions,
		Count:        len(revisions),
	}, nil
}

type RolloutUndoResult struct {
	ResourceType string
	Name         string
	Namespace    string
	FromRevision int64
	Status       string
}

func (t *RolloutTool) rolloutUndo(ctx context.Context, resourceType, namespace, name string, revision int64) (*RolloutUndoResult, error) {
	switch resourceType {
	case "deployment", "deployments", "deploy":
		return t.deploymentRolloutUndo(ctx, namespace, name, revision)
	default:
		return nil, fmt.Errorf("unsupported resource type for rollout undo: %s", resourceType)
	}
}

func (t *RolloutTool) deploymentRolloutUndo(ctx context.Context, namespace, name string, targetRevision int64) (*RolloutUndoResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	rsList, err := t.client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deploy.Spec.Selector),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list replicasets: %w", err)
	}

	var targetRS *appsv1.ReplicaSet
	for i := range rsList.Items {
		rs := &rsList.Items[i]
		if rev, ok := rs.Annotations["deployment.kubernetes.io/revision"]; ok {
			var revNum int64
			_, _ = fmt.Sscanf(rev, "%d", &revNum)
			if targetRevision == 0 || revNum == targetRevision {
				targetRS = rs
				break
			}
		}
	}

	if targetRS == nil {
		return nil, fmt.Errorf("target revision not found")
	}

	deploy.Spec.Template = targetRS.Spec.Template
	if deploy.Spec.Replicas == nil {
		replicas := int32(1)
		deploy.Spec.Replicas = &replicas
	}

	_, err = t.client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to rollback deployment: %w", err)
	}

	return &RolloutUndoResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		FromRevision: targetRevision,
		Status:       "Rollback initiated",
	}, nil
}

type RolloutPauseResult struct {
	ResourceType string
	Name         string
	Namespace    string
	Status       string
}

func (t *RolloutTool) rolloutPause(ctx context.Context, resourceType, namespace, name string) (*RolloutPauseResult, error) {
	switch resourceType {
	case "deployment", "deployments", "deploy":
		return t.deploymentRolloutPause(ctx, namespace, name)
	default:
		return nil, fmt.Errorf("unsupported resource type for rollout pause: %s", resourceType)
	}
}

func (t *RolloutTool) deploymentRolloutPause(ctx context.Context, namespace, name string) (*RolloutPauseResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	if deploy.Spec.Paused {
		return &RolloutPauseResult{
			ResourceType: "deployment",
			Name:         name,
			Namespace:    namespace,
			Status:       "Already paused",
		}, nil
	}

	deploy.Spec.Paused = true
	_, err = t.client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pause deployment: %w", err)
	}

	return &RolloutPauseResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		Status:       "Paused",
	}, nil
}

type RolloutResumeResult struct {
	ResourceType string
	Name         string
	Namespace    string
	Status       string
}

func (t *RolloutTool) rolloutResume(ctx context.Context, resourceType, namespace, name string) (*RolloutResumeResult, error) {
	switch resourceType {
	case "deployment", "deployments", "deploy":
		return t.deploymentRolloutResume(ctx, namespace, name)
	default:
		return nil, fmt.Errorf("unsupported resource type for rollout resume: %s", resourceType)
	}
}

func (t *RolloutTool) deploymentRolloutResume(ctx context.Context, namespace, name string) (*RolloutResumeResult, error) {
	deploy, err := t.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	if !deploy.Spec.Paused {
		return &RolloutResumeResult{
			ResourceType: "deployment",
			Name:         name,
			Namespace:    namespace,
			Status:       "Not paused",
		}, nil
	}

	deploy.Spec.Paused = false
	_, err = t.client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to resume deployment: %w", err)
	}

	return &RolloutResumeResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		Status:       "Resumed",
	}, nil
}

type RolloutRestartResult struct {
	ResourceType string
	Name         string
	Namespace    string
	Status       string
}

func (t *RolloutTool) rolloutRestart(ctx context.Context, resourceType, namespace, name string) (*RolloutRestartResult, error) {
	switch resourceType {
	case "deployment", "deployments", "deploy":
		return t.deploymentRolloutRestart(ctx, namespace, name)
	default:
		return nil, fmt.Errorf("unsupported resource type for rollout restart: %s", resourceType)
	}
}

func (t *RolloutTool) deploymentRolloutRestart(ctx context.Context, namespace, name string) (*RolloutRestartResult, error) {
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
		return nil, fmt.Errorf("failed to restart deployment: %w", err)
	}

	return &RolloutRestartResult{
		ResourceType: "deployment",
		Name:         name,
		Namespace:    namespace,
		Status:       "Rolling restart triggered",
	}, nil
}
