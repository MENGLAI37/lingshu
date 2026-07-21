package agent

import (
	"testing"
)

// ===========================================================================
// Circuit Breaker Tests
// ===========================================================================

func TestCircuitBreaker_Allow_Closed(t *testing.T) {
	cb := NewCircuitBreaker(5, 3)

	// L0 reads pass
	if err := cb.Allow(false, false); err != nil {
		t.Errorf("L0 read should be allowed: %v", err)
	}
	// L2 write passes in closed state
	if err := cb.Allow(true, true); err != nil {
		t.Errorf("L2 write should be allowed in closed state: %v", err)
	}
}

func TestCircuitBreaker_Allow_HalfOpen_OnConsecutiveWrites(t *testing.T) {
	cb := NewCircuitBreaker(5, 3)

	// 3 consecutive writes → should trigger half-open on 4th
	for i := 0; i < 3; i++ {
		if err := cb.Allow(true, true); err != nil {
			t.Fatalf("L2 write %d should be allowed: %v", i+1, err)
		}
	}

	// 4th consecutive write should be blocked (half-open)
	err := cb.Allow(true, true)
	if err == nil {
		t.Errorf("4th consecutive L2 write should be blocked (half-open)")
	}
	t.Logf("half-open block: %v", err)

	// L0 read should still pass in half-open
	if err := cb.Allow(false, false); err != nil {
		t.Errorf("L0 read should still be allowed in half-open: %v", err)
	}

	// After L0 read, state returns to closed
	if state := cb.State(); state != BreakerClosed {
		t.Errorf("expected closed after read reset, got %s", state)
	}
}

func TestCircuitBreaker_Allow_Open_OnExceedL2Ops(t *testing.T) {
	cb := NewCircuitBreaker(3, 10)

	// Exceed max L2 ops
	for i := 0; i < 3; i++ {
		if err := cb.Allow(true, true); err != nil {
			t.Fatalf("L2 write %d should be allowed: %v", i+1, err)
		}
	}

	// 4th L2 should trigger open
	err := cb.Allow(true, true)
	if err == nil {
		t.Errorf("4th L2 write should be blocked (open)")
	}
	t.Logf("open state block: %v", err)

	if state := cb.State(); state != BreakerOpen {
		t.Errorf("expected open state, got %s", state)
	}

	// L0 read should still pass in open state
	if err := cb.Allow(false, false); err != nil {
		t.Errorf("L0 read should still be allowed in open: %v", err)
	}

	// Writes remain blocked in open
	if err := cb.Allow(true, false); err == nil {
		t.Errorf("L1 write should be blocked in open state")
	}
}

func TestCircuitBreaker_RecordFailure_OpensAfterThree(t *testing.T) {
	cb := NewCircuitBreaker(5, 10)

	// Do some L2 ops so failure tracking triggers open
	_ = cb.Allow(true, true)
	_ = cb.Allow(true, true)

	cb.RecordFailure()
	cb.RecordFailure()

	if state := cb.State(); state != BreakerClosed {
		t.Errorf("expected closed after 2 failures, got %s", state)
	}

	cb.RecordFailure()

	if state := cb.State(); state != BreakerOpen {
		t.Errorf("expected open after 3 failures, got %s", state)
	}
}

func TestCircuitBreaker_RecordSuccess_ResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(5, 10)

	_ = cb.Allow(true, true)
	_ = cb.Allow(true, true)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // Reset before 3rd failure
	cb.RecordFailure()

	if state := cb.State(); state != BreakerClosed {
		t.Errorf("expected closed after success reset, got %s", state)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(3, 3)

	// Push to half-open
	for i := 0; i < 3; i++ {
		_ = cb.Allow(true, true)
	}
	_ = cb.Allow(true, true) // triggers half-open

	cb.Reset()

	if state := cb.State(); state != BreakerClosed {
		t.Errorf("expected closed after reset, got %s", state)
	}

	l2, cw, _, _ := cb.Stats()
	if l2 != 0 {
		t.Errorf("expected l2Count=0 after reset, got %d", l2)
	}
	if cw != 0 {
		t.Errorf("expected consecWrites=0 after reset, got %d", cw)
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(10, 10)

	_ = cb.Allow(true, true)
	_ = cb.Allow(true, true)
	_ = cb.Allow(false, false)

	l2, cw, total, failed := cb.Stats()
	if l2 != 2 {
		t.Errorf("expected l2Count=2, got %d", l2)
	}
	if cw != 0 {
		t.Errorf("expected consecWrites=0 (reset by read), got %d", cw)
	}
	_ = total
	_ = failed
}

func TestCircuitBreaker_DefaultValues(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)

	if state := cb.State(); state != BreakerClosed {
		t.Errorf("expected closed, got %s", state)
	}

	// Defaults: maxL2Ops=5, maxConsecWrites=3
	// The consecutive-writes limit (3) triggers BEFORE maxL2Ops (5).
	// So with 3 consecutive L2 writes, the 4th is blocked (half-open).
	for i := 0; i < 3; i++ {
		if err := cb.Allow(true, true); err != nil {
			t.Fatalf("L2 write %d should be allowed: %v", i+1, err)
		}
	}

	// 4th consecutive L2 write → half-open (maxConsecWrites=3 exceeded)
	err := cb.Allow(true, true)
	if err == nil {
		t.Errorf("4th consecutive L2 write should be blocked by half-open (maxConsecWrites=3)")
	}

	// After a reset, we can verify maxL2Ops defaults
	cb.Reset()
	// Now add writes interleaved with reads to avoid triggering consecutive-write limit
	for i := 0; i < 5; i++ {
		_ = cb.Allow(true, true)
		_ = cb.Allow(false, false) // reset consecutive writes
	}

	// 6th L2 should be blocked (maxL2Ops=5 exceeded)
	l2Err := cb.Allow(true, true)
	if l2Err == nil {
		t.Errorf("6th L2 write should be blocked after maxL2Ops=5")
	}
}

func TestBreakerState_Constants(t *testing.T) {
	// Ensure state constants are distinct
	if BreakerClosed == BreakerHalfOpen || BreakerClosed == BreakerOpen || BreakerHalfOpen == BreakerOpen {
		t.Errorf("breaker states must be distinct")
	}
}
