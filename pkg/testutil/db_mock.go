package testutil

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// SQL Mock helpers
// ===========================================================================

// NewSQLMock creates a new sqlmock for testing database operations.
func NewSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "failed to create sqlmock")
	
	cleanup := func() {
		_ = db.Close()
	}
	
	return db, mock, cleanup
}

// ExpectQuery creates an expectation for a query with rows.
func ExpectQuery(mock sqlmock.Sqlmock, query string) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(query)
}

// ExpectExec creates an expectation for an exec statement.
func ExpectExec(mock sqlmock.Sqlmock, exec string) *sqlmock.ExpectedExec {
	return mock.ExpectExec(exec)
}

// ExpectBegin creates an expectation for a transaction begin.
func ExpectBegin(mock sqlmock.Sqlmock) *sqlmock.ExpectedBegin {
	return mock.ExpectBegin()
}

// ExpectCommit creates an expectation for a transaction commit.
func ExpectCommit(mock sqlmock.Sqlmock) *sqlmock.ExpectedCommit {
	return mock.ExpectCommit()
}

// ExpectRollback creates an expectation for a transaction rollback.
func ExpectRollback(mock sqlmock.Sqlmock) *sqlmock.ExpectedRollback {
	return mock.ExpectRollback()
}

// NewRows creates sqlmock rows from column names.
func NewRows(columns []string) *sqlmock.Rows {
	return sqlmock.NewRows(columns)
}

// ===========================================================================
// Redis Mock helpers (using redismock or custom interface)
// ===========================================================================

// FakeRedisClient is a fake implementation for Redis testing.
type FakeRedisClient struct {
	data map[string]string
}

// NewFakeRedisClient creates a new fake Redis client.
func NewFakeRedisClient() *FakeRedisClient {
	return &FakeRedisClient{
		data: make(map[string]string),
	}
}

// Get returns the value for the key.
func (c *FakeRedisClient) Get(key string) (string, bool) {
	v, ok := c.data[key]
	return v, ok
}

// Set sets the value for the key.
func (c *FakeRedisClient) Set(key, value string) {
	c.data[key] = value
}

// Delete removes the key.
func (c *FakeRedisClient) Delete(key string) {
	delete(c.data, key)
}

// Exists checks if the key exists.
func (c *FakeRedisClient) Exists(key string) bool {
	_, ok := c.data[key]
	return ok
}

// Clear clears all data.
func (c *FakeRedisClient) Clear() {
	c.data = make(map[string]string)
}

// Len returns the number of keys.
func (c *FakeRedisClient) Len() int {
	return len(c.data)
}
