// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestBinaryName_Validate(t *testing.T) {
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
		{"whitespace only", BinaryName(" "), false, true},
		{"tab only", BinaryName("\t"), false, true},
		{"spaces and tabs", BinaryName("  \t  "), false, true},
		{"forward slash", BinaryName("usr/bin/git"), false, true},
		{"backslash", BinaryName("C:\\git"), false, true},
		{"just slash", BinaryName("/"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.binary.Validate()
			if (err == nil) != tt.want {
				t.Errorf("BinaryName(%q).Validate() error = %v, want valid=%v", tt.binary, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("BinaryName(%q).Validate() returned nil, want error", tt.binary)
				}
				if !errors.Is(err, ErrInvalidBinaryName) {
					t.Errorf("error should wrap ErrInvalidBinaryName, got: %v", err)
				}
				var bnErr *InvalidBinaryNameError
				if !errors.As(err, &bnErr) {
					t.Errorf("error should be *InvalidBinaryNameError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("BinaryName(%q).Validate() returned unexpected error: %v", tt.binary, err)
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
