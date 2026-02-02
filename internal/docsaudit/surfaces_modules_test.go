// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverModuleSurfaces(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "modules", "com.example.test.invkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module dir: %v", err)
	}

	invkmodPath := filepath.Join(moduleDir, "invkmod.cue")
	invkmodContent := []byte("module: \"com.example.test\"\n")
	if err := os.WriteFile(invkmodPath, invkmodContent, 0o644); err != nil {
		t.Fatalf("write invkmod.cue: %v", err)
	}

	opts := Options{
		RepoRoot:       root,
		OutputPath:     filepath.Join(root, "out.md"),
		OutputFormat:   OutputFormatHuman,
		ExcludePkgAPIs: true,
	}

	surfaces, err := DiscoverModuleSurfaces(opts)
	if err != nil {
		t.Fatalf("DiscoverModuleSurfaces: %v", err)
	}

	if len(surfaces) != 1 {
		t.Fatalf("expected 1 surface, got %d", len(surfaces))
	}
	if surfaces[0].Name != "com.example.test" {
		t.Fatalf("unexpected module name: %s", surfaces[0].Name)
	}
}
