package llm

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ===========================================================================
// Token Metrics Collector
// ===========================================================================

// DefaultMetricsCollector implements MetricsCollector with DB persistence.
type DefaultMetricsCollector struct {
	db        *sql.DB
	batch     []UsageRecord
	batchSize int
	flushInterval time.Duration
	mu        sync.Mutex
	done      chan struct{}
}

// UsageRecord represents a single LLM usage record.
type UsageRecord struct {
	RecordID      string        `json:"record_id"`
	SessionID     string        `json:"session_id"`
	UserID        string        `json:"user_id"`
	TeamID        string        `json:"team_id"`
	Provider      ProviderType  `json:"provider"`
	Model         string        `json:"model"`
	Operation     string        `json:"operation"`
	InputTokens   int64         `json:"input_tokens"`
	OutputTokens  int64         `json:"output_tokens"`
	CostUSDMilli  int64         `json:"cost_usd_milli"`
	LatencyMs     int64         `json:"latency_ms"`
	Error         string        `json:"error,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}

// CostRates defines token cost per 1K tokens (in milli-cents).
type CostRates struct {
	InputRate  float64
	OutputRate float64
}

// Default cost rates (approximate, should be configurable).
var defaultCostRates = map[string]CostRates{
	"gpt-4o":          {InputRate: 2.5, OutputRate: 10.0},
	"gpt-4o-mini":     {InputRate: 0.15, OutputRate: 0.6},
	"claude-3-opus":   {InputRate: 15.0, OutputRate: 75.0},
	"claude-3-sonnet": {InputRate: 3.0, OutputRate: 15.0},
	"qwen2.5-coder:7b": {InputRate: 0.0, OutputRate: 0.0}, // local = free
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(db *sql.DB) *DefaultMetricsCollector {
	m := &DefaultMetricsCollector{
		db:            db,
		batch:         make([]UsageRecord, 0, 100),
		batchSize:     100,
		flushInterval: 5 * time.Second,
		done:          make(chan struct{}),
	}
	go m.flushLoop()
	return m
}

// Close stops the background flush loop.
func (m *DefaultMetricsCollector) Close() error {
	close(m.done)
	return m.Flush()
}

// RecordUsage records LLM usage metrics.
func (m *DefaultMetricsCollector) RecordUsage(ctx context.Context, provider ProviderType, model string, usage TokenUsage, latency time.Duration, err error) {
	record := UsageRecord{
		RecordID:     uuid.New().String(),
		Provider:     provider,
		Model:        model,
		Operation:    "completion",
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		LatencyMs:    latency.Milliseconds(),
		CreatedAt:    time.Now(),
	}

	// Calculate cost
	rates, ok := defaultCostRates[model]
	if !ok {
		// Try to find by prefix
		for k, v := range defaultCostRates {
			if len(model) >= len(k) && model[:len(k)] == k {
				rates = v
				break
			}
		}
	}
	cost := (float64(usage.InputTokens)*rates.InputRate + float64(usage.OutputTokens)*rates.OutputRate) / 1000.0
	record.CostUSDMilli = int64(cost * 1000) // Convert to milli-USD

	if err != nil {
		record.Error = err.Error()
	}

	// Extract session/user from context if available
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		record.SessionID = sessionID
	}
	if userID, ok := ctx.Value("user_id").(string); ok {
		record.UserID = userID
	}
	if teamID, ok := ctx.Value("team_id").(string); ok {
		record.TeamID = teamID
	}

	m.mu.Lock()
	m.batch = append(m.batch, record)
	shouldFlush := len(m.batch) >= m.batchSize
	m.mu.Unlock()

	if shouldFlush {
		_ = m.Flush()
	}
}

// Flush writes all pending records to the database.
func (m *DefaultMetricsCollector) Flush() error {
	m.mu.Lock()
	batch := make([]UsageRecord, len(m.batch))
	copy(batch, m.batch)
	m.batch = m.batch[:0]
	m.mu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	if m.db == nil {
		// DB not available, log or buffer to file
		return fmt.Errorf("database not available, dropped %d usage records", len(batch))
	}

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO usage_records (
			record_id, session_id, user_id, team_id, model, operation,
			input_tokens, output_tokens, cost_usd_milli, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range batch {
		_, err := stmt.Exec(
			r.RecordID, r.SessionID, r.UserID, r.TeamID,
			r.Model, r.Operation, r.InputTokens, r.OutputTokens,
			r.CostUSDMilli, r.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetUsageStats returns aggregated usage statistics.
func (m *DefaultMetricsCollector) GetUsageStats(ctx context.Context, sessionID string) (*UsageStats, error) {
	if m.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var stats UsageStats
	err := m.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd_milli), 0),
			COUNT(*)
		FROM usage_records
		WHERE session_id = $1
	`, sessionID).Scan(
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCostUSDMilli,
		&stats.TotalCalls,
	)

	return &stats, err
}

// UsageStats holds aggregated usage statistics.
type UsageStats struct {
	TotalInputTokens  int64 `json:"total_input_tokens"`
	TotalOutputTokens int64 `json:"total_output_tokens"`
	TotalCostUSDMilli int64 `json:"total_cost_usd_milli"`
	TotalCalls        int64 `json:"total_calls"`
}

// ===========================================================================
// Background flush
// ===========================================================================

func (m *DefaultMetricsCollector) flushLoop() {
	ticker := time.NewTicker(m.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = m.Flush()
		case <-m.done:
			return
		}
	}
}

// ===========================================================================
// Noop Metrics (for testing or when metrics disabled)
// ===========================================================================

// NoopMetricsCollector is a no-op metrics collector.
type NoopMetricsCollector struct{}

// NewNoopMetricsCollector creates a no-op metrics collector.
func NewNoopMetricsCollector() *NoopMetricsCollector {
	return &NoopMetricsCollector{}
}

// RecordUsage does nothing.
func (n *NoopMetricsCollector) RecordUsage(ctx context.Context, provider ProviderType, model string, usage TokenUsage, latency time.Duration, err error) {
}
