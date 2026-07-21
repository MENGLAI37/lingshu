package gitops

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makePod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func makePodWithAnnotations(name, namespace string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
	}
}

func makePodWithLabels(name, namespace string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func TestDetectOwnership_EmptyName(t *testing.T) {
	client := fake.NewSimpleClientset()
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if own.IsManaged {
		t.Errorf("empty name should return not managed")
	}
}

func TestDetectOwnership_NilClient(t *testing.T) {
	d := NewDetector(nil)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "test-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if own.IsManaged {
		t.Errorf("nil client should return not managed")
	}
}

func TestDetectOwnership_NotManaged(t *testing.T) {
	pod := makePod("test-pod", "default")
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "test-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if own.IsManaged {
		t.Errorf("unannotated pod should not be managed")
	}
	if own.Controller != ControllerNone {
		t.Errorf("expected ControllerNone, got %s", own.Controller)
	}
}

func TestDetectOwnership_ArgoCD_ByAnnotation(t *testing.T) {
	pod := makePodWithAnnotations("my-app-pod", "default", map[string]string{
		"argocd.argoproj.io/instance": "my-app",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "my-app-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("ArgoCD-annotated pod should be managed")
	}
	if own.Controller != ControllerArgoCD {
		t.Errorf("expected ArgoCD, got %s", own.Controller)
	}
	if own.AppName != "my-app" {
		t.Errorf("expected app name 'my-app', got '%s'", own.AppName)
	}
}

func TestDetectOwnership_ArgoCD_ByTrackingID(t *testing.T) {
	pod := makePodWithAnnotations("tracked-pod", "default", map[string]string{
		"argocd.argoproj.io/tracking-id": "apps/Deployment/default/nginx",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "tracked-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("ArgoCD tracking-id pod should be managed")
	}
	if own.Controller != ControllerArgoCD {
		t.Errorf("expected ArgoCD, got %s", own.Controller)
	}
}

func TestDetectOwnership_Flux_ByAnnotation(t *testing.T) {
	pod := makePodWithAnnotations("flux-pod", "default", map[string]string{
		"kustomize.toolkit.fluxcd.io/name": "my-kustomization",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "flux-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("Flux-annotated pod should be managed")
	}
	if own.Controller != ControllerFlux {
		t.Errorf("expected Flux, got %s", own.Controller)
	}
}

func TestDetectOwnership_Flux_ByHelmAnnotation(t *testing.T) {
	pod := makePodWithAnnotations("helm-flux-pod", "default", map[string]string{
		"helm.toolkit.fluxcd.io/name": "my-helm-release",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "helm-flux-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("Flux helm-annotated pod should be managed")
	}
	if own.Controller != ControllerFlux {
		t.Errorf("expected Flux, got %s", own.Controller)
	}
}

func TestDetectOwnership_ArgoCD_ByLabel(t *testing.T) {
	pod := makePodWithLabels("argo-label-pod", "default", map[string]string{
		"app.kubernetes.io/managed-by": "argocd",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "argo-label-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("argoCD-labeled pod should be managed")
	}
	if own.Controller != ControllerArgoCD {
		t.Errorf("expected ArgoCD, got %s", own.Controller)
	}
}

func TestDetectOwnership_Flux_ByLabel(t *testing.T) {
	pod := makePodWithLabels("flux-label-pod", "default", map[string]string{
		"app.kubernetes.io/managed-by": "flux",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "flux-label-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("flux-labeled pod should be managed")
	}
	if own.Controller != ControllerFlux {
		t.Errorf("expected Flux, got %s", own.Controller)
	}
}

func TestDetectOwnership_Helm_ByLabel(t *testing.T) {
	pod := makePodWithLabels("helm-pod", "default", map[string]string{
		"app.kubernetes.io/managed-by": "helm",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "helm-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("helm-labeled pod should be managed")
	}
	if own.Controller != ControllerHelm {
		t.Errorf("expected Helm, got %s", own.Controller)
	}
}

func TestDetectOwnership_ResourceNotFound(t *testing.T) {
	client := fake.NewSimpleClientset() // empty cluster
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if own.IsManaged {
		t.Errorf("non-existent resource should not be managed")
	}
}

func TestDetectOwnership_AnnotationPriority(t *testing.T) {
	// If both ArgoCD and Flux annotations exist, ArgoCD wins (checked first)
	pod := makePodWithAnnotations("hybrid-pod", "default", map[string]string{
		"argocd.argoproj.io/instance":          "argo-app",
		"kustomize.toolkit.fluxcd.io/name":     "flux-app",
	})
	client := fake.NewSimpleClientset(pod)
	d := NewDetector(client)

	own, err := d.DetectOwnership(context.Background(), "default", "pod", "hybrid-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !own.IsManaged {
		t.Errorf("should be managed")
	}
	if own.Controller != ControllerArgoCD {
		t.Errorf("ArgoCD annotation should take priority, got %s", own.Controller)
	}
}

// ===========================================================================
// Warning Message Tests
// ===========================================================================

func TestOwnership_GetWarningMessage(t *testing.T) {
	tests := []struct {
		name    string
		own     *Ownership
		emptyOk bool
	}{
		{"not managed", &Ownership{IsManaged: false, Controller: ControllerNone}, true},
		{"argocd", &Ownership{IsManaged: true, Controller: ControllerArgoCD, AppName: "my-app"}, false},
		{"flux", &Ownership{IsManaged: true, Controller: ControllerFlux, AppName: "my-kust"}, false},
		{"helm", &Ownership{IsManaged: true, Controller: ControllerHelm, AppName: "my-release"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.own.GetWarningMessage()
			if tt.emptyOk && msg != "" {
				t.Errorf("expected empty warning, got '%s'", msg)
			}
			if !tt.emptyOk && msg == "" {
				t.Errorf("expected non-empty warning")
			}
			t.Logf("warning: %s", msg)
		})
	}
}

// ===========================================================================
// Controller Constants
// ===========================================================================

func TestController_Constants(t *testing.T) {
	if ControllerArgoCD == ControllerFlux {
		t.Errorf("controller constants must be distinct")
	}
	if ControllerArgoCD == ControllerNone {
		t.Errorf("ControllerArgoCD should differ from ControllerNone")
	}
	if ControllerFlux == ControllerNone {
		t.Errorf("ControllerFlux should differ from ControllerNone")
	}
	if ControllerHelm == ControllerNone {
		t.Errorf("ControllerHelm should differ from ControllerNone")
	}
}
