// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestCreateModuleDryRunDoesNotWrite(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	result, err := CreateModule(t.Context(), ModuleCreateOptions{
		ModuleID:    "io.example.tools",
		Description: "create a tools module",
		DryRun:      true,
		Completer:   fakeCompleter{response: moduleResponse("io.example.tools", "module generated")},
	})
	if err != nil {
		t.Fatalf("CreateModule() error = %v", err)
	}
	if result.ModuleID != "io.example.tools" {
		t.Fatalf("ModuleID = %q", result.ModuleID)
	}
	if !strings.Contains(result.Diff, "io.example.tools.invowkmod/invowkmod.cue") {
		t.Fatalf("dry-run diff missing module path:\n%s", result.Diff)
	}
	if _, err := os.Stat("io.example.tools.invowkmod"); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote module directory, stat err = %v", err)
	}
}

func TestCreateModuleRepairsModuleIDMismatch(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	completer := &sequenceCompleter{
		responses: []string{
			moduleResponse("io.example.other", "wrong"),
			moduleResponse("io.example.tools", "fixed"),
		},
	}

	result, err := CreateModule(t.Context(), ModuleCreateOptions{
		ModuleID:    "io.example.tools",
		Description: "create a tools module",
		DryRun:      true,
		Completer:   completer,
	})
	if err != nil {
		t.Fatalf("CreateModule() error = %v", err)
	}
	if result.ModuleID != "io.example.tools" {
		t.Fatalf("ModuleID = %q", result.ModuleID)
	}
	if len(completer.prompts) != 2 {
		t.Fatalf("completion calls = %d, want 2", len(completer.prompts))
	}
	for _, want := range []string{"generated module ID", "io.example.other", "io.example.tools"} {
		if !strings.Contains(completer.prompts[1], want) {
			t.Fatalf("repair prompt missing %q:\n%s", want, completer.prompts[1])
		}
	}
}

func TestChangeModuleUpdatesOnlyCueFiles(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	modulePath := writeModuleFixture(t, tmpDir, "io.example.tools")
	extraPath := filepath.Join(modulePath, "notes.txt")
	if err := os.WriteFile(extraPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write extra fixture: %v", err)
	}

	result, err := ChangeModule(t.Context(), ModuleChangeOptions{
		Target:      "io.example.tools",
		Description: "change the module",
		Completer:   fakeCompleter{response: moduleResponse("io.example.tools", "module changed")},
	})
	if err != nil {
		t.Fatalf("ChangeModule() error = %v", err)
	}
	if result.ModuleID != "io.example.tools" {
		t.Fatalf("ModuleID = %q", result.ModuleID)
	}
	extra, err := os.ReadFile(extraPath)
	if err != nil {
		t.Fatalf("read extra file: %v", err)
	}
	if string(extra) != "keep me" {
		t.Fatalf("extra file changed: %q", extra)
	}
}

func TestParseModuleGenerationResponseRejectsExtraFiles(t *testing.T) {
	t.Parallel()

	raw := `{"invowkmod_cue":"module: \"io.example.tools\"\nversion: \"1.0.0\"","invowkfile_cue":"cmds: []","summary":"bad","files":{"scripts/run.sh":"echo bad"}}`
	_, err := ParseModuleGenerationResponse(raw)
	if err == nil {
		t.Fatal("expected error for arbitrary extra file field")
	}
}

func TestRemoveModuleRequiresForceAndRejectsSymlink(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	modulePath := writeModuleFixture(t, tmpDir, "io.example.tools")

	dryRun, err := RemoveModule(t.Context(), ModuleRemoveOptions{
		Target: "io.example.tools",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("RemoveModule() dry-run error = %v", err)
	}
	if !strings.Contains(dryRun.Diff, "delete io.example.tools.invowkmod") {
		t.Fatalf("dry-run delete plan = %q", dryRun.Diff)
	}
	if _, statErr := os.Stat(modulePath); statErr != nil {
		t.Fatalf("dry-run removed module: %v", statErr)
	}

	_, err = RemoveModule(t.Context(), ModuleRemoveOptions{Target: "io.example.tools"})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("RemoveModule() force error = %v, want --force", err)
	}

	if runtime.GOOS != "windows" {
		linkPath := filepath.Join(tmpDir, "io.example.link.invowkmod")
		if symlinkErr := os.Symlink(modulePath, linkPath); symlinkErr != nil {
			t.Fatalf("create symlink fixture: %v", symlinkErr)
		}
		_, err = RemoveModule(t.Context(), ModuleRemoveOptions{Target: linkPath, DryRun: true})
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("RemoveModule() symlink error = %v, want symlink rejection", err)
		}
	}

	_, err = RemoveModule(t.Context(), ModuleRemoveOptions{
		Target: "io.example.tools",
		Force:  true,
	})
	if err != nil {
		t.Fatalf("RemoveModule() force error = %v", err)
	}
	if _, err := os.Stat(modulePath); !os.IsNotExist(err) {
		t.Fatalf("force remove kept module, stat err = %v", err)
	}
}

func TestRemoveCommandDeletesFinalInvowkfile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(target, []byte(wrapCommandObject(testCommandObject)), 0o644); err != nil {
		t.Fatalf("write target fixture: %v", err)
	}

	result, err := RemoveCommand(t.Context(), RemoveOptions{
		Name:       "hello generated",
		TargetPath: target,
	})
	if err != nil {
		t.Fatalf("RemoveCommand() error = %v", err)
	}
	if result.Content != "" {
		t.Fatalf("Content = %q, want empty deletion marker", result.Content)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("RemoveCommand() kept target file, stat err = %v", err)
	}
}

func writeModuleFixture(t *testing.T, root, moduleID string) string {
	t.Helper()

	modulePath := filepath.Join(root, moduleID+invowkmod.ModuleSuffix)
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		t.Fatalf("create module fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte(moduleCUE(moduleID)), 0o644); err != nil {
		t.Fatalf("write invowkmod fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkfile.cue"), []byte(moduleInvowkfileCUE()), 0o644); err != nil {
		t.Fatalf("write invowkfile fixture: %v", err)
	}
	return modulePath
}

func moduleResponse(moduleID, summary string) string {
	return `{"invowkmod_cue":` + quoteForJSON(moduleCUE(moduleID)) +
		`,"invowkfile_cue":` + quoteForJSON(moduleInvowkfileCUE()) +
		`,"summary":` + quoteForJSON(summary) + `}`
}

func moduleCUE(moduleID string) string {
	return `module: "` + moduleID + `"
version: "1.0.0"
description: "Generated tools"
`
}

func moduleInvowkfileCUE() string {
	return `cmds: [{
	name: "modhello"
	description: "Print module greeting"
	implementations: [{
		script: {content: "echo module"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`
}
