package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lingshu/lingshu/pkg/alertd"
	"github.com/lingshu/lingshu/pkg/audit"
	"github.com/lingshu/lingshu/pkg/logger"
	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Autonomous Engine - 自主运维引擎
// ===========================================================================
//
// 核心职责：
// 1. 接收告警/定时检查触发 → 自主启动 Agent Loop 诊断
// 2. 通过安全网关评估修复风险
// 3. L0/L1 自动执行，L2 请求确认，L3/L4 拒绝并升级
// 4. 全程审计留痕

// AutoAction represents an action the autonomous engine decided to take.
type AutoAction struct {
	ID            string
	Type          string // "diagnose", "fix", "rollback", "escalate"
	Trigger       string // "alert", "schedule", "manual"
	ToolCall      ParsedToolCall
	RiskLevel     tools.ToolRiskLevel
	AutoApproved  bool
	RequestConfirm bool
	Result        *ToolExecutionResult
	Timestamp     time.Time
}

// AutoSession tracks one autonomous operation session.
type AutoSession struct {
	ID           string
	TriggerType  string // "alert", "schedule", "manual"
	TriggerData  any    // *alertd.Alert or *scheduler.ScheduledJob
	StartTime    time.Time
	EndTime      time.Time
	Actions      []AutoAction
	FinalResult  string
	Approved     bool
}

// AutoApprovePolicy defines how the autonomous engine handles confirmation requests.
type AutoApprovePolicy string

const (
	// AutoApproveSafe auto-approves L0/L1, denies L2+ (default autonomous behavior).
	AutoApproveSafe AutoApprovePolicy = "safe"
	// AutoApproveAll auto-approves L0/L1/L2, denies L3+ (CI/CD pipeline mode).
	AutoApproveAll AutoApprovePolicy = "all"
	// AutoApproveStrict only auto-approves L0, requires confirmation for everything else.
	AutoApproveStrict AutoApprovePolicy = "strict"
)

// AutonomousEngine bridges triggers (alerts, schedules) to the Agent Loop.
type AutonomousEngine struct {
	agentLoop       *DefaultAgentLoop
	securityGateway SecurityGateway
	auditManager    *audit.Manager
	confirmer       func(ConfirmationRequest) bool
	approvePolicy   AutoApprovePolicy

	currentSession *AutoSession
}

// NewAutonomousEngine creates a new autonomous engine.
func NewAutonomousEngine(
	agentLoop *DefaultAgentLoop,
	securityGateway SecurityGateway,
	auditMgr *audit.Manager,
) *AutonomousEngine {
	ae := &AutonomousEngine{
		agentLoop:     agentLoop,
		securityGateway: securityGateway,
		auditManager:  auditMgr,
		approvePolicy: AutoApproveSafe,
	}
	// Wire the built-in autonomous confirmation handler into the agent loop.
	ae.confirmer = ae.buildConfirmationHandler()
	return ae
}

// SetApprovalPolicy sets the auto-approval policy.
func (ae *AutonomousEngine) SetApprovalPolicy(policy AutoApprovePolicy) {
	ae.approvePolicy = policy
	ae.confirmer = ae.buildConfirmationHandler()
}

// Confirmer returns the current confirmation handler (built-in policy or custom).
func (ae *AutonomousEngine) Confirmer() func(ConfirmationRequest) bool {
	return ae.confirmer
}

// SetConfirmer sets a custom confirmation handler (overrides the built-in policy).
func (ae *AutonomousEngine) SetConfirmer(fn func(ConfirmationRequest) bool) {
	ae.confirmer = fn
}

// buildConfirmationHandler creates a confirmation handler based on the approval policy.
// In autonomous mode, there is no interactive terminal — decisions are made by policy.
func (ae *AutonomousEngine) buildConfirmationHandler() func(ConfirmationRequest) bool {
	return func(req ConfirmationRequest) bool {
		switch ae.approvePolicy {
		case AutoApproveAll:
			// CI/CD mode: auto-approve L0-L2, deny L3+
			if req.RiskLevel == tools.RiskLevelL0 ||
				req.RiskLevel == tools.RiskLevelL1 ||
				req.RiskLevel == tools.RiskLevelL2 {
				logger.Info("🤖 Auto-approved (--yes)",
					"tool", req.ToolName,
					"risk", req.RiskLevel,
				)
				return true
			}
			logger.Warn("🚫 Auto-denied (L3+ requires manual approval)",
				"tool", req.ToolName,
				"risk", req.RiskLevel,
			)
			return false
		case AutoApproveStrict:
			// Strict: auto-approve only L0
			if req.RiskLevel == tools.RiskLevelL0 {
				logger.Info("🤖 Auto-approved L0",
					"tool", req.ToolName,
					"risk", req.RiskLevel,
				)
				return true
			}
			logger.Warn("🔒 Requires confirmation (strict policy)",
				"tool", req.ToolName,
				"risk", req.RiskLevel,
			)
			return false
		default:
			// AutoApproveSafe (default): auto-approve L0/L1, deny L2+
			if req.RiskLevel == tools.RiskLevelL0 ||
				req.RiskLevel == tools.RiskLevelL1 {
				logger.Info("🤖 Auto-approved (safe policy)",
					"tool", req.ToolName,
					"risk", req.RiskLevel,
				)
				return true
			}
			logger.Warn("🔒 Operation requires manual approval (L2+)",
				"tool", req.ToolName,
				"risk", req.RiskLevel,
				"message", req.Message,
			)
			return false
		}
	}
}

// ===========================================================================
// Alert Handler (注册到 alertd)
// ===========================================================================

// HandleAlert is the alertd.AlertHandler implementation.
// When an alert fires (e.g., CrashLoopBackOff), trigger autonomous diagnosis.
func (ae *AutonomousEngine) HandleAlert(alert *alertd.Alert) error {
	if alert == nil {
		return fmt.Errorf("nil alert")
	}

	sessionID := uuid.New().String()

	logger.Info("🛎 Autonomous engine received alert",
		"alert_id", alert.ID,
		"severity", alert.Severity,
		"cluster", alert.Cluster,
		"namespace", alert.Namespace,
		"resource", alert.ResourceName,
		"session", sessionID,
	)

	// 1. Log alert trigger to audit
	ae.logAudit(alert.Cluster, alert.Namespace, audit.ActionAlertTrigger, audit.RiskL0,
		map[string]interface{}{
			"alert_id":   alert.ID,
			"severity":   alert.Severity,
			"resource":   alert.ResourceName,
			"annotations": alert.Annotations,
		},
		map[string]interface{}{"session": sessionID},
		nil,
	)

	// 2. Build diagnosis prompt from alert
	prompt := ae.buildAlertPrompt(alert)

	// 3. Run agent loop autonomously with proper execution context
	ctx := context.WithValue(context.Background(), "environment", "development")
	ctx = context.WithValue(ctx, "on_call", true)
	ctx = context.WithValue(ctx, "namespace", alert.Namespace)
	ctx = context.WithValue(ctx, "cluster", alert.Cluster)
	result, err := ae.agentLoop.Execute(ctx, prompt, ae.createEventHandler(sessionID))
	if err != nil {
		logger.Error("Autonomous diagnosis failed", "error", err, "session", sessionID)
		ae.logAudit(alert.Cluster, alert.Namespace, audit.ActionToolCall, audit.RiskL0,
			map[string]interface{}{"diagnosis_failed": err.Error()},
			nil, nil,
		)
		return fmt.Errorf("autonomous diagnosis failed: %w", err)
	}

	logger.Info("✅ Autonomous diagnosis completed",
		"session", sessionID,
		"iterations", result.TotalIterations,
		"duration", result.TotalDuration,
		"response_len", len(result.FinalResponse),
	)

	// 4. Log diagnosis result to audit
	ae.logAudit(alert.Cluster, alert.Namespace, audit.ActionLLMResponse, audit.RiskL0,
		map[string]interface{}{
			"diagnosis":  result.FinalResponse,
			"iterations": result.TotalIterations,
			"tokens":     result.TokenUsage.TotalTokens,
		},
		nil, nil,
	)

	return nil
}

// ===========================================================================
// Scheduled Health Check Handler
// ===========================================================================

// RunHealthCheck runs a scheduled health check for a namespace/cluster.
func (ae *AutonomousEngine) RunHealthCheck(cluster, namespace string) error {
	sessionID := uuid.New().String()

	logger.Info("🔍 Running scheduled health check",
		"cluster", cluster,
		"namespace", namespace,
		"session", sessionID,
	)

	prompt := fmt.Sprintf(
		"你对 %s 集群的 %s 命名空间执行一次完整的健康检查。"+
			"检查所有 Pod、Deployment、Service 的状态，查看是否有异常事件或错误日志。"+
			"如果有问题，诊断根因并给出修复建议。"+
			"如果一切正常，报告集群健康状态良好。",
		cluster, namespace,
	)

	ctx := context.Background()
	result, err := ae.agentLoop.Execute(ctx, prompt, ae.createEventHandler(sessionID))
	if err != nil {
		logger.Error("Health check failed", "error", err, "session", sessionID)
		return fmt.Errorf("health check failed: %w", err)
	}

	logger.Info("✅ Health check completed",
		"session", sessionID,
		"iterations", result.TotalIterations,
		"duration", result.TotalDuration,
	)

	ae.logAudit(cluster, namespace, audit.ActionLLMResponse, audit.RiskL0,
		map[string]interface{}{
			"health_check": result.FinalResponse,
			"iterations":   result.TotalIterations,
		},
		nil, nil,
	)

	return nil
}

// ===========================================================================
// Audit Integration
// ===========================================================================

func (ae *AutonomousEngine) logAudit(
	cluster, namespace string,
	action audit.AuditAction,
	riskLevel audit.RiskLevel,
	result map[string]interface{},
	preCheck map[string]interface{},
	toolName *string,
) {
	if ae.auditManager == nil {
		return
	}

	sessionID := "autonomous"
	if ae.currentSession != nil {
		sessionID = ae.currentSession.ID
	}

	userID := "autonomous-engine"

	_ = ae.auditManager.Log(context.Background(), &audit.CreateAuditEventRequest{
		SessionID: &sessionID,
		UserID:    &userID,
		Cluster:   cluster,
		Namespace: namespace,
		Action:    action,
		RiskLevel: riskLevel,
		ToolName:  toolName,
		Result:    result,
		PreCheck:  preCheck,
	})
}

// ===========================================================================
// Prompt Building
// ===========================================================================

func (ae *AutonomousEngine) buildAlertPrompt(alert *alertd.Alert) string {
	var sb strings.Builder

	sb.WriteString("🚨 收到告警，请立即诊断并尝试自动修复。\n\n")
	sb.WriteString("告警信息:\n")
	sb.WriteString(fmt.Sprintf("- ID: %s\n", alert.ID))
	sb.WriteString(fmt.Sprintf("- 严重程度: %s\n", alert.Severity))
	sb.WriteString(fmt.Sprintf("- 状态: %s\n", alert.Status))
	sb.WriteString(fmt.Sprintf("- 集群: %s\n", alert.Cluster))
	sb.WriteString(fmt.Sprintf("- 命名空间: %s\n", alert.Namespace))

	if alert.ResourceKind != "" {
		sb.WriteString(fmt.Sprintf("- 资源类型: %s\n", alert.ResourceKind))
	}
	if alert.ResourceName != "" {
		sb.WriteString(fmt.Sprintf("- 资源名称: %s\n", alert.ResourceName))
	}

	if len(alert.Labels) > 0 {
		sb.WriteString("- 标签:\n")
		for k, v := range alert.Labels {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	if len(alert.Annotations) > 0 {
		sb.WriteString("- 注解:\n")
		for k, v := range alert.Annotations {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	sb.WriteString("\n")
	sb.WriteString("请按照以下步骤处理:\n")
	sb.WriteString("1. 先使用只读工具查看当前状态 (k8s_get, k8s_describe, k8s_logs, k8s_events)\n")
	sb.WriteString("2. 分析根因，在回复中清晰说明诊断结论\n")
	sb.WriteString("3. 提出修复方案。如果是低风险操作（L0/L1），直接给出修复命令\n")
	sb.WriteString("4. 如果修复涉及 L2+ 修改操作，明确说明风险和影响范围\n")
	sb.WriteString("5. 在回复末尾，用一行单独说明：[建议操作] 操作描述 [风险等级: Lx]\n")

	return sb.String()
}

// ===========================================================================
// Event Handler
// ===========================================================================

func (ae *AutonomousEngine) createEventHandler(sessionID string) LoopEventHandler {
	return func(event LoopEvent) {
		switch event.Type {
		case "thinking":
			if thought, ok := event.Data.(string); ok && thought != "" {
				fmt.Printf("\n💭 [自主诊断] %s\n", truncate(thought, 150))
			}
		case "state_change":
			switch event.State {
			case StateExecuting:
				fmt.Println("🔧 [自主执行] 正在调用 K8s 工具...")
			case StateObserving:
				fmt.Println("👁 [自主观察] 正在分析工具返回...")
			}
		case "tool_result":
			if result, ok := event.Data.(ToolExecutionResult); ok {
				status := "✓"
				if result.Error != nil {
					status = "✗"
				}
				fmt.Printf("  %s %s [%s]\n", status, result.ToolName, result.Duration)
				if result.Error != nil {
					fmt.Printf("    错误: %v\n", result.Error)
				}
			}
		case "error":
			if err, ok := event.Data.(error); ok {
				fmt.Printf("  ✗ 错误: %v\n", err)
			}
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
