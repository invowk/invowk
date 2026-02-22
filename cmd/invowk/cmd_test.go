// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"testing"
)

// All tests from this file have been migrated to specialized test files:
// - cmd_deps_test.go: Tool, command, filepath, capability, env var dependency tests
// - cmd_deps_caps_env_test.go: Capability and environment variable dependency tests
// - cmd_runtime_test.go: Platform and runtime tests
// - cmd_flags_test.go: Flag handling and user environment capture tests
// - cmd_args_test.go: Positional argument tests
// - cmd_source_test.go: Source filter tests

func TestArgErrType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		aet     ArgErrType
		want    bool
		wantErr bool
	}{
		{"missing_required", ArgErrMissingRequired, true, false},
		{"too_many", ArgErrTooMany, true, false},
		{"invalid_value", ArgErrInvalidValue, true, false},
		{"negative", ArgErrType(-1), false, true},
		{"out_of_range", ArgErrType(99), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.aet.IsValid()
			if isValid != tt.want {
				t.Errorf("ArgErrType(%d).IsValid() = %v, want %v", tt.aet, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ArgErrType(%d).IsValid() returned no errors, want error", tt.aet)
				}
				if !errors.Is(errs[0], ErrInvalidArgErrType) {
					t.Errorf("error should wrap ErrInvalidArgErrType, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ArgErrType(%d).IsValid() returned unexpected errors: %v", tt.aet, errs)
			}
		})
	}
}
