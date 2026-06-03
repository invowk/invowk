// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type discoveryMutationCommandInfoWant struct {
	name         invowkfile.CommandName
	description  invowkfile.DescriptionText
	source       Source
	filePath     types.FilesystemPath
	simpleName   invowkfile.CommandName
	sourceID     SourceID
	moduleID     invowkmod.ModuleID
	globalModule bool
}

func TestSourceIDValidateReportsInvalidValue(t *testing.T) {
	t.Parallel()

	const value SourceID = "1bad"

	err := value.Validate()
	if err == nil {
		t.Fatal("SourceID.Validate() error = nil, want invalid source ID error")
	}
	if !errors.Is(err, ErrInvalidSourceID) {
		t.Fatalf("SourceID.Validate() error = %v, want ErrInvalidSourceID", err)
	}
	var invalid *InvalidSourceIDError
	if !errors.As(err, &invalid) {
		t.Fatalf("SourceID.Validate() error type = %T, want *InvalidSourceIDError", err)
	}
	if invalid.Value != value {
		t.Fatalf("InvalidSourceIDError.Value = %q, want %q", invalid.Value, value)
	}
}

func TestDiscoveredCommandSetMutationAnalyzeEdges(t *testing.T) {
	t.Parallel()

	t.Run("single command does not become ambiguous", func(t *testing.T) {
		t.Parallel()

		cmd := &CommandInfo{SimpleName: "build", SourceID: SourceIDInvowkfile}
		set := NewDiscoveredCommandSet()
		set.Add(cmd)

		set.Analyze()

		if set.AmbiguousNames["build"] {
			t.Fatal("single command marked ambiguous")
		}
		if cmd.IsAmbiguous {
			t.Fatal("single command IsAmbiguous = true, want false")
		}
	})

	t.Run("same simple name from different sources is ambiguous", func(t *testing.T) {
		t.Parallel()

		root := &CommandInfo{SimpleName: "deploy", SourceID: SourceIDInvowkfile}
		module := &CommandInfo{SimpleName: "deploy", SourceID: "tools"}
		set := NewDiscoveredCommandSet()
		set.Add(root)
		set.Add(module)

		set.Analyze()

		if !set.AmbiguousNames["deploy"] {
			t.Fatal("deploy not marked ambiguous")
		}
		if !root.IsAmbiguous || !module.IsAmbiguous {
			t.Fatalf("ambiguous flags root=%v module=%v, want both true", root.IsAmbiguous, module.IsAmbiguous)
		}
	})

	t.Run("source order keeps invowkfile first", func(t *testing.T) {
		t.Parallel()

		set := NewDiscoveredCommandSet()
		set.Add(&CommandInfo{SimpleName: "mod", SourceID: "aaa"})
		set.Add(&CommandInfo{SimpleName: "root", SourceID: SourceIDInvowkfile})
		set.Add(&CommandInfo{SimpleName: "other", SourceID: "bbb"})

		set.Analyze()

		want := []SourceID{SourceIDInvowkfile, "aaa", "bbb"}
		if !slices.Equal(set.SourceOrder, want) {
			t.Fatalf("SourceOrder = %v, want %v", set.SourceOrder, want)
		}
	})
}

func TestDiscoverCommandSetMutationCommandPayloads(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	rootPath := filepath.Join(tmpDir, "invowkfile.cue")
	writeDiscoveryMutationInvowkfile(t, rootPath, "build", "Root build")
	moduleDir := filepath.Join(tmpDir, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "build")

	result, err := newTestDiscovery(t, config.DefaultConfig(), tmpDir).DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}

	root := requireDiscoveryMutationCommand(t, result.Set, "build")
	requireDiscoveryMutationCommandInfo(t, root, discoveryMutationCommandInfoWant{
		name:        "build",
		description: "Root build",
		source:      SourceCurrentDir,
		filePath:    types.FilesystemPath(rootPath),
		simpleName:  "build",
		sourceID:    SourceIDInvowkfile,
	})

	module := requireDiscoveryMutationCommand(t, result.Set, "tools build")
	requireDiscoveryMutationCommandInfo(t, module, discoveryMutationCommandInfoWant{
		name:       "tools build",
		source:     SourceModule,
		filePath:   types.FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue")),
		simpleName: "build",
		sourceID:   "tools",
		moduleID:   "tools",
	})

	if !result.Set.AmbiguousNames["build"] {
		t.Fatal("build not marked ambiguous across root and module")
	}
	if !root.IsAmbiguous || !module.IsAmbiguous {
		t.Fatalf("IsAmbiguous root=%v module=%v, want both true", root.IsAmbiguous, module.IsAmbiguous)
	}
}

func TestDiscoverCommandSetMutationSkipsDuplicateNonModuleCommands(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	rootPath := filepath.Join(tmpDir, "invowkfile.cue")
	content := `cmds: [
	{name: "build", description: "First build", implementations: [{script: {content: "echo first"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "build", description: "Second build", implementations: [{script: {content: "echo second"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
]`
	if err := os.WriteFile(rootPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write duplicate invowkfile: %v", err)
	}

	result, err := newTestDiscovery(t, config.DefaultConfig(), tmpDir).DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}

	builds := result.Set.BySimpleName["build"]
	if len(builds) != 1 {
		t.Fatalf("BySimpleName[build] length = %d, want first non-module command only", len(builds))
	}
	if result.Set.ByName["build"] != builds[0] {
		t.Fatal("ByName[build] does not point to retained first command")
	}
}

func TestDiscoverCommandSetMutationGlobalModulePayload(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, ".invowk", "cmds", "global.invowkmod")
	createTestModule(t, globalDir, "global", "lint")

	result, err := newTestDiscovery(t, config.DefaultConfig(), tmpDir).DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}

	cmd := requireDiscoveryMutationCommand(t, result.Set, "global lint")
	requireDiscoveryMutationCommandInfo(t, cmd, discoveryMutationCommandInfoWant{
		name:         "global lint",
		source:       SourceModule,
		filePath:     types.FilesystemPath(filepath.Join(globalDir, "invowkfile.cue")),
		simpleName:   "lint",
		sourceID:     "global",
		moduleID:     "global",
		globalModule: true,
	})
}

func TestDiscoverCommandSetMutationParseErrorsBecomeDiagnostics(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte("cmds: ["), 0o644); err != nil {
		t.Fatalf("write invalid invowkfile: %v", err)
	}

	result, err := newTestDiscovery(t, config.DefaultConfig(), tmpDir).DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v, want diagnostics-only parse skip", err)
	}
	if len(result.Set.Commands) != 0 {
		t.Fatalf("commands length = %d, want 0 after parse skip", len(result.Set.Commands))
	}
	requireDiscoveryMutationDiagnostic(t, result.Diagnostics, CodeInvowkfileParseSkipped, "skipping invowkfile")
}

func TestGetCommandMutationRejectsInvalidCommandName(t *testing.T) {
	t.Parallel()

	lookup, err := newTestDiscovery(t, config.DefaultConfig(), t.TempDir()).GetCommand(t.Context(), "")
	if err == nil {
		t.Fatal("GetCommand() error = nil, want invalid command name")
	}
	if !strings.Contains(err.Error(), "invalid command name") {
		t.Fatalf("GetCommand() error = %q, want invalid command name detail", err)
	}
	if !errors.Is(err, invowkfile.ErrInvalidCommandName) {
		t.Fatalf("GetCommand() error = %v, want ErrInvalidCommandName", err)
	}
	if lookup.Command != nil || len(lookup.Diagnostics) != 0 {
		t.Fatalf("GetCommand() lookup = %+v, want empty result on invalid name", lookup)
	}
}

func TestGetCommandMutationPreservesDiscoveryDiagnostics(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeDiscoveryMutationInvowkfile(t, filepath.Join(tmpDir, "invowkfile.cue"), "build", "Build")
	if err := os.Mkdir(filepath.Join(tmpDir, "broken.invowkmod"), 0o755); err != nil {
		t.Fatalf("create invalid module dir: %v", err)
	}

	lookup, err := newTestDiscovery(t, config.DefaultConfig(), tmpDir).GetCommand(t.Context(), "build")
	if err != nil {
		t.Fatalf("GetCommand() error = %v", err)
	}
	if lookup.Command == nil || lookup.Command.Name != "build" {
		t.Fatalf("GetCommand() command = %+v, want build", lookup.Command)
	}
	requireDiscoveryMutationDiagnostic(t, lookup.Diagnostics, CodeModuleLoadSkipped, "broken.invowkmod")
}

func writeDiscoveryMutationInvowkfile(t *testing.T, path, name, description string) {
	t.Helper()

	content := `cmds: [{name: "` + name + `", description: "` + description + `", implementations: [{script: {content: "echo test"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write invowkfile: %v", err)
	}
}

func requireDiscoveryMutationCommand(t *testing.T, set *DiscoveredCommandSet, name invowkfile.CommandName) *CommandInfo {
	t.Helper()

	cmd := set.ByName[name]
	if cmd == nil {
		t.Fatalf("ByName[%q] = nil; commands: %v", name, set.Commands)
	}
	return cmd
}

func requireDiscoveryMutationCommandInfo(t *testing.T, got *CommandInfo, want discoveryMutationCommandInfoWant) {
	t.Helper()

	if got.Name != want.name {
		t.Fatalf("Name = %q, want %q", got.Name, want.name)
	}
	if got.Description != want.description {
		t.Fatalf("Description = %q, want %q", got.Description, want.description)
	}
	if got.Source != want.source {
		t.Fatalf("Source = %v, want %v", got.Source, want.source)
	}
	if got.FilePath != want.filePath {
		t.Fatalf("FilePath = %q, want %q", got.FilePath, want.filePath)
	}
	if got.Command == nil {
		t.Fatal("Command = nil, want command pointer")
	}
	if got.Invowkfile == nil {
		t.Fatal("Invowkfile = nil, want parent invowkfile")
	}
	if got.SimpleName != want.simpleName {
		t.Fatalf("SimpleName = %q, want %q", got.SimpleName, want.simpleName)
	}
	if got.SourceID != want.sourceID {
		t.Fatalf("SourceID = %q, want %q", got.SourceID, want.sourceID)
	}
	if got.IsGlobalModule != want.globalModule {
		t.Fatalf("IsGlobalModule = %v, want %v", got.IsGlobalModule, want.globalModule)
	}
	if want.moduleID == "" {
		if got.ModuleID != nil {
			t.Fatalf("ModuleID = %q, want nil", *got.ModuleID)
		}
		return
	}
	if got.ModuleID == nil {
		t.Fatalf("ModuleID = nil, want %q", want.moduleID)
	}
	if *got.ModuleID != want.moduleID {
		t.Fatalf("ModuleID = %q, want %q", *got.ModuleID, want.moduleID)
	}
}

func requireDiscoveryMutationDiagnostic(
	t *testing.T,
	diagnostics []Diagnostic,
	code DiagnosticCode,
	messagePart string,
) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.code == code && strings.Contains(diagnostic.message, messagePart) {
			return
		}
	}
	t.Fatalf("diagnostics missing code=%q message containing %q: %#v", code, messagePart, diagnostics)
}
