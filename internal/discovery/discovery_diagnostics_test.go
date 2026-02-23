// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
)

func TestDiscoverCommandSet_DiagnosticsForInvalidIncludePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invalidInclude := filepath.Join(tmpDir, "not-a-module")
	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(invalidInclude)},
	}

	d := newTestDiscovery(t, cfg, tmpDir)
	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}

	if !containsDiagnostic(result.Diagnostics, "include_not_module", invalidInclude) {
		t.Fatalf("expected include_not_module diagnostic for %s, got: %#v", invalidInclude, result.Diagnostics)
	}
}

func TestDiscoverCommandSet_DiagnosticsForReservedIncludeModuleName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	reservedModulePath := filepath.Join(tmpDir, "invowkfile.invowkmod")
	if err := os.MkdirAll(reservedModulePath, 0o755); err != nil {
		t.Fatalf("failed to create reserved module path: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(reservedModulePath)},
	}

	d := newTestDiscovery(t, cfg, tmpDir)
	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}

	if !containsDiagnostic(result.Diagnostics, "include_reserved_module_skipped", reservedModulePath) {
		t.Fatalf("expected include_reserved_module_skipped diagnostic for %s, got: %#v", reservedModulePath, result.Diagnostics)
	}
}

func TestDiscoverCommandSet_DiagnosticsForInvalidIncludedModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	modulePath := filepath.Join(tmpDir, "broken.invowkmod")
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		t.Fatalf("failed to create module path: %v", err)
	}

	// Intentionally invalid CUE to force invowkmod.Load failure.
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte("module: ["), 0o644); err != nil {
		t.Fatalf("failed to write invalid invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkfile.cue"), []byte("cmds: []"), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(modulePath)},
	}

	d := newTestDiscovery(t, cfg, tmpDir)
	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}

	if !containsDiagnostic(result.Diagnostics, "include_module_load_failed", modulePath) {
		t.Fatalf("expected include_module_load_failed diagnostic for %s, got: %#v", modulePath, result.Diagnostics)
	}
}

func containsDiagnostic(diags []Diagnostic, code DiagnosticCode, path string) bool {
	for _, diag := range diags {
		if diag.Code == code && string(diag.Path) == path {
			return true
		}
	}

	return false
}
