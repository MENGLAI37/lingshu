//go:build cgo

package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/db"
	"github.com/lingshu/lingshu/pkg/logger"
)

func setupAuditTestDB(t *testing.T) *db.Database {
	t.Helper()

	if os.Getenv("SKIP_SQLITE_TESTS") == "true" {
		t.Skip("SKIP_SQLITE_TESTS is set")
	}

	cfg := &config.DBConfig{
		Type:         "sqlite",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}

	database, err := db.Init(cfg)
	require.NoError(t, err)

	createAuditEventsTable(t, database)
	clearAuditEventsTable(t, database)

	return database
}

func createAuditEventsTable(t *testing.T, d *db.Database) {
	t.Helper()

	query := `
		CREATE TABLE IF NOT EXISTS audit_events (
			event_id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			user_id TEXT,
			cluster TEXT NOT NULL DEFAULT 'default',
			namespace TEXT NOT NULL DEFAULT 'default',
			action TEXT NOT NULL,
			tool_name TEXT,
			risk_level TEXT NOT NULL DEFAULT 'L0',
			target TEXT NOT NULL DEFAULT '{}',
			pre_check TEXT NOT NULL DEFAULT '{}',
			impact_analysis TEXT NOT NULL DEFAULT '{}',
			result TEXT NOT NULL DEFAULT '{}',
			rollback_info TEXT,
			approval TEXT,
			evidence_chain_hash TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`

	_, err := d.DB().Exec(query)
	require.NoError(t, err)
}

func clearAuditEventsTable(t *testing.T, d *db.Database) {
	t.Helper()
	_, err := d.DB().Exec(`DELETE FROM audit_events`)
	require.NoError(t, err)
}

func newTestManager(t *testing.T, database *db.Database) *Manager {
	t.Helper()

	tmpDir := t.TempDir()
	fallbackDir := filepath.Join(tmpDir, "audit_fallback")

	fileFallback, err := NewFileFallback(fallbackDir)
	require.NoError(t, err)

	m := &Manager{
		db:            database,
		fileFallback:  fileFallback,
		eventQueue:    make(chan AuditEvent, DefaultQueueSize),
		batchSize:     10,
		flushInterval: 100 * time.Millisecond,
		stopCh:        make(chan struct{}),
		sessionLastHash: make(map[string]string),
	}

	t.Cleanup(func() {
		_ = m.Stop()
		_ = fileFallback.Close()
	})

	return m
}

func TestAuditLogEvent(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	toolName := "kubectl"
	req := &CreateAuditEventRequest{
		Cluster:    "test-cluster",
		Namespace:  "test-ns",
		Action:     ActionToolCall,
		ToolName:   &toolName,
		RiskLevel:  RiskL2,
		Target:     map[string]interface{}{"resource": "pods"},
		PreCheck:   map[string]interface{}{"safe": true},
		Result:     map[string]interface{}{"success": true},
	}

	err = manager.Log(ctx, req)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	stats := manager.GetStatsInfo()
	assert.Equal(t, int64(1), stats["total_events"])
	assert.Equal(t, int64(1), stats["flushed_events"])
}

func TestAuditBatchFlush(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	manager.SetBatchSize(5)
	manager.SetFlushInterval(5 * time.Second)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 7; i++ {
		req := &CreateAuditEventRequest{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    ActionToolCall,
			RiskLevel: RiskL1,
			Target:    map[string]interface{}{"index": i},
		}
		err := manager.Log(ctx, req)
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	stats := manager.GetStatsInfo()
	assert.Equal(t, int64(7), stats["total_events"])
	assert.Equal(t, int64(5), stats["flushed_events"])

	time.Sleep(6 * time.Second)
	stats = manager.GetStatsInfo()
	assert.Equal(t, int64(7), stats["flushed_events"])
}

func TestAuditListEvents(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		req := &CreateAuditEventRequest{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    ActionToolCall,
			RiskLevel: RiskL2,
			Target:    map[string]interface{}{"index": i},
		}
		err := manager.Log(ctx, req)
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	result, err := manager.List(ctx, &AuditFilter{
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), result.Total)
	assert.Len(t, result.Events, 5)
}

func TestAuditFilterByRiskLevel(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	riskLevels := []RiskLevel{RiskL0, RiskL1, RiskL2, RiskL3, RiskL4}
	for i, level := range riskLevels {
		req := &CreateAuditEventRequest{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    ActionToolCall,
			RiskLevel: level,
			Target:    map[string]interface{}{"index": i},
		}
		err := manager.Log(ctx, req)
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	riskL2 := RiskL2
	result, err := manager.List(ctx, &AuditFilter{
		RiskLevel: &riskL2,
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Events, 1)
	assert.Equal(t, RiskL2, result.Events[0].RiskLevel)
}

func TestAuditGetStats(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	actions := []AuditAction{ActionToolCall, ActionToolCall, ActionSessionStart, ActionApproval}
	for _, action := range actions {
		req := &CreateAuditEventRequest{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    action,
			RiskLevel: RiskL1,
			Target:    map[string]interface{}{},
		}
		err := manager.Log(ctx, req)
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	stats, err := manager.GetStats(ctx, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(4), stats.TotalEvents)
	assert.Equal(t, int64(2), stats.ByAction[ActionToolCall])
	assert.Equal(t, int64(1), stats.ByAction[ActionSessionStart])
}

func TestAuditFileFallback(t *testing.T) {
	logger.Init("debug", "text")
	tmpDir := t.TempDir()
	fallbackDir := filepath.Join(tmpDir, "audit_fallback")

	fb, err := NewFileFallback(fallbackDir)
	require.NoError(t, err)
	defer func() {
		_ = fb.Close()
	}()

	events := []AuditEvent{
		{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    ActionToolCall,
			RiskLevel: RiskL2,
			Target:    map[string]interface{}{"key": "value1"},
			CreatedAt: time.Now(),
		},
		{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    ActionSessionStart,
			RiskLevel: RiskL0,
			Target:    map[string]interface{}{"key": "value2"},
			CreatedAt: time.Now(),
		},
	}

	err = fb.WriteBatch(events)
	require.NoError(t, err)

	readEvents, err := fb.ReadAll()
	require.NoError(t, err)
	assert.Len(t, readEvents, 2)
}

func TestAuditFileFallbackClear(t *testing.T) {
	logger.Init("debug", "text")
	tmpDir := t.TempDir()
	fallbackDir := filepath.Join(tmpDir, "audit_fallback")

	fb, err := NewFileFallback(fallbackDir)
	require.NoError(t, err)
	defer func() {
		_ = fb.Close()
	}()

	event := &AuditEvent{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
		Action:    ActionToolCall,
		RiskLevel: RiskL1,
		Target:    map[string]interface{}{},
		CreatedAt: time.Now(),
	}

	err = fb.Write(event)
	require.NoError(t, err)

	before, err := fb.ReadAll()
	require.NoError(t, err)
	assert.Len(t, before, 1)

	err = fb.Clear()
	require.NoError(t, err)

	after, err := fb.ReadAll()
	require.NoError(t, err)
	assert.Len(t, after, 0)
}

func TestAuditManagerStop(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)
	assert.True(t, manager.IsRunning())

	ctx := context.Background()
	req := &CreateAuditEventRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
		Action:    ActionToolCall,
		RiskLevel: RiskL1,
		Target:    map[string]interface{}{},
	}
	err = manager.Log(ctx, req)
	require.NoError(t, err)

	err = manager.Stop()
	require.NoError(t, err)
	assert.False(t, manager.IsRunning())

	stats := manager.GetStatsInfo()
	assert.Equal(t, int64(1), stats["flushed_events"])
}

func TestAuditDefaultValues(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	req := &CreateAuditEventRequest{
		Action:    ActionToolCall,
		RiskLevel: RiskL0,
	}

	err = manager.Log(ctx, req)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	result, err := manager.List(ctx, &AuditFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, "default", result.Events[0].Cluster)
	assert.Equal(t, "default", result.Events[0].Namespace)
}

func TestEvidenceHashChain(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()
	sessionID := "test-session-hash"
	userID := "test-user"

	// Log 3 events in the same session
	for i := 0; i < 3; i++ {
		req := &CreateAuditEventRequest{
			SessionID: &sessionID,
			UserID:    &userID,
			Cluster:   "prod-cluster",
			Namespace: "prod-ns",
			Action:    ActionToolCall,
			ToolName:  strPtr("k8s_scale"),
			RiskLevel: RiskL2,
			Target:    map[string]interface{}{"step": i},
			Result:    map[string]interface{}{"success": true},
		}
		err := manager.Log(ctx, req)
		require.NoError(t, err)
	}

	time.Sleep(300 * time.Millisecond)

	// Verify all events have hash chain set
	result, err := manager.List(ctx, &AuditFilter{
		SessionID: &sessionID,
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.Total)

	for _, event := range result.Events {
		assert.NotNil(t, event.EvidenceChainHash, "event should have an evidence chain hash")
		assert.Len(t, *event.EvidenceChainHash, 64, "SHA-256 hash should be 64 hex chars")
	}

	// Events in the same session should have different hashes (no duplicates)
	hashes := make(map[string]bool)
	for _, event := range result.Events {
		hash := *event.EvidenceChainHash
		assert.False(t, hashes[hash], "each event should have a unique chain hash")
		hashes[hash] = true
	}
}

func TestAuditReportGeneration(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	// Log events at various risk levels
	sessionID := "report-test-session"
	for i := 0; i < 10; i++ {
		req := &CreateAuditEventRequest{
			SessionID: &sessionID,
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    ActionToolCall,
			RiskLevel: []RiskLevel{RiskL0, RiskL1}[i%2], // L0/L1 alternating
			Result:    map[string]interface{}{"index": i},
		}
		_ = manager.Log(ctx, req)
	}

	// Add a few L2 operations
	for i := 0; i < 3; i++ {
		req := &CreateAuditEventRequest{
			SessionID: &sessionID,
			Cluster:   "test-cluster",
			Namespace: "test-ns",
			Action:    ActionToolCall,
			RiskLevel: RiskL2,
			Result:    map[string]interface{}{"index": i},
		}
		_ = manager.Log(ctx, req)
	}

	time.Sleep(300 * time.Millisecond)

	// Generate a text report
	opts := &ReportOptions{
		Format:      ReportFormatText,
		SessionID:   &sessionID,
		VerifyChain: true,
	}
	report, err := manager.GenerateReport(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, int64(13), report.Summary.TotalEvents)
	assert.Equal(t, int64(1), report.Summary.TotalSessions)
	assert.Equal(t, int64(3), report.Summary.L2PlusOperations)

	// Verify text report renders
	textReport := report.toText()
	assert.Contains(t, textReport, "灵枢 (LingShu)")
	assert.Contains(t, textReport, "审计报告")
	assert.Contains(t, textReport, "执行摘要")

	// Verify JSON report renders
	jsonReport := report.toJSON()
	assert.Contains(t, jsonReport, "generated_at")
	assert.Contains(t, jsonReport, "total_events")
	assert.Contains(t, jsonReport, "risk_distribution")

	// Verify HTML report renders
	htmlReport := report.toHTML()
	assert.Contains(t, htmlReport, "<!DOCTYPE html>")
	assert.Contains(t, htmlReport, "运行摘要")
}

func TestAuditReportFormats(t *testing.T) {
	logger.Init("debug", "text")
	database := setupAuditTestDB(t)
	manager := newTestManager(t, database)

	err := manager.Start()
	require.NoError(t, err)

	ctx := context.Background()

	sessionID := "format-test"
	req := &CreateAuditEventRequest{
		SessionID: &sessionID,
		Cluster:   "prod",
		Namespace: "default",
		Action:    ActionToolCall,
		RiskLevel: RiskL0,
		Result:    map[string]interface{}{"ok": true},
	}
	_ = manager.Log(ctx, req)

	time.Sleep(200 * time.Millisecond)

	// Test text format
	opts := &ReportOptions{Format: ReportFormatText}
	report, err := manager.GenerateReport(ctx, opts)
	require.NoError(t, err)
	textOutput := report.toText()
	assert.NotEmpty(t, textOutput)
	assert.Contains(t, textOutput, "L0 (只读)")

	// Test JSON format
	jsonOutput := report.toJSON()
	assert.NotEmpty(t, jsonOutput)
	assert.Contains(t, jsonOutput, "\"L0\"")

	// Test HTML format
	htmlOutput := report.toHTML()
	assert.NotEmpty(t, htmlOutput)
	assert.Contains(t, htmlOutput, "lang=\"zh-CN\"")
}

func strPtr(s string) *string {
	return &s
}
