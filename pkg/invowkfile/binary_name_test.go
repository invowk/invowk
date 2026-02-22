// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestBinaryName_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		binary  BinaryName
		want    bool
		wantErr bool
	}{
		{"simple name", BinaryName("git"), true, false},
		{"name with dash", BinaryName("my-tool"), true, false},
		{"name with dot", BinaryName("python3.11"), true, false},
		{"name with underscore", BinaryName("my_tool"), true, false},
		{"empty is invalid", BinaryName(""), false, true},
		{"forward slash", BinaryName("usr/bin/git"), false, true},
		{"backslash", BinaryName("C:\\git"), false, true},
		{"just slash", BinaryName("/"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.binary.IsValid()
			if isValid != tt.want {
				t.Errorf("BinaryName(%q).IsValid() = %v, want %v", tt.binary, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("BinaryName(%q).IsValid() returned no errors, want error", tt.binary)
				}
				if !errors.Is(errs[0], ErrInvalidBinaryName) {
					t.Errorf("error should wrap ErrInvalidBinaryName, got: %v", errs[0])
				}
				var bnErr *InvalidBinaryNameError
				if !errors.As(errs[0], &bnErr) {
					t.Errorf("error should be *InvalidBinaryNameError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("BinaryName(%q).IsValid() returned unexpected errors: %v", tt.binary, errs)
			}
		})
	}
}

func TestBinaryName_String(t *testing.T) {
	t.Parallel()
	b := BinaryName("git")
	if b.String() != "git" {
		t.Errorf("BinaryName.String() = %q, want %q", b.String(), "git")
	}
}
