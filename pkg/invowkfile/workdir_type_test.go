// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestWorkDir_IsValid(t *testing.T) {
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
			isValid, errs := tt.workdir.IsValid()
			if isValid != tt.want {
				t.Errorf("WorkDir(%q).IsValid() = %v, want %v", tt.workdir, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("WorkDir(%q).IsValid() returned no errors, want error", tt.workdir)
				}
				if !errors.Is(errs[0], ErrInvalidWorkDir) {
					t.Errorf("error should wrap ErrInvalidWorkDir, got: %v", errs[0])
				}
				var wdErr *InvalidWorkDirError
				if !errors.As(errs[0], &wdErr) {
					t.Errorf("error should be *InvalidWorkDirError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("WorkDir(%q).IsValid() returned unexpected errors: %v", tt.workdir, errs)
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

func TestShellPath_IsValid(t *testing.T) {
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
			isValid, errs := tt.shell.IsValid()
			if isValid != tt.want {
				t.Errorf("ShellPath(%q).IsValid() = %v, want %v", tt.shell, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ShellPath(%q).IsValid() returned no errors, want error", tt.shell)
				}
				if !errors.Is(errs[0], ErrInvalidShellPath) {
					t.Errorf("error should wrap ErrInvalidShellPath, got: %v", errs[0])
				}
				var spErr *InvalidShellPathError
				if !errors.As(errs[0], &spErr) {
					t.Errorf("error should be *InvalidShellPathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ShellPath(%q).IsValid() returned unexpected errors: %v", tt.shell, errs)
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
