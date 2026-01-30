// SPDX-License-Identifier: MPL-2.0

// Package contracts defines the API contracts for new testutil helpers.
// These are reference signatures; actual implementation lives in internal/testutil/.
package contracts

import (
	"sync"
	"time"
)

// Clock abstracts time operations for deterministic testing.
// Production code uses RealClock; tests use FakeClock.
type Clock interface {
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
type RealClock struct{}

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

// FakeClock implements Clock with manually controlled time for testing.
// Time only advances when Advance() or Set() is called.
type FakeClock struct {
	current time.Time
	mu      sync.Mutex
	waiters []chan time.Time
}

// NewFakeClock creates a FakeClock initialized to the given time.
// If initial is zero, defaults to time.Now() for convenience.
func NewFakeClock(initial time.Time) *FakeClock {
	if initial.IsZero() {
		initial = time.Now()
	}
	return &FakeClock{current: initial}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// After returns a channel that receives the time when it would be reached.
// The channel receives when Advance() moves past the target time.
func (c *FakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.current
		return ch
	}
	c.waiters = append(c.waiters, ch)
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

	// Notify all waiters (simplified: in real impl, track target times)
	for _, ch := range c.waiters {
		select {
		case ch <- c.current:
		default:
		}
	}
	c.waiters = nil
}

// Set sets the fake time to t.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = t
}
