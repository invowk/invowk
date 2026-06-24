// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestParseBytesRejectsScriptSourceMistakes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  string
		want string
	}{
		{
			name: "empty implementation script object",
			cue:  validCommandCUE(`script: {}`),
			want: "script",
		},
		{
			name: "duplicated implementation script source",
			cue:  validCommandCUE(`script: {content: "echo hi", file: "scripts/build.sh"}`),
			want: "script",
		},
		{
			name: "empty custom check script content",
			cue: validInvowkfileWithDependsOnCUE(`
	custom_checks: [{name: "empty", script: {content: ""}}]
`),
			want: "script",
		},
		{
			name: "non-module implementation script file",
			cue:  validCommandCUE(`script: {file: "scripts/build.sh"}`),
			want: "script file requires module invowkfile",
		},
		{
			name: "non-module custom check script file",
			cue: validInvowkfileWithDependsOnCUE(`
	custom_checks: [{name: "file-check", script: {file: "scripts/check.sh"}}]
`),
			want: "script file requires module invowkfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseBytes([]byte(tt.cue), "invowkfile.cue")
			if err == nil {
				t.Fatalf("ParseBytes() error = nil, want error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseBytes() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestParseLoadedModuleInvowkfileAcceptsModuleScriptFiles(t *testing.T) {
	t.Parallel()

	moduleDir := createScriptSourceModule(t, `cmds: [{
	name: "build"
	implementations: [{
		script: {file: "scripts/build"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}]
	}]
}]
depends_on: {
	custom_checks: [{name: "check-build", script: {file: "./scripts/check.sh"}}]
}
`)

	loaded, err := invowkmod.Load(FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	inv, err := ParseLoadedModuleInvowkfile(loaded)
	if err != nil {
		t.Fatalf("ParseLoadedModuleInvowkfile() error = %v", err)
	}
	if got := inv.Commands[0].Implementations[0].Script.File; got == nil || string(*got) != "scripts/build" {
		t.Fatalf("implementation script file = %v, want scripts/build", got)
	}
	checks := inv.DependsOn.CustomChecks[0].GetChecks()
	if got := checks[0].Script.File; got == nil || string(*got) != "./scripts/check.sh" {
		t.Fatalf("custom check script file = %v, want ./scripts/check.sh", got)
	}
}

func TestParseLoadedModuleInvowkfileRejectsOutsideModuleScriptFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  string
	}{
		{
			name: "implementation escapes module",
			cue: `cmds: [{
	name: "build"
	implementations: [{
		script: {file: "../outside.sh"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}]
	}]
}]
`,
		},
		{
			name: "custom check escapes module",
			cue: validInvowkfileWithDependsOnCUE(`
	custom_checks: [{name: "outside", script: {file: "../outside.sh"}}]
`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			moduleDir := createScriptSourceModule(t, tt.cue)
			loaded, err := invowkmod.Load(FilesystemPath(moduleDir))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			_, err = ParseLoadedModuleInvowkfile(loaded)
			if err == nil {
				t.Fatal("ParseLoadedModuleInvowkfile() error = nil, want invalid script file path")
			}
			if !errors.Is(err, ErrInvalidScriptFilePath) {
				t.Fatalf("ParseLoadedModuleInvowkfile() error = %v, want ErrInvalidScriptFilePath", err)
			}
		})
	}
}

func validCommandCUE(scriptLine string) string {
	return `cmds: [{
	name: "test"
	implementations: [{
		` + scriptLine + `
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}]
	}]
}]
`
}

func validInvowkfileWithDependsOnCUE(dependsOnBody string) string {
	return validCommandCUE(`script: {content: "echo test"}`) + `
depends_on: {` + dependsOnBody + `}
`
}

func createScriptSourceModule(t *testing.T, invowkfileCUE string) string {
	t.Helper()

	moduleDir := filepath.Join(t.TempDir(), "io.example.scripts.invowkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	writeParseModuleFile(t, filepath.Join(moduleDir, "invowkmod.cue"), `module: "io.example.scripts"
version: "1.0.0"
`)
	scriptsDir := filepath.Join(moduleDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	writeParseModuleFile(t, filepath.Join(scriptsDir, "build"), "echo build\n")
	writeParseModuleFile(t, filepath.Join(scriptsDir, "check.sh"), "echo check\n")
	writeParseModuleFile(t, filepath.Join(moduleDir, "invowkfile.cue"), invowkfileCUE)

	return moduleDir
}
