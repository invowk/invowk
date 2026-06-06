// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestGlobPatternMutationInvalidSyntaxPayload(t *testing.T) {
	t.Parallel()

	pattern := GlobPattern("[invalid")
	err := pattern.Validate()
	if !errors.Is(err, ErrInvalidGlobPattern) {
		t.Fatalf("GlobPattern(%q).Validate() error = %v, want ErrInvalidGlobPattern", pattern, err)
	}
	var patternErr *InvalidGlobPatternError
	if !errors.As(err, &patternErr) {
		t.Fatalf("GlobPattern(%q).Validate() error type = %T, want *InvalidGlobPatternError", pattern, err)
	}
	if patternErr.Value != pattern {
		t.Fatalf("InvalidGlobPatternError.Value = %q, want %q", patternErr.Value, pattern)
	}
	if !strings.Contains(patternErr.Reason, "invalid syntax") {
		t.Fatalf("InvalidGlobPatternError.Reason = %q, want invalid syntax detail", patternErr.Reason)
	}
	if got := err.Error(); !strings.Contains(got, string(pattern)) || !strings.Contains(got, patternErr.Reason) {
		t.Fatalf("InvalidGlobPatternError.Error() = %q, want pattern and reason", got)
	}
}
