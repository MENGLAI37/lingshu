package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Test fixtures
// ===========================================================================

// TempDir creates a temporary directory for testing.
func TempDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "lingshu-test-*")
	require.NoError(t, err)
	
	cleanup := func() {
		os.RemoveAll(dir)
	}
	
	return dir, cleanup
}

// TempFile creates a temporary file for testing.
func TempFile(t *testing.T, pattern string) (string, func()) {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	require.NoError(t, err)
	f.Close()
	
	cleanup := func() {
		os.Remove(f.Name())
	}
	
	return f.Name(), cleanup
}

// WriteTestFile writes content to a temporary test file.
func WriteTestFile(t *testing.T, pattern, content string) (string, func()) {
	t.Helper()
	path, cleanup := TempFile(t, pattern)
	
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	
	return path, cleanup
}

// ===========================================================================
// Test config helpers
// ===========================================================================

// TestConfig is a test configuration structure.
type TestConfig struct {
	DatabaseURL  string
	RedisURL     string
	KubeConfig   string
	LLMAPIKey    string
	LLMBaseURL   string
}

// LoadTestConfig loads test configuration from environment.
func LoadTestConfig() *TestConfig {
	return &TestConfig{
		DatabaseURL:  getEnv("TEST_DATABASE_URL", "postgres://lingshu:lingshu@localhost:5432/lingshu?sslmode=disable"),
		RedisURL:     getEnv("TEST_REDIS_URL", "localhost:6379"),
		KubeConfig:   getEnv("KUBECONFIG", ""),
		LLMAPIKey:    os.Getenv("TEST_LLM_API_KEY"),
		LLMBaseURL:   getEnv("TEST_LLM_BASE_URL", "https://api.openai.com/v1"),
	}
}

// SkipIfMissingEnv skips the test if required environment variables are missing.
func SkipIfMissingEnv(t *testing.T, envVars ...string) {
	t.Helper()
	for _, env := range envVars {
		if os.Getenv(env) == "" {
			t.Skipf("skipping test: missing environment variable %s", env)
		}
	}
}

// ===========================================================================
// File helpers
// ===========================================================================

// CopyDir copies a directory for testing.
func CopyDir(t *testing.T, src, dst string) {
	t.Helper()
	
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		
		if entry.IsDir() {
			err := os.MkdirAll(dstPath, 0755)
			require.NoError(t, err)
			CopyDir(t, srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			require.NoError(t, err)
			err = os.WriteFile(dstPath, data, 0644)
			require.NoError(t, err)
		}
	}
}

// FileExists checks if a file exists.
func FileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// ===========================================================================
// Environment helpers
// ===========================================================================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SetEnv sets an environment variable for testing.
func SetEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// UnsetEnv unsets an environment variable for testing.
func UnsetEnv(t *testing.T, key string) {
	t.Helper()
	os.Unsetenv(key)
}

// ===========================================================================
// Cleanup helpers
// ===========================================================================

// DeferCleanup registers a cleanup function to be called.
func DeferCleanup(t *testing.T, cleanup func()) {
	t.Cleanup(cleanup)
}
