// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestCheckName_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		check   CheckName
		want    bool
		wantErr bool
	}{
		{"simple name", CheckName("docker"), true, false},
		{"name with spaces", CheckName("docker installed"), true, false},
		{"name with special chars", CheckName("check-v2.1"), true, false},
		{"single char", CheckName("x"), true, false},
		{"empty is invalid", CheckName(""), false, true},
		{"whitespace only is invalid", CheckName("   "), false, true},
		{"tab only is invalid", CheckName("\t"), false, true},
		{"newline only is invalid", CheckName("\n"), false, true},
		{"mixed whitespace is invalid", CheckName(" \t\n "), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.check.IsValid()
			if isValid != tt.want {
				t.Errorf("CheckName(%q).IsValid() = %v, want %v", tt.check, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("CheckName(%q).IsValid() returned no errors, want error", tt.check)
				}
				if !errors.Is(errs[0], ErrInvalidCheckName) {
					t.Errorf("error should wrap ErrInvalidCheckName, got: %v", errs[0])
				}
				var cnErr *InvalidCheckNameError
				if !errors.As(errs[0], &cnErr) {
					t.Errorf("error should be *InvalidCheckNameError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("CheckName(%q).IsValid() returned unexpected errors: %v", tt.check, errs)
			}
		})
	}
}

func TestCheckName_String(t *testing.T) {
	t.Parallel()
	c := CheckName("docker")
	if c.String() != "docker" {
		t.Errorf("CheckName.String() = %q, want %q", c.String(), "docker")
	}
}

func TestScriptContent_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		script  ScriptContent
		want    bool
		wantErr bool
	}{
		{"valid script", ScriptContent("echo hello"), true, false},
		{"multiline script", ScriptContent("#!/bin/bash\necho hello"), true, false},
		{"empty is valid (zero value)", ScriptContent(""), true, false},
		{"whitespace only is invalid", ScriptContent("   "), false, true},
		{"tab only is invalid", ScriptContent("\t"), false, true},
		{"newline only is invalid", ScriptContent("\n"), false, true},
		{"mixed whitespace is invalid", ScriptContent(" \t\n "), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.script.IsValid()
			if isValid != tt.want {
				t.Errorf("ScriptContent(%q).IsValid() = %v, want %v", tt.script, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ScriptContent(%q).IsValid() returned no errors, want error", tt.script)
				}
				if !errors.Is(errs[0], ErrInvalidScriptContent) {
					t.Errorf("error should wrap ErrInvalidScriptContent, got: %v", errs[0])
				}
				var scErr *InvalidScriptContentError
				if !errors.As(errs[0], &scErr) {
					t.Errorf("error should be *InvalidScriptContentError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ScriptContent(%q).IsValid() returned unexpected errors: %v", tt.script, errs)
			}
		})
	}
}

func TestScriptContent_String(t *testing.T) {
	t.Parallel()
	s := ScriptContent("echo hello")
	if s.String() != "echo hello" {
		t.Errorf("ScriptContent.String() = %q, want %q", s.String(), "echo hello")
	}
}
