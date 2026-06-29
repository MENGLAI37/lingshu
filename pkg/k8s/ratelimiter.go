package k8s

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	DefaultQPS   = 20
	DefaultBurst = 50

	DefaultMaxRetries      = 3
	DefaultInitialBackoff  = 100 * time.Millisecond
	DefaultMaxBackoff      = 2 * time.Second
	DefaultBackoffMultiplier = 2.0
)

type RateLimiterConfig struct {
	QPS               float64
	Burst             int
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		QPS:               DefaultQPS,
		Burst:             DefaultBurst,
		MaxRetries:        DefaultMaxRetries,
		InitialBackoff:    DefaultInitialBackoff,
		MaxBackoff:        DefaultMaxBackoff,
		BackoffMultiplier: DefaultBackoffMultiplier,
	}
}

type RateLimiter struct {
	limiter *rate.Limiter
	config  *RateLimiterConfig
}

func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}
	if config.QPS <= 0 {
		config.QPS = DefaultQPS
	}
	if config.Burst <= 0 {
		config.Burst = DefaultBurst
	}

	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(config.QPS), config.Burst),
		config:  config,
	}
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}

func (rl *RateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

func (rl *RateLimiter) Reserve() *rate.Reservation {
	return rl.limiter.Reserve()
}

func (rl *RateLimiter) Tokens() float64 {
	return rl.limiter.Tokens()
}

func (rl *RateLimiter) Limit() rate.Limit {
	return rl.limiter.Limit()
}

func (rl *RateLimiter) BurstSize() int {
	return rl.limiter.Burst()
}

func (rl *RateLimiter) SetLimit(newLimit rate.Limit) {
	rl.limiter.SetLimit(newLimit)
}

func (rl *RateLimiter) SetBurst(newBurst int) {
	rl.limiter.SetBurst(newBurst)
}

type RetryFunc func(ctx context.Context) error

type IsRetryableFunc func(err error) bool

func (rl *RateLimiter) RetryWithBackoff(ctx context.Context, fn RetryFunc, isRetryable IsRetryableFunc) error {
	var lastErr error
	backoff := rl.config.InitialBackoff

	for attempt := 0; attempt <= rl.config.MaxRetries; attempt++ {
		if err := rl.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter wait failed: %w", err)
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		if !isRetryable(err) {
			return err
		}

		if attempt == rl.config.MaxRetries {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff = time.Duration(float64(backoff) * rl.config.BackoffMultiplier)
		if backoff > rl.config.MaxBackoff {
			backoff = rl.config.MaxBackoff
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", rl.config.MaxRetries, lastErr)
}

func ConfigureRestConfigRateLimiter(config *rest.Config, qps float32, burst int) {
	if qps <= 0 {
		qps = float32(DefaultQPS)
	}
	if burst <= 0 {
		burst = DefaultBurst
	}

	config.QPS = qps
	config.Burst = burst
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
}

func DefaultRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"i/o timeout",
		"too many requests",
		"429",
		"500",
		"502",
		"503",
		"504",
		"server unavailable",
		"temporary failure",
		"EOF",
		"broken pipe",
	}

	for _, pattern := range retryablePatterns {
		if containsIgnoreCase(errStr, pattern) {
			return true
		}
	}

	return false
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if len(s[i:]) >= len(substr) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
