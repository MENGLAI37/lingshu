package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lingshu/lingshu/pkg/logger"
)

const (
	DefaultCleanupInterval = 1 * time.Hour
	DefaultBatchSize         = 100
)

type ExpiryCleaner struct {
	db              *Manager
	interval        time.Duration
	batchSize       int
	stopCh          chan struct{}
	wg              sync.WaitGroup
	running         bool
	mu              sync.Mutex
}

func NewExpiryCleaner(manager *Manager) *ExpiryCleaner {
	return &ExpiryCleaner{
		db:        manager,
		interval:  DefaultCleanupInterval,
		batchSize: DefaultBatchSize,
		stopCh:    make(chan struct{}),
	}
}

func (c *ExpiryCleaner) SetInterval(interval time.Duration) {
	if interval > 0 {
		c.interval = interval
	}
}

func (c *ExpiryCleaner) SetBatchSize(size int) {
	if size > 0 {
		c.batchSize = size
	}
}

func (c *ExpiryCleaner) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("expiry cleaner already running")
	}

	c.stopCh = make(chan struct{})
	c.running = true
	c.wg.Add(1)

	go c.run()

	logger.Info("Session expiry cleaner started",
		"interval", c.interval.String(),
		"batch_size", c.batchSize,
	)

	return nil
}

func (c *ExpiryCleaner) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	close(c.stopCh)
	c.wg.Wait()
	c.running = false

	logger.Info("Session expiry cleaner stopped")

	return nil
}

func (c *ExpiryCleaner) run() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanupExpired()
		}
	}
}

func (c *ExpiryCleaner) cleanupExpired() {
	ctx := context.Background()

	deletedCount, err := c.DeleteExpired(ctx)
	if err != nil {
		logger.Error("Failed to cleanup expired sessions", "error", err)
		return
	}

	if deletedCount > 0 {
		logger.Info("Cleaned up expired sessions", "count", deletedCount)
	}
}

func (c *ExpiryCleaner) DeleteExpired(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM sessions
		WHERE expires_at < NOW()
		AND ctid IN (
			SELECT ctid FROM sessions
			WHERE expires_at < NOW()
			LIMIT $1
		)
	`

	result, err := c.db.db.ExecContext(ctx, query, c.batchSize)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return rowsAffected, nil
}

func (c *ExpiryCleaner) CountExpired(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE expires_at < NOW()`

	var count int64
	err := c.db.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count expired sessions: %w", err)
	}

	return count, nil
}

func (c *ExpiryCleaner) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
