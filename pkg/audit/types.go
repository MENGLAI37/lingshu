package audit

import (
	"time"
)

type RiskLevel string

const (
	RiskL0 RiskLevel = "L0"
	RiskL1 RiskLevel = "L1"
	RiskL2 RiskLevel = "L2"
	RiskL3 RiskLevel = "L3"
	RiskL4 RiskLevel = "L4"
)

type AuditAction string

const (
	ActionToolCall     AuditAction = "tool_call"
	ActionSessionStart AuditAction = "session_start"
	ActionSessionEnd   AuditAction = "session_end"
	ActionUserInput    AuditAction = "user_input"
	ActionLLMResponse  AuditAction = "llm_response"
	ActionApproval     AuditAction = "approval"
	ActionRollback     AuditAction = "rollback"
	ActionSnapshot     AuditAction = "snapshot"
	ActionAlertTrigger AuditAction = "alert_trigger"
	ActionConfigChange AuditAction = "config_change"
)

type AuditEvent struct {
	EventID         int64                  `db:"event_id" json:"event_id"`
	SessionID       *string                `db:"session_id" json:"session_id,omitempty"`
	UserID          *string                `db:"user_id" json:"user_id,omitempty"`
	Cluster         string                 `db:"cluster" json:"cluster"`
	Namespace       string                 `db:"namespace" json:"namespace"`
	Action          AuditAction            `db:"action" json:"action"`
	ToolName        *string                `db:"tool_name" json:"tool_name,omitempty"`
	RiskLevel       RiskLevel              `db:"risk_level" json:"risk_level"`
	Target          map[string]interface{} `db:"target" json:"target"`
	PreCheck        map[string]interface{} `db:"pre_check" json:"pre_check"`
	ImpactAnalysis  map[string]interface{} `db:"impact_analysis" json:"impact_analysis"`
	Result          map[string]interface{} `db:"result" json:"result"`
	RollbackInfo    *map[string]interface{} `db:"rollback_info" json:"rollback_info,omitempty"`
	Approval        *map[string]interface{} `db:"approval" json:"approval,omitempty"`
	EvidenceChainHash *string              `db:"evidence_chain_hash" json:"evidence_chain_hash,omitempty"`
	CreatedAt       time.Time              `db:"created_at" json:"created_at"`
}

type CreateAuditEventRequest struct {
	SessionID       *string
	UserID          *string
	Cluster         string
	Namespace       string
	Action          AuditAction
	ToolName        *string
	RiskLevel       RiskLevel
	Target          map[string]interface{}
	PreCheck        map[string]interface{}
	ImpactAnalysis  map[string]interface{}
	Result          map[string]interface{}
	RollbackInfo    *map[string]interface{}
	Approval        *map[string]interface{}
	Metadata        map[string]interface{}
}

type AuditFilter struct {
	SessionID     *string
	UserID        *string
	Cluster       *string
	Namespace     *string
	Action        *AuditAction
	ToolName      *string
	RiskLevel     *RiskLevel
	StartTime     *time.Time
	EndTime       *time.Time
	Limit         int
	Offset        int
}

type AuditListResult struct {
	Events  []AuditEvent
	Total   int64
	HasMore bool
}

type AuditStats struct {
	TotalEvents   int64
	ByRiskLevel   map[RiskLevel]int64
	ByAction      map[AuditAction]int64
	ByCluster     map[string]int64
	ByNamespace   map[string]int64
	TimeRange     struct {
		Start time.Time
		End   time.Time
	}
}
