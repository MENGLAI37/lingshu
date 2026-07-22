//go:build !cgo

package audit

import (
	"testing"
	"time"

	"github.com/lingshu/lingshu/pkg/logger"
)

// ===========================================================================
// Tests that don't require CGO (no SQLite)
// Tests for: report generation, hash chain, format validation
// ===========================================================================

func TestAuditReportTextFormat(t *testing.T) {
	report := &AuditReport{
		GeneratedAt: time.Now(),
		Period:      "2026-06",
		Filters: ReportFilters{
			Cluster:   "prod-cluster",
			Namespace: "default",
		},
		Summary: AuditSummary{
			TotalEvents:     150,
			TotalSessions:   12,
			L2PlusOperations: 8,
			ComplianceScore:  94.7,
			DateRange:        "2026-06-01 ~ 2026-06-30",
		},
		RiskDistribution: map[RiskLevel]int64{
			RiskL0: 100,
			RiskL1: 42,
			RiskL2: 8,
		},
		ActionDistribution: map[AuditAction]int64{
			ActionToolCall:  140,
			ActionSessionStart: 10,
		},
	}

	text := report.toText()
	if text == "" {
		t.Fatal("text report should not be empty")
	}

	checks := []string{
		"灵枢 (LingShu)",
		"审计报告",
		"执行摘要",
		"风险等级分布",
		"操作类型分布",
		"150",
		"94.7%",
		"L0 (只读)",
		"L1 (安全写入)",
		"L2 (中等风险)",
	}
	for _, check := range checks {
		if !containsString(text, check) {
			t.Errorf("text report missing expected content: %s", check)
		}
	}
}

func TestAuditReportJSONFormat(t *testing.T) {
	report := &AuditReport{
		GeneratedAt: time.Now(),
		Summary: AuditSummary{
			TotalEvents:     10,
			TotalSessions:   1,
			L2PlusOperations: 2,
			ComplianceScore:  80.0,
			DateRange:        "全部时间",
		},
		RiskDistribution: map[RiskLevel]int64{
			RiskL0: 8,
			RiskL2: 2,
		},
		ActionDistribution: map[AuditAction]int64{
			ActionToolCall: 10,
		},
	}

	jsonText := report.toJSON()
	if jsonText == "" {
		t.Fatal("JSON report should not be empty")
	}

	checks := []string{
		"generated_at",
		"total_events",
		"risk_distribution",
		"action_distribution",
	}
	for _, check := range checks {
		if !containsString(jsonText, check) {
			t.Errorf("JSON report missing expected content: %s", check)
		}
	}
}

func TestAuditReportHTMLFormat(t *testing.T) {
	report := &AuditReport{
		GeneratedAt: time.Now(),
		Summary: AuditSummary{
			TotalEvents:     5,
			TotalSessions:   1,
			L2PlusOperations: 0,
			ComplianceScore:  100.0,
			DateRange:        "全部时间",
		},
	}

	html := report.toHTML()
	if html == "" {
		t.Fatal("HTML report should not be empty")
	}

	checks := []string{
		"<!DOCTYPE html>",
		"lang=\"zh-CN\"",
		"执行摘要",
		"compliance-score",
	}
	for _, check := range checks {
		if !containsString(html, check) {
			t.Errorf("HTML report missing expected content: %s", check)
		}
	}
}

func TestFormatDateRange(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC)

	if format := formatDateRange(&start, &end); format != "2026-06-01 ~ 2026-06-30" {
		t.Errorf("full range mismatch: %s", format)
	}
	if format := formatDateRange(nil, nil); format != "全部时间" {
		t.Errorf("empty range mismatch: %s", format)
	}
	if format := formatDateRange(&start, nil); !containsString(format, "至今") {
		t.Errorf("start-only range mismatch: %s", format)
	}
}

func TestHasFilters(t *testing.T) {
	if hasFilters(ReportFilters{}) {
		t.Error("empty filters should return false")
	}
	if !hasFilters(ReportFilters{Cluster: "test"}) {
		t.Error("non-empty filters should return true")
	}
}

func TestStableJSON(t *testing.T) {
	m := map[string]interface{}{
		"b": 2,
		"a": 1,
		"c": "hello",
	}
	result := stableJSON(m)
	expected := `{"a":1,"b":2,"c":"hello"}`
	if result != expected {
		t.Errorf("stableJSON mismatch: got %s, want %s", result, expected)
	}

	if result := stableJSON(nil); result != "{}" {
		t.Errorf("nil should return empty object, got %s", result)
	}

	r1 := stableJSON(m)
	r2 := stableJSON(m)
	if r1 != r2 {
		t.Error("stableJSON should be idempotent")
	}
}

func TestHashEventContent(t *testing.T) {
	event := &AuditEvent{
		Action:    ActionToolCall,
		Cluster:   "test",
		Namespace: "default",
		RiskLevel: RiskL2,
	}

	hash := hashEventContent(event)
	if len(hash) != 64 {
		t.Errorf("expected SHA-256 hex string of 64 chars, got %d", len(hash))
	}

	// Same event should produce same hash (deterministic)
	hash2 := hashEventContent(event)
	if hash != hash2 {
		t.Error("hashEventContent should be deterministic")
	}

	// Different event should produce different hash
	event2 := &AuditEvent{
		Action:    ActionToolCall,
		Cluster:   "test",
		Namespace: "default",
		RiskLevel: RiskL1,
	}
	hash3 := hashEventContent(event2)
	if hash == hash3 {
		t.Error("different events should have different hashes")
	}
}

func TestComputeChainHash(t *testing.T) {
	logger.Init("debug", "text")

	// computeChainHash only uses sessionLastHash — no DB needed
	m := &Manager{
		eventQueue:      make(chan AuditEvent, 100),
		sessionLastHash: make(map[string]string),
	}

	sessionID := "chain-test-session"
	event := &AuditEvent{
		SessionID: &sessionID,
		Action:    ActionToolCall,
		Cluster:   "prod",
		Namespace: "default",
		RiskLevel: RiskL2,
		Target:    map[string]interface{}{"key": "value"},
		Result:    map[string]interface{}{"success": true},
	}

	hash1 := m.computeChainHash(event)
	if hash1 == nil {
		t.Fatal("first event should have a chain hash")
	}
	if len(*hash1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d", len(*hash1))
	}

	hash2 := m.computeChainHash(event)
	if hash2 == nil {
		t.Fatal("second event should have a chain hash")
	}
	if *hash1 == *hash2 {
		t.Error("consecutive events should have different chain hashes (chaining)")
	}

	// Different session should start a separate chain
	sessionID2 := "chain-test-session-2"
	event2 := &AuditEvent{
		SessionID: &sessionID2,
		Action:    ActionToolCall,
		Cluster:   "prod",
		Namespace: "default",
		RiskLevel: RiskL2,
	}
	hash3 := m.computeChainHash(event2)
	if hash3 == nil {
		t.Fatal("different-session event should have a chain hash")
	}
	if *hash3 == *hash1 || *hash3 == *hash2 {
		t.Error("different sessions should have independent hash chains")
	}
}

func TestExportReportToFile(t *testing.T) {
	report := &AuditReport{
		GeneratedAt: time.Now(),
		Summary: AuditSummary{
			TotalEvents: 1,
			DateRange:   "全部时间",
		},
	}

	tmpFile := t.TempDir() + "/report.txt"
	err := ExportReport(report, ReportFormatText, tmpFile)
	if err != nil {
		t.Errorf("export text: %v", err)
	}
}

func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
