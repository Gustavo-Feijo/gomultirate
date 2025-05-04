package gomultirate

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Single rate limit entry.
type Limit struct {
	count       int
	interval    time.Duration
	lastReset   time.Time
	limit       int
	timeBetween time.Duration
}

// Create a new limit entry.
func NewLimit(interval time.Duration, limit int) *Limit {
	if limit <= 0 {
		panic("limit must be greater than zero")
	}

	return &Limit{
		count:       0,
		interval:    interval,
		lastReset:   time.Now(),
		limit:       limit,
		timeBetween: interval / time.Duration(limit),
	}
}

// Try to reset the limit and return if it's available.
func (l *Limit) checkLimit() bool {
	now := time.Now()
	if time.Since(l.lastReset) >= l.interval {
		l.count = 0
		l.lastReset = now
	}
	return l.count < l.limit
}

// Get how much time until the next reset.
// The mutex must be held by the caller.
func (l *Limit) getRemainingTime() time.Duration {
	// It's already free to use.
	if l.count < l.limit {
		return 0
	}

	// Verify how much time has elapsed since the last reset.
	elapsed := time.Since(l.lastReset)

	// Return how much until the next reset.
	remaining := l.interval - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Main RateLimiter.
// Simply provide a map of limit windows and a mutex.
type rateLimiter struct {
	limits map[string]*Limit
	mu     sync.Mutex
}

// Create the rate limiter with the provided map of limits.
func NewRateLimiter(limits map[string]*Limit) (*rateLimiter, error) {
	if len(limits) == 0 {
		return nil, errors.New("can't provide a rate limiter with no limits")
	}

	return &rateLimiter{
		limits: limits,
	}, nil
}

// Verify if the limits are available, if they are, consume them.
// The mutex must be held by the caller.
func (r *rateLimiter) allowAndIncrement() bool {
	// Check all windows
	for _, win := range r.limits {
		if !win.checkLimit() {
			return false
		}
	}

	// All limits available, increment counters
	r.incrementCounts()
	return true
}

// Calculate the minimum wait time necessary for all windows to be refilled.
// The mutex must be held by the caller.
func (r *rateLimiter) getMinWaitTime() time.Duration {
	var minWaitTime time.Duration

	// Go through each limit and get the remaining time.
	for _, lim := range r.limits {
		waitTime := lim.getRemainingTime()

		// We get the highest wait time between the limits.
		// This one is the minimum wait time to proceed with the execution.
		if waitTime > 0 && (minWaitTime == 0 || waitTime < minWaitTime) {
			minWaitTime = waitTime
		}
	}

	return minWaitTime
}

// Consume one of each window limit.
// The mutex must be held by the caller.
func (r *rateLimiter) incrementCounts() {
	for _, win := range r.limits {
		win.count++
	}
}

// Try to get the limit without blocking.
// Returns true/false depending on if the limit is available.
// If not, returns the time until the next reset.
func (r *rateLimiter) Try() (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.allowAndIncrement()
	if limit {
		return true, 0
	}

	// Get how much time until the next reset and return it.
	waitTime := r.getMinWaitTime()
	return false, waitTime
}

// Wait for all the limit windows to be available.
// Receive a context for handling timeouts.
func (r *rateLimiter) Wait(ctx context.Context) error {
	// Get the lock.
	r.mu.Lock()

	// If it's free to use, just unlock and return.
	if r.allowAndIncrement() {
		r.mu.Unlock()
		return nil
	}

	// Calculate how much time until the next reset.
	waitTime := r.getMinWaitTime()

	// Unlock since it will wait.
	r.mu.Unlock()

	// Create a timer.
	timer := time.NewTimer(waitTime)

	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// After the time has reached, try to get the rate again.
			r.mu.Lock()
			if r.allowAndIncrement() {
				r.mu.Unlock()
				return nil
			}

			// If couldn't, reset the timer and run again.
			waitTime = r.getMinWaitTime()
			r.mu.Unlock()
			timer.Reset(waitTime)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Wait for all the limit windows to be available.
// Get the limits at a fixed ratio based on the limit key.
// Usefull if don't need to have a burst of usage.
func (r *rateLimiter) WaitEvenly(ctx context.Context, key string) error {
	for {
		r.mu.Lock()

		// Check if the limit exists
		lim, exists := r.limits[key]
		if !exists {
			r.mu.Unlock()
			return errors.New("limiter doesn't exist")
		}

		now := time.Now()

		// Reset if interval has passed.
		if now.Sub(lim.lastReset) >= lim.interval {
			lim.count = 0
			lim.lastReset = now
		}

		// If under the limit, proceed.
		if lim.count < lim.limit {
			// Calculate the next timing.
			nextTime := lim.lastReset.Add(lim.timeBetween * time.Duration(lim.count))
			waitTime := nextTime.Sub(now)

			// Increment the counter before waiting (If necessary)
			lim.count++
			r.mu.Unlock()

			// Wait if needed (Distribute evenly)
			if waitTime > 0 {
				timer := time.NewTimer(waitTime)
				select {
				case <-timer.C:
					// We've waited long enough, return success
					return nil
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				}
			}

			return nil
		}

		// We're at the limit, calculate time until reset
		waitTime := max(lim.interval-now.Sub(lim.lastReset), 0)

		// Unlock since it will wait.
		r.mu.Unlock()

		// Wait for the reset time
		timer := time.NewTimer(waitTime)
		select {
		case <-timer.C:
			// Continue the loop to try again.
			timer.Stop()
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}
