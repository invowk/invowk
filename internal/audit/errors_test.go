// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"errors"
	"testing"
)

func TestScanErrorContainsCheckerFindsJoinedLLMFailure(t *testing.T) {
	t.Parallel()

	err := errors.Join(
		&CheckerFailedError{CheckerName: "network", Err: errors.New("network failed")},
		&CheckerFailedError{CheckerName: LLMCheckerName, Err: errors.New("llm failed")},
	)

	if !ScanErrorContainsChecker(err, LLMCheckerName) {
		t.Fatal("expected joined LLM checker failure to be detected")
	}
}

func TestScanErrorContainsCheckerIgnoresOtherCheckers(t *testing.T) {
	t.Parallel()

	err := &CheckerFailedError{CheckerName: "network", Err: errors.New("network failed")}

	if ScanErrorContainsChecker(err, LLMCheckerName) {
		t.Fatal("did not expect non-LLM checker failure to be detected as LLM")
	}
}
