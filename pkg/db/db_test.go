package db

import (
	"context"
	"testing"
	"time"

	"github.com/lingshu/ops-ai/pkg/config"
)

func TestSQLiteFallback(t *testing.T) {
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
		t.Fatalf("Failed to init database with fallback: %v", err)
	}
	defer db.Close()

	if !db.IsFallback() {
		t.Error("Expected to be using fallback mode")
	}

	if db.DB() == nil {
		t.Error("Expected DB() to return non-nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestClose(t *testing.T) {
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
