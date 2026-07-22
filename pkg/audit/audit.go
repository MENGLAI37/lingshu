package audit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/db"
	"github.com/lingshu/lingshu/pkg/logger"
)

const (
	DefaultBatchSize    = 100
	DefaultFlushInterval = 5 * time.Second
	DefaultQueueSize     = 10000
	DefaultListLimit     = 50
	MaxListLimit         = 500
)

type auditEventDBRow struct {
	EventID         int64     `db:"event_id"`
	SessionID       *string   `db:"session_id"`
	UserID          *string   `db:"user_id"`
	Cluster         string    `db:"cluster"`
	Namespace       string    `db:"namespace"`
	Action          string    `db:"action"`
	ToolName        *string   `db:"tool_name"`
	RiskLevel       string    `db:"risk_level"`
	Target          string    `db:"target"`
	PreCheck        string    `db:"pre_check"`
	ImpactAnalysis  string    `db:"impact_analysis"`
	Result          string    `db:"result"`
	RollbackInfo    *string   `db:"rollback_info"`
	Approval        *string   `db:"approval"`
	EvidenceChainHash *string `db:"evidence_chain_hash"`
	CreatedAt       time.Time `db:"created_at"`
}

type Manager struct {
	db         *db.Database
	fileFallback *FileFallback

	eventQueue   chan AuditEvent
	batchSize    int
	flushInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex

	totalEvents     int64
	droppedEvents   int64
	flushedEvents   int64
	fallbackEvents  int64

	// Evidence hash chain: tracks the last content hash per session
	// to build an immutable prev_hash → content_hash chain.
	sessionLastHash map[string]string
	hashMu          sync.RWMutex
}

var (
	instance *Manager
)

func Init(cfg *config.Config) (*Manager, error) {
	database := db.Get()
	if database == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	fileFallback, err := NewFileFallback("")
	if err != nil {
		logger.Warn("Failed to initialize file fallback", "error", err)
	}

	instance = &Manager{
		db:            database,
		fileFallback:  fileFallback,
		eventQueue:    make(chan AuditEvent, DefaultQueueSize),
		batchSize:     DefaultBatchSize,
		flushInterval: DefaultFlushInterval,
		stopCh:        make(chan struct{}),
		sessionLastHash: make(map[string]string),
	}

	if err := instance.Start(); err != nil {
		return nil, fmt.Errorf("start audit manager: %w", err)
	}

	logger.Info("Audit manager initialized")
	return instance, nil
}

func Get() *Manager {
	return instance
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("audit manager already running")
	}

	m.stopCh = make(chan struct{})
	m.running = true
	m.wg.Add(1)

	go m.run()

	logger.Info("Audit manager started",
		"batch_size", m.batchSize,
		"flush_interval", m.flushInterval.String(),
	)

	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	close(m.stopCh)
	m.wg.Wait()
	m.running = false

	m.flushRemaining()

	logger.Info("Audit manager stopped",
		"total_events", atomic.LoadInt64(&m.totalEvents),
		"flushed_events", atomic.LoadInt64(&m.flushedEvents),
		"dropped_events", atomic.LoadInt64(&m.droppedEvents),
		"fallback_events", atomic.LoadInt64(&m.fallbackEvents),
	)

	return nil
}

func (m *Manager) run() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.flushInterval)
	defer ticker.Stop()

	batch := make([]AuditEvent, 0, m.batchSize)

	for {
		select {
		case <-m.stopCh:
			if len(batch) > 0 {
				m.flushBatch(batch)
			}
			return

		case event := <-m.eventQueue:
			batch = append(batch, event)
			if len(batch) >= m.batchSize {
				m.flushBatch(batch)
				batch = make([]AuditEvent, 0, m.batchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				m.flushBatch(batch)
				batch = make([]AuditEvent, 0, m.batchSize)
			}
		}
	}
}

func (m *Manager) flushRemaining() {
	for {
		select {
		case event := <-m.eventQueue:
			batch := []AuditEvent{event}
		outer:
			for i := 0; i < m.batchSize-1; i++ {
				select {
				case e := <-m.eventQueue:
					batch = append(batch, e)
				default:
					break outer
				}
			}
			m.flushBatch(batch)
		default:
			return
		}
	}
}

func (m *Manager) flushBatch(batch []AuditEvent) {
	if len(batch) == 0 {
		return
	}

	ctx := context.Background()

	err := m.insertBatch(ctx, batch)
	if err != nil {
		logger.Warn("Failed to insert audit batch to database, falling back to file",
			"error", err,
			"batch_size", len(batch),
		)

		if m.fileFallback != nil {
			fallbackErr := m.fileFallback.WriteBatch(batch)
			if fallbackErr != nil {
				logger.Error("Failed to write audit batch to file fallback",
					"error", fallbackErr,
					"batch_size", len(batch),
				)
				atomic.AddInt64(&m.droppedEvents, int64(len(batch)))
			} else {
				atomic.AddInt64(&m.fallbackEvents, int64(len(batch)))
			}
		} else {
			atomic.AddInt64(&m.droppedEvents, int64(len(batch)))
		}
		return
	}

	atomic.AddInt64(&m.flushedEvents, int64(len(batch)))
}

func (m *Manager) insertBatch(ctx context.Context, events []AuditEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := m.db.DB().BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		INSERT INTO audit_events (
			session_id, user_id, cluster, namespace, action, tool_name,
			risk_level, target, pre_check, impact_analysis, result,
			rollback_info, approval, evidence_chain_hash, created_at
		) VALUES (
			:session_id, :user_id, :cluster, :namespace, :action, :tool_name,
			:risk_level, :target, :pre_check, :impact_analysis, :result,
			:rollback_info, :approval, :evidence_chain_hash, :created_at
		)
	`

	for _, event := range events {
		row := eventToDBRow(&event)
		if _, err := tx.NamedExecContext(ctx, query, row); err != nil {
			return fmt.Errorf("insert audit event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (m *Manager) Log(ctx context.Context, req *CreateAuditEventRequest) error {
	if req == nil {
		return fmt.Errorf("audit event request is nil")
	}

	cluster := req.Cluster
	if cluster == "" {
		cluster = "default"
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = "default"
	}

	if req.Target == nil {
		req.Target = map[string]interface{}{}
	}
	if req.PreCheck == nil {
		req.PreCheck = map[string]interface{}{}
	}
	if req.ImpactAnalysis == nil {
		req.ImpactAnalysis = map[string]interface{}{}
	}
	if req.Result == nil {
		req.Result = map[string]interface{}{}
	}

	event := AuditEvent{
		SessionID:      req.SessionID,
		UserID:         req.UserID,
		Cluster:        cluster,
		Namespace:      namespace,
		Action:         req.Action,
		ToolName:       req.ToolName,
		RiskLevel:      req.RiskLevel,
		Target:         req.Target,
		PreCheck:       req.PreCheck,
		ImpactAnalysis: req.ImpactAnalysis,
		Result:         req.Result,
		RollbackInfo:   req.RollbackInfo,
		Approval:       req.Approval,
		CreatedAt:      time.Now(),
	}

	// Build immutable evidence hash chain: prev_hash → content_hash
	event.EvidenceChainHash = m.computeChainHash(&event)

	atomic.AddInt64(&m.totalEvents, 1)

	select {
	case m.eventQueue <- event:
		return nil
	default:
		atomic.AddInt64(&m.droppedEvents, 1)
		logger.Warn("Audit event queue full, dropping event",
			"action", req.Action,
			"queue_size", len(m.eventQueue),
		)
		return fmt.Errorf("audit event queue full")
	}
}

func (m *Manager) Get(ctx context.Context, eventID int64) (*AuditEvent, error) {
	var row auditEventDBRow
	query := `SELECT * FROM audit_events WHERE event_id = $1`

	err := m.db.GetContext(ctx, &row, query, eventID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("audit event not found: %d", eventID)
		}
		return nil, fmt.Errorf("get audit event: %w", err)
	}

	return dbRowToEvent(&row), nil
}

func (m *Manager) List(ctx context.Context, filter *AuditFilter) (*AuditListResult, error) {
	if filter == nil {
		filter = &AuditFilter{}
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

	if filter.SessionID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("session_id = $%d", argIdx))
		args = append(args, *filter.SessionID)
		argIdx++
	}
	if filter.UserID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
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
	if filter.Action != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, string(*filter.Action))
		argIdx++
	}
	if filter.ToolName != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("tool_name = $%d", argIdx))
		args = append(args, *filter.ToolName)
		argIdx++
	}
	if filter.RiskLevel != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("risk_level = $%d", argIdx))
		args = append(args, string(*filter.RiskLevel))
		argIdx++
	}
	if filter.StartTime != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.StartTime)
		argIdx++
	}
	if filter.EndTime != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filter.EndTime)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + joinStrings(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM audit_events %s`, whereSQL)
	var total int64
	if err := m.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, fmt.Errorf("count audit events: %w", err)
	}

	listQuery := fmt.Sprintf(`
		SELECT * FROM audit_events %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	listArgs := append(args, limit, offset)

	var rows []auditEventDBRow
	if err := m.db.SelectContext(ctx, &rows, listQuery, listArgs...); err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}

	events := make([]AuditEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, *dbRowToEvent(&row))
	}

	hasMore := int64(offset+len(events)) < total

	return &AuditListResult{
		Events:  events,
		Total:   total,
		HasMore: hasMore,
	}, nil
}

func (m *Manager) GetStats(ctx context.Context, startTime, endTime *time.Time) (*AuditStats, error) {
	stats := &AuditStats{
		ByRiskLevel: make(map[RiskLevel]int64),
		ByAction:    make(map[AuditAction]int64),
		ByCluster:   make(map[string]int64),
		ByNamespace: make(map[string]int64),
	}

	if startTime != nil {
		stats.TimeRange.Start = *startTime
	}
	if endTime != nil {
		stats.TimeRange.End = *endTime
	}

	whereClauses := []string{}
	args := []interface{}{}

	if startTime != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", len(args)+1))
		args = append(args, *startTime)
	}
	if endTime != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", len(args)+1))
		args = append(args, *endTime)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + joinStrings(whereClauses, " AND ")
	}

	totalQuery := fmt.Sprintf(`SELECT COUNT(*) FROM audit_events %s`, whereSQL)
	if err := m.db.GetContext(ctx, &stats.TotalEvents, totalQuery, args...); err != nil {
		return nil, fmt.Errorf("get total events: %w", err)
	}

	riskQuery := fmt.Sprintf(`
		SELECT risk_level, COUNT(*) as count
		FROM audit_events %s
		GROUP BY risk_level
	`, whereSQL)

	type riskCount struct {
		RiskLevel string `db:"risk_level"`
		Count     int64  `db:"count"`
	}
	var riskCounts []riskCount
	if err := m.db.SelectContext(ctx, &riskCounts, riskQuery, args...); err == nil {
		for _, rc := range riskCounts {
			stats.ByRiskLevel[RiskLevel(rc.RiskLevel)] = rc.Count
		}
	}

	actionQuery := fmt.Sprintf(`
		SELECT action, COUNT(*) as count
		FROM audit_events %s
		GROUP BY action
	`, whereSQL)

	type actionCount struct {
		Action string `db:"action"`
		Count  int64  `db:"count"`
	}
	var actionCounts []actionCount
	if err := m.db.SelectContext(ctx, &actionCounts, actionQuery, args...); err == nil {
		for _, ac := range actionCounts {
			stats.ByAction[AuditAction(ac.Action)] = ac.Count
		}
	}

	return stats, nil
}

func (m *Manager) GetQueueSize() int {
	return len(m.eventQueue)
}

func (m *Manager) GetStatsInfo() map[string]int64 {
	return map[string]int64{
		"total_events":    atomic.LoadInt64(&m.totalEvents),
		"flushed_events":  atomic.LoadInt64(&m.flushedEvents),
		"dropped_events":  atomic.LoadInt64(&m.droppedEvents),
		"fallback_events": atomic.LoadInt64(&m.fallbackEvents),
		"queue_size":      int64(len(m.eventQueue)),
	}
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Manager) SetBatchSize(size int) {
	if size > 0 {
		m.batchSize = size
	}
}

func (m *Manager) SetFlushInterval(interval time.Duration) {
	if interval > 0 {
		m.flushInterval = interval
	}
}

func (m *Manager) Close() error {
	return m.Stop()
}

func eventToDBRow(event *AuditEvent) *auditEventDBRow {
	targetJSON, _ := json.Marshal(event.Target)
	preCheckJSON, _ := json.Marshal(event.PreCheck)
	impactAnalysisJSON, _ := json.Marshal(event.ImpactAnalysis)
	resultJSON, _ := json.Marshal(event.Result)

	var rollbackJSON *string
	if event.RollbackInfo != nil {
		data, _ := json.Marshal(*event.RollbackInfo)
		s := string(data)
		rollbackJSON = &s
	}

	var approvalJSON *string
	if event.Approval != nil {
		data, _ := json.Marshal(*event.Approval)
		s := string(data)
		approvalJSON = &s
	}

	return &auditEventDBRow{
		EventID:         event.EventID,
		SessionID:       event.SessionID,
		UserID:          event.UserID,
		Cluster:         event.Cluster,
		Namespace:       event.Namespace,
		Action:          string(event.Action),
		ToolName:        event.ToolName,
		RiskLevel:       string(event.RiskLevel),
		Target:          string(targetJSON),
		PreCheck:        string(preCheckJSON),
		ImpactAnalysis:  string(impactAnalysisJSON),
		Result:          string(resultJSON),
		RollbackInfo:    rollbackJSON,
		Approval:        approvalJSON,
		EvidenceChainHash: event.EvidenceChainHash,
		CreatedAt:       event.CreatedAt,
	}
}

func dbRowToEvent(row *auditEventDBRow) *AuditEvent {
	var target map[string]interface{}
	if err := json.Unmarshal([]byte(row.Target), &target); err != nil {
		target = map[string]interface{}{}
	}

	var preCheck map[string]interface{}
	if err := json.Unmarshal([]byte(row.PreCheck), &preCheck); err != nil {
		preCheck = map[string]interface{}{}
	}

	var impactAnalysis map[string]interface{}
	if err := json.Unmarshal([]byte(row.ImpactAnalysis), &impactAnalysis); err != nil {
		impactAnalysis = map[string]interface{}{}
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(row.Result), &result); err != nil {
		result = map[string]interface{}{}
	}

	var rollbackInfo *map[string]interface{}
	if row.RollbackInfo != nil {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(*row.RollbackInfo), &m); err == nil {
			rollbackInfo = &m
		}
	}

	var approval *map[string]interface{}
	if row.Approval != nil {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(*row.Approval), &m); err == nil {
			approval = &m
		}
	}

	return &AuditEvent{
		EventID:         row.EventID,
		SessionID:       row.SessionID,
		UserID:          row.UserID,
		Cluster:         row.Cluster,
		Namespace:       row.Namespace,
		Action:          AuditAction(row.Action),
		ToolName:        row.ToolName,
		RiskLevel:       RiskLevel(row.RiskLevel),
		Target:          target,
		PreCheck:        preCheck,
		ImpactAnalysis:  impactAnalysis,
		Result:          result,
		RollbackInfo:    rollbackInfo,
		Approval:        approval,
		EvidenceChainHash: row.EvidenceChainHash,
		CreatedAt:       row.CreatedAt,
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

// ===========================================================================
// Evidence Hash Chain — immutable audit trail
// ===========================================================================
//
// Every audit event is hashed with the previous event's hash, forming a
// cryptographically verifiable chain: prev_hash → content_hash.
// Tampering with any event breaks the chain, making the audit trail
// immutable and compliance-ready.

// computeChainHash computes the evidence chain hash for an event.
// It combines the previous event's hash (per session) with the current
// event's content to produce a SHA-256 hash, then stores it as the new
// "last hash" for the session.
func (m *Manager) computeChainHash(event *AuditEvent) *string {
	if event == nil {
		return nil
	}

	// Get the previous hash for this session
	sessionKey := "global"
	if event.SessionID != nil && *event.SessionID != "" {
		sessionKey = *event.SessionID
	}

	m.hashMu.RLock()
	prevHash := m.sessionLastHash[sessionKey]
	m.hashMu.RUnlock()

	// Compute content hash for this event
	contentHash := hashEventContent(event)

	// Build chain hash: SHA-256(prev_hash || content_hash)
	hasher := sha256.New()
	hasher.Write([]byte(prevHash))
	hasher.Write([]byte(contentHash))
	chainHash := hex.EncodeToString(hasher.Sum(nil))

	// Store as the new "last hash" for next event in session
	m.hashMu.Lock()
	m.sessionLastHash[sessionKey] = chainHash
	m.hashMu.Unlock()

	return &chainHash
}

// hashEventContent computes a deterministic SHA-256 hash of the event's core content
// (excluding metadata like timestamps and IDs that vary per write).
func hashEventContent(event *AuditEvent) string {
	hasher := sha256.New()

	// Use stable JSON serialization for deterministic hashing
	writeField := func(key, value string) {
		hasher.Write([]byte(key))
		hasher.Write([]byte{0})
		hasher.Write([]byte(value))
		hasher.Write([]byte{0})
	}

	writeField("action", string(event.Action))
	writeField("cluster", event.Cluster)
	writeField("namespace", event.Namespace)
	writeField("risk_level", string(event.RiskLevel))

	if event.ToolName != nil {
		writeField("tool_name", *event.ToolName)
	}
	if event.SessionID != nil {
		writeField("session_id", *event.SessionID)
	}
	if event.UserID != nil {
		writeField("user_id", *event.UserID)
	}

	// Hash maps with sorted keys for determinism
	writeField("target", stableJSON(event.Target))
	writeField("pre_check", stableJSON(event.PreCheck))
	writeField("impact_analysis", stableJSON(event.ImpactAnalysis))
	writeField("result", stableJSON(event.Result))

	if event.RollbackInfo != nil {
		writeField("rollback_info", stableJSON(*event.RollbackInfo))
	}
	if event.Approval != nil {
		writeField("approval", stableJSON(*event.Approval))
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// stableJSON produces a deterministic JSON string from a map by sorting keys.
func stableJSON(m map[string]interface{}) string {
	if m == nil {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result string
	result = "{"
	for i, k := range keys {
		if i > 0 {
			result += ","
		}
		valBytes, _ := json.Marshal(m[k])
		result += fmt.Sprintf(`"%s":%s`, k, string(valBytes))
	}
	result += "}"
	return result
}

// VerifyChain verifies the integrity of an audit event chain.
// Returns true if the chain is intact (no tampering detected).
func (m *Manager) VerifyChain(ctx context.Context, sessionID string) (bool, error) {
	filter := &AuditFilter{}
	if sessionID != "" {
		filter.SessionID = &sessionID
	}
	filter.Limit = MaxListLimit

	result, err := m.List(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("list events for verification: %w", err)
	}

	if len(result.Events) < 2 {
		return true, nil // Need at least 2 events to verify chain
	}

	// Verify each event's chain hash
	for i := 1; i < len(result.Events); i++ {
		prev := &result.Events[i-1]
		curr := &result.Events[i]

		// Recompute: SHA-256(prev_chain_hash || curr_content_hash)
		prevChainHash := ""
		if prev.EvidenceChainHash != nil {
			prevChainHash = *prev.EvidenceChainHash
		}
		currContentHash := hashEventContent(curr)

		hasher := sha256.New()
		hasher.Write([]byte(prevChainHash))
		hasher.Write([]byte(currContentHash))
		expected := hex.EncodeToString(hasher.Sum(nil))

		actual := ""
		if curr.EvidenceChainHash != nil {
			actual = *curr.EvidenceChainHash
		}

		if expected != actual {
			return false, fmt.Errorf("chain broken at event %d: expected hash %s, got %s",
				curr.EventID, expected, actual)
		}
	}

	return true, nil
}

var _ = os.File{}
