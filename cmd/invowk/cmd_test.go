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

func TestDependencyMessage_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		msg       DependencyMessage
		wantValid bool
		wantErr   bool
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
			err := tt.msg.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("DependencyMessage(%q).Validate() error = %v, wantValid %v", tt.msg, err, tt.wantValid)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DependencyMessage(%q).Validate() returned nil, want error", tt.msg)
				}
				if !errors.Is(err, ErrInvalidDependencyMessage) {
					t.Errorf("error should wrap ErrInvalidDependencyMessage, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("DependencyMessage(%q).Validate() returned unexpected error: %v", tt.msg, err)
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

func TestArgErrType_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		aet       ArgErrType
		wantValid bool
		wantErr   bool
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
			err := tt.aet.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("ArgErrType(%d).Validate() error = %v, wantValid %v", tt.aet, err, tt.wantValid)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ArgErrType(%d).Validate() returned nil, want error", tt.aet)
				}
				if !errors.Is(err, ErrInvalidArgErrType) {
					t.Errorf("error should wrap ErrInvalidArgErrType, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("ArgErrType(%d).Validate() returned unexpected error: %v", tt.aet, err)
			}
		})
	}
}

func TestArgErrType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		aet  ArgErrType
		want string
	}{
		{"missing_required", ArgErrMissingRequired, "missing_required"},
		{"too_many", ArgErrTooMany, "too_many"},
		{"invalid_value", ArgErrInvalidValue, "invalid_value"},
		{"unknown_negative", ArgErrType(-1), "unknown(-1)"},
		{"unknown_out_of_range", ArgErrType(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.aet.String()
			if got != tt.want {
				t.Errorf("ArgErrType(%d).String() = %q, want %q", tt.aet, got, tt.want)
			}
		})
	}
}
