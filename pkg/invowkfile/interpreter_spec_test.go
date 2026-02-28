// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestInterpreterSpec_Validate(t *testing.T) {
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
			err := tt.spec.Validate()
			if (err == nil) != tt.want {
				t.Errorf("InterpreterSpec(%q).Validate() error = %v, want valid=%v", tt.spec, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("InterpreterSpec(%q).Validate() returned nil, want error", tt.spec)
				}
				if !errors.Is(err, ErrInvalidInterpreterSpec) {
					t.Errorf("error should wrap ErrInvalidInterpreterSpec, got: %v", err)
				}
				var isErr *InvalidInterpreterSpecError
				if !errors.As(err, &isErr) {
					t.Errorf("error should be *InvalidInterpreterSpecError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("InterpreterSpec(%q).Validate() returned unexpected error: %v", tt.spec, err)
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
