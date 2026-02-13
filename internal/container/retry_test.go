// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWithBackoff_SucceedsFirstAttempt(t *testing.T) {
	t.Parallel()
	calls := 0
	err := RetryWithBackoff(context.Background(), 3, 10*time.Millisecond, func(attempt int) (bool, error) {
		calls++
		return false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_RetriesThenSucceeds(t *testing.T) {
	t.Parallel()
	calls := 0
	err := RetryWithBackoff(context.Background(), 5, 10*time.Millisecond, func(attempt int) (bool, error) {
		calls++
		if attempt < 2 {
			return true, errors.New("transient")
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_ExhaustsRetries(t *testing.T) {
	t.Parallel()
	calls := 0
	err := RetryWithBackoff(context.Background(), 3, 10*time.Millisecond, func(attempt int) (bool, error) {
		calls++
		return true, errors.New("always transient")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "always transient" {
		t.Fatalf("expected last error, got: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_ContextCancelledBetweenRetries(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
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

func TestRetryWithBackoff_NonTransientExitsImmediately(t *testing.T) {
	t.Parallel()
	calls := 0
	permanentErr := errors.New("permanent")
	err := RetryWithBackoff(context.Background(), 5, 10*time.Millisecond, func(attempt int) (bool, error) {
		calls++
		return false, permanentErr
	})
	if !errors.Is(err, permanentErr) {
		t.Fatalf("expected permanent error, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_BackoffTiming(t *testing.T) {
	t.Parallel()
	start := time.Now()
	_ = RetryWithBackoff(context.Background(), 3, 50*time.Millisecond, func(attempt int) (bool, error) {
		return true, errors.New("retry")
	})
	elapsed := time.Since(start)
	// Expected: 50ms (attempt 0->1) + 100ms (attempt 1->2) = 150ms minimum
	if elapsed < 100*time.Millisecond {
		t.Fatalf("expected at least 100ms of backoff, got %v", elapsed)
	}
}
