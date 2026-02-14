// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"sync"
	"testing"
	"time"
)

func TestRealClock_Now(t *testing.T) {
	t.Parallel()

	clock := RealClock{}
	before := time.Now()
	result := clock.Now()
	after := time.Now()

	if result.Before(before) || result.After(after) {
		t.Errorf("RealClock.Now() returned %v, expected between %v and %v", result, before, after)
	}
}

func TestRealClock_Since(t *testing.T) {
	t.Parallel()

	clock := RealClock{}
	past := time.Now().Add(-1 * time.Second)
	elapsed := clock.Since(past)

	if elapsed < 1*time.Second {
		t.Errorf("RealClock.Since() returned %v, expected >= 1s", elapsed)
	}
}

func TestRealClock_After(t *testing.T) {
	t.Parallel()

	clock := RealClock{}
	ch := clock.After(1 * time.Millisecond)

	select {
	case <-ch:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("RealClock.After() did not fire within 100ms")
	}
}

func TestFakeClock_Now(t *testing.T) {
	t.Parallel()

	initial := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(initial)

	if got := clock.Now(); !got.Equal(initial) {
		t.Errorf("FakeClock.Now() = %v, want %v", got, initial)
	}
}

func TestFakeClock_Now_DefaultTime(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Time{})
	expected := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	if got := clock.Now(); !got.Equal(expected) {
		t.Errorf("FakeClock.Now() with zero time = %v, want %v", got, expected)
	}
}

func TestFakeClock_Advance(t *testing.T) {
	t.Parallel()

	initial := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(initial)

	clock.Advance(1 * time.Hour)

	expected := initial.Add(1 * time.Hour)
	if got := clock.Now(); !got.Equal(expected) {
		t.Errorf("After Advance(1h), Now() = %v, want %v", got, expected)
	}
}

func TestFakeClock_Set(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Time{})
	newTime := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)

	clock.Set(newTime)

	if got := clock.Now(); !got.Equal(newTime) {
		t.Errorf("After Set(), Now() = %v, want %v", got, newTime)
	}
}

func TestFakeClock_Since(t *testing.T) {
	t.Parallel()

	initial := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(initial)

	past := initial.Add(-30 * time.Minute)
	elapsed := clock.Since(past)

	if elapsed != 30*time.Minute {
		t.Errorf("FakeClock.Since() = %v, want 30m", elapsed)
	}

	// Advance and check again
	clock.Advance(15 * time.Minute)
	elapsed = clock.Since(past)

	if elapsed != 45*time.Minute {
		t.Errorf("After Advance(15m), Since() = %v, want 45m", elapsed)
	}
}

func TestFakeClock_After_ImmediateForZeroOrNegative(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Time{})

	// Zero duration should fire immediately
	ch := clock.After(0)
	select {
	case <-ch:
		// Expected
	default:
		t.Error("After(0) should fire immediately")
	}

	// Negative duration should fire immediately
	ch = clock.After(-1 * time.Second)
	select {
	case <-ch:
		// Expected
	default:
		t.Error("After(-1s) should fire immediately")
	}
}

func TestFakeClock_After_FiresOnAdvance(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Time{})

	ch := clock.After(10 * time.Minute)

	// Should not fire yet
	select {
	case <-ch:
		t.Error("After(10m) should not fire before Advance")
	default:
		// Expected
	}

	// Advance past the target
	clock.Advance(15 * time.Minute)

	// Now it should fire
	select {
	case <-ch:
		// Expected
	default:
		t.Error("After(10m) should fire after Advance(15m)")
	}
}

func TestFakeClock_After_FiresOnSet(t *testing.T) {
	t.Parallel()

	initial := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewFakeClock(initial)

	ch := clock.After(1 * time.Hour)

	// Set past the target
	clock.Set(initial.Add(2 * time.Hour))

	select {
	case <-ch:
		// Expected
	default:
		t.Error("After() should fire after Set() past target")
	}
}

func TestFakeClock_After_MultipleWaiters(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Time{})

	ch1 := clock.After(5 * time.Minute)
	ch2 := clock.After(10 * time.Minute)
	ch3 := clock.After(15 * time.Minute)

	// Advance to 7 minutes - only ch1 should fire
	clock.Advance(7 * time.Minute)

	select {
	case <-ch1:
		// Expected
	default:
		t.Error("ch1 should fire at 7m")
	}

	select {
	case <-ch2:
		t.Error("ch2 should not fire at 7m")
	default:
		// Expected
	}

	select {
	case <-ch3:
		t.Error("ch3 should not fire at 7m")
	default:
		// Expected
	}

	// Advance to 12 minutes total - ch2 should now fire
	clock.Advance(5 * time.Minute)

	select {
	case <-ch2:
		// Expected
	default:
		t.Error("ch2 should fire at 12m")
	}

	select {
	case <-ch3:
		t.Error("ch3 should not fire at 12m")
	default:
		// Expected
	}

	// Advance to 20 minutes total - ch3 should fire
	clock.Advance(8 * time.Minute)

	select {
	case <-ch3:
		// Expected
	default:
		t.Error("ch3 should fire at 20m")
	}
}

func TestFakeClock_Concurrent(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Time{})
	var wg sync.WaitGroup

	// Multiple goroutines reading Now()
	for range 10 {
		wg.Go(func() {
			for range 100 {
				_ = clock.Now()
			}
		})
	}

	// While another goroutine advances
	wg.Go(func() {
		for range 50 {
			clock.Advance(1 * time.Millisecond)
		}
	})

	wg.Wait()
	// If no race condition, test passes
}

func TestClock_Interface(t *testing.T) {
	t.Parallel()

	// Ensure both types implement Clock interface
	var _ Clock = RealClock{}
	var _ Clock = &FakeClock{}
}
