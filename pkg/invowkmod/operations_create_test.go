// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      CreateOptions
		expectErr bool
		validate  func(t *testing.T, modulePath string)
	}{
		{
			name: "create simple module",
			opts: CreateOptions{
				Name: "mycommands",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				// Check module directory exists
				info, err := os.Stat(modulePath)
				if err != nil {
					t.Fatalf("module directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("module path is not a directory")
				}

				// Check invowkmod.cue exists (required)
				invowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
				if _, statErr := os.Stat(invowkmodPath); statErr != nil {
					t.Errorf("invowkmod.cue not created: %v", statErr)
				}

				// Check invowkfile.cue exists
				invowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
				if _, statErr := os.Stat(invowkfilePath); statErr != nil {
					t.Errorf("invowkfile.cue not created: %v", statErr)
				}

				// Verify module is valid
				_, err = Load(types.FilesystemPath(modulePath))
				if err != nil {
					t.Errorf("created module is not valid: %v", err)
				}
			},
		},
		{
			name: "create RDNS module",
			opts: CreateOptions{
				Name: "com.example.mytools",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				if !strings.HasSuffix(modulePath, "com.example.mytools.invowkmod") {
					t.Errorf("unexpected module path: %s", modulePath)
				}
				// Verify invowkmod.cue contains correct module ID
				content, err := os.ReadFile(filepath.Join(modulePath, "invowkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invowkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `module: "com.example.mytools"`) {
					t.Error("module ID not set correctly in invowkmod.cue")
				}
			},
		},
		{
			name: "create module with scripts directory",
			opts: CreateOptions{
				Name:             "mytools",
				CreateScriptsDir: true,
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				scriptsDir := filepath.Join(modulePath, "scripts")
				info, err := os.Stat(scriptsDir)
				if err != nil {
					t.Fatalf("scripts directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("scripts path is not a directory")
				}

				// Check .gitkeep exists
				gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
				if _, err := os.Stat(gitkeepPath); err != nil {
					t.Errorf(".gitkeep not created: %v", err)
				}
			},
		},
		{
			name: "create module with custom module identifier",
			opts: CreateOptions{
				Name:   "mytools",
				Module: "custom.module",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				// Custom module ID should be in invowkmod.cue (not invowkfile.cue)
				content, err := os.ReadFile(filepath.Join(modulePath, "invowkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invowkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `module: "custom.module"`) {
					t.Error("custom module not set in invowkmod.cue")
				}
			},
		},
		{
			name: "create module with custom description",
			opts: CreateOptions{
				Name:        "mytools",
				Description: "My custom description",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				// Description should be in invowkmod.cue (not invowkfile.cue)
				content, err := os.ReadFile(filepath.Join(modulePath, "invowkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invowkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `description: "My custom description"`) {
					t.Error("custom description not set in invowkmod.cue")
				}
			},
		},
		{
			name: "empty name fails",
			opts: CreateOptions{
				Name: "",
			},
			expectErr: true,
		},
		{
			name: "invalid name fails",
			opts: CreateOptions{
				Name: "123invalid",
			},
			expectErr: true,
		},
		{
			name: "name with hyphen fails",
			opts: CreateOptions{
				Name: "my-commands",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Use temp directory as parent
			tmpDir := t.TempDir()
			opts := tt.opts
			opts.ParentDir = types.FilesystemPath(tmpDir)

			modulePath, err := Create(opts)
			if tt.expectErr {
				if err == nil {
					t.Error("Create() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Create() unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, modulePath)
			}
		})
	}
}

func TestCreate_ExistingModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create module first time
	opts := CreateOptions{
		Name:      "mytools",
		ParentDir: types.FilesystemPath(tmpDir),
	}

	_, err := Create(opts)
	if err != nil {
		t.Fatalf("first Create() failed: %v", err)
	}

	// Try to create again - should fail
	_, err = Create(opts)
	if err == nil {
		t.Error("Create() expected error for existing module, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}
