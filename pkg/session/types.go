package session

import (
	"time"
)

type SessionStatus string

const (
	StatusActive    SessionStatus = "active"
	StatusCrashed   SessionStatus = "crashed"
	StatusCompleted SessionStatus = "completed"
	StatusExported  SessionStatus = "exported"
	StatusImported  SessionStatus = "imported"
)

type Session struct {
	SessionID          string                 `db:"session_id" json:"session_id"`
	ParentSessionID    *string                `db:"parent_session_id" json:"parent_session_id,omitempty"`
	Cluster            string                 `db:"cluster" json:"cluster"`
	Namespace          string                 `db:"namespace" json:"namespace"`
	Environment        string                 `db:"environment" json:"environment"`
	Status             SessionStatus          `db:"status" json:"status"`
	UserID             *string                `db:"user_id" json:"user_id,omitempty"`
	TeamID             *string                `db:"team_id" json:"team_id,omitempty"`
	IncidentID         *string                `db:"incident_id" json:"incident_id,omitempty"`
	ConversationHistory []map[string]interface{} `db:"conversation_history" json:"conversation_history"`
	ToolCallHistory    []map[string]interface{} `db:"tool_call_history" json:"tool_call_history"`
	Metadata           map[string]interface{} `db:"metadata" json:"metadata"`
	CostUSDMilli       int64                  `db:"cost_usd_milli" json:"cost_usd_milli"`
	TokenBudgetUsed    int64                  `db:"token_budget_used" json:"token_budget_used"`
	TokenBudgetLimit   int64                  `db:"token_budget_limit" json:"token_budget_limit"`
	CreatedAt          time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time              `db:"updated_at" json:"updated_at"`
	ExpiresAt          time.Time              `db:"expires_at" json:"expires_at"`
}

type CreateSessionRequest struct {
	ParentSessionID *string
	Cluster         string
	Namespace       string
	Environment     string
	UserID          *string
	TeamID          *string
	IncidentID      *string
	Metadata        map[string]interface{}
	TokenBudgetLimit *int64
	TTL             *time.Duration
}

type UpdateSessionRequest struct {
	Status              *SessionStatus
	ConversationHistory *[]map[string]interface{}
	ToolCallHistory     *[]map[string]interface{}
	Metadata            *map[string]interface{}
	CostUSDMilli        *int64
	TokenBudgetUsed     *int64
	TokenBudgetLimit    *int64
	ExpiresAt           *time.Time
}

type SessionListFilter struct {
	UserID      *string
	TeamID      *string
	Status      *SessionStatus
	Cluster     *string
	Namespace   *string
	ParentID    *string
	IncidentID  *string
	CreatedAfter *time.Time
	CreatedBefore *time.Time
	Limit       int
	Offset      int
}

type SessionListResult struct {
	Sessions []Session
	Total    int64
	HasMore  bool
}
