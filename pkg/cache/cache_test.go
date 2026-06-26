package cache

import (
	"context"
	"testing"
	"time"

	"github.com/lingshu/ops-ai/pkg/config"
)

func TestInitSingleMode(t *testing.T) {
	cfg := &config.RedisConfig{
		Mode:         "single",
		Addresses:    []string{"localhost:6379"},
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 3,
	}

	cache, err := Init(cfg)
	if err == nil {
		defer cache.Close()
		ctx := context.Background()

		err = cache.Set(ctx, "test_key", "test_value", 10*time.Second)
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}

		val, err := cache.Get(ctx, "test_key")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if val != "test_value" {
			t.Errorf("Expected test_value, got %s", val)
		}

		err = cache.Del(ctx, "test_key")
		if err != nil {
			t.Errorf("Del failed: %v", err)
		}
	} else {
		t.Logf("Redis not available, skipping live test: %v", err)
	}
}

func TestLockUnlock(t *testing.T) {
	cfg := &config.RedisConfig{
		Mode:      "single",
		Addresses: []string{"localhost:6379"},
	}

	cache, err := Init(cfg)
	if err == nil {
		defer cache.Close()
		ctx := context.Background()
		key := "test_lock"

		ok, err := cache.Lock(ctx, key, 5*time.Second)
		if err != nil {
			t.Errorf("Lock failed: %v", err)
		}
		if !ok {
			t.Error("Expected lock to be acquired")
		}

		ok2, err := cache.Lock(ctx, key, 5*time.Second)
		if err != nil {
			t.Errorf("Lock 2 failed: %v", err)
		}
		if ok2 {
			t.Error("Expected second lock to fail")
		}

		err = cache.Unlock(ctx, key)
		if err != nil {
			t.Errorf("Unlock failed: %v", err)
		}
	} else {
		t.Logf("Redis not available, skipping live test: %v", err)
	}
}
