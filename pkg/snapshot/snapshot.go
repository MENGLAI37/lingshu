package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/lingshu/lingshu/pkg/logger"
)

// ===========================================================================
// Snapshotter - Pre-mutation Resource Snapshot
// ===========================================================================
//
// Before any L2+ mutation, captures the current state of the target resource
// as YAML so it can be restored if needed. Snapshots are saved locally and
// referenced in the audit trail via snapshot ID.

// SnapshotMeta describes a saved snapshot.
type SnapshotMeta struct {
	ID           string    `json:"id" yaml:"id"`
	SessionID    string    `json:"session_id" yaml:"session_id"`
	ResourceKey  string    `json:"resource_key" yaml:"resource_key"`
	ResourceType string    `json:"resource_type" yaml:"resource_type"`
	Namespace    string    `json:"namespace" yaml:"namespace"`
	Name         string    `json:"name" yaml:"name"`
	Timestamp    time.Time `json:"timestamp" yaml:"timestamp"`
	Checksum     string    `json:"checksum" yaml:"checksum"`
	FilePath     string    `json:"-" yaml:"-"`
}

// Snapshotter captures resource states before mutations.
type Snapshotter struct {
	dynamicClient dynamic.Interface
	snapshotDir   string
}

// NewSnapshotter creates a new snapshotter.
func NewSnapshotter(client dynamic.Interface, snapshotDir string) *Snapshotter {
	if snapshotDir == "" {
		snapshotDir = filepath.Join(os.TempDir(), "lingshu-snapshots")
	}
	return &Snapshotter{
		dynamicClient: client,
		snapshotDir:   snapshotDir,
	}
}

// Snapshot captures the current state of a resource and saves it.
func (s *Snapshotter) Snapshot(ctx context.Context, resourceType, namespace, name string) (*SnapshotMeta, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name is required for snapshot")
	}
	if namespace == "" {
		namespace = "default"
	}

	gvr, err := resolveGVR(resourceType)
	if err != nil {
		return nil, fmt.Errorf("resolve GVR for %s: %w", resourceType, err)
	}

	// Fetch current resource state
	unstructured, err := s.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetch resource %s/%s/%s: %w", namespace, resourceType, name, err)
	}

	// Serialize to YAML
	yamlData, err := yaml.Marshal(unstructured.Object)
	if err != nil {
		return nil, fmt.Errorf("marshal resource to YAML: %w", err)
	}

	// Build snapshot metadata
	checksum := fmt.Sprintf("%x", hashBytes(yamlData))[:16]
	meta := &SnapshotMeta{
		ID:           fmt.Sprintf("snap-%s-%s-%s-%d", namespace, resourceType, name, time.Now().Unix()),
		SessionID:    "auto",
		ResourceKey:  fmt.Sprintf("%s/%s/%s", namespace, resourceType, name),
		ResourceType: resourceType,
		Namespace:    namespace,
		Name:         name,
		Timestamp:    time.Now(),
		Checksum:     checksum,
	}

	// Save to disk
	if err := s.save(meta, yamlData); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}

	logger.Info("Snapshot taken",
		"resource", meta.ResourceKey,
		"id", meta.ID,
		"checksum", checksum,
	)

	return meta, nil
}

// SnapshotForTool captures snapshots for tool arguments before execution.
func (s *Snapshotter) SnapshotForTool(ctx context.Context, toolName string, args map[string]any) (*SnapshotMeta, error) {
	resourceType, _ := args["resource_type"].(string)
	name, _ := args["name"].(string)
	namespace, _ := args["namespace"].(string)

	if resourceType == "" || name == "" {
		// Not enough info to snapshot, skip
		return nil, nil
	}

	return s.Snapshot(ctx, resourceType, namespace, name)
}

// List returns all snapshots for a session.
func (s *Snapshotter) List(sessionID string) ([]SnapshotMeta, error) {
	sessionDir := filepath.Join(s.snapshotDir, sessionID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var metas []SnapshotMeta
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(sessionDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Extract snapshot info from frontmatter
		var meta SnapshotMeta
		if err := yaml.Unmarshal(data, &meta); err != nil {
			continue
		}
		meta.FilePath = path
		metas = append(metas, meta)
	}
	return metas, nil
}

// Restore restores a resource from a snapshot. Returns the YAML data.
func (s *Snapshotter) Restore(ctx context.Context, snapshotID, sessionID string) ([]byte, error) {
	path := filepath.Join(s.snapshotDir, sessionID, snapshotID+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read snapshot file: %w", err)
	}
	return data, nil
}

// RestoreByID searches for a snapshot by ID across all sessions and restores it.
// This is used by the auto-rollback pathway which only has the snapshot ID.
func (s *Snapshotter) RestoreByID(ctx context.Context, snapshotID string) error {
	// Search all session directories for the snapshot file
	entries, err := os.ReadDir(s.snapshotDir)
	if err != nil {
		return fmt.Errorf("list snapshot directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		snapshotPath := filepath.Join(s.snapshotDir, entry.Name(), snapshotID+".yaml")
		if _, statErr := os.Stat(snapshotPath); statErr == nil {
			// Found the snapshot, now apply it via dynamic client
			data, err := os.ReadFile(snapshotPath)
			if err != nil {
				return fmt.Errorf("read snapshot file: %w", err)
			}

			// Parse YAML to get resource info
			var unstructuredObj map[string]interface{}
			if err := yaml.Unmarshal(data, &unstructuredObj); err != nil {
				return fmt.Errorf("unmarshal snapshot YAML: %w", err)
			}

			// Extract resource metadata
			apiVersion, _ := unstructuredObj["apiVersion"].(string)
			kind, _ := unstructuredObj["kind"].(string)
			metadata, _ := unstructuredObj["metadata"].(map[string]interface{})
			namespace, _ := metadata["namespace"].(string)
			name, _ := metadata["name"].(string)

			if apiVersion == "" || kind == "" || name == "" {
				return fmt.Errorf("invalid snapshot: missing apiVersion/kind/name")
			}

			// Resolve GVR from apiVersion + kind
			gvr, err := resolveGVRFromAPIVersion(apiVersion, kind)
			if err != nil {
				return fmt.Errorf("resolve GVR from snapshot: %w", err)
			}

			// Convert map to *unstructured.Unstructured for Apply
			usObj := &unstructured.Unstructured{Object: unstructuredObj}

			// Apply the snapshot to restore the resource
			opts := metav1.ApplyOptions{FieldManager: "lingshu-auto-rollback"}
			_, err = s.dynamicClient.Resource(gvr).Namespace(namespace).Apply(
				ctx, name, usObj, opts,
			)
			if err != nil {
				return fmt.Errorf("apply snapshot for rollback: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("snapshot %s not found in any session directory", snapshotID)
}

// resolveGVRFromAPIVersion resolves GVR from apiVersion and kind strings.
func resolveGVRFromAPIVersion(apiVersion, kind string) (schema.GroupVersionResource, error) {
	// Map common K8s resource kinds to GVR
	kindLower := strings.ToLower(kind)
	plural := kindLower + "s" // simple pluralization

	// Handle irregular plurals
	switch kindLower {
	case "pod":
		plural = "pods"
	case "service":
		plural = "services"
	case "deployment":
		plural = "deployments"
	case "statefulset":
		plural = "statefulsets"
	case "replicaset":
		plural = "replicasets"
	case "daemonset":
		plural = "daemonsets"
	case "configmap":
		plural = "configmaps"
	case "ingress":
		plural = "ingresses"
	case "namespace":
		plural = "namespaces"
	}

	// Parse group from apiVersion
	parts := strings.SplitN(apiVersion, "/", 2)
	var group string
	var version string
	if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	} else {
		version = parts[0]
	}

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}, nil
}

func (s *Snapshotter) save(meta *SnapshotMeta, data []byte) error {
	sessionDir := filepath.Join(s.snapshotDir, meta.SessionID)
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	// Prepend metadata as YAML frontmatter
	metaYAML, _ := yaml.Marshal(meta)
	fullData := append([]byte("# SNAPSHOT METADATA\n"), metaYAML...)
	fullData = append(fullData, []byte("\n---\n")...)
	fullData = append(fullData, data...)

	path := filepath.Join(sessionDir, meta.ID+".yaml")
	if err := os.WriteFile(path, fullData, 0600); err != nil {
		return fmt.Errorf("write snapshot file: %w", err)
	}
	meta.FilePath = path
	return nil
}

func (s *Snapshotter) GetDir() string {
	return s.snapshotDir
}

// ===========================================================================
// Helpers
// ===========================================================================

// resolveGVR resolves a resource type string to a GVR.
func resolveGVR(resourceType string) (schema.GroupVersionResource, error) {
	switch resourceType {
	case "deployment", "deployments":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "statefulset", "statefulsets":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, nil
	case "daemonset", "daemonsets":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, nil
	case "replicaset", "replicasets":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, nil
	case "pod", "pods":
		return schema.GroupVersionResource{Version: "v1", Resource: "pods"}, nil
	case "service", "services":
		return schema.GroupVersionResource{Version: "v1", Resource: "services"}, nil
	case "configmap", "configmaps":
		return schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, nil
	case "secret", "secrets":
		return schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, nil
	case "ingress", "ingresses":
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, nil
	case "namespace", "namespaces":
		return schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, nil
	case "persistentvolumeclaim", "persistentvolumeclaims", "pvc":
		return schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// hashBytes computes a simple hash of bytes for checksum.
func hashBytes(data []byte) [16]byte {
	var h [16]byte
	for i, b := range data {
		h[i%16] ^= b
	}
	return h
}
