// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestNewModuleScaffold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      CreateOptions
		expectErr bool
		validate  func(t *testing.T, scaffold ModuleScaffold)
	}{
		{
			name: "simple module",
			opts: CreateOptions{Name: "mycommands"},
			validate: func(t *testing.T, scaffold ModuleScaffold) {
				t.Helper()
				if scaffold.DirectoryName() != "mycommands.invowkmod" {
					t.Fatalf("DirectoryName = %q, want mycommands.invowkmod", scaffold.DirectoryName())
				}
				if !strings.Contains(scaffold.InvowkmodContent().String(), `module: "mycommands"`) {
					t.Fatalf("InvowkmodContent missing default module ID: %s", scaffold.InvowkmodContent())
				}
				if !strings.Contains(scaffold.InvowkfileContent().String(), "Hello from mycommands!") {
					t.Fatalf("InvowkfileContent missing sample command: %s", scaffold.InvowkfileContent())
				}
			},
		},
		{
			name: "custom metadata",
			opts: CreateOptions{
				Name:             "com.example.mytools",
				Module:           "custom.module",
				Description:      "My custom description",
				CreateScriptsDir: true,
			},
			validate: func(t *testing.T, scaffold ModuleScaffold) {
				t.Helper()
				if scaffold.DirectoryName() != "com.example.mytools.invowkmod" {
					t.Fatalf("DirectoryName = %q, want com.example.mytools.invowkmod", scaffold.DirectoryName())
				}
				if !strings.Contains(scaffold.InvowkmodContent().String(), `module: "custom.module"`) {
					t.Fatalf("InvowkmodContent missing custom module ID: %s", scaffold.InvowkmodContent())
				}
				if !strings.Contains(scaffold.InvowkmodContent().String(), `description: "My custom description"`) {
					t.Fatalf("InvowkmodContent missing custom description: %s", scaffold.InvowkmodContent())
				}
				if !scaffold.CreateScriptsDir() {
					t.Fatal("CreateScriptsDir = false, want true")
				}
			},
		},
		{
			name:      "empty name fails",
			opts:      CreateOptions{Name: ""},
			expectErr: true,
		},
		{
			name:      "invalid name fails",
			opts:      CreateOptions{Name: "123invalid"},
			expectErr: true,
		},
		{
			name:      "name with hyphen fails",
			opts:      CreateOptions{Name: "my-commands"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scaffold, err := NewModuleScaffold(tt.opts)
			if tt.expectErr {
				if err == nil {
					t.Fatal("NewModuleScaffold() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewModuleScaffold() error = %v", err)
			}
			tt.validate(t, scaffold)
		})
	}
}

func TestNewModuleScaffoldRejectsInvalidOptionsBeforeScaffoldWork(t *testing.T) {
	t.Parallel()

	_, err := NewModuleScaffold(CreateOptions{
		ParentDir:   types.FilesystemPath(t.TempDir()),
		Name:        ModuleShortName("valid"),
		Description: types.DescriptionText("   "),
	})
	if err == nil {
		t.Fatal("NewModuleScaffold() returned nil error, want invalid create options")
	}
	if !errors.Is(err, ErrInvalidCreateOptions) {
		t.Fatalf("NewModuleScaffold() error = %v, want ErrInvalidCreateOptions", err)
	}
}
