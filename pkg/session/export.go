package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/lingshu/lingshu/pkg/logger"
)

// ===========================================================================
// Session Export / Import — 会话导出/导入
// ===========================================================================
//
// Exports a session's full context (conversation history, tool call history,
// metadata) to a portable JSON file that can be imported on another machine.

// ExportedSession is the portable representation of a session.
type ExportedSession struct {
	Version             string                   `json:"version"`
	ExportedAt          time.Time                `json:"exported_at"`
	SourceHost          string                   `json:"source_host,omitempty"`
	Session             Session                  `json:"session"`
	ConversationHistory []map[string]interface{} `json:"conversation_history"`
	ToolCallHistory     []map[string]interface{} `json:"tool_call_history"`
	Metadata            map[string]interface{}   `json:"metadata"`
}

// Export exports a session to a JSON file.
func (m *Manager) Export(ctx context.Context, sessionID string, outputPath string) error {
	if sessionID == "" {
		return fmt.Errorf("session_id is required for export")
	}

	session, err := m.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session for export: %w", err)
	}

	exported := ExportedSession{
		Version:             "v1.0",
		ExportedAt:          time.Now(),
		Session:             *session,
		ConversationHistory: session.ConversationHistory,
		ToolCallHistory:     session.ToolCallHistory,
		Metadata:            session.Metadata,
	}

	hostname, _ := os.Hostname()
	exported.SourceHost = hostname

	data, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session for export: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}

	// Mark session as exported
	exportedStatus := StatusExported
	_, _ = m.Update(ctx, sessionID, &UpdateSessionRequest{
		Status: &exportedStatus,
	})

	// Add export event to metadata
	exportMeta := session.Metadata
	if exportMeta == nil {
		exportMeta = map[string]interface{}{}
	}
	exportMeta["exported_at"] = exported.ExportedAt.Format(time.RFC3339)
	exportMeta["export_file"] = outputPath

	logger.InfoContext(ctx, "Session exported",
		"session_id", sessionID,
		"output", outputPath,
		"messages", len(session.ConversationHistory),
		"tool_calls", len(session.ToolCallHistory),
	)

	return nil
}

// Import imports a session from a JSON file.
func (m *Manager) Import(ctx context.Context, inputPath string) (*Session, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read import file: %w", err)
	}

	var exported ExportedSession
	if err := json.Unmarshal(data, &exported); err != nil {
		return nil, fmt.Errorf("unmarshal import file: %w", err)
	}

	if exported.Version == "" {
		return nil, fmt.Errorf("invalid export file: missing version")
	}

	newID := uuid.New().String()
	now := time.Now()

	// Build metadata with import provenance
	metadata := exported.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["imported_from"] = exported.SourceHost
	metadata["imported_at"] = now.Format(time.RFC3339)
	metadata["original_session_id"] = exported.Session.SessionID
	metadata["original_exported_at"] = exported.ExportedAt.Format(time.RFC3339)

	session := &Session{
		SessionID:          newID,
		Cluster:            exported.Session.Cluster,
		Namespace:          exported.Session.Namespace,
		Environment:        exported.Session.Environment,
		Status:             StatusImported,
		UserID:             exported.Session.UserID,
		TeamID:             exported.Session.TeamID,
		IncidentID:         exported.Session.IncidentID,
		ConversationHistory: exported.ConversationHistory,
		ToolCallHistory:    exported.ToolCallHistory,
		Metadata:           metadata,
		TokenBudgetLimit:   exported.Session.TokenBudgetLimit,
		CreatedAt:          now,
		UpdatedAt:          now,
		ExpiresAt:          now.Add(DefaultSessionTTL),
	}

	// Store via database
	convJSON, _ := json.Marshal(session.ConversationHistory)
	toolJSON, _ := json.Marshal(session.ToolCallHistory)
	metaJSON, _ := json.Marshal(session.Metadata)

	row := &sessionDBRow{
		SessionID:           newID,
		ParentSessionID:     exported.Session.ParentSessionID,
		Cluster:             session.Cluster,
		Namespace:           session.Namespace,
		Environment:         session.Environment,
		Status:              string(StatusImported),
		UserID:              session.UserID,
		TeamID:              session.TeamID,
		IncidentID:          session.IncidentID,
		ConversationHistory: string(convJSON),
		ToolCallHistory:     string(toolJSON),
		Metadata:            string(metaJSON),
		CostUSDMilli:        session.CostUSDMilli,
		TokenBudgetUsed:     session.TokenBudgetUsed,
		TokenBudgetLimit:    session.TokenBudgetLimit,
		CreatedAt:           now,
		UpdatedAt:           now,
		ExpiresAt:           session.ExpiresAt,
	}

	query := `
		INSERT INTO sessions (
			session_id, parent_session_id, cluster, namespace, environment,
			status, user_id, team_id, incident_id, conversation_history,
			tool_call_history, metadata, cost_usd_milli, token_budget_used,
			token_budget_limit, created_at, updated_at, expires_at
		) VALUES (
			:session_id, :parent_session_id, :cluster, :namespace, :environment,
			:status, :user_id, :team_id, :incident_id, :conversation_history,
			:tool_call_history, :metadata, :cost_usd_milli, :token_budget_used,
			:token_budget_limit, :created_at, :updated_at, :expires_at
		)
	`

	if _, err := m.db.NamedExecContext(ctx, query, row); err != nil {
		logger.ErrorContext(ctx, "Failed to import session", "error", err, "original", exported.Session.SessionID)
		return nil, fmt.Errorf("import session: %w", err)
	}

	logger.InfoContext(ctx, "Session imported",
		"new_session_id", newID,
		"original_session_id", exported.Session.SessionID,
		"source_host", exported.SourceHost,
		"messages", len(session.ConversationHistory),
		"tool_calls", len(session.ToolCallHistory),
	)

	return session, nil
}
