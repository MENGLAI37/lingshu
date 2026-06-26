package config

import (
	"os"
	"sync"
	"testing"
)

func resetConfig() {
	mu.Lock()
	defer mu.Unlock()
	instance = nil
	once = sync.Once{}
}

func TestLoadConfig(t *testing.T) {
	resetConfig()
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load config failed: %v", err)
	}
	if cfg.SchemaVersion != "v2.3.0" {
		t.Errorf("Expected schema_version v2.3.0, got %s", cfg.SchemaVersion)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected server port 8080, got %d", cfg.Server.Port)
	}
}

func TestEnvOverride(t *testing.T) {
	resetConfig()
	os.Setenv("OPSAI_SERVER_PORT", "9090")
	defer os.Unsetenv("OPSAI_SERVER_PORT")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load config failed: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected server port 9090 from env, got %d", cfg.Server.Port)
	}
}

func TestValidateSchemaVersion(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"v2.3.0", true},
		{"v1.8.0", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		err := validateSchemaVersion(tt.version)
		if tt.valid && err != nil {
			t.Errorf("Expected version %s to be valid, got error: %v", tt.version, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("Expected version %s to be invalid, got no error", tt.version)
		}
	}
}
