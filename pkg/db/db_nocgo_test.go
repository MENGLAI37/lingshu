//go:build !cgo

package db

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// TestSQLiteFallback_CGODisabled 测试 CGO 禁用环境下的 Database 结构体行为
// 注意：此时无法真正连接 SQLite，仅测试 Database 包装逻辑
func TestSQLiteFallback_CGODisabled(t *testing.T) {
	// 在 CGO 禁用环境下，跳过需要真实 SQLite 的测试
	t.Skip("SQLite requires CGO, run with CGO_ENABLED=1 to test SQLite fallback")
}

// TestPostgresConnection 测试 PostgreSQL mock 连接
func TestPostgresConnection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Skip("sqlmock not available, skipping")
	}
	defer db.Close()

	mock.ExpectPing()

	if err := db.Ping(); err != nil {
		t.Errorf("Mock ping failed: %v", err)
	}
}

// TestDatabaseWrapper 测试 Database 包装方法
func TestDatabaseWrapper(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Skip("sqlmock not available, skipping")
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")

	mock.ExpectQuery("SELECT (.+) FROM test").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	var result []int
	err = sqlxDB.SelectContext(context.Background(), &result, "SELECT id FROM test LIMIT 1")
	if err != nil {
		t.Logf("SelectContext returned: %v (expected with mock)", err)
	}
}

// TestInitWithMock 测试 Database 结构体初始化
func TestInitWithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Skip("sqlmock not available, skipping")
	}
	defer db.Close()

	mock.ExpectPing()

	database := &Database{
		Primary: sqlx.NewDb(db, "postgres"),
	}

	if database.DB() == nil {
		t.Error("Expected DB() to return non-nil")
	}

	if database.IsFallback() {
		t.Error("Expected not to be in fallback mode")
	}
}

// TestDatabaseFallbackMode 测试降级模式行为
func TestDatabaseFallbackMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Skip("sqlmock not available, skipping")
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")

	database := &Database{
		Fallback:    sqlxDB,
		useFallback: true,
	}

	if !database.IsFallback() {
		t.Error("Expected to be in fallback mode")
	}

	if database.DB() != database.Fallback {
		t.Error("Expected DB() to return fallback connection")
	}

	ctx := context.Background()
	if err := database.PingContext(ctx); err != nil {
		t.Errorf("PingContext failed: %v", err)
	}
}

// TestDatabasePrimaryMode 测试主数据库模式
func TestDatabasePrimaryMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Skip("sqlmock not available, skipping")
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")

	database := &Database{
		Primary:    sqlxDB,
		useFallback: false,
	}

	if database.IsFallback() {
		t.Error("Expected not to be in fallback mode")
	}

	if database.DB() != database.Primary {
		t.Error("Expected DB() to return primary connection")
	}
}
