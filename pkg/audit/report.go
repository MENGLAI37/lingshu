package audit

import (
	"context"
	"fmt"
	"html"
	"os"
	"strings"
	"time"
)

// ===========================================================================
// Audit Report Generator — 一键生成审计报告
// ===========================================================================
//
// Generates compliance-ready audit reports from the audit event store.
// Supported formats: text, json, html.
// Reports include evidence chain verification, risk distribution,
// and per-session activity summaries.

// ReportFormat represents the output format for audit reports.
type ReportFormat string

const (
	ReportFormatText ReportFormat = "text"
	ReportFormatJSON ReportFormat = "json"
	ReportFormatHTML ReportFormat = "html"
)

// ReportOptions configures report generation.
type ReportOptions struct {
	Format      ReportFormat
	Period      string    // "2026-06" for monthly, "2026-W30" for weekly, or empty for all
	StartTime   *time.Time
	EndTime     *time.Time
	Cluster     *string
	Namespace   *string
	RiskLevel   *RiskLevel
	SessionID   *string
	VerifyChain bool // Whether to verify evidence chain integrity
	OutputPath  string // File path to write; empty = stdout
}

// AuditReport represents a generated audit report.
type AuditReport struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	Period          string                 `json:"period,omitempty"`
	Filters         ReportFilters          `json:"filters"`
	Summary         AuditSummary           `json:"summary"`
	RiskDistribution map[RiskLevel]int64   `json:"risk_distribution"`
	ActionDistribution map[AuditAction]int64 `json:"action_distribution"`
	ClusterDistribution map[string]int64   `json:"cluster_distribution,omitempty"`
	NamespaceDistribution map[string]int64 `json:"namespace_distribution,omitempty"`
	TopOperations   []OperationSummary     `json:"top_operations"`
	ChainVerified   *bool                  `json:"chain_verified,omitempty"`
	Events          []AuditEvent           `json:"events,omitempty"`
}

// ReportFilters describes the filters used for the report.
type ReportFilters struct {
	Cluster   string `json:"cluster,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	RiskLevel string `json:"risk_level,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
}

// AuditSummary provides a high-level summary for the report.
type AuditSummary struct {
	TotalEvents      int64   `json:"total_events"`
	TotalSessions    int64   `json:"total_sessions"`
	L2PlusOperations int64   `json:"l2_plus_operations"`
	BlockedOperations int64  `json:"blocked_operations"`
	ComplianceScore  float64 `json:"compliance_score"` // 0-100
	DateRange        string  `json:"date_range"`
}

// OperationSummary provides a summary of a specific operation.
type OperationSummary struct {
	ToolName  string `json:"tool_name"`
	Action    string `json:"action"`
	Count     int64  `json:"count"`
	RiskLevel string `json:"risk_level"`
}

// GenerateReport generates an audit report with the given options.
func (m *Manager) GenerateReport(ctx context.Context, opts *ReportOptions) (*AuditReport, error) {
	if opts == nil {
		opts = &ReportOptions{Format: ReportFormatText}
	}

	// Build filter from options
	filter := &AuditFilter{}
	if opts.StartTime != nil {
		filter.StartTime = opts.StartTime
	}
	if opts.EndTime != nil {
		filter.EndTime = opts.EndTime
	}
	if opts.Cluster != nil {
		filter.Cluster = opts.Cluster
	}
	if opts.Namespace != nil {
		filter.Namespace = opts.Namespace
	}
	if opts.RiskLevel != nil {
		filter.RiskLevel = opts.RiskLevel
	}
	if opts.SessionID != nil {
		filter.SessionID = opts.SessionID
	}
	filter.Limit = MaxListLimit

	// Get stats
	stats, err := m.GetStats(ctx, opts.StartTime, opts.EndTime)
	if err != nil {
		return nil, fmt.Errorf("get audit stats: %w", err)
	}

	// Get events
	listResult, err := m.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}

	report := &AuditReport{
		GeneratedAt:         time.Now(),
		Period:              opts.Period,
		Filters:             buildReportFilters(opts),
		RiskDistribution:     stats.ByRiskLevel,
		ActionDistribution:   stats.ByAction,
		ClusterDistribution:  stats.ByCluster,
		NamespaceDistribution: stats.ByNamespace,
		Events:              listResult.Events,
	}

	// Build summary
	sessionSet := make(map[string]bool)
	var l2PlusCount int64
	for _, e := range listResult.Events {
		if e.SessionID != nil {
			sessionSet[*e.SessionID] = true
		}
		if e.RiskLevel == RiskL2 || e.RiskLevel == RiskL3 || e.RiskLevel == RiskL4 {
			l2PlusCount++
		}
	}

	report.Summary = AuditSummary{
		TotalEvents:      stats.TotalEvents,
		TotalSessions:    int64(len(sessionSet)),
		L2PlusOperations: l2PlusCount,
		BlockedOperations: 0, // Blocked ops are not stored in audit — they never execute
		DateRange:        formatDateRange(opts.StartTime, opts.EndTime),
	}

	// Compliance score: ratio of L0-L1 to total, with higher ratio = better score
	if stats.TotalEvents > 0 {
		l0l1 := (stats.ByRiskLevel[RiskL0] + stats.ByRiskLevel[RiskL1])
		report.Summary.ComplianceScore = float64(l0l1) / float64(stats.TotalEvents) * 100
	}

	// Build top operations
	report.TopOperations = buildTopOperations(stats)

	// Verify evidence chain if requested
	if opts.VerifyChain {
		sessionID := ""
		if opts.SessionID != nil {
			sessionID = *opts.SessionID
		}
		verified, err := m.VerifyChain(ctx, sessionID)
		if err != nil {
			verified = false
		}
		report.ChainVerified = &verified
	}

	return report, nil
}

// ExportReport exports the report in the specified format.
// If outputPath is empty, writes to stdout.
func ExportReport(report *AuditReport, format ReportFormat, outputPath string) error {
	var content string
	switch format {
	case ReportFormatJSON:
		content = report.toJSON()
	case ReportFormatHTML:
		content = report.toHTML()
	case ReportFormatText:
		fallthrough
	default:
		content = report.toText()
	}

	if outputPath != "" {
		return os.WriteFile(outputPath, []byte(content), 0644)
	}

	// Write to stdout
	_, err := fmt.Print(content)
	return err
}

// ===========================================================================
// Format Renderers
// ===========================================================================

func (r *AuditReport) toText() string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║          灵枢 (LingShu) — 审计报告                       ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════╝\n\n")

	sb.WriteString(fmt.Sprintf("生成时间: %s\n", r.GeneratedAt.Format("2006-01-02 15:04:05")))
	if r.Period != "" {
		sb.WriteString(fmt.Sprintf("报告周期: %s\n", r.Period))
	}
	sb.WriteString(fmt.Sprintf("时间范围: %s\n\n", r.Summary.DateRange))

	// Filters
	if hasFilters(r.Filters) {
		sb.WriteString("─── 筛选条件 ───\n")
		if r.Filters.Cluster != "" {
			sb.WriteString(fmt.Sprintf("  集群: %s\n", r.Filters.Cluster))
		}
		if r.Filters.Namespace != "" {
			sb.WriteString(fmt.Sprintf("  命名空间: %s\n", r.Filters.Namespace))
		}
		if r.Filters.RiskLevel != "" {
			sb.WriteString(fmt.Sprintf("  风险等级: %s\n", r.Filters.RiskLevel))
		}
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("─── 执行摘要 ───\n")
	sb.WriteString(fmt.Sprintf("  操作总数:       %d\n", r.Summary.TotalEvents))
	sb.WriteString(fmt.Sprintf("  涉及会话数:     %d\n", r.Summary.TotalSessions))
	sb.WriteString(fmt.Sprintf("  L2+ 操作数:     %d\n", r.Summary.L2PlusOperations))
	sb.WriteString(fmt.Sprintf("  合规评分:       %.1f%%\n", r.Summary.ComplianceScore))

	if r.ChainVerified != nil {
		status := "❌ 链已断裂"
		if *r.ChainVerified {
			status = "✅ 完整无篡改"
		}
		sb.WriteString(fmt.Sprintf("  证据链完整性:   %s\n", status))
	}
	sb.WriteString("\n")

	// Risk distribution
	sb.WriteString("─── 风险等级分布 ───\n")
	riskOrder := []RiskLevel{RiskL0, RiskL1, RiskL2, RiskL3, RiskL4}
	riskLabels := map[RiskLevel]string{
		RiskL0: "L0 (只读)",
		RiskL1: "L1 (安全写入)",
		RiskL2: "L2 (中等风险)",
		RiskL3: "L3 (高风险)",
		RiskL4: "L4 (极高风险)",
	}
	for _, level := range riskOrder {
		if count, ok := r.RiskDistribution[level]; ok && count > 0 {
			bar := strings.Repeat("█", int(float64(count)/float64(max64(r.Summary.TotalEvents, 1))*40))
			if bar == "" {
				bar = "▏"
			}
			sb.WriteString(fmt.Sprintf("  %-14s %5d  %s\n", riskLabels[level], count, bar))
		}
	}
	sb.WriteString("\n")

	// Action distribution
	sb.WriteString("─── 操作类型分布 ───\n")
	for action, count := range r.ActionDistribution {
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", action, count))
	}
	sb.WriteString("\n")

	// Cluster distribution
	if len(r.ClusterDistribution) > 0 {
		sb.WriteString("─── 集群分布 ───\n")
		for cluster, count := range r.ClusterDistribution {
			sb.WriteString(fmt.Sprintf("  %-30s %d\n", cluster, count))
		}
		sb.WriteString("\n")
	}

	// Recent events
	if len(r.Events) > 0 {
		sb.WriteString("─── 最近操作记录 ───\n")
		limit := len(r.Events)
		if limit > 50 {
			limit = 50
		}
		for i := 0; i < limit; i++ {
			e := r.Events[i]
			toolName := ""
			if e.ToolName != nil {
				toolName = *e.ToolName
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s | %s | %s | %s/%s\n",
				e.CreatedAt.Format("01-02 15:04"),
				e.RiskLevel,
				e.Action,
				toolName,
				e.Cluster, e.Namespace,
			))
		}
		if len(r.Events) > limit {
			sb.WriteString(fmt.Sprintf("  ... 还有 %d 条记录\n", len(r.Events)-limit))
		}
	}

	sb.WriteString("\n══════════════════════════════════════════════════════════\n")
	sb.WriteString("报告结束。本报告由灵枢 (LingShu) 自动生成。\n")

	return sb.String()
}

func (r *AuditReport) toJSON() string {
	// Use a simplified JSON format that's easy to parse
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString(fmt.Sprintf("  \"generated_at\": %q,\n", r.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  \"period\": %q,\n", r.Period))
	sb.WriteString(fmt.Sprintf("  \"summary\": {\n"))
	sb.WriteString(fmt.Sprintf("    \"total_events\": %d,\n", r.Summary.TotalEvents))
	sb.WriteString(fmt.Sprintf("    \"total_sessions\": %d,\n", r.Summary.TotalSessions))
	sb.WriteString(fmt.Sprintf("    \"l2_plus_operations\": %d,\n", r.Summary.L2PlusOperations))
	sb.WriteString(fmt.Sprintf("    \"compliance_score\": %.1f,\n", r.Summary.ComplianceScore))
	sb.WriteString(fmt.Sprintf("    \"date_range\": %q\n", r.Summary.DateRange))
	sb.WriteString("  },\n")

	sb.WriteString("  \"risk_distribution\": {\n")
	riskLevels := []RiskLevel{RiskL0, RiskL1, RiskL2, RiskL3, RiskL4}
	for i, level := range riskLevels {
		count := r.RiskDistribution[level]
		comma := ","
		if i == len(riskLevels)-1 {
			comma = ""
		}
		sb.WriteString(fmt.Sprintf("    %q: %d%s\n", string(level), count, comma))
	}
	sb.WriteString("  },\n")

	sb.WriteString("  \"action_distribution\": {\n")
	actions := make([]AuditAction, 0, len(r.ActionDistribution))
	for a := range r.ActionDistribution {
		actions = append(actions, a)
	}
	for i, a := range actions {
		comma := ","
		if i == len(actions)-1 {
			comma = ""
		}
		sb.WriteString(fmt.Sprintf("    %q: %d%s\n", string(a), r.ActionDistribution[a], comma))
	}
	sb.WriteString("  },\n")

	if r.ChainVerified != nil {
		sb.WriteString(fmt.Sprintf("  \"chain_verified\": %v,\n", *r.ChainVerified))
	}

	sb.WriteString(fmt.Sprintf("  \"events_count\": %d\n", len(r.Events)))
	sb.WriteString("}\n")
	return sb.String()
}

func (r *AuditReport) toHTML() string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>灵枢 (LingShu) — 审计报告</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 960px; margin: 0 auto; padding: 20px; color: #1a1a1a; background: #fafafa; }
  h1 { color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; }
  h2 { color: #2c3e50; margin-top: 30px; }
  table { border-collapse: collapse; width: 100%; margin: 10px 0; }
  th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
  th { background: #3498db; color: white; }
  tr:nth-child(even) { background: #f2f2f2; }
  .summary-card { background: white; border-radius: 8px; padding: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin: 15px 0; }
  .compliance-score { font-size: 2em; font-weight: bold; }
  .score-good { color: #27ae60; }
  .score-warn { color: #f39c12; }
  .score-bad { color: #e74c3c; }
  .L0 { color: #27ae60; } .L1 { color: #2980b9; } .L2 { color: #f39c12; } .L3 { color: #e74c3c; } .L4 { color: #8e44ad; }
  .bar { display: inline-block; height: 14px; border-radius: 2px; vertical-align: middle; }
  .bar-L0 { background: #27ae60; } .bar-L1 { background: #2980b9; } .bar-L2 { background: #f39c12; } .bar-L3 { background: #e74c3c; } .bar-L4 { background: #8e44ad; }
  .chain-ok { color: #27ae60; font-weight: bold; }
  .chain-broken { color: #e74c3c; font-weight: bold; }
  .footer { margin-top: 40px; color: #888; font-size: 0.9em; border-top: 1px solid #ddd; padding-top: 10px; }
</style>
</head>
<body>
`)

	sb.WriteString("<h1>灵枢 (LingShu) — 审计报告</h1>\n")
	sb.WriteString(fmt.Sprintf("<p><strong>生成时间:</strong> %s</p>\n", r.GeneratedAt.Format("2006-01-02 15:04:05")))
	if r.Period != "" {
		sb.WriteString(fmt.Sprintf("<p><strong>报告周期:</strong> %s</p>\n", html.EscapeString(r.Period)))
	}
	sb.WriteString(fmt.Sprintf("<p><strong>时间范围:</strong> %s</p>\n", html.EscapeString(r.Summary.DateRange)))

	// Summary cards
	sb.WriteString(`<div class="summary-card">`)
	sb.WriteString("<h2>执行摘要</h2>\n")
	sb.WriteString("<table><tr><th>指标</th><th>数值</th></tr>\n")
	sb.WriteString(fmt.Sprintf("<tr><td>操作总数</td><td>%d</td></tr>\n", r.Summary.TotalEvents))
	sb.WriteString(fmt.Sprintf("<tr><td>涉及会话数</td><td>%d</td></tr>\n", r.Summary.TotalSessions))
	sb.WriteString(fmt.Sprintf("<tr><td>L2+ 操作数</td><td>%d</td></tr>\n", r.Summary.L2PlusOperations))

	scoreClass := "score-good"
	if r.Summary.ComplianceScore < 70 {
		scoreClass = "score-warn"
	}
	if r.Summary.ComplianceScore < 50 {
		scoreClass = "score-bad"
	}
	sb.WriteString(fmt.Sprintf("<tr><td>合规评分</td><td class=\"compliance-score %s\">%.1f%%</td></tr>\n", scoreClass, r.Summary.ComplianceScore))

	if r.ChainVerified != nil {
		if *r.ChainVerified {
			sb.WriteString(`<tr><td>证据链完整性</td><td class="chain-ok">✅ 完整无篡改</td></tr>` + "\n")
		} else {
			sb.WriteString(`<tr><td>证据链完整性</td><td class="chain-broken">❌ 链已断裂</td></tr>` + "\n")
		}
	}
	sb.WriteString("</table></div>\n")

	// Risk distribution
	sb.WriteString("<h2>风险等级分布</h2>\n")
	sb.WriteString("<table><tr><th>风险等级</th><th>数量</th><th>占比</th></tr>\n")
	riskLabels := map[RiskLevel]string{
		RiskL0: "L0 (只读)", RiskL1: "L1 (安全写入)", RiskL2: "L2 (中等风险)",
		RiskL3: "L3 (高风险)", RiskL4: "L4 (极高风险)",
	}
	riskOrder := []RiskLevel{RiskL0, RiskL1, RiskL2, RiskL3, RiskL4}
	for _, level := range riskOrder {
		if count, ok := r.RiskDistribution[level]; ok && count > 0 {
			pct := float64(count) / float64(max64(r.Summary.TotalEvents, 1)) * 100
			sb.WriteString(fmt.Sprintf("<tr><td class=\"%s\">%s</td><td>%d</td><td>%.1f%%</td></tr>\n",
				level, riskLabels[level], count, pct))
		}
	}
	sb.WriteString("</table>\n")

	// Recent events table
	if len(r.Events) > 0 {
		sb.WriteString("<h2>最近操作记录</h2>\n")
		sb.WriteString("<table><tr><th>时间</th><th>风险</th><th>操作</th><th>工具</th><th>位置</th></tr>\n")
		limit := len(r.Events)
		if limit > 100 {
			limit = 100
		}
		for i := 0; i < limit; i++ {
			e := r.Events[i]
			toolName := ""
			if e.ToolName != nil {
				toolName = *e.ToolName
			}
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td class=\"%s\">%s</td><td>%s</td><td>%s</td><td>%s/%s</td></tr>\n",
				e.CreatedAt.Format("2006-01-02 15:04"),
				e.RiskLevel, e.RiskLevel,
				html.EscapeString(string(e.Action)),
				html.EscapeString(toolName),
				html.EscapeString(e.Cluster), html.EscapeString(e.Namespace),
			))
		}
		sb.WriteString("</table>\n")
		if len(r.Events) > limit {
			sb.WriteString(fmt.Sprintf("<p>... 还有 %d 条记录</p>\n", len(r.Events)-limit))
		}
	}

	sb.WriteString(`<div class="footer"><p>本报告由灵枢 (LingShu) 自动生成。</p></div>` + "\n")
	sb.WriteString("</body></html>\n")

	return sb.String()
}

// ===========================================================================
// Helper Functions
// ===========================================================================

func buildReportFilters(opts *ReportOptions) ReportFilters {
	f := ReportFilters{}
	if opts.Cluster != nil {
		f.Cluster = *opts.Cluster
	}
	if opts.Namespace != nil {
		f.Namespace = *opts.Namespace
	}
	if opts.RiskLevel != nil {
		f.RiskLevel = string(*opts.RiskLevel)
	}
	if opts.SessionID != nil {
		f.SessionID = *opts.SessionID
	}
	if opts.StartTime != nil {
		f.StartTime = opts.StartTime.Format(time.RFC3339)
	}
	if opts.EndTime != nil {
		f.EndTime = opts.EndTime.Format(time.RFC3339)
	}
	return f
}

func formatDateRange(start, end *time.Time) string {
	if start != nil && end != nil {
		return fmt.Sprintf("%s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	} else if start != nil {
		return fmt.Sprintf("%s ~ 至今", start.Format("2006-01-02"))
	} else if end != nil {
		return fmt.Sprintf("最早 ~ %s", end.Format("2006-01-02"))
	}
	return "全部时间"
}

func buildTopOperations(stats *AuditStats) []OperationSummary {
	var ops []OperationSummary
	for action, count := range stats.ByAction {
		ops = append(ops, OperationSummary{
			Action: string(action),
			Count:  count,
		})
	}
	// Sort by count descending (simple bubble sort for small sets)
	for i := 0; i < len(ops); i++ {
		for j := i + 1; j < len(ops); j++ {
			if ops[j].Count > ops[i].Count {
				ops[i], ops[j] = ops[j], ops[i]
			}
		}
	}
	if len(ops) > 10 {
		ops = ops[:10]
	}
	return ops
}

func hasFilters(f ReportFilters) bool {
	return f.Cluster != "" || f.Namespace != "" || f.RiskLevel != "" || f.SessionID != ""
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
