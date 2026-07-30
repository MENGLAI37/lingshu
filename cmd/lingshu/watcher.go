package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lingshu/lingshu/pkg/agent"
	"github.com/lingshu/lingshu/pkg/alertd"
	"github.com/lingshu/lingshu/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// startClusterWatcher runs periodic cluster scans to autonomously discover issues.
func startClusterWatcher(k8sClient *k8s.ClientManager, autoEngine *agent.AutonomousEngine, stop <-chan struct{}) {
	seenIssues := make(map[string]bool)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	scanCluster(k8sClient, autoEngine, seenIssues)

	for {
		select {
		case <-ticker.C:
			scanCluster(k8sClient, autoEngine, seenIssues)
		case <-stop:
			return
		}
	}
}

// scanCluster scans all pods for actionable failure states and auto-triggers diagnosis.
func scanCluster(k8sClient *k8s.ClientManager, autoEngine *agent.AutonomousEngine, seen map[string]bool) {
	clientset, err := k8sClient.GetClientSet(context.Background(), "")
	if err != nil {
		return
	}

	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		for _, cs := range pod.Status.ContainerStatuses {
			var reason string
			var message string

			if cs.State.Waiting != nil {
				reason = cs.State.Waiting.Reason
				message = cs.State.Waiting.Message
			}

			if reason == "" && cs.State.Terminated != nil {
				reason = cs.State.Terminated.Reason
				message = cs.State.Terminated.Message
				if reason == "Error" && cs.RestartCount > 0 {
					reason = "CrashLoopBackOff"
				}
			}

			if reason == "" {
				continue
			}

			switch reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull",
				"CreateContainerError", "StartError", "OOMKilled", "Error":
			default:
				continue
			}

			key := pod.Namespace + "/" + pod.Name + "/" + cs.Name + "/" + reason
			if seen[key] {
				continue
			}
			seen[key] = true

			alert := &alertd.Alert{
				ID:           uuid.New().String(),
				Fingerprint:  key,
				Source:       alertd.SourceGeneric,
				Status:       alertd.StatusFiring,
				Severity:     alertd.SeverityCritical,
				Cluster:      "kind-lingshu-dev",
				Namespace:    pod.Namespace,
				ResourceKind: "Pod",
				ResourceName: pod.Name,
				Labels: map[string]string{
					"alertname":   "KubePod" + reason,
					"container":   cs.Name,
					"pod":         pod.Name,
					"namespace":   pod.Namespace,
					"detected_by": "lingshu-cluster-watcher",
				},
				Annotations: map[string]string{
					"summary": fmt.Sprintf(
						"Pod %s/%s container %s is in %s state (auto-detected by cluster watcher)",
						pod.Namespace, pod.Name, cs.Name, reason,
					),
					"description": fmt.Sprintf(
						"Container %s in pod %s/%s is in %s. Message: %s",
						cs.Name, pod.Namespace, pod.Name, reason, message,
					),
				},
				ReceivedAt: time.Now(),
			}

			_ = autoEngine.HandleAlert(alert)
		}
	}
}
