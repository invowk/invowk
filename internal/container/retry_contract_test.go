// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

const (
	outcomeSuccess retryOutcome = iota
	outcomeRetryable
	outcomePermanent
)

var errRetryContractSleeper = errors.New("sleeper failed")

type (
	retryOutcome int

	// retryInterrupt stops the loop before a given attempt, either by
	// cancelling the context or by making the sleeper fail.
	retryInterrupt struct {
		beforeAttempt int // 0 means never
		viaSleeper    bool
	}
)

// enumerateRetryOutcomes returns every outcome sequence of the given length.
func enumerateRetryOutcomes(length int) [][]retryOutcome {
	sequences := [][]retryOutcome{{}}
	for range length {
		var next [][]retryOutcome
		for _, seq := range sequences {
			for _, o := range []retryOutcome{outcomeSuccess, outcomeRetryable, outcomePermanent} {
				next = append(next, append(append([]retryOutcome(nil), seq...), o))
			}
		}
		sequences = next
	}
	return sequences
}

// TestRetryWithBackoff_Contract exhaustively enumerates outcome sequences up to
// four attempts, with and without an interruption before each retry, and checks
// the retry contract: op runs at most maxAttempts times (never for
// maxAttempts <= 0), a nil error wins over the retry flag, a permanent error is
// returned unchanged without sleeping, an interruption returns an error wrapping
// ctx.Err() or the sleeper's error, sleeps follow base*2^(attempt-1), and
// exhaustion returns the last error.
func TestRetryWithBackoff_Contract(t *testing.T) {
	t.Parallel()

	const base = time.Millisecond
	cases := 0
	for maxAttempts := -1; maxAttempts <= 4; maxAttempts++ {
		for _, outcomes := range enumerateRetryOutcomes(max(maxAttempts, 0)) {
			interrupts := []retryInterrupt{{}}
			for before := 1; before < maxAttempts; before++ {
				interrupts = append(interrupts, retryInterrupt{before, false}, retryInterrupt{before, true})
			}
			for _, interrupt := range interrupts {
				cases++
				checkRetryContract(t, maxAttempts, base, outcomes, interrupt)
			}
		}
	}
	if cases == 0 {
		t.Fatal("no retry cases enumerated")
	}
}

func checkRetryContract(t *testing.T, maxAttempts int, base time.Duration, outcomes []retryOutcome, interrupt retryInterrupt) {
	t.Helper()

	name := fmt.Sprintf("max=%d outcomes=%v interrupt=%+v", maxAttempts, outcomes, interrupt)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errs := make([]error, len(outcomes))
	for i := range outcomes {
		errs[i] = fmt.Errorf("attempt %d failed", i)
	}
	var calls []int
	var sleeps []time.Duration
	op := func(attempt int) (bool, error) {
		calls = append(calls, attempt)
		if interrupt.beforeAttempt == attempt+1 && !interrupt.viaSleeper {
			cancel() // takes effect at the next between-retry check
		}
		switch outcomes[attempt] {
		case outcomeSuccess:
			return true, nil // a nil error must win over retry=true
		case outcomeRetryable:
			return true, errs[attempt]
		case outcomePermanent:
			return false, errs[attempt]
		default:
			t.Fatalf("unknown outcome %d", outcomes[attempt])
			return false, nil
		}
	}
	sleep := func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		if interrupt.viaSleeper && interrupt.beforeAttempt == len(calls) {
			return errRetryContractSleeper
		}
		return nil
	}

	err := retryWithBackoff(ctx, maxAttempts, base, op, sleep)

	// Reference outcome of the contract.
	var wantErr error
	wantCalls, wantSleeps := 0, 0
	interrupted := false
	for attempt := range max(maxAttempts, 0) {
		if attempt > 0 {
			if interrupt.beforeAttempt == attempt {
				interrupted = true
				if interrupt.viaSleeper {
					wantSleeps++
				}
				break
			}
			wantSleeps++
		}
		wantCalls++
		if outcomes[attempt] == outcomeSuccess {
			wantErr = nil
			break
		}
		wantErr = errs[attempt]
		if outcomes[attempt] == outcomePermanent {
			break
		}
	}

	if len(calls) != wantCalls {
		t.Fatalf("%s: op calls = %d, want %d", name, len(calls), wantCalls)
	}
	if len(sleeps) != wantSleeps {
		t.Fatalf("%s: sleeps = %d, want %d", name, len(sleeps), wantSleeps)
	}
	for i, d := range sleeps {
		if want := base * time.Duration(1<<i); d != want {
			t.Fatalf("%s: sleep %d = %v, want %v", name, i, d, want)
		}
	}
	switch {
	case interrupted && interrupt.viaSleeper:
		if !errors.Is(err, errRetryContractSleeper) {
			t.Fatalf("%s: error = %v, want wrapping the sleeper error", name, err)
		}
	case interrupted:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%s: error = %v, want wrapping context.Canceled", name, err)
		}
	case wantErr == nil:
		if err != nil {
			t.Fatalf("%s: error = %v, want nil", name, err)
		}
	default:
		if !errors.Is(err, wantErr) || err.Error() != wantErr.Error() {
			t.Fatalf("%s: error = %v, want exactly %v", name, err, wantErr)
		}
	}
}
