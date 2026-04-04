// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadScriptFileContent_BoundaryCheck(t *testing.T) {
	t.Parallel()

	// Create a module directory with a legitimate script inside it.
	moduleDir := t.TempDir()
	scriptContent := "#!/bin/sh\necho hello"
	if err := os.WriteFile(filepath.Join(moduleDir, "run.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Create a sensitive file outside the module boundary.
	outsideDir := t.TempDir()
	sensitiveContent := "SECRET=hunter2"
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.env"), []byte(sensitiveContent), 0o644); err != nil {
		t.Fatalf("failed to write sensitive file: %v", err)
	}

	// Compute a traversal path that escapes the module directory.
	relToSensitive, err := filepath.Rel(moduleDir, filepath.Join(outsideDir, "secret.env"))
	if err != nil {
		t.Fatalf("failed to compute relative path: %v", err)
	}

	tests := []struct {
		name       string
		scriptPath string
		modulePath string
		want       string
	}{
		{
			name:       "legitimate script within module",
			scriptPath: "run.sh",
			modulePath: moduleDir,
			want:       scriptContent,
		},
		{
			name:       "traversal path blocked by boundary check",
			scriptPath: relToSensitive,
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "explicit dotdot traversal blocked",
			scriptPath: "../../etc/passwd",
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "absolute path without module context",
			scriptPath: filepath.Join(moduleDir, "run.sh"),
			modulePath: "",
			want:       scriptContent,
		},
		{
			name:       "nonexistent file returns empty",
			scriptPath: "nonexistent.sh",
			modulePath: moduleDir,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := readScriptFileContent(tt.scriptPath, tt.modulePath)
			if got != tt.want {
				t.Errorf("readScriptFileContent(%q, %q) = %q, want %q",
					tt.scriptPath, tt.modulePath, got, tt.want)
			}
		})
	}
}
