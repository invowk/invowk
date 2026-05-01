// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestWatchSessionInitialExecution(t *testing.T) {
	t.Parallel()

	t.Run("infrastructure error blocks startup", func(t *testing.T) {
		t.Parallel()
		infraErr := errors.New("config disappeared")
		session := newTestWatchSession(t, func(context.Context, []string) WatchExecutionOutcome {
			return WatchExecutionOutcome{Err: infraErr}
		})

		_, err := session.InitialExecution(t.Context())
		if err == nil || !strings.Contains(err.Error(), "cannot start watch mode") {
			t.Fatalf("InitialExecution() error = %v, want startup failure", err)
		}
		if !errors.Is(err, infraErr) {
			t.Fatalf("InitialExecution() error = %v, want wrapped infra error", err)
		}
	})

	t.Run("nonzero command exit allows watch startup", func(t *testing.T) {
		t.Parallel()
		session := newTestWatchSession(t, func(context.Context, []string) WatchExecutionOutcome {
			return WatchExecutionOutcome{ExitCode: 2}
		})

		outcome, err := session.InitialExecution(t.Context())
		if err != nil {
			t.Fatalf("InitialExecution() error = %v, want nil", err)
		}
		if outcome.ExitCode != 2 {
			t.Fatalf("ExitCode = %d, want 2", outcome.ExitCode)
		}
	})
}

func TestWatchSessionHandleChangePolicy(t *testing.T) {
	t.Parallel()

	t.Run("nonzero exit resets infrastructure failures", func(t *testing.T) {
		t.Parallel()
		infraErr := errors.New("runtime missing")
		outcomes := []WatchExecutionOutcome{
			{Err: infraErr},
			{ExitCode: 2},
			{Err: infraErr},
			{Err: infraErr},
			{ExitCode: 0},
		}
		session := newTestWatchSession(t, sequenceWatchExecutor(outcomes))

		for range outcomes {
			if _, err := session.HandleChange(t.Context(), []string{"main.go"}); err != nil {
				t.Fatalf("HandleChange() error = %v, want nil before consecutive infra limit", err)
			}
		}
	})

	t.Run("aborts after consecutive infrastructure failures", func(t *testing.T) {
		t.Parallel()
		infraErr := errors.New("runtime missing")
		session := newTestWatchSession(t, func(context.Context, []string) WatchExecutionOutcome {
			return WatchExecutionOutcome{Err: infraErr}
		})

		for i := range int(defaultWatchInfraErrorLimit - 1) {
			if _, err := session.HandleChange(t.Context(), []string{"main.go"}); err != nil {
				t.Fatalf("HandleChange(%d) error = %v, want nil before limit", i, err)
			}
		}
		_, err := session.HandleChange(t.Context(), []string{"main.go"})
		if err == nil || !strings.Contains(err.Error(), "aborting watch: 3 consecutive infrastructure failures") {
			t.Fatalf("HandleChange() error = %v, want abort", err)
		}
		if !errors.Is(err, infraErr) {
			t.Fatalf("HandleChange() error = %v, want wrapped infra error", err)
		}
	})
}

func newTestWatchSession(t *testing.T, execute WatchExecutionFunc) *WatchSession {
	t.Helper()
	session, err := NewWatchSession(WatchPlan{InfraErrorAbortLimit: defaultWatchInfraErrorLimit}, execute)
	if err != nil {
		t.Fatalf("NewWatchSession() error = %v", err)
	}
	return session
}

func sequenceWatchExecutor(outcomes []WatchExecutionOutcome) WatchExecutionFunc {
	var calls int
	return func(context.Context, []string) WatchExecutionOutcome {
		if calls >= len(outcomes) {
			return WatchExecutionOutcome{ExitCode: types.ExitCode(0)}
		}
		outcome := outcomes[calls]
		calls++
		return outcome
	}
}
