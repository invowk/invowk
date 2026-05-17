// SPDX-License-Identifier: MPL-2.0

package audit

import (
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
