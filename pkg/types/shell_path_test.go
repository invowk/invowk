// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestShellPath_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    ShellPath
		wantErr bool
	}{
		{name: "empty allowed as zero value", path: "", wantErr: false},
		{name: "absolute path", path: "/bin/sh", wantErr: false},
		{name: "command name", path: "sh", wantErr: false},
		{name: "whitespace rejected", path: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.path.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidShellPath) {
					t.Fatalf("error should wrap ErrInvalidShellPath, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestShellPath_String(t *testing.T) {
	t.Parallel()

	if got := ShellPath("/bin/bash").String(); got != "/bin/bash" {
		t.Fatalf("String() = %q", got)
	}
}
