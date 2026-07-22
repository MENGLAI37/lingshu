package snapshot

import (
	"context"

	"github.com/lingshu/lingshu/pkg/agent"
)

// ===========================================================================
// Agent Adapter — bridges snapshot.Snapshotter to agent.Snapshotter interface
// ===========================================================================

// AgentSnapshotterAdapter adapts the snapshot package Snapshotter to the
// agent.Snapshotter interface for use in the Agent Loop auto-rollback pathway.
type AgentSnapshotterAdapter struct {
	snapshotter *Snapshotter
}

// NewAgentSnapshotterAdapter creates a new agent-compatible snapshotter adapter.
func NewAgentSnapshotterAdapter(s *Snapshotter) *AgentSnapshotterAdapter {
	return &AgentSnapshotterAdapter{snapshotter: s}
}

// Snapshot delegates to the underlying Snapshotter and converts return types.
func (a *AgentSnapshotterAdapter) Snapshot(ctx context.Context, resourceType, namespace, name string) (agent.SnapshotMeta, error) {
	meta, err := a.snapshotter.Snapshot(ctx, resourceType, namespace, name)
	if err != nil {
		return agent.SnapshotMeta{}, err
	}

	return agent.SnapshotMeta{
		ID:           meta.ID,
		SessionID:    meta.SessionID,
		ResourceKey:  meta.ResourceKey,
		ResourceType: meta.ResourceType,
		Namespace:    meta.Namespace,
		Name:         meta.Name,
	}, nil
}

// Restore delegates to RestoreByID which searches for the snapshot across sessions.
func (a *AgentSnapshotterAdapter) Restore(ctx context.Context, snapshotID string) error {
	return a.snapshotter.RestoreByID(ctx, snapshotID)
}
