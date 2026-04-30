// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"testing"
	"time"
)

// errAlwaysTransient is a test-local sentinel for retry exhaustion assertions.
var errAlwaysTransient = errors.New("always transient")

func TestRetryWithBackoff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		maxRetry  int
		fn        func(int) (bool, error)
		wantErr   error
		wantCalls int
	}{
		{
			name:     "SucceedsFirstAttempt",
			maxRetry: 3,
			fn: func(_ int) (bool, error) {
				return false, nil
			},
			wantErr:   nil,
			wantCalls: 1,
		},
		{
			name:     "RetriesThenSucceeds",
			maxRetry: 5,
			fn: func(attempt int) (bool, error) {
				if attempt < 2 {
					return true, errors.New("transient")
				}
				return false, nil
			},
			wantErr:   nil,
			wantCalls: 3,
		},
		{
			name:     "ExhaustsRetries",
			maxRetry: 3,
			fn: func(_ int) (bool, error) {
				return true, errAlwaysTransient
			},
			wantErr:   errAlwaysTransient,
			wantCalls: 3,
		},
		{
			name:     "NonTransientExitsImmediately",
			maxRetry: 5,
			fn: func(_ int) (bool, error) {
				return false, errAlwaysTransient
			},
			wantErr:   errAlwaysTransient,
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			wrapped := func(attempt int) (bool, error) {
				calls++
				return tt.fn(attempt)
			}
			err := RetryWithBackoff(t.Context(), tt.maxRetry, 10*time.Millisecond, wrapped)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got: %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != tt.wantCalls {
				t.Fatalf("expected %d call(s), got %d", tt.wantCalls, calls)
			}
		})
	}
}

func TestRetryWithBackoff_ContextCancelledBetweenRetries(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	calls := 0
	err := RetryWithBackoff(ctx, 5, 10*time.Millisecond, func(attempt int) (bool, error) {
		calls++
		if attempt == 0 {
			cancel() // Cancel after first attempt
			return true, errors.New("transient")
		}
		t.Fatal("should not reach second attempt")
		return false, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_SleeperReceivesBackoffSchedule(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	calls := 0
	err := retryWithBackoff(
		t.Context(),
		3,
		25*time.Millisecond,
		func(_ int) (bool, error) {
			calls++
			return true, errAlwaysTransient
		},
		func(_ context.Context, duration time.Duration) error {
			sleeps = append(sleeps, duration)
			return nil
		},
	)
	if !errors.Is(err, errAlwaysTransient) {
		t.Fatalf("expected %v, got: %v", errAlwaysTransient, err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	want := []time.Duration{25 * time.Millisecond, 50 * time.Millisecond}
	if len(sleeps) != len(want) {
		t.Fatalf("sleeps = %v, want %v", sleeps, want)
	}
	for i := range want {
		if sleeps[i] != want[i] {
			t.Fatalf("sleeps = %v, want %v", sleeps, want)
		}
	}
}

func TestRetryWithBackoff_BackoffTiming(t *testing.T) {
	t.Parallel()
	start := time.Now()
	_ = RetryWithBackoff(t.Context(), 3, 50*time.Millisecond, func(_ int) (bool, error) {
		return true, errors.New("retry")
	})
	elapsed := time.Since(start)
	// Expected: 50ms (attempt 0->1) + 100ms (attempt 1->2) = 150ms minimum
	if elapsed < 100*time.Millisecond {
		t.Fatalf("expected at least 100ms of backoff, got %v", elapsed)
	}
}
