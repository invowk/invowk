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

func TestDependencyMessage_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msg     DependencyMessage
		want    bool
		wantErr bool
	}{
		{"non-empty is valid", DependencyMessage("  - kubectl - not found in PATH"), true, false},
		{"short message is valid", DependencyMessage("test"), true, false},
		{"empty is invalid", DependencyMessage(""), false, true},
		{"whitespace only is invalid", DependencyMessage("   "), false, true},
		{"tab only is invalid", DependencyMessage("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.msg.IsValid()
			if isValid != tt.want {
				t.Errorf("DependencyMessage(%q).IsValid() = %v, want %v", tt.msg, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("DependencyMessage(%q).IsValid() returned no errors, want error", tt.msg)
				}
				if !errors.Is(errs[0], ErrInvalidDependencyMessage) {
					t.Errorf("error should wrap ErrInvalidDependencyMessage, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("DependencyMessage(%q).IsValid() returned unexpected errors: %v", tt.msg, errs)
			}
		})
	}
}

func TestDependencyMessage_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  DependencyMessage
		want string
	}{
		{"returns value", DependencyMessage("  - kubectl - not found in PATH"), "  - kubectl - not found in PATH"},
		{"empty string", DependencyMessage(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.msg.String(); got != tt.want {
				t.Errorf("DependencyMessage(%q).String() = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestInvalidDependencyMessageError(t *testing.T) {
	t.Parallel()

	err := &InvalidDependencyMessageError{Value: ""}
	if !errors.Is(err, ErrInvalidDependencyMessage) {
		t.Error("InvalidDependencyMessageError should wrap ErrInvalidDependencyMessage")
	}
	if err.Error() != `invalid dependency message: ""` {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

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
