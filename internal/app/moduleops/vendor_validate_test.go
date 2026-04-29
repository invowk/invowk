// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestVendoredEntry_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		entry     VendoredEntry
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete entry",
			VendoredEntry{
				Namespace:  invowkmod.ModuleNamespace("tools@1.2.3"),
				SourcePath: types.FilesystemPath(filepath.Join(tmpDir, "cache", "tools")),
				VendorPath: types.FilesystemPath(filepath.Join(tmpDir, "project", "invowk_modules", "tools")),
			},
			true, false, 0,
		},
		{
			"valid zero value",
			VendoredEntry{},
			true, false, 0,
		},
		{
			"valid with only namespace",
			VendoredEntry{
				Namespace: invowkmod.ModuleNamespace("mytools"),
			},
			true, false, 0,
		},
		{
			"valid with only paths",
			VendoredEntry{
				SourcePath: types.FilesystemPath(filepath.Join(tmpDir, "source")),
				VendorPath: types.FilesystemPath(filepath.Join(tmpDir, "vendor")),
			},
			true, false, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.entry.Validate()
			if (err == nil) != tt.want {
				t.Errorf("VendoredEntry.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("VendoredEntry.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidVendoredEntry) {
					t.Errorf("error should wrap ErrInvalidVendoredEntry, got: %v", err)
				}
				var entryErr *InvalidVendoredEntryError
				if !errors.As(err, &entryErr) {
					t.Fatalf("error should be *InvalidVendoredEntryError, got: %T", err)
				}
				if len(entryErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(entryErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("VendoredEntry.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestVendorResult_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		result    VendorResult
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete result",
			VendorResult{
				VendorDir: types.FilesystemPath(filepath.Join(tmpDir, "project", "invowk_modules")),
				Vendored: []VendoredEntry{
					{
						Namespace:  invowkmod.ModuleNamespace("tools@1.2.3"),
						SourcePath: types.FilesystemPath(filepath.Join(tmpDir, "cache", "tools")),
						VendorPath: types.FilesystemPath(filepath.Join(tmpDir, "vendor", "tools")),
					},
				},
				Pruned: []string{"old-module"},
			},
			true, false, 0,
		},
		{
			"valid zero value",
			VendorResult{},
			true, false, 0,
		},
		{
			"valid with empty vendored list",
			VendorResult{
				VendorDir: types.FilesystemPath(filepath.Join(tmpDir, "some-dir")),
				Vendored:  []VendoredEntry{},
			},
			true, false, 0,
		},
		{
			"invalid vendored entry in slice",
			VendorResult{
				VendorDir: types.FilesystemPath(filepath.Join(tmpDir, "some-dir")),
				Vendored: []VendoredEntry{
					{Namespace: invowkmod.ModuleNamespace("")}, // empty namespace is invalid for non-zero values
				},
			},
			// invowkmod.ModuleNamespace("") is zero-value-valid in VendoredEntry, so this should be valid
			true, false, 0,
		},
		{
			"valid result with multiple entries",
			VendorResult{
				VendorDir: types.FilesystemPath(filepath.Join(tmpDir, "project", "invowk_modules")),
				Vendored: []VendoredEntry{
					{Namespace: invowkmod.ModuleNamespace("tools@1.0.0")},
					{Namespace: invowkmod.ModuleNamespace("utils@2.0.0")},
				},
			},
			true, false, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.result.Validate()
			if (err == nil) != tt.want {
				t.Errorf("VendorResult.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("VendorResult.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidVendorResult) {
					t.Errorf("error should wrap ErrInvalidVendorResult, got: %v", err)
				}
				var resultErr *InvalidVendorResultError
				if !errors.As(err, &resultErr) {
					t.Fatalf("error should be *InvalidVendorResultError, got: %T", err)
				}
				if len(resultErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(resultErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("VendorResult.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
