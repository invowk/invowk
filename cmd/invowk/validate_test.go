// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"
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

// containsSuffix checks if path ends with the given suffix (using filepath separators).
func containsSuffix(fullPath, suffix string) bool {
	// Normalize both to forward-slash for comparison.
	return filepath.ToSlash(fullPath[len(fullPath)-len(suffix):]) == filepath.ToSlash(suffix)
}
