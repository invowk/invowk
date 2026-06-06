// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestCustomCheckResultMutationContracts(t *testing.T) {
	t.Parallel()

	result, err := NewCustomCheckResult("ok", 0)
	if err != nil {
		t.Fatalf("NewCustomCheckResult() error = %v, want nil", err)
	}
	if got := result.Output(); got != "ok" {
		t.Fatalf("CustomCheckResult.Output() = %q, want ok", got)
	}
	if got := result.ExitCode(); got != 0 {
		t.Fatalf("CustomCheckResult.ExitCode() = %d, want 0", got)
	}

	invalid := CustomCheckResult{output: "bad", exitCode: types.ExitCode(-1)}
	if err := invalid.Validate(); !errors.Is(err, types.ErrInvalidExitCode) {
		t.Fatalf("CustomCheckResult.Validate() error = %v, want ErrInvalidExitCode", err)
	}
	if _, err := NewCustomCheckResult("bad", types.ExitCode(-1)); !errors.Is(err, types.ErrInvalidExitCode) {
		t.Fatalf("NewCustomCheckResult() error = %v, want ErrInvalidExitCode", err)
	}
}
