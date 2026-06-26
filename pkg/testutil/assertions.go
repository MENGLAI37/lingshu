package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Context helpers
// ===========================================================================

// WithTimeout creates a context with the specified timeout for testing.
func WithTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// WithTestName adds test name to context for logging.
func WithTestName(ctx context.Context, t *testing.T) context.Context {
	return context.WithValue(ctx, "test_name", t.Name())
}

// ===========================================================================
// Assertion helpers
// ===========================================================================

// AssertNil asserts that the value is nil.
func AssertNil(t *testing.T, val interface{}, msgAndArgs ...interface{}) {
	assert.Nil(t, val, msgAndArgs...)
}

// AssertNotNil asserts that the value is not nil.
func AssertNotNil(t *testing.T, val interface{}, msgAndArgs ...interface{}) {
	assert.NotNil(t, val, msgAndArgs...)
}

// AssertEqual asserts that the two values are equal.
func AssertEqual[T any](t *testing.T, expected, actual T, msgAndArgs ...interface{}) {
	assert.Equal(t, expected, actual, msgAndArgs...)
}

// AssertNotEqual asserts that the two values are not equal.
func AssertNotEqual[T any](t *testing.T, expected, actual T, msgAndArgs ...interface{}) {
	assert.NotEqual(t, expected, actual, msgAndArgs...)
}

// AssertTrue asserts that the value is true.
func AssertTrue(t *testing.T, val bool, msgAndArgs ...interface{}) {
	assert.True(t, val, msgAndArgs...)
}

// AssertFalse asserts that the value is false.
func AssertFalse(t *testing.T, val bool, msgAndArgs ...interface{}) {
	assert.False(t, val, msgAndArgs...)
}

// AssertError asserts that the function returns an error.
func AssertError(t *testing.T, err error, msgAndArgs ...interface{}) {
	assert.Error(t, err, msgAndArgs...)
}

// AssertNoError asserts that the function does not return an error.
func AssertNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	assert.NoError(t, err, msgAndArgs...)
}

// RequireNil asserts that the value is nil, fails immediately on error.
func RequireNil(t *testing.T, val interface{}, msgAndArgs ...interface{}) {
	require.Nil(t, val, msgAndArgs...)
}

// RequireNotNil asserts that the value is not nil, fails immediately on error.
func RequireNotNil(t *testing.T, val interface{}, msgAndArgs ...interface{}) {
	require.NotNil(t, val, msgAndArgs...)
}

// RequireNoError asserts no error, fails immediately on error.
func RequireNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	require.NoError(t, err, msgAndArgs...)
}

// RequireError asserts error occurs, fails immediately on error.
func RequireError(t *testing.T, err error, msgAndArgs ...interface{}) {
	require.Error(t, err, msgAndArgs...)
}

// ===========================================================================
// Slice helpers
// ===========================================================================

// AssertSliceContains asserts that the slice contains the element.
func AssertSliceContains[T comparable](t *testing.T, slice []T, elem T, msgAndArgs ...interface{}) {
	found := false
	for _, v := range slice {
		if v == elem {
			found = true
			break
		}
	}
	assert.True(t, found, msgAndArgs...)
}

// AssertSliceNotContains asserts that the slice does not contain the element.
func AssertSliceNotContains[T comparable](t *testing.T, slice []T, elem T, msgAndArgs ...interface{}) {
	for _, v := range slice {
		if v == elem {
			assert.Fail(t, "slice should not contain element", msgAndArgs...)
			return
		}
	}
}

// AssertSliceEmpty asserts that the slice is empty.
func AssertSliceEmpty[T any](t *testing.T, slice []T, msgAndArgs ...interface{}) {
	assert.Empty(t, slice, msgAndArgs...)
}

// AssertSliceNotEmpty asserts that the slice is not empty.
func AssertSliceNotEmpty[T any](t *testing.T, slice []T, msgAndArgs ...interface{}) {
	assert.NotEmpty(t, slice, msgAndArgs...)
}

// ===========================================================================
// Map helpers
// ===========================================================================

// AssertMapContains asserts that the map contains the key.
func AssertMapContains[K comparable, V any](t *testing.T, m map[K]V, key K, msgAndArgs ...interface{}) {
	_, found := m[key]
	assert.True(t, found, msgAndArgs...)
}

// AssertMapNotContains asserts that the map does not contain the key.
func AssertMapNotContains[K comparable, V any](t *testing.T, m map[K]V, key K, msgAndArgs ...interface{}) {
	_, found := m[key]
	assert.False(t, found, msgAndArgs...)
}

// ===========================================================================
// String helpers
// ===========================================================================

// AssertStringContains asserts that the string contains the substring.
func AssertStringContains(t *testing.T, str, substr string, msgAndArgs ...interface{}) {
	assert.Contains(t, str, substr, msgAndArgs...)
}

// AssertStringNotContains asserts that the string does not contain the substring.
func AssertStringNotContains(t *testing.T, str, substr string, msgAndArgs ...interface{}) {
	assert.NotContains(t, str, substr, msgAndArgs...)
}

// AssertStringEqual asserts that the string equals the expected value.
func AssertStringEqual(t *testing.T, expected, actual string, msgAndArgs ...interface{}) {
	assert.Equal(t, expected, actual, msgAndArgs...)
}
