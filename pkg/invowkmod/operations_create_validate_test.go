// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestCreateOptions_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		opts      CreateOptions
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete options",
			CreateOptions{
				Name:        ModuleDirectoryName("mymodule"),
				ParentDir:   types.FilesystemPath(filepath.Join(tmpDir, "projects")),
				Module:      ModuleID("io.example.mymodule"),
				Description: types.DescriptionText("A test module"),
			},
			true, false, 0,
		},
		{
			"valid zero value",
			CreateOptions{},
			true, false, 0,
		},
		{
			"valid minimal (only name)",
			CreateOptions{
				Name: ModuleDirectoryName("mymodule"),
			},
			true, false, 0,
		},
		{
			"valid with empty optional fields",
			CreateOptions{
				Name:      ModuleDirectoryName("mymodule"),
				ParentDir: types.FilesystemPath(filepath.Join(tmpDir, "some-dir")),
			},
			true, false, 0,
		},
		{
			"invalid name (starts with digit)",
			CreateOptions{
				Name: ModuleDirectoryName("1invalid"),
			},
			false, true, 1,
		},
		{
			"invalid module ID",
			CreateOptions{
				Name:   ModuleDirectoryName("mymodule"),
				Module: ModuleID("1bad"),
			},
			false, true, 1,
		},
		{
			"invalid description (whitespace-only)",
			CreateOptions{
				Name:        ModuleDirectoryName("mymodule"),
				Description: types.DescriptionText("   "),
			},
			false, true, 1,
		},
		{
			"invalid parent directory",
			CreateOptions{
				Name:      ModuleDirectoryName("mymodule"),
				ParentDir: types.FilesystemPath("   "),
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			CreateOptions{
				Name:        ModuleDirectoryName("1invalid"),
				Module:      ModuleID("1bad"),
				Description: types.DescriptionText("   "),
			},
			false, true, 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.Validate()
			if (err == nil) != tt.want {
				t.Errorf("CreateOptions.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CreateOptions.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidCreateOptions) {
					t.Errorf("error should wrap ErrInvalidCreateOptions, got: %v", err)
				}
				var optsErr *InvalidCreateOptionsError
				if !errors.As(err, &optsErr) {
					t.Fatalf("error should be *InvalidCreateOptionsError, got: %T", err)
				}
				if len(optsErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(optsErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("CreateOptions.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
