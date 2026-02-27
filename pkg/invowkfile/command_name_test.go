// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestCommandName_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cn      CommandName
		want    bool
		wantErr bool
	}{
		{"simple", CommandName("build"), true, false},
		{"with_spaces", CommandName("test unit"), true, false},
		{"with_hyphens", CommandName("my-command"), true, false},
		{"single_char", CommandName("a"), true, false},
		{"empty", CommandName(""), false, true},
		{"whitespace_only", CommandName("   "), false, true},
		{"tab_only", CommandName("\t"), false, true},
		{"newline_only", CommandName("\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cn.Validate()
			if (err == nil) != tt.want {
				t.Errorf("CommandName(%q).Validate() error = %v, want valid=%v", tt.cn, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CommandName(%q).Validate() returned nil, want error", tt.cn)
				}
				if !errors.Is(err, ErrInvalidCommandName) {
					t.Errorf("error should wrap ErrInvalidCommandName, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("CommandName(%q).Validate() returned unexpected error: %v", tt.cn, err)
			}
		})
	}
}

func TestCommandName_String(t *testing.T) {
	t.Parallel()
	cn := CommandName("build")
	if cn.String() != "build" {
		t.Errorf("CommandName.String() = %q, want %q", cn.String(), "build")
	}
}
