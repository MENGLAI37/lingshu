//go:build cgo

package session

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/db"
	"github.com/lingshu/lingshu/pkg/logger"
)

func setupTestDB(t *testing.T) *db.Database {
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

	createSessionsTable(t, database)
	clearSessionsTable(t, database)

	return database
}

func clearSessionsTable(t *testing.T, d *db.Database) {
	t.Helper()
	_, err := d.DB().Exec(`DELETE FROM sessions`)
	require.NoError(t, err)
}

func createSessionsTable(t *testing.T, d *db.Database) {
	t.Helper()

	query := `
		CREATE TABLE IF NOT EXISTS sessions (
			session_id TEXT PRIMARY KEY,
			parent_session_id TEXT,
			cluster TEXT NOT NULL DEFAULT 'default',
			namespace TEXT NOT NULL DEFAULT 'default',
			environment TEXT NOT NULL DEFAULT 'production',
			status TEXT NOT NULL DEFAULT 'active',
			user_id TEXT,
			team_id TEXT,
			incident_id TEXT,
			conversation_history TEXT NOT NULL DEFAULT '[]',
			tool_call_history TEXT NOT NULL DEFAULT '[]',
			metadata TEXT NOT NULL DEFAULT '{}',
			cost_usd_milli INTEGER NOT NULL DEFAULT 0,
			token_budget_used INTEGER NOT NULL DEFAULT 0,
			token_budget_limit INTEGER NOT NULL DEFAULT 100000,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		)
	`

	_, err := d.DB().Exec(query)
	require.NoError(t, err)
}

func TestCreateSession(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	req := &CreateSessionRequest{
		Cluster:     "test-cluster",
		Namespace:   "test-ns",
		Environment: "staging",
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	sess, err := manager.Create(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, sess)

	assert.NotEmpty(t, sess.SessionID)
	assert.Equal(t, "test-cluster", sess.Cluster)
	assert.Equal(t, "test-ns", sess.Namespace)
	assert.Equal(t, "staging", sess.Environment)
	assert.Equal(t, StatusActive, sess.Status)
	assert.Equal(t, int64(0), sess.CostUSDMilli)
	assert.Equal(t, int64(0), sess.TokenBudgetUsed)
	assert.Equal(t, int64(DefaultTokenBudgetLimit), sess.TokenBudgetLimit)
	assert.False(t, sess.CreatedAt.IsZero())
	assert.False(t, sess.ExpiresAt.IsZero())
	assert.True(t, sess.ExpiresAt.After(time.Now()))
}

func TestCreateSessionWithParent(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	parent, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	child, err := manager.Create(ctx, &CreateSessionRequest{
		ParentSessionID: &parent.SessionID,
		Cluster:         "test-cluster",
		Namespace:       "test-ns",
	})
	require.NoError(t, err)

	assert.Equal(t, parent.SessionID, *child.ParentSessionID)
}

func TestGetSession(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	req := &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	}

	created, err := manager.Create(ctx, req)
	require.NoError(t, err)

	sess, err := manager.Get(ctx, created.SessionID)
	require.NoError(t, err)
	require.NotNil(t, sess)

	assert.Equal(t, created.SessionID, sess.SessionID)
	assert.Equal(t, created.Cluster, sess.Cluster)
}

func TestGetSessionNotFound(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	sess, err := manager.Get(ctx, "nonexistent-id")
	assert.Nil(t, sess)
	assert.Error(t, err)
}

func TestUpdateSession(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	newStatus := StatusCompleted
	newCost := int64(100)
	newTokens := int64(5000)

	updated, err := manager.Update(ctx, created.SessionID, &UpdateSessionRequest{
		Status:          &newStatus,
		CostUSDMilli:    &newCost,
		TokenBudgetUsed: &newTokens,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, StatusCompleted, updated.Status)
	assert.Equal(t, int64(100), updated.CostUSDMilli)
	assert.Equal(t, int64(5000), updated.TokenBudgetUsed)
}

func TestDeleteSession(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	err = manager.Delete(ctx, created.SessionID)
	require.NoError(t, err)

	sess, err := manager.Get(ctx, created.SessionID)
	assert.Nil(t, sess)
	assert.Error(t, err)
}

func TestListSessions(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := manager.Create(ctx, &CreateSessionRequest{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
		})
		require.NoError(t, err)
	}

	result, err := manager.List(ctx, &SessionListFilter{
		Limit: 10,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int64(5), result.Total)
	assert.Len(t, result.Sessions, 5)
	assert.False(t, result.HasMore)
}

func TestListSessionsWithPagination(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, err := manager.Create(ctx, &CreateSessionRequest{
			Cluster:   "test-cluster",
			Namespace: "test-ns",
		})
		require.NoError(t, err)
	}

	result1, err := manager.List(ctx, &SessionListFilter{
		Limit:  3,
		Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, result1.Sessions, 3)
	assert.True(t, result1.HasMore)

	result2, err := manager.List(ctx, &SessionListFilter{
		Limit:  3,
		Offset: 9,
	})
	require.NoError(t, err)
	assert.Len(t, result2.Sessions, 1)
	assert.False(t, result2.HasMore)
}

func TestAppendConversation(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	message := map[string]interface{}{
		"role":    "user",
		"content": "hello",
	}

	err = manager.AppendConversation(ctx, created.SessionID, message)
	require.NoError(t, err)

	sess, err := manager.Get(ctx, created.SessionID)
	require.NoError(t, err)
	assert.Len(t, sess.ConversationHistory, 1)
}

func TestAddCost(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	err = manager.AddCost(ctx, created.SessionID, 100, 5000)
	require.NoError(t, err)

	err = manager.AddCost(ctx, created.SessionID, 50, 3000)
	require.NoError(t, err)

	sess, err := manager.Get(ctx, created.SessionID)
	require.NoError(t, err)
	assert.Equal(t, int64(150), sess.CostUSDMilli)
	assert.Equal(t, int64(8000), sess.TokenBudgetUsed)
}

func TestCheckTokenBudget(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	tokenLimit := int64(10000)
	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:          "test-cluster",
		Namespace:        "test-ns",
		TokenBudgetLimit: &tokenLimit,
	})
	require.NoError(t, err)

	ok, err := manager.CheckTokenBudget(ctx, created.SessionID, 5000)
	require.NoError(t, err)
	assert.True(t, ok)

	err = manager.AddCost(ctx, created.SessionID, 0, 8000)
	require.NoError(t, err)

	ok, err = manager.CheckTokenBudget(ctx, created.SessionID, 3000)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCompleteSession(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	sess, err := manager.Complete(ctx, created.SessionID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, sess.Status)
}

func TestParentChain(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	root, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	mid, err := manager.Create(ctx, &CreateSessionRequest{
		ParentSessionID: &root.SessionID,
		Cluster:         "test-cluster",
		Namespace:       "test-ns",
	})
	require.NoError(t, err)

	leaf, err := manager.Create(ctx, &CreateSessionRequest{
		ParentSessionID: &mid.SessionID,
		Cluster:         "test-cluster",
		Namespace:       "test-ns",
	})
	require.NoError(t, err)

	chain, err := manager.GetParentChain(ctx, leaf.SessionID)
	require.NoError(t, err)
	assert.Len(t, chain, 3)
	assert.Equal(t, root.SessionID, chain[0].SessionID)
	assert.Equal(t, mid.SessionID, chain[1].SessionID)
	assert.Equal(t, leaf.SessionID, chain[2].SessionID)
}

func TestGetChildren(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	parent, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := manager.Create(ctx, &CreateSessionRequest{
			ParentSessionID: &parent.SessionID,
			Cluster:         "test-cluster",
			Namespace:       "test-ns",
		})
		require.NoError(t, err)
	}

	children, err := manager.GetChildren(ctx, parent.SessionID, false)
	require.NoError(t, err)
	assert.Len(t, children, 3)
}

func TestSessionTTL(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	shortTTL := 1 * time.Second
	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
		TTL:       &shortTTL,
	})
	require.NoError(t, err)

	expired, err := manager.IsExpired(ctx, created.SessionID)
	require.NoError(t, err)
	assert.False(t, expired)

	time.Sleep(2 * time.Second)

	expired, err = manager.IsExpired(ctx, created.SessionID)
	require.NoError(t, err)
	assert.True(t, expired)
}

func TestRefreshTTL(t *testing.T) {
	logger.Init("debug", "text")
	database := setupTestDB(t)

	manager := &Manager{db: database}
	ctx := context.Background()

	shortTTL := 1 * time.Second
	created, err := manager.Create(ctx, &CreateSessionRequest{
		Cluster:   "test-cluster",
		Namespace: "test-ns",
		TTL:       &shortTTL,
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	newTTL := 10 * time.Second
	refreshed, err := manager.RefreshTTL(ctx, created.SessionID, &newTTL)
	require.NoError(t, err)

	assert.True(t, refreshed.ExpiresAt.After(created.ExpiresAt))
}
