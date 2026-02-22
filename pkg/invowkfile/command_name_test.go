// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestCommandName_IsValid(t *testing.T) {
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
			isValid, errs := tt.cn.IsValid()
			if isValid != tt.want {
				t.Errorf("CommandName(%q).IsValid() = %v, want %v", tt.cn, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("CommandName(%q).IsValid() returned no errors, want error", tt.cn)
				}
				if !errors.Is(errs[0], ErrInvalidCommandName) {
					t.Errorf("error should wrap ErrInvalidCommandName, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("CommandName(%q).IsValid() returned unexpected errors: %v", tt.cn, errs)
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
