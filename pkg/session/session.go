package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/db"
	"github.com/lingshu/lingshu/pkg/logger"
)

const (
	DefaultTokenBudgetLimit = 100000
	DefaultSessionTTL       = 24 * time.Hour
	DefaultListLimit        = 50
	MaxListLimit            = 500
)

type sessionDBRow struct {
	SessionID          string     `db:"session_id"`
	ParentSessionID    *string    `db:"parent_session_id"`
	Cluster            string     `db:"cluster"`
	Namespace          string     `db:"namespace"`
	Environment        string     `db:"environment"`
	Status             string     `db:"status"`
	UserID             *string    `db:"user_id"`
	TeamID             *string    `db:"team_id"`
	IncidentID         *string    `db:"incident_id"`
	ConversationHistory string   `db:"conversation_history"`
	ToolCallHistory    string     `db:"tool_call_history"`
	Metadata           string     `db:"metadata"`
	CostUSDMilli       int64      `db:"cost_usd_milli"`
	TokenBudgetUsed    int64      `db:"token_budget_used"`
	TokenBudgetLimit   int64      `db:"token_budget_limit"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
	ExpiresAt          time.Time  `db:"expires_at"`
}

type Manager struct {
	db *db.Database
}

var (
	instance *Manager
)

func Init(cfg *config.Config) (*Manager, error) {
	database := db.Get()
	if database == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	instance = &Manager{
		db: database,
	}

	logger.Info("Session manager initialized")
	return instance, nil
}

func Get() *Manager {
	return instance
}

func (m *Manager) Create(ctx context.Context, req *CreateSessionRequest) (*Session, error) {
	if req == nil {
		return nil, fmt.Errorf("create session request is nil")
	}

	sessionID := uuid.New().String()
	now := time.Now()

	ttl := DefaultSessionTTL
	if req.TTL != nil {
		ttl = *req.TTL
	}
	expiresAt := now.Add(ttl)

	tokenBudgetLimit := int64(DefaultTokenBudgetLimit)
	if req.TokenBudgetLimit != nil {
		tokenBudgetLimit = *req.TokenBudgetLimit
	}

	cluster := req.Cluster
	if cluster == "" {
		cluster = "default"
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = "default"
	}

	environment := req.Environment
	if environment == "" {
		environment = "production"
	}

	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}

	convHistoryJSON, _ := json.Marshal([]map[string]interface{}{})
	toolCallJSON, _ := json.Marshal([]map[string]interface{}{})
	metadataJSON, _ := json.Marshal(req.Metadata)

	row := &sessionDBRow{
		SessionID:           sessionID,
		ParentSessionID:     req.ParentSessionID,
		Cluster:             cluster,
		Namespace:           namespace,
		Environment:         environment,
		Status:              string(StatusActive),
		UserID:              req.UserID,
		TeamID:              req.TeamID,
		IncidentID:          req.IncidentID,
		ConversationHistory: string(convHistoryJSON),
		ToolCallHistory:     string(toolCallJSON),
		Metadata:            string(metadataJSON),
		CostUSDMilli:        0,
		TokenBudgetUsed:     0,
		TokenBudgetLimit:    tokenBudgetLimit,
		CreatedAt:           now,
		UpdatedAt:           now,
		ExpiresAt:           expiresAt,
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

	_, err := m.db.NamedExecContext(ctx, query, row)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create session", "error", err, "session_id", sessionID)
		return nil, fmt.Errorf("create session: %w", err)
	}

	logger.InfoContext(ctx, "Session created",
		"session_id", sessionID,
		"parent_session_id", req.ParentSessionID,
		"cluster", cluster,
		"namespace", namespace,
	)

	return rowToSession(row), nil
}

func (m *Manager) Get(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is empty")
	}

	var row sessionDBRow
	query := `SELECT * FROM sessions WHERE session_id = $1`

	err := m.db.GetContext(ctx, &row, query, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		logger.ErrorContext(ctx, "Failed to get session", "error", err, "session_id", sessionID)
		return nil, fmt.Errorf("get session: %w", err)
	}

	return rowToSession(&row), nil
}

func (m *Manager) Update(ctx context.Context, sessionID string, req *UpdateSessionRequest) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is empty")
	}
	if req == nil {
		return nil, fmt.Errorf("update request is nil")
	}

	_, err := m.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.Status != nil {
		updates["status"] = string(*req.Status)
	}
	if req.ConversationHistory != nil {
		data, _ := json.Marshal(*req.ConversationHistory)
		updates["conversation_history"] = string(data)
	}
	if req.ToolCallHistory != nil {
		data, _ := json.Marshal(*req.ToolCallHistory)
		updates["tool_call_history"] = string(data)
	}
	if req.Metadata != nil {
		data, _ := json.Marshal(*req.Metadata)
		updates["metadata"] = string(data)
	}
	if req.CostUSDMilli != nil {
		updates["cost_usd_milli"] = *req.CostUSDMilli
	}
	if req.TokenBudgetUsed != nil {
		updates["token_budget_used"] = *req.TokenBudgetUsed
	}
	if req.TokenBudgetLimit != nil {
		updates["token_budget_limit"] = *req.TokenBudgetLimit
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}

	if len(updates) == 0 {
		return m.Get(ctx, sessionID)
	}

	updates["session_id"] = sessionID
	updates["updated_at"] = time.Now()

	setClauses := []string{}
	for field := range updates {
		if field == "session_id" {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = :%s", field, field))
	}

	query := fmt.Sprintf(`UPDATE sessions SET %s WHERE session_id = :session_id`,
		joinStrings(setClauses, ", "))

	_, err = m.db.NamedExecContext(ctx, query, updates)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update session", "error", err, "session_id", sessionID)
		return nil, fmt.Errorf("update session: %w", err)
	}

	return m.Get(ctx, sessionID)
}

func (m *Manager) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session_id is empty")
	}

	query := `DELETE FROM sessions WHERE session_id = $1`

	result, err := m.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete session", "error", err, "session_id", sessionID)
		return fmt.Errorf("delete session: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	logger.InfoContext(ctx, "Session deleted", "session_id", sessionID)
	return nil
}

func (m *Manager) List(ctx context.Context, filter *SessionListFilter) (*SessionListResult, error) {
	if filter == nil {
		filter = &SessionListFilter{}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	whereClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if filter.UserID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.TeamID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("team_id = $%d", argIdx))
		args = append(args, *filter.TeamID)
		argIdx++
	}
	if filter.Status != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}
	if filter.Cluster != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("cluster = $%d", argIdx))
		args = append(args, *filter.Cluster)
		argIdx++
	}
	if filter.Namespace != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("namespace = $%d", argIdx))
		args = append(args, *filter.Namespace)
		argIdx++
	}
	if filter.ParentID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("parent_session_id = $%d", argIdx))
		args = append(args, *filter.ParentID)
		argIdx++
	}
	if filter.IncidentID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("incident_id = $%d", argIdx))
		args = append(args, *filter.IncidentID)
		argIdx++
	}
	if filter.CreatedAfter != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at > $%d", argIdx))
		args = append(args, *filter.CreatedAfter)
		argIdx++
	}
	if filter.CreatedBefore != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at < $%d", argIdx))
		args = append(args, *filter.CreatedBefore)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + joinStrings(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM sessions %s`, whereSQL)
	var total int64
	if err := m.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		logger.ErrorContext(ctx, "Failed to count sessions", "error", err)
		return nil, fmt.Errorf("count sessions: %w", err)
	}

	listQuery := fmt.Sprintf(`
		SELECT * FROM sessions %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	listArgs := append(args, limit, offset)

	var rows []sessionDBRow
	if err := m.db.SelectContext(ctx, &rows, listQuery, listArgs...); err != nil {
		logger.ErrorContext(ctx, "Failed to list sessions", "error", err)
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	sessions := make([]Session, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, *rowToSession(&row))
	}

	hasMore := int64(offset+len(sessions)) < total

	return &SessionListResult{
		Sessions: sessions,
		Total:    total,
		HasMore:  hasMore,
	}, nil
}

func (m *Manager) GetParentChain(ctx context.Context, sessionID string) ([]Session, error) {
	var chain []Session
	currentID := sessionID
	visited := map[string]bool{}

	for currentID != "" {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		sess, err := m.Get(ctx, currentID)
		if err != nil {
			break
		}

		chain = append([]Session{*sess}, chain...)

		if sess.ParentSessionID == nil {
			break
		}
		currentID = *sess.ParentSessionID
	}

	return chain, nil
}

func (m *Manager) GetChildren(ctx context.Context, parentID string, recursive bool) ([]Session, error) {
	if parentID == "" {
		return nil, fmt.Errorf("parent_id is empty")
	}

	if !recursive {
		filter := &SessionListFilter{
			ParentID: &parentID,
		}
		result, err := m.List(ctx, filter)
		if err != nil {
			return nil, err
		}
		return result.Sessions, nil
	}

	var allChildren []Session
	queue := []string{parentID}
	visited := map[string]bool{parentID: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		filter := &SessionListFilter{
			ParentID: &current,
		}
		result, err := m.List(ctx, filter)
		if err != nil {
			continue
		}

		for _, child := range result.Sessions {
			if !visited[child.SessionID] {
				visited[child.SessionID] = true
				allChildren = append(allChildren, child)
				queue = append(queue, child.SessionID)
			}
		}
	}

	return allChildren, nil
}

func (m *Manager) AppendConversation(ctx context.Context, sessionID string, message map[string]interface{}) error {
	if sessionID == "" {
		return fmt.Errorf("session_id is empty")
	}
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	sess, err := m.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	history := append(sess.ConversationHistory, message)
	_, err = m.Update(ctx, sessionID, &UpdateSessionRequest{
		ConversationHistory: &history,
	})
	return err
}

func (m *Manager) AppendToolCall(ctx context.Context, sessionID string, toolCall map[string]interface{}) error {
	if sessionID == "" {
		return fmt.Errorf("session_id is empty")
	}
	if toolCall == nil {
		return fmt.Errorf("tool_call is nil")
	}

	sess, err := m.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	history := append(sess.ToolCallHistory, toolCall)
	_, err = m.Update(ctx, sessionID, &UpdateSessionRequest{
		ToolCallHistory: &history,
	})
	return err
}

func (m *Manager) AddCost(ctx context.Context, sessionID string, costUSDMilli int64, tokensUsed int64) error {
	if sessionID == "" {
		return fmt.Errorf("session_id is empty")
	}

	sess, err := m.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	newCost := sess.CostUSDMilli + costUSDMilli
	newTokens := sess.TokenBudgetUsed + tokensUsed

	_, err = m.Update(ctx, sessionID, &UpdateSessionRequest{
		CostUSDMilli:    &newCost,
		TokenBudgetUsed: &newTokens,
	})
	return err
}

func (m *Manager) Complete(ctx context.Context, sessionID string) (*Session, error) {
	status := StatusCompleted
	return m.Update(ctx, sessionID, &UpdateSessionRequest{
		Status: &status,
	})
}

func (m *Manager) MarkCrashed(ctx context.Context, sessionID string) (*Session, error) {
	status := StatusCrashed
	return m.Update(ctx, sessionID, &UpdateSessionRequest{
		Status: &status,
	})
}

func (m *Manager) RefreshTTL(ctx context.Context, sessionID string, ttl *time.Duration) (*Session, error) {
	_, err := m.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	duration := DefaultSessionTTL
	if ttl != nil {
		duration = *ttl
	}

	newExpiresAt := time.Now().Add(duration)
	_, err = m.Update(ctx, sessionID, &UpdateSessionRequest{
		ExpiresAt: &newExpiresAt,
	})
	if err != nil {
		return nil, err
	}

	return m.Get(ctx, sessionID)
}

func (m *Manager) IsExpired(ctx context.Context, sessionID string) (bool, error) {
	sess, err := m.Get(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return time.Now().After(sess.ExpiresAt), nil
}

func (m *Manager) CheckTokenBudget(ctx context.Context, sessionID string, tokensToUse int64) (bool, error) {
	sess, err := m.Get(ctx, sessionID)
	if err != nil {
		return false, err
	}

	return sess.TokenBudgetUsed+tokensToUse <= sess.TokenBudgetLimit, nil
}

func (m *Manager) Close() error {
	return nil
}

func rowToSession(row *sessionDBRow) *Session {
	var convHistory []map[string]interface{}
	if err := json.Unmarshal([]byte(row.ConversationHistory), &convHistory); err != nil {
		convHistory = []map[string]interface{}{}
	}

	var toolCallHistory []map[string]interface{}
	if err := json.Unmarshal([]byte(row.ToolCallHistory), &toolCallHistory); err != nil {
		toolCallHistory = []map[string]interface{}{}
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
		metadata = map[string]interface{}{}
	}

	return &Session{
		SessionID:           row.SessionID,
		ParentSessionID:     row.ParentSessionID,
		Cluster:             row.Cluster,
		Namespace:           row.Namespace,
		Environment:         row.Environment,
		Status:              SessionStatus(row.Status),
		UserID:              row.UserID,
		TeamID:              row.TeamID,
		IncidentID:          row.IncidentID,
		ConversationHistory: convHistory,
		ToolCallHistory:     toolCallHistory,
		Metadata:            metadata,
		CostUSDMilli:        row.CostUSDMilli,
		TokenBudgetUsed:     row.TokenBudgetUsed,
		TokenBudgetLimit:    row.TokenBudgetLimit,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		ExpiresAt:           row.ExpiresAt,
	}
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
