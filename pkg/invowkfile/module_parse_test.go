// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseModuleUsesInvowkmodLoadValidation(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "wrong-name.invowkmod")
	if err := os.Mkdir(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	writeParseModuleFile(t, filepath.Join(moduleDir, "invowkmod.cue"), `module: "io.example.actual"
version: "1.0.0"
`)

	_, err := ParseModule(FilesystemPath(moduleDir))
	if err == nil {
		t.Fatal("ParseModule() returned nil error, want validation error")
	}
	if !strings.Contains(err.Error(), "invalid module") {
		t.Fatalf("ParseModule() error = %v, want invowkmod.Load validation error", err)
	}
}

func TestParseModuleLoadsValidatedMetadataAndCommands(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "io.example.demo.invowkmod")
	if err := os.Mkdir(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	writeParseModuleFile(t, filepath.Join(moduleDir, "invowkmod.cue"), `module: "io.example.demo"
version: "1.0.0"
`)
	writeParseModuleFile(t, filepath.Join(moduleDir, "invowkfile.cue"), GenerateCUE(&Invowkfile{
		Commands: []Command{
			{
				Name: "build",
				Implementations: []Implementation{
					{
						Script:    "echo build",
						Runtimes:  []RuntimeConfig{{Name: RuntimeVirtual}},
						Platforms: AllPlatformConfigs(),
					},
				},
			},
		},
	}))

	mod, err := ParseModule(FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("ParseModule() error = %v", err)
	}
	if mod.Metadata == nil {
		t.Fatal("ParseModule() metadata is nil")
	}
	if mod.Commands == nil {
		t.Fatal("ParseModule() commands are nil")
	}
	if got, want := mod.Commands.GetModule(), "io.example.demo"; string(got) != want {
		t.Fatalf("Commands.GetModule() = %q, want %q", got, want)
	}
}

func writeParseModuleFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
