// Package gitops provides GitOps conflict detection for ArgoCD and Flux managed resources.
// When the agent tries to modify a resource managed by a GitOps controller,
// it warns the user that the change will likely be reverted by the controller.
package gitops

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Controller identifies which GitOps controller manages a resource.
type Controller string

const (
	ControllerArgoCD Controller = "argocd"
	ControllerFlux   Controller = "flux"
	ControllerHelm   Controller = "helm"
	ControllerNone   Controller = "none"
)

// Ownership describes which GitOps controller manages a resource.
type Ownership struct {
	IsManaged  bool       // Whether the resource is managed by any GitOps controller
	Controller Controller // Which controller manages it
	AppName    string     // The application name
}

// Detector detects GitOps ownership of K8s resources via annotations and labels.
type Detector struct {
	client kubernetes.Interface
}

// NewDetector creates a new GitOps detector.
func NewDetector(client kubernetes.Interface) *Detector {
	return &Detector{client: client}
}

// Known GitOps annotations and labels.
var (
	argoCDAnnotations = []string{
		"argocd.argoproj.io/tracking-id",
		"argocd.argoproj.io/instance",
	}
	fluxAnnotations = []string{
		"kustomize.toolkit.fluxcd.io/name",
		"helm.toolkit.fluxcd.io/name",
	}
	managedByLabel = "app.kubernetes.io/managed-by"
)

// DetectOwnership checks if a K8s resource is managed by a GitOps controller.
func (d *Detector) DetectOwnership(ctx context.Context, namespace, resourceType, name string) (*Ownership, error) {
	if name == "" || d.client == nil {
		return &Ownership{IsManaged: false, Controller: ControllerNone}, nil
	}

	meta, err := d.getResourceMeta(ctx, namespace, resourceType, name)
	if err != nil {
		// Resource not accessible — skip detection silently
		return &Ownership{IsManaged: false, Controller: ControllerNone}, nil
	}

	if meta == nil {
		return &Ownership{IsManaged: false, Controller: ControllerNone}, nil
	}

	annotations := meta.GetAnnotations()
	labels := meta.GetLabels()
	own := &Ownership{IsManaged: false, Controller: ControllerNone}

	// Check ArgoCD annotations
	for _, key := range argoCDAnnotations {
		if val, ok := annotations[key]; ok {
			own.IsManaged = true
			own.Controller = ControllerArgoCD
			own.AppName = val
			return own, nil
		}
	}

	// Check Flux annotations
	for _, key := range fluxAnnotations {
		if val, ok := annotations[key]; ok {
			own.IsManaged = true
			own.Controller = ControllerFlux
			own.AppName = val
			return own, nil
		}
	}

	// Check managed-by label
	if managedBy, ok := labels[managedByLabel]; ok {
		switch managedBy {
		case "argocd":
			own.IsManaged = true
			own.Controller = ControllerArgoCD
		case "flux":
			own.IsManaged = true
			own.Controller = ControllerFlux
		case "helm":
			own.IsManaged = true
			own.Controller = ControllerHelm
		}
	}

	return own, nil
}

// getResourceMeta retrieves the ObjectMeta for a resource.
func (d *Detector) getResourceMeta(ctx context.Context, namespace, resourceType, name string) (metav1.Object, error) {
	switch resourceType {
	case "pod", "pods":
		obj, err := d.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, nil
	case "deployment", "deployments":
		obj, err := d.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, nil
	case "service", "services":
		obj, err := d.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, nil
	case "configmap", "configmaps":
		obj, err := d.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, nil
	case "statefulset", "statefulsets":
		obj, err := d.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, nil
	case "daemonset", "daemonsets":
		obj, err := d.client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// GetWarningMessage returns a user-facing warning about modifying a GitOps-managed resource.
func (own *Ownership) GetWarningMessage() string {
	if !own.IsManaged {
		return ""
	}
	switch own.Controller {
	case ControllerArgoCD:
		return fmt.Sprintf(
			"此资源由 ArgoCD 管理 (%s)。直接修改将在同步周期内被回滚。建议通过 Git 仓库修改或使用 argocd CLI。",
			own.AppName,
		)
	case ControllerFlux:
		return fmt.Sprintf(
			"此资源由 Flux 管理 (%s)。直接修改将在同步周期内被回滚。建议通过 Git 仓库修改或使用 flux CLI。",
			own.AppName,
		)
	case ControllerHelm:
		return fmt.Sprintf(
			"此资源由 Helm 管理 (%s)。直接修改将被 Helm 回滚。建议使用 helm upgrade 修改。",
			own.AppName,
		)
	default:
		return ""
	}
}
