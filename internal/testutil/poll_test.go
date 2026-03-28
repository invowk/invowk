// SPDX-License-Identifier: MPL-2.0

package testutil_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/testutil"
)

func TestPollUntil_ConditionMetImmediately(t *testing.T) {
	t.Parallel()

	got := testutil.PollUntil(t, time.Second, 10*time.Millisecond, func() bool {
		return true
	})
	if !got {
		t.Fatal("expected PollUntil to return true when condition is immediately met")
	}
}

func TestPollUntil_ConditionMetAfterDelay(t *testing.T) {
	t.Parallel()

	var ready atomic.Bool
	go func() {
		time.Sleep(50 * time.Millisecond)
		ready.Store(true)
	}()

	got := testutil.PollUntil(t, time.Second, 10*time.Millisecond, ready.Load)
	if !got {
		t.Fatal("expected PollUntil to return true after goroutine sets ready")
	}
}

func TestPollUntil_Timeout(t *testing.T) {
	t.Parallel()

	got := testutil.PollUntil(t, 50*time.Millisecond, 10*time.Millisecond, func() bool {
		return false
	})
	if got {
		t.Fatal("expected PollUntil to return false on timeout")
	}
}

func TestAssertNeverTrue_NoFalsePositive(t *testing.T) {
	t.Parallel()

	// Should not fail — condition is always false.
	testutil.AssertNeverTrue(t, 50*time.Millisecond, 10*time.Millisecond,
		"should never be true", func() bool {
			return false
		})
}
