// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProbeEngineAvailabilityWithRetryConfig_SucceedsImmediately(t *testing.T) {
	t.Parallel()

	attempts := 0
	sleeps := 0
	ok := probeEngineAvailabilityWithRetryConfig(
		func(context.Context) error {
			attempts++
			return nil
		},
		func(time.Duration) { sleeps++ },
	)
	if !ok {
		t.Fatal("expected availability probe to succeed")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if sleeps != 0 {
		t.Fatalf("sleep calls = %d, want 0", sleeps)
	}
}

func TestProbeEngineAvailabilityWithRetryConfig_RetriesTransientThenSucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	var gotSleeps []time.Duration
	ok := probeEngineAvailabilityWithRetryConfig(
		func(context.Context) error {
			attempts++
			if attempts < 3 {
				return context.DeadlineExceeded
			}
			return nil
		},
		func(d time.Duration) { gotSleeps = append(gotSleeps, d) },
	)
	if !ok {
		t.Fatal("expected availability probe to succeed after transient failures")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	wantSleeps := []time.Duration{
		availabilityProbeBaseBackoff,
		2 * availabilityProbeBaseBackoff,
	}
	if len(gotSleeps) != len(wantSleeps) {
		t.Fatalf("sleep count = %d, want %d", len(gotSleeps), len(wantSleeps))
	}
	for i := range wantSleeps {
		if gotSleeps[i] != wantSleeps[i] {
			t.Fatalf("sleep[%d] = %s, want %s", i, gotSleeps[i], wantSleeps[i])
		}
	}
}

func TestProbeEngineAvailabilityWithRetryConfig_StopsOnNonTransient(t *testing.T) {
	t.Parallel()

	attempts := 0
	sleeps := 0
	ok := probeEngineAvailabilityWithRetryConfig(
		func(context.Context) error {
			attempts++
			return errors.New("permission denied")
		},
		func(time.Duration) { sleeps++ },
	)
	if ok {
		t.Fatal("expected availability probe to fail on non-transient error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if sleeps != 0 {
		t.Fatalf("sleep calls = %d, want 0", sleeps)
	}
}

func TestProbeEngineAvailabilityWithRetryConfig_ExhaustsTransientFailures(t *testing.T) {
	t.Parallel()

	attempts := 0
	sleeps := 0
	ok := probeEngineAvailabilityWithRetryConfig(
		func(context.Context) error {
			attempts++
			return context.DeadlineExceeded
		},
		func(time.Duration) { sleeps++ },
	)
	if ok {
		t.Fatal("expected availability probe to fail after exhausting retries")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if sleeps != 2 {
		t.Fatalf("sleep calls = %d, want 2", sleeps)
	}
}

func TestProbeEngineAvailabilityWithRetryConfig_NilProbe(t *testing.T) {
	t.Parallel()

	if probeEngineAvailabilityWithRetryConfig(nil, nil) {
		t.Fatal("nil probe should fail")
	}
}

func TestIsTransientAvailabilityError(t *testing.T) {
	t.Parallel()

	if !isTransientAvailabilityError(context.DeadlineExceeded) {
		t.Fatal("deadline exceeded should be transient for availability checks")
	}
	if !isTransientAvailabilityError(context.Canceled) {
		t.Fatal("context canceled should be transient for availability checks")
	}
	if !isTransientAvailabilityError(errors.New("connection refused")) {
		t.Fatal("transient container/network errors should be treated as transient")
	}
	if isTransientAvailabilityError(errors.New("permission denied")) {
		t.Fatal("non-transient errors should not be retried")
	}
}
