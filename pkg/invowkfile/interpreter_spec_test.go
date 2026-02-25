// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestInterpreterSpec_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    InterpreterSpec
		want    bool
		wantErr bool
	}{
		{"auto keyword", InterpreterSpec("auto"), true, false},
		{"python3", InterpreterSpec("python3"), true, false},
		{"absolute path", InterpreterSpec("/usr/bin/bash"), true, false},
		{"with args style", InterpreterSpec("node"), true, false},
		{"empty is valid (zero value = auto)", InterpreterSpec(""), true, false},
		{"whitespace only is invalid", InterpreterSpec("   "), false, true},
		{"tab only is invalid", InterpreterSpec("\t"), false, true},
		{"newline only is invalid", InterpreterSpec("\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.spec.IsValid()
			if isValid != tt.want {
				t.Errorf("InterpreterSpec(%q).IsValid() = %v, want %v", tt.spec, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("InterpreterSpec(%q).IsValid() returned no errors, want error", tt.spec)
				}
				if !errors.Is(errs[0], ErrInvalidInterpreterSpec) {
					t.Errorf("error should wrap ErrInvalidInterpreterSpec, got: %v", errs[0])
				}
				var isErr *InvalidInterpreterSpecError
				if !errors.As(errs[0], &isErr) {
					t.Errorf("error should be *InvalidInterpreterSpecError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("InterpreterSpec(%q).IsValid() returned unexpected errors: %v", tt.spec, errs)
			}
		})
	}
}

func TestInterpreterSpec_String(t *testing.T) {
	t.Parallel()
	s := InterpreterSpec("python3")
	if s.String() != "python3" {
		t.Errorf("InterpreterSpec.String() = %q, want %q", s.String(), "python3")
	}
}
