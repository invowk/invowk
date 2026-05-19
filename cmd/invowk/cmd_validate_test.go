// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDetectPathType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setup        func(t *testing.T, dir string) string // returns absPath to test
		wantType     pathType
		wantResolved string // expected suffix of resolved path ("" = same as input)
	}{
		{
			name: "invowkmod directory",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				modDir := filepath.Join(dir, "mymod.invowkmod")
				if err := os.MkdirAll(modDir, 0o755); err != nil {
					t.Fatal(err)
				}
				return modDir
			},
			wantType: pathTypeModule,
		},
		{
			name: "invowkfile.cue file",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				p := filepath.Join(dir, "invowkfile.cue")
				if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
				return p
			},
			wantType: pathTypeInvowkfile,
		},
		{
			name: "invowkmod.cue file resolves to parent",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				modDir := filepath.Join(dir, "test.invowkmod")
				if err := os.MkdirAll(modDir, 0o755); err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(modDir, "invowkmod.cue")
				if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
				return p
			},
			wantType:     pathTypeModule,
			wantResolved: "test.invowkmod",
		},
		{
			name: "module invowkfile.cue resolves to parent module",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				modDir := filepath.Join(dir, "test.invowkmod")
				if err := os.MkdirAll(modDir, 0o755); err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(modDir, "invowkfile.cue")
				if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
				return p
			},
			wantType:     pathTypeModule,
			wantResolved: "test.invowkmod",
		},
		{
			name: "directory containing invowkfile.cue",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				subDir := filepath.Join(dir, "myproject")
				if err := os.MkdirAll(subDir, 0o755); err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(subDir, "invowkfile.cue")
				if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
				return subDir
			},
			wantType:     pathTypeInvowkfile,
			wantResolved: filepath.Join("myproject", "invowkfile.cue"),
		},
		{
			name: "unknown path type",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				p := filepath.Join(dir, "random.txt")
				if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
					t.Fatal(err)
				}
				return p
			},
			wantType: pathTypeUnknown,
		},
		{
			name: "empty directory (unknown)",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				subDir := filepath.Join(dir, "emptydir")
				if err := os.MkdirAll(subDir, 0o755); err != nil {
					t.Fatal(err)
				}
				return subDir
			},
			wantType: pathTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			absPath := tt.setup(t, dir)
			gotType, gotResolved := detectPathType(absPath)

			if gotType != tt.wantType {
				t.Errorf("detectPathType(%q) type = %d, want %d", absPath, gotType, tt.wantType)
			}

			if tt.wantResolved != "" {
				if !filepath.IsAbs(gotResolved) {
					t.Errorf("detectPathType(%q) resolved = %q, expected absolute path", absPath, gotResolved)
				}
				if !containsSuffix(gotResolved, tt.wantResolved) {
					t.Errorf("detectPathType(%q) resolved = %q, want suffix %q", absPath, gotResolved, tt.wantResolved)
				}
			}
		})
	}
}

func TestPathType_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     pathType
		wantValid bool
		wantErr   bool
	}{
		{"unknown", pathTypeUnknown, true, false},
		{"invowkfile", pathTypeInvowkfile, true, false},
		{"module", pathTypeModule, true, false},
		{"out of range positive", pathType(99), false, true},
		{"negative", pathType(-1), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("pathType(%d).validate() error = %v, wantValid %v", tt.value, err, tt.wantValid)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("expected validation error, got nil")
				} else if !errors.Is(err, errInvalidPathType) {
					t.Errorf("expected errors.Is(err, errInvalidPathType), got %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestPathType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value pathType
		want  string
	}{
		{"unknown", pathTypeUnknown, "unknown"},
		{"invowkfile", pathTypeInvowkfile, "invowkfile"},
		{"module", pathTypeModule, "module"},
		{"out_of_range", pathType(99), "unknown(99)"},
		{"negative", pathType(-1), "unknown(-1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.value.String()
			if got != tt.want {
				t.Errorf("pathType(%d).String() = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestRunPathValidationAcceptsDirectModuleInvowkfileScriptFile(t *testing.T) {
	t.Parallel()

	modulePath := filepath.Join(t.TempDir(), "com.example.tools.invowkmod")
	if err := os.MkdirAll(filepath.Join(modulePath, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte(`module: "com.example.tools"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "scripts", "hello.sh"), []byte("echo hello\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkfile.cue"), []byte(`cmds: [{
	name: "hello"
	implementations: [{
		script: {file: "scripts/hello.sh"}
		runtimes: [{name: "virtual"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	targetPath := filepath.Join(modulePath, "invowkfile.cue")
	if err := runPathValidation(cmd, targetPath); err != nil {
		t.Fatalf("runPathValidation() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Module is valid") {
		t.Fatalf("stdout = %q, want module-valid message", stdout.String())
	}
}

// containsSuffix checks if path ends with the given suffix (using filepath separators).
func containsSuffix(fullPath, suffix string) bool {
	if len(suffix) > len(fullPath) {
		return false
	}
	// Normalize both to forward-slash for comparison.
	return filepath.ToSlash(fullPath[len(fullPath)-len(suffix):]) == filepath.ToSlash(suffix)
}
