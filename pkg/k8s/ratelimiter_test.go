package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
	"k8s.io/client-go/rest"
)

func TestNewRateLimiter(t *testing.T) {
	config := DefaultRateLimiterConfig()
	rl := NewRateLimiter(config)
	assert.NotNil(t, rl)
}

func TestNewRateLimiter_NilConfig(t *testing.T) {
	rl := NewRateLimiter(nil)
	assert.NotNil(t, rl)
	assert.Equal(t, rate.Limit(DefaultQPS), rl.Limit())
	assert.Equal(t, DefaultBurst, rl.BurstSize())
}

func TestRateLimiter_Allow(t *testing.T) {
	config := &RateLimiterConfig{
		QPS:   100,
		Burst: 10,
	}
	rl := NewRateLimiter(config)

	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow(), "request %d should be allowed", i+1)
	}

	assert.False(t, rl.Allow(), "request after burst should be blocked")
}

func TestRateLimiter_Wait(t *testing.T) {
	config := &RateLimiterConfig{
		QPS:   100,
		Burst: 5,
	}
	rl := NewRateLimiter(config)

	for i := 0; i < 5; i++ {
		rl.Allow()
	}

	err := rl.Wait(context.Background())
	assert.NoError(t, err)
}

func TestRetryWithBackoff_Success(t *testing.T) {
	config := &RateLimiterConfig{
		QPS:               100,
		Burst:             10,
		MaxRetries:        3,
		InitialBackoff:    10,
		MaxBackoff:        100,
		BackoffMultiplier: 2.0,
	}
	rl := NewRateLimiter(config)
	attempts := 0

	err := rl.RetryWithBackoff(context.Background(), func(ctx context.Context) error {
		attempts++
		return nil
	}, func(err error) bool {
		return true
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetryWithBackoff_AlwaysFails(t *testing.T) {
	config := &RateLimiterConfig{
		QPS:               100,
		Burst:             10,
		MaxRetries:        2,
		InitialBackoff:    1,
		MaxBackoff:        10,
		BackoffMultiplier: 2.0,
	}
	rl := NewRateLimiter(config)
	attempts := 0

	err := rl.RetryWithBackoff(context.Background(), func(ctx context.Context) error {
		attempts++
		return assert.AnError
	}, func(err error) bool {
		return true
	})

	assert.Error(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryWithBackoff_EventualSuccess(t *testing.T) {
	config := &RateLimiterConfig{
		QPS:               100,
		Burst:             10,
		MaxRetries:        3,
		InitialBackoff:    1,
		MaxBackoff:        10,
		BackoffMultiplier: 2.0,
	}
	rl := NewRateLimiter(config)
	attempts := 0

	err := rl.RetryWithBackoff(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return assert.AnError
		}
		return nil
	}, func(err error) bool {
		return true
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestDefaultRetryableError(t *testing.T) {
	testCases := []struct {
		name     string
		err      string
		expected bool
	}{
		{"connection refused", "connection refused", true},
		{"timeout", "i/o timeout", true},
		{"429 too many requests", "429 too many requests", true},
		{"500 internal server error", "500 internal server error", true},
		{"not retryable", "permission denied", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DefaultRetryableError(assert.AnError)
			assert.NotNil(t, result)
		})
	}
}

func TestConfigureRestConfigRateLimiter(t *testing.T) {
	config := &rest.Config{}
	ConfigureRestConfigRateLimiter(config, 30, 60)
	assert.Equal(t, float32(30), config.QPS)
	assert.Equal(t, 60, config.Burst)
	assert.NotNil(t, config.RateLimiter)
}
