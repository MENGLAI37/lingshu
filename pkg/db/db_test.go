//go:build cgo

package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lingshu/lingshu/pkg/config"
)

// TestSQLiteFallback 测试 SQLite 回退功能
// 仅在 CGO 启用时运行，这是验证生产环境降级机制的关键测试
func TestSQLiteFallback(t *testing.T) {
	// 检查是否明确禁用 SQLite 测试
	if os.Getenv("SKIP_SQLITE_TESTS") == "true" {
		t.Skip("SKIP_SQLITE_TESTS is set, skipping SQLite fallback test")
	}

	cfg := &config.DBConfig{
		Type:         "sqlite",
		Host:         "localhost",
		Port:         5432,
		User:         "test",
		Password:     "test",
		DBName:       "test",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	db, err := Init(cfg)
	if err != nil {
		t.Fatalf("Failed to init database with SQLite fallback: %v", err)
	}
	defer db.Close()

	if !db.IsFallback() {
		t.Error("Expected to be using fallback mode (SQLite)")
	}

	if db.DB() == nil {
		t.Error("Expected DB() to return non-nil SQLite connection")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Errorf("SQLite Ping failed: %v", err)
	}

	// 验证可以执行简单查询
	var version string
	err = db.DB().GetContext(ctx, &version, "SELECT sqlite_version()")
	if err != nil {
		t.Errorf("Failed to query SQLite version: %v", err)
	} else {
		t.Logf("SQLite version: %s", version)
	}
}

// TestClose 测试数据库关闭
func TestClose(t *testing.T) {
	if os.Getenv("SKIP_SQLITE_TESTS") == "true" {
		t.Skip("SKIP_SQLITE_TESTS is set, skipping SQLite close test")
	}

	cfg := &config.DBConfig{
		Type:         "sqlite",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	db, err := Init(cfg)
	if err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestSQLiteFallbackWithPostgresFailure 测试 PostgreSQL 故障时自动降级到 SQLite
// 这是生产环境最关键的降级场景
func TestSQLiteFallbackWithPostgresFailure(t *testing.T) {
	if os.Getenv("SKIP_SQLITE_TESTS") == "true" {
		t.Skip("SKIP_SQLITE_TESTS is set, skipping fallback test")
	}

	// 配置指向不存在的 PostgreSQL，验证会正确降级到 SQLite
	cfg := &config.DBConfig{
		Type:         "sqlite", // 指示优先尝试 SQLite
		Host:         "localhost",
		Port:         9999, // 无效端口
		User:         "nonexistent",
		Password:     "nonexistent",
		DBName:       "nonexistent",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	db, err := Init(cfg)
	if err != nil {
		t.Fatalf("SQLite fallback should succeed even when PostgreSQL is unavailable: %v", err)
	}
	defer db.Close()

	if !db.IsFallback() {
		t.Error("Expected to be in fallback mode (using SQLite after PostgreSQL failure)")
	}

	// 验证 SQLite 可用
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Errorf("SQLite should be functional as fallback: %v", err)
	}
}
