// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/app/deps"
)

// Most command behavior tests have been migrated to specialized test files:
// - cmd_runtime_test.go: Platform and runtime tests
// - cmd_flags_test.go: Flag handling and user environment capture tests
// - cmd_args_test.go: Positional argument tests
// - cmd_source_test.go: Source filter tests

func TestDependencyMessage_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		msg       deps.DependencyMessage
		wantValid bool
		wantErr   bool
	}{
		{"non-empty is valid", deps.DependencyMessage("  - kubectl - not found in PATH"), true, false},
		{"short message is valid", deps.DependencyMessage("test"), true, false},
		{"empty is invalid", deps.DependencyMessage(""), false, true},
		{"whitespace only is invalid", deps.DependencyMessage("   "), false, true},
		{"tab only is invalid", deps.DependencyMessage("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.msg.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("deps.DependencyMessage(%q).Validate() error = %v, wantValid %v", tt.msg, err, tt.wantValid)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("deps.DependencyMessage(%q).Validate() returned nil, want error", tt.msg)
				}
				if !errors.Is(err, deps.ErrInvalidDependencyMessage) {
					t.Errorf("error should wrap deps.ErrInvalidDependencyMessage, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("deps.DependencyMessage(%q).Validate() returned unexpected error: %v", tt.msg, err)
			}
		})
	}
}

func TestDependencyMessage_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  deps.DependencyMessage
		want string
	}{
		{"returns value", deps.DependencyMessage("  - kubectl - not found in PATH"), "  - kubectl - not found in PATH"},
		{"empty string", deps.DependencyMessage(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.msg.String(); got != tt.want {
				t.Errorf("deps.DependencyMessage(%q).String() = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestInvalidDependencyMessageError(t *testing.T) {
	t.Parallel()

	err := &deps.InvalidDependencyMessageError{Value: ""}
	if !errors.Is(err, deps.ErrInvalidDependencyMessage) {
		t.Error("InvalidDependencyMessageError should wrap deps.ErrInvalidDependencyMessage")
	}
	if err.Error() != `invalid dependency message: ""` {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestArgErrType_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		aet       deps.ArgErrType
		wantValid bool
		wantErr   bool
	}{
		{"missing_required", deps.ArgErrMissingRequired, true, false},
		{"too_many", deps.ArgErrTooMany, true, false},
		{"invalid_value", deps.ArgErrInvalidValue, true, false},
		{"negative", deps.ArgErrType(-1), false, true},
		{"out_of_range", deps.ArgErrType(99), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.aet.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("deps.ArgErrType(%d).Validate() error = %v, wantValid %v", tt.aet, err, tt.wantValid)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("deps.ArgErrType(%d).Validate() returned nil, want error", tt.aet)
				}
				if !errors.Is(err, deps.ErrInvalidArgErrType) {
					t.Errorf("error should wrap deps.ErrInvalidArgErrType, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("deps.ArgErrType(%d).Validate() returned unexpected error: %v", tt.aet, err)
			}
		})
	}
}

func TestArgErrType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		aet  deps.ArgErrType
		want string
	}{
		{"missing_required", deps.ArgErrMissingRequired, "missing_required"},
		{"too_many", deps.ArgErrTooMany, "too_many"},
		{"invalid_value", deps.ArgErrInvalidValue, "invalid_value"},
		{"unknown_negative", deps.ArgErrType(-1), "unknown(-1)"},
		{"unknown_out_of_range", deps.ArgErrType(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.aet.String()
			if got != tt.want {
				t.Errorf("deps.ArgErrType(%d).String() = %q, want %q", tt.aet, got, tt.want)
			}
		})
	}
}
