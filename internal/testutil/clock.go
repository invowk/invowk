// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"sync"
	"time"
)

type (
	// Clock abstracts time operations for deterministic testing.
	// Production code uses RealClock; tests use FakeClock.
	Clock interface {
		// Now returns the current time.
		Now() time.Time

		// After waits for the duration to elapse and then returns the current time.
		// For FakeClock, returns immediately when Advance() is called.
		After(d time.Duration) <-chan time.Time

		// Since returns the time elapsed since t.
		Since(t time.Time) time.Duration
	}

	// RealClock implements Clock using actual system time.
	// This is the default for production code.
	RealClock struct{}

	// FakeClock implements Clock with manually controlled time for testing.
	// Time only advances when Advance() or Set() is called.
	FakeClock struct {
		current time.Time
		mu      sync.Mutex
		waiters []waiter
	}

	// waiter tracks a pending After() call.
	waiter struct {
		target time.Time
		ch     chan time.Time
	}
)

// Now returns the current system time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// After returns a channel that receives the time after duration d.
func (RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Since returns the time elapsed since t.
func (RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// NewFakeClock creates a FakeClock initialized to the given time.
// If initial is zero, defaults to a fixed reference time for reproducibility.
func NewFakeClock(initial time.Time) *FakeClock {
	if initial.IsZero() {
		// Use a fixed reference time for reproducibility in tests
		initial = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return &FakeClock{current: initial}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// After returns a channel that receives the time when the target time is reached.
// The channel receives when Advance() or Set() moves past the target time.
func (c *FakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.current
		return ch
	}

	target := c.current.Add(d)
	c.waiters = append(c.waiters, waiter{target: target, ch: ch})
	return ch
}

// Since returns the fake time elapsed since t.
func (c *FakeClock) Since(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current.Sub(t)
}

// Advance moves the fake time forward by d.
// This triggers any After() channels waiting for times before the new current.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.current = c.current.Add(d)
	c.notifyWaiters()
}

// Set sets the fake time to t.
// This triggers any After() channels waiting for times before the new current.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = t
	c.notifyWaiters()
}

// notifyWaiters notifies all waiters whose target time has been reached.
// Must be called with mu held.
func (c *FakeClock) notifyWaiters() {
	remaining := c.waiters[:0]
	for _, w := range c.waiters {
		if !c.current.Before(w.target) {
			// Target time reached, notify
			select {
			case w.ch <- c.current:
			default:
			}
		} else {
			// Keep waiting
			remaining = append(remaining, w)
		}
	}
	c.waiters = remaining
}
