package agent

import (
	"fmt"
	"sync"
	"time"
)

// ===========================================================================
// Circuit Breaker - 会话级操作熔断器
// ===========================================================================
//
// 防止 Agent Loop 在同一会话中执行过多 L2+ 操作:
// 1. 累计 L2+ 操作超限 → 熔断 (open)
// 2. 连续写操作超限 → 半开 (half-open), 强制诊断
// 3. 失败率过高 → 熔断 (open)

// BreakerState represents the circuit breaker state.
type BreakerState string

const (
	BreakerClosed   BreakerState = "closed"    // 正常, 允许操作
	BreakerHalfOpen BreakerState = "half_open" // 半开, 仅允许 L0 诊断
	BreakerOpen     BreakerState = "open"       // 熔断, 拒绝所有 L1+ 操作
)

// CircuitBreaker tracks operation counts and failure rates.
type CircuitBreaker struct {
	maxL2Ops        int
	maxConsecWrites int

	l2Count          int
	consecutiveWrites int
	totalOps         int
	failedOps        int

	state       BreakerState
	lastFailure time.Time
	mu          sync.Mutex
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(maxL2Ops, maxConsecWrites int) *CircuitBreaker {
	if maxL2Ops <= 0 {
		maxL2Ops = 5
	}
	if maxConsecWrites <= 0 {
		maxConsecWrites = 3
	}
	return &CircuitBreaker{
		maxL2Ops:         maxL2Ops,
		maxConsecWrites:  maxConsecWrites,
		state:            BreakerClosed,
	}
}

// Allow checks if an operation should be allowed. Returns an error if blocked.
func (cb *CircuitBreaker) Allow(isWrite bool, isL2Plus bool) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// If circuit is open, reject all L1+
	if cb.state == BreakerOpen && isWrite {
		return fmt.Errorf("circuit breaker open: too many L2+ failures; session paused for safety")
	}

	// If half-open, only allow L0 reads
	if cb.state == BreakerHalfOpen && isWrite {
		return fmt.Errorf("circuit breaker half-open: %d consecutive writes detected; run a diagnostic first (L0 tools only)",
			cb.consecutiveWrites)
	}

	// Count operations
	if isL2Plus {
		cb.l2Count++
		if cb.l2Count > cb.maxL2Ops {
			cb.state = BreakerOpen
			return fmt.Errorf("circuit breaker opened: exceeded max L2+ operations (%d) in this session",
				cb.maxL2Ops)
		}
	}

	if isWrite {
		cb.consecutiveWrites++
		cb.totalOps++
		if cb.consecutiveWrites > cb.maxConsecWrites {
			cb.state = BreakerHalfOpen
			return fmt.Errorf("circuit breaker half-open: %d consecutive writes exceeds limit (%d)",
				cb.consecutiveWrites, cb.maxConsecWrites)
		}
	} else {
		// L0 read resets consecutive write counter and transitions from half-open back to closed
		cb.consecutiveWrites = 0
		if cb.state == BreakerHalfOpen {
			cb.state = BreakerClosed
		}
	}

	return nil
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failedOps++

	// If 3+ consecutive failures on L2+, open the circuit
	if cb.failedOps >= 3 && cb.l2Count > 0 {
		cb.state = BreakerOpen
		cb.lastFailure = time.Now()
	}
}

// RecordSuccess records a successful operation and resets failure count.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failedOps = 0
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.l2Count = 0
	cb.consecutiveWrites = 0
	cb.totalOps = 0
	cb.failedOps = 0
	cb.state = BreakerClosed
}

// State returns the current breaker state.
func (cb *CircuitBreaker) State() BreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Stats returns current statistics.
func (cb *CircuitBreaker) Stats() (l2Count, consecWrites, totalOps, failedOps int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.l2Count, cb.consecutiveWrites, cb.totalOps, cb.failedOps
}
