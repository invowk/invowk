// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	discoveryMutationSameModule     invowkmod.ModuleID = "io.example.same"
	discoveryMutationToolsNamespace SourceID           = "tools"
)

func TestNewMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "defaults keep vendored integrity enabled", run: func(t *testing.T) {
			t.Helper()

			d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))
			if !d.verifyVendoredIntegrity {
				t.Fatal("verifyVendoredIntegrity = false, want default true")
			}
		}},

		{name: "explicit empty dirs skip default discovery sources", run: func(t *testing.T) {
			t.Helper()

			d := New(nil, WithBaseDir(""), WithCommandsDir(""))
			if d.baseDir != "" || d.commandsDir != "" {
				t.Fatalf("New() dirs = (%q, %q), want explicit empty dirs", d.baseDir, d.commandsDir)
			}
			if !d.baseDirSet || !d.commandsDirSet {
				t.Fatalf("New() set flags = (%v, %v), want both true", d.baseDirSet, d.commandsDirSet)
			}
			if len(d.initDiagnostics) != 0 {
				t.Fatalf("initDiagnostics = %#v, want none", d.initDiagnostics)
			}
		}},

		{name: "preseeded dirs are not overwritten by defaults", run: func(t *testing.T) {
			t.Helper()

			d := New(nil, func(d *Discovery) {
				d.baseDir = "/preseeded/work"
				d.commandsDir = "/preseeded/cmds"
			})
			if d.baseDir != "/preseeded/work" {
				t.Fatalf("baseDir = %q, want preseeded value", d.baseDir)
			}
			if d.commandsDir != "/preseeded/cmds" {
				t.Fatalf("commandsDir = %q, want preseeded value", d.commandsDir)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestNewMutationReportsUnavailableWorkingDirectory(t *testing.T) {
	t.Parallel()

	errWorkingDirectoryUnavailable := errors.New("working directory unavailable")
	d := New(config.DefaultConfig(), WithCommandsDir(""), func(d *Discovery) {
		d.workingDirectory = func() (types.FilesystemPath, error) {
			return "", errWorkingDirectoryUnavailable
		}
	})
	if d.baseDir != "" {
		t.Fatalf("baseDir = %q, want empty when cwd is unavailable", d.baseDir)
	}
	requireDiscoveryMutationDiagnostic(t, d.initDiagnostics, CodeWorkingDirUnavailable, "current directory unavailable")
}

//nolint:paralleltest // uses t.Setenv for process-wide home variables.
func TestNewMutationReportsUnavailableCommandsDirectory(t *testing.T) {
	for _, key := range []string{"HOME", "home", "USERPROFILE", "HOMEDRIVE", "HOMEPATH"} {
		t.Setenv(key, "")
	}

	d := New(config.DefaultConfig(), WithBaseDir(types.FilesystemPath(t.TempDir())))
	if d.commandsDir != "" {
		t.Fatalf("commandsDir = %q, want empty when user home is unavailable", d.commandsDir)
	}
	requireDiscoveryMutationDiagnostic(t, d.initDiagnostics, CodeCommandsDirUnavailable, "user commands directory unavailable")
}

func TestCheckModuleCollisionsMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "skips nil and errored files before later collision", run: testCheckModuleCollisionsSkipsErroredFiles},
		{name: "module collision reports module paths and local source kind", run: testCheckModuleCollisionsReportsModulePaths},
		{name: "command source collision reports namespace and second source", run: testCheckModuleCollisionsReportsCommandSource},
		{name: "vendored collision annotates parent module", run: testCheckModuleCollisionsReportsVendoredParent},
		{name: "module metadata wins over invowkfile fallback metadata", run: testCheckModuleCollisionsUsesModuleMetadata},
		{name: "invalid duplicate module namespace reports validation error", run: testCheckModuleCollisionsInvalidModuleNamespace},
		{name: "invalid duplicate command namespace reports validation error", run: testCheckModuleCollisionsInvalidCommandNamespace},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testCheckModuleCollisionsSkipsErroredFiles(t *testing.T) {
	t.Helper()

	files := []*DiscoveredFile{
		nil,
		{
			Path:       "/bad/error.invowkmod/invowkfile.cue",
			Error:      os.ErrInvalid,
			Invowkfile: discoveryMutationInvowkfile(t, discoveryMutationSameModule),
		},
		discoveryMutationModuleFile(t, "/first/one.invowkmod", "/first/one.invowkmod/invowkfile.cue", discoveryMutationSameModule, discoveryMutationSameModule),
		discoveryMutationModuleFile(t, "/second/two.invowkmod", "/second/two.invowkmod/invowkfile.cue", discoveryMutationSameModule, discoveryMutationSameModule),
	}

	collision := requireDiscoveryMutationCollision(t, discoveryMutationDiscovery().CheckModuleCollisions(files))
	if collision.FirstSource != "/first/one.invowkmod" {
		t.Fatalf("FirstSource = %q, want first valid module path", collision.FirstSource)
	}
	if collision.SecondSource != "/second/two.invowkmod" {
		t.Fatalf("SecondSource = %q, want second valid module path", collision.SecondSource)
	}
}

func testCheckModuleCollisionsReportsModulePaths(t *testing.T) {
	t.Helper()

	files := []*DiscoveredFile{
		discoveryMutationModuleFile(t, "/first/one.invowkmod", "/ignored/first/invowkfile.cue", discoveryMutationSameModule, discoveryMutationSameModule),
		discoveryMutationModuleFile(t, "/second/two.invowkmod", "/ignored/second/invowkfile.cue", discoveryMutationSameModule, discoveryMutationSameModule),
	}

	collision := requireDiscoveryMutationCollision(t, discoveryMutationDiscovery().CheckModuleCollisions(files))
	if collision.Namespace != SourceID(discoveryMutationSameModule) {
		t.Fatalf("Namespace = %q, want %q", collision.Namespace, discoveryMutationSameModule)
	}
	if collision.FirstSource != "/first/one.invowkmod" {
		t.Fatalf("FirstSource = %q, want module path", collision.FirstSource)
	}
	if collision.SecondSource != "/second/two.invowkmod" {
		t.Fatalf("SecondSource = %q, want module path", collision.SecondSource)
	}
	if collision.SecondKind != ModuleCollisionSourceLocal {
		t.Fatalf("SecondKind = %q, want %q", collision.SecondKind, ModuleCollisionSourceLocal)
	}
}

func testCheckModuleCollisionsReportsCommandSource(t *testing.T) {
	t.Helper()

	files := []*DiscoveredFile{
		discoveryMutationModuleFile(t, "/first/tools.invowkmod", "/first/tools.invowkmod/invowkfile.cue", "io.example.first", "io.example.first"),
		discoveryMutationModuleFile(t, "/second/tools.invowkmod", "/second/tools.invowkmod/invowkfile.cue", "io.example.second", "io.example.second"),
	}

	collision := requireDiscoveryMutationCollision(t, discoveryMutationDiscovery().CheckModuleCollisions(files))
	if collision.Namespace != discoveryMutationToolsNamespace {
		t.Fatalf("Namespace = %q, want %q", collision.Namespace, discoveryMutationToolsNamespace)
	}
	if collision.FirstSource != "/first/tools.invowkmod" {
		t.Fatalf("FirstSource = %q, want first command source", collision.FirstSource)
	}
	if collision.SecondSource != "/second/tools.invowkmod" {
		t.Fatalf("SecondSource = %q, want second command source", collision.SecondSource)
	}
}

func testCheckModuleCollisionsReportsVendoredParent(t *testing.T) {
	t.Helper()

	files := []*DiscoveredFile{
		discoveryMutationVendoredModuleFile(
			t,
			"/parent1.invowkmod/invowk_modules/tools.invowkmod",
			"io.example.child1",
			"io.example.parent1",
			discoveryMutationToolsNamespace,
		),
		discoveryMutationVendoredModuleFile(
			t,
			"/parent2.invowkmod/invowk_modules/tools.invowkmod",
			"io.example.child2",
			"io.example.parent2",
			discoveryMutationToolsNamespace,
		),
	}

	collision := requireDiscoveryMutationCollision(t, discoveryMutationDiscovery().CheckModuleCollisions(files))
	if collision.SecondSource != "/parent2.invowkmod/invowk_modules/tools.invowkmod (vendored in io.example.parent2)" {
		t.Fatalf("SecondSource = %q, want vendored parent annotation", collision.SecondSource)
	}
	if collision.SecondKind != ModuleCollisionSourceVendored {
		t.Fatalf("SecondKind = %q, want %q", collision.SecondKind, ModuleCollisionSourceVendored)
	}
}

func testCheckModuleCollisionsUsesModuleMetadata(t *testing.T) {
	t.Helper()

	files := []*DiscoveredFile{
		discoveryMutationModuleFile(t, "/first/one.invowkmod", "/first/one.invowkmod/invowkfile.cue", "io.example.module", "io.example.other"),
		discoveryMutationModuleFile(t, "/second/two.invowkmod", "/second/two.invowkmod/invowkfile.cue", "io.example.other", "io.example.module"),
	}

	if err := discoveryMutationDiscovery().CheckModuleCollisions(files); err != nil {
		t.Fatalf("CheckModuleCollisions() error = %v, want nil", err)
	}
}

func testCheckModuleCollisionsInvalidModuleNamespace(t *testing.T) {
	t.Helper()

	files := []*DiscoveredFile{
		{
			Module: &invowkmod.Module{
				Path:     "/first/invalid.invowkmod",
				Metadata: &invowkmod.Invowkmod{Module: "1bad"},
			},
		},
		{
			Module: &invowkmod.Module{
				Path:     "/second/invalid.invowkmod",
				Metadata: &invowkmod.Invowkmod{Module: "1bad"},
			},
		},
	}

	requireInvalidDiscoveryMutationNamespace(t, discoveryMutationDiscovery().CheckModuleCollisions(files), "invalid module namespace")
}

func testCheckModuleCollisionsInvalidCommandNamespace(t *testing.T) {
	t.Helper()

	files := []*DiscoveredFile{
		discoveryMutationModuleFile(t, "/first/1bad.invowkmod", "/first/1bad.invowkmod/invowkfile.cue", "io.example.first", "io.example.first"),
		discoveryMutationModuleFile(t, "/second/1bad.invowkmod", "/second/1bad.invowkmod/invowkfile.cue", "io.example.second", "io.example.second"),
	}

	requireInvalidDiscoveryMutationNamespace(t, discoveryMutationDiscovery().CheckModuleCollisions(files), "invalid command namespace")
}

func TestModuleIdentityMutationContracts(t *testing.T) {
	t.Parallel()

	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))
	file := discoveryMutationModuleFile(
		t,
		"/modules/tools.invowkmod",
		"/modules/tools.invowkmod/invowkfile.cue",
		"io.example.module",
		"io.example.invowkfile",
	)

	identity, ok := d.moduleIdentityFor(file)
	if !ok {
		t.Fatal("moduleIdentityFor() ok = false, want true")
	}
	if identity.ModuleID != "io.example.module" {
		t.Fatalf("ModuleID = %q, want module metadata identity", identity.ModuleID)
	}
	if identity.SourceID != "io.example.module" {
		t.Fatalf("SourceID = %q, want module metadata source", identity.SourceID)
	}
	if identity.SourcePath != "/modules/tools.invowkmod" {
		t.Fatalf("SourcePath = %q, want module path", identity.SourcePath)
	}
}

func TestCommandSourceMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("module without invowkfile has no effective command namespace", func(t *testing.T) {
		t.Parallel()

		d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))
		file := &DiscoveredFile{
			Module: &invowkmod.Module{
				Path:     "/modules/tools.invowkmod",
				Metadata: &invowkmod.Invowkmod{Module: "io.example.tools"},
			},
		}

		if got := d.GetEffectiveCommandNamespace(file); got != "" {
			t.Fatalf("GetEffectiveCommandNamespace() = %q, want empty", got)
		}
	})

	t.Run("alias lookup ignores empty aliases before matching configured alias", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		cfg.Includes = []config.IncludeEntry{
			{Path: "/modules/tools.invowkmod", Alias: ""},
			{Path: "/modules/tools.invowkmod/.", Alias: "tools"},
		}
		d := New(cfg, WithBaseDir(""), WithCommandsDir(""))

		if got := d.getAliasForModulePath("/modules/tools.invowkmod"); got != "tools" {
			t.Fatalf("getAliasForModulePath() = %q, want tools", got)
		}
	})
}

func TestLoadFirstMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("invalid root invowkfile returns parse error on file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte("cmds: ["), 0o644); err != nil {
			t.Fatalf("write invalid invowkfile: %v", err)
		}
		d := newTestDiscovery(t, config.DefaultConfig(), tmpDir)

		file, err := d.LoadFirst()
		if err == nil {
			t.Fatal("LoadFirst() error = nil, want parse error")
		}
		if file == nil {
			t.Fatal("LoadFirst() file = nil, want errored discovered file")
		}
		if file.Error == nil {
			t.Fatalf("LoadFirst() file.Error = nil, want parse error %v", err)
		}
		if file.Invowkfile != nil {
			t.Fatalf("LoadFirst() Invowkfile = %#v, want nil after parse error", file.Invowkfile)
		}
	})
}

func TestLoadAllMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("parse errors stay attached to discovered files", func(t *testing.T) {
		t.Parallel()
		testLoadAllMutationParseErrorsStayAttached(t)
	})
	t.Run("module collisions return load error", func(t *testing.T) {
		t.Parallel()
		testLoadAllMutationModuleCollisionsReturnError(t)
	})
}

func testLoadAllMutationParseErrorsStayAttached(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte("cmds: ["), 0o644); err != nil {
		t.Fatalf("write invalid invowkfile: %v", err)
	}

	files, err := newTestDiscovery(t, config.DefaultConfig(), tmpDir).LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v, want nil with file-level parse error", err)
	}
	if len(files) != 1 {
		t.Fatalf("LoadAll() files length = %d, want 1", len(files))
	}
	if files[0].Error == nil {
		t.Fatal("LoadAll() file.Error = nil, want parse error")
	}
	if files[0].Invowkfile != nil {
		t.Fatalf("LoadAll() Invowkfile = %#v, want nil after parse error", files[0].Invowkfile)
	}
}

func testLoadAllMutationModuleCollisionsReturnError(t *testing.T) {
	t.Helper()

	firstModuleDir, secondModuleDir := createDiscoveryMutationCollisionModules(t)
	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(firstModuleDir)},
		{Path: config.ModuleIncludePath(secondModuleDir)},
	}

	files, err := New(cfg, WithBaseDir(""), WithCommandsDir("")).LoadAll()
	if err == nil {
		t.Fatal("LoadAll() error = nil, want module collision error")
	}
	if files != nil {
		t.Fatalf("LoadAll() files = %#v, want nil after module collision", files)
	}
	collision := requireDiscoveryMutationCollision(t, err)
	if collision.Namespace != SourceID(discoveryMutationSameModule) {
		t.Fatalf("Namespace = %q, want %q", collision.Namespace, discoveryMutationSameModule)
	}
}

func createDiscoveryMutationCollisionModules(t *testing.T) (firstModuleDir, secondModuleDir string) {
	t.Helper()

	firstModuleDir = filepath.Join(t.TempDir(), string(discoveryMutationSameModule)+invowkmod.ModuleSuffix)
	secondModuleDir = filepath.Join(t.TempDir(), string(discoveryMutationSameModule)+invowkmod.ModuleSuffix)
	createTestModule(t, firstModuleDir, string(discoveryMutationSameModule), "first")
	createTestModule(t, secondModuleDir, string(discoveryMutationSameModule), "second")
	return firstModuleDir, secondModuleDir
}

func discoveryMutationDiscovery() *Discovery {
	return New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))
}

func requireInvalidDiscoveryMutationNamespace(t *testing.T, err error, wantMessage string) {
	t.Helper()

	if err == nil {
		t.Fatal("CheckModuleCollisions() error = nil, want namespace validation error")
	}
	if errors.Is(err, ErrModuleCollision) {
		t.Fatalf("CheckModuleCollisions() error = %v, want validation error before collision", err)
	}
	if !errors.Is(err, ErrInvalidSourceID) {
		t.Fatalf("CheckModuleCollisions() error = %v, want ErrInvalidSourceID", err)
	}
	if !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("CheckModuleCollisions() error = %v, want %s", err, wantMessage)
	}
}

func requireDiscoveryMutationCollision(t *testing.T, err error) *ModuleCollisionError {
	t.Helper()

	if err == nil {
		t.Fatal("CheckModuleCollisions() error = nil, want collision")
	}
	if !errors.Is(err, ErrModuleCollision) {
		t.Fatalf("CheckModuleCollisions() error = %v, want ErrModuleCollision", err)
	}
	var collision *ModuleCollisionError
	if !errors.As(err, &collision) {
		t.Fatalf("CheckModuleCollisions() error = %T, want *ModuleCollisionError", err)
	}
	return collision
}

func discoveryMutationInvowkfile(t *testing.T, moduleID invowkmod.ModuleID) *invowkfile.Invowkfile {
	t.Helper()

	return &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, moduleID)}
}

func discoveryMutationModuleFile(
	t *testing.T,
	modulePath string,
	filePath string,
	moduleID invowkmod.ModuleID,
	invowkfileModuleID invowkmod.ModuleID,
) *DiscoveredFile {
	t.Helper()

	return &DiscoveredFile{
		Path:       types.FilesystemPath(filePath),
		Source:     SourceModule,
		Invowkfile: discoveryMutationInvowkfile(t, invowkfileModuleID),
		Module: &invowkmod.Module{
			Path:     types.FilesystemPath(modulePath),
			Metadata: &invowkmod.Invowkmod{Module: moduleID},
		},
	}
}

func discoveryMutationVendoredModuleFile(
	t *testing.T,
	modulePath string,
	moduleID invowkmod.ModuleID,
	parentID invowkmod.ModuleID,
	namespace SourceID,
) *DiscoveredFile {
	t.Helper()

	file := discoveryMutationModuleFile(t, modulePath, modulePath+"/invowkfile.cue", moduleID, moduleID)
	file.ParentModule = &invowkmod.Module{
		Metadata: &invowkmod.Invowkmod{Module: parentID},
	}
	file.CommandNamespace = invowkmod.ModuleNamespace(namespace)
	return file
}
