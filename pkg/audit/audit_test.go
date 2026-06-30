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
