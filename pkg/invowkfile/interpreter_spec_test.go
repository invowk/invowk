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

func TestInterpreterSpecMutationValidationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "invalid whitespace preserves value", run: testInterpreterSpecInvalidWhitespacePreservesValue},
		{name: "env path forms", run: testInterpreterSpecEnvPathForms},
		{name: "unsafe env diagnostics", run: testInterpreterSpecUnsafeEnvDiagnostics},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testInterpreterSpecInvalidWhitespacePreservesValue(t *testing.T) {
	t.Helper()

	spec := InterpreterSpec(" \t ")
	err := spec.Validate()
	if !errors.Is(err, ErrInvalidInterpreterSpec) {
		t.Fatalf("InterpreterSpec.Validate() error = %v, want ErrInvalidInterpreterSpec", err)
	}
	var invalidErr *InvalidInterpreterSpecError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("InterpreterSpec.Validate() error type = %T, want *InvalidInterpreterSpecError", err)
	}
	if invalidErr.Value != spec {
		t.Fatalf("InvalidInterpreterSpecError.Value = %q, want %q", invalidErr.Value, spec)
	}
	if got, want := err.Error(), `invalid interpreter spec " \t ": non-empty value must not be whitespace-only`; got != want {
		t.Fatalf("InvalidInterpreterSpecError.Error() = %q, want %q", got, want)
	}
}

func testInterpreterSpecEnvPathForms(t *testing.T) {
	t.Helper()

	validSpecs := []InterpreterSpec{
		"python3.exe",
		"/usr/bin/env python3",
		"/bin/env python3",
		"/usr/bin/env python3.exe",
	}
	for _, spec := range validSpecs {
		if err := spec.Validate(); err != nil {
			t.Fatalf("InterpreterSpec(%q).Validate() error = %v, want nil", spec, err)
		}
	}
}

func testInterpreterSpecUnsafeEnvDiagnostics(t *testing.T) {
	t.Helper()

	tests := []struct {
		spec       InterpreterSpec
		wantReason string
	}{
		{
			spec:       "env python3",
			wantReason: "bare 'env' requires full path (/usr/bin/env or /bin/env)",
		},
		{
			spec:       "/usr/bin/env",
			wantReason: "env requires an interpreter argument",
		},
	}

	for _, tt := range tests {
		requireUnsafeInterpreterSpecError(t, tt.spec, tt.wantReason)
	}
}

func requireUnsafeInterpreterSpecError(t *testing.T, spec InterpreterSpec, wantReason string) {
	t.Helper()

	err := spec.Validate()
	if !errors.Is(err, ErrUnsafeInterpreterSpec) {
		t.Fatalf("InterpreterSpec(%q).Validate() error = %v, want ErrUnsafeInterpreterSpec", spec, err)
	}
	var unsafeErr *UnsafeInterpreterSpecError
	if !errors.As(err, &unsafeErr) {
		t.Fatalf("InterpreterSpec(%q).Validate() error type = %T, want *UnsafeInterpreterSpecError", spec, err)
	}
	if unsafeErr.Value != spec {
		t.Fatalf("UnsafeInterpreterSpecError.Value = %q, want %q", unsafeErr.Value, spec)
	}
	if unsafeErr.Reason != wantReason {
		t.Fatalf("UnsafeInterpreterSpecError.Reason = %q, want %q", unsafeErr.Reason, wantReason)
	}
}
