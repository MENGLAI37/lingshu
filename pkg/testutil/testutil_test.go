package testutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssertNil(t *testing.T) {
	AssertNil(t, nil)
	AssertNil(t, (*string)(nil))
}

func TestAssertNotNil(t *testing.T) {
	s := "test"
	AssertNotNil(t, &s)
	AssertNotNil(t, []int{1, 2, 3})
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, 1, 1)
	AssertEqual(t, "a", "a")
	AssertEqual(t, []int{1, 2}, []int{1, 2})
}

func TestAssertNotEqual(t *testing.T) {
	AssertNotEqual(t, 1, 2)
	AssertNotEqual(t, "a", "b")
}

func TestAssertTrue(t *testing.T) {
	AssertTrue(t, true)
}

func TestAssertFalse(t *testing.T) {
	AssertFalse(t, false)
}

func TestAssertError(t *testing.T) {
	AssertError(t, assert.AnError)
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
}

func TestAssertSliceContains(t *testing.T) {
	slice := []int{1, 2, 3}
	AssertSliceContains(t, slice, 2)
}

func TestAssertSliceNotContains(t *testing.T) {
	slice := []int{1, 2, 3}
	AssertSliceNotContains(t, slice, 4)
}

func TestAssertSliceEmpty(t *testing.T) {
	AssertSliceEmpty(t, []int{})
}

func TestAssertSliceNotEmpty(t *testing.T) {
	AssertSliceNotEmpty(t, []int{1})
}

func TestAssertMapContains(t *testing.T) {
	m := map[string]int{"a": 1}
	AssertMapContains(t, m, "a")
}

func TestAssertMapNotContains(t *testing.T) {
	m := map[string]int{"a": 1}
	AssertMapNotContains(t, m, "b")
}

func TestAssertStringContains(t *testing.T) {
	AssertStringContains(t, "hello world", "world")
}

func TestAssertStringNotContains(t *testing.T) {
	AssertStringNotContains(t, "hello world", "foo")
}

func TestAssertStringEqual(t *testing.T) {
	AssertStringEqual(t, "hello", "hello")
}

func TestTempDir(t *testing.T) {
	dir, cleanup := TempDir(t)
	defer cleanup()
	
	require.NotEmpty(t, dir)
	
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestTempFile(t *testing.T) {
	path, cleanup := TempFile(t, "test-*.txt")
	defer cleanup()
	
	require.NotEmpty(t, path)
	
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
}

func TestWriteTestFile(t *testing.T) {
	path, cleanup := WriteTestFile(t, "test-*.txt", "hello world")
	defer cleanup()
	
	require.NotEmpty(t, path)
	
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

func TestLoadTestConfig(t *testing.T) {
	cfg := LoadTestConfig()
	require.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.DatabaseURL)
	assert.NotEmpty(t, cfg.RedisURL)
}

func TestSetEnv(t *testing.T) {
	SetEnv(t, "TEST_VAR", "test-value")
	assert.Equal(t, "test-value", os.Getenv("TEST_VAR"))
}

func TestFileExists(t *testing.T) {
	path, cleanup := TempFile(t, "exists-*.txt")
	defer cleanup()
	
	assert.True(t, FileExists(t, path))
	assert.False(t, FileExists(t, "/nonexistent/file"))
}

func TestFakeRedisClient(t *testing.T) {
	client := NewFakeRedisClient()
	
	client.Set("key1", "value1")
	v, ok := client.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "value1", v)
	
	assert.True(t, client.Exists("key1"))
	assert.False(t, client.Exists("key2"))
	
	client.Delete("key1")
	assert.False(t, client.Exists("key1"))
	
	client.Set("key2", "value2")
	client.Clear()
	assert.Equal(t, 0, client.Len())
}
