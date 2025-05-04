package gomultirate

import (
	"context"
	"testing"
	"time"
)

// Run some try calls to verify behaviour.
func TestTryCalls(t *testing.T) {
	limits := map[string]*Limit{
		"test": NewLimit(time.Second, 2),
	}

	limiter, _ := NewRateLimiter(limits)

	// Try 3 times without waiting.
	if ok, _ := limiter.Try(); !ok {
		t.Error("expected Try to succeed on first call")
	}

	if ok, _ := limiter.Try(); !ok {
		t.Error("expected Try to succeed on second call")
	}

	// This one should fail.
	if ok, _ := limiter.Try(); ok {
		t.Error("expected Try to fail on third call")
	}

	time.Sleep(time.Second)

	if ok, _ := limiter.Try(); !ok {
		t.Error("expected Try to succeed on fourth call after waiting one second")
	}
}

// Test the wait calls to verify if they are working correctly.
func TestWaitCalls(t *testing.T) {
	limits := map[string]*Limit{
		"test": NewLimit(time.Second, 1),
	}
	limiter, _ := NewRateLimiter(limits)

	ctx := context.Background()

	start := time.Now()
	_ = limiter.Wait(ctx)
	_ = limiter.Wait(ctx)

	elapsed := time.Since(start)
	if elapsed < time.Second {
		t.Errorf("Wait should take at least one second, took: %v", elapsed)
	}
}

func TestWaitEvenly(t *testing.T) {
	limits := map[string]*Limit{
		"test": NewLimit(time.Second, 3),
	}

	limiter, _ := NewRateLimiter(limits)

	ctx := context.Background()

	start := time.Now()
	for i := 0; i < 3; i++ {
		_ = limiter.WaitEvenly(ctx, "test")

		// Skip the first execution since it doesn't wait.
		if i == 0 {
			continue
		}

		// Verify how much time was elapsed between the calls.
		elapsed := time.Since(start)

		// Add some margin.
		expected := time.Second / 3
		delta := elapsed - expected
		if delta < -50*time.Millisecond || delta > 50*time.Millisecond {
			t.Errorf("WaitEvenly should space calls ~%v apart, got %v", expected, elapsed)
		}
		start = time.Now()
	}
}

// Test to see if the wait works with timeout.
func TestWaitTimeout(t *testing.T) {
	limits := map[string]*Limit{
		"test": NewLimit(3*time.Second, 2),
	}

	limiter, _ := NewRateLimiter(limits)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	limiter.Wait(ctx)
	limiter.Wait(ctx)
	if err := limiter.Wait(ctx); err == nil {
		t.Errorf("Wait should return error due to timeout")
	}

}
