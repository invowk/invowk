// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"testing"
)

func TestScanFailureIsFatalFindsJoinedLLMFailure(t *testing.T) {
	t.Parallel()

	err := errors.Join(
		&CheckerFailedError{CheckerName: "network", Err: errors.New("network failed")},
		&CheckerFailedError{CheckerName: LLMCheckerName, Err: errors.New("llm failed")},
	)

	if !ScanFailureIsFatal(err) {
		t.Fatal("expected joined LLM checker failure to be fatal")
	}
}

func TestScanFailureIsFatalIgnoresOtherCheckers(t *testing.T) {
	t.Parallel()

	err := &CheckerFailedError{CheckerName: "network", Err: errors.New("network failed")}

	if ScanFailureIsFatal(err) {
		t.Fatal("did not expect non-LLM checker failure to be fatal")
	}
}

func TestScanFailureIsFatalFindsCancellation(t *testing.T) {
	t.Parallel()

	err := errors.Join(
		&CheckerFailedError{CheckerName: lockFileCheckerName, Err: context.Canceled},
		&CheckerFailedError{CheckerName: "script", Err: errors.New("script check failed")},
	)

	if !ScanFailureIsFatal(err) {
		t.Fatal("expected joined context cancellation to be fatal")
	}
}

func TestScanFailureIsFatalFindsDeadline(t *testing.T) {
	t.Parallel()

	err := &CheckerFailedError{CheckerName: "script", Err: context.DeadlineExceeded}

	if !ScanFailureIsFatal(err) {
		t.Fatal("expected context deadline to be fatal")
	}
}
