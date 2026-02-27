// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestCheckName_Validate(t *testing.T) {
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
			err := tt.check.Validate()
			if (err == nil) != tt.want {
				t.Errorf("CheckName(%q).Validate() error = %v, want valid=%v", tt.check, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CheckName(%q).Validate() returned nil, want error", tt.check)
				}
				if !errors.Is(err, ErrInvalidCheckName) {
					t.Errorf("error should wrap ErrInvalidCheckName, got: %v", err)
				}
				var cnErr *InvalidCheckNameError
				if !errors.As(err, &cnErr) {
					t.Errorf("error should be *InvalidCheckNameError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("CheckName(%q).Validate() returned unexpected error: %v", tt.check, err)
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

func TestScriptContent_Validate(t *testing.T) {
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
			err := tt.script.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ScriptContent(%q).Validate() error = %v, want valid=%v", tt.script, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ScriptContent(%q).Validate() returned nil, want error", tt.script)
				}
				if !errors.Is(err, ErrInvalidScriptContent) {
					t.Errorf("error should wrap ErrInvalidScriptContent, got: %v", err)
				}
				var scErr *InvalidScriptContentError
				if !errors.As(err, &scErr) {
					t.Errorf("error should be *InvalidScriptContentError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ScriptContent(%q).Validate() returned unexpected error: %v", tt.script, err)
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
