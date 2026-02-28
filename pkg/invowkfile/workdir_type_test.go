// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestWorkDir_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		workdir WorkDir
		want    bool
		wantErr bool
	}{
		{"empty is valid (inherit)", WorkDir(""), true, false},
		{"relative path", WorkDir("build/output"), true, false},
		{"absolute path", WorkDir("/tmp/work"), true, false},
		{"forward slashes", WorkDir("src/main"), true, false},
		{"single dot", WorkDir("."), true, false},
		{"space only", WorkDir(" "), false, true},
		{"tab only", WorkDir("\t"), false, true},
		{"multiple spaces", WorkDir("   "), false, true},
		{"newline only", WorkDir("\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.workdir.Validate()
			if (err == nil) != tt.want {
				t.Errorf("WorkDir(%q).Validate() error = %v, want valid=%v", tt.workdir, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("WorkDir(%q).Validate() returned nil, want error", tt.workdir)
				}
				if !errors.Is(err, ErrInvalidWorkDir) {
					t.Errorf("error should wrap ErrInvalidWorkDir, got: %v", err)
				}
				var wdErr *InvalidWorkDirError
				if !errors.As(err, &wdErr) {
					t.Errorf("error should be *InvalidWorkDirError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("WorkDir(%q).Validate() returned unexpected error: %v", tt.workdir, err)
			}
		})
	}
}

func TestWorkDir_String(t *testing.T) {
	t.Parallel()
	w := WorkDir("build/output")
	if w.String() != "build/output" {
		t.Errorf("WorkDir.String() = %q, want %q", w.String(), "build/output")
	}
}

func TestShellPath_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		shell   ShellPath
		want    bool
		wantErr bool
	}{
		{"empty is valid (system default)", ShellPath(""), true, false},
		{"absolute path", ShellPath("/bin/bash"), true, false},
		{"relative path", ShellPath("bash"), true, false},
		{"windows path", ShellPath("C:\\Windows\\System32\\cmd.exe"), true, false},
		{"space only", ShellPath(" "), false, true},
		{"tab only", ShellPath("\t"), false, true},
		{"multiple spaces", ShellPath("   "), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.shell.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ShellPath(%q).Validate() error = %v, want valid=%v", tt.shell, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ShellPath(%q).Validate() returned nil, want error", tt.shell)
				}
				if !errors.Is(err, ErrInvalidShellPath) {
					t.Errorf("error should wrap ErrInvalidShellPath, got: %v", err)
				}
				var spErr *InvalidShellPathError
				if !errors.As(err, &spErr) {
					t.Errorf("error should be *InvalidShellPathError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ShellPath(%q).Validate() returned unexpected error: %v", tt.shell, err)
			}
		})
	}
}

func TestShellPath_String(t *testing.T) {
	t.Parallel()
	s := ShellPath("/bin/bash")
	if s.String() != "/bin/bash" {
		t.Errorf("ShellPath.String() = %q, want %q", s.String(), "/bin/bash")
	}
}
