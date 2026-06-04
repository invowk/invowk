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

func TestSource_InvalidErrorValue(t *testing.T) {
	t.Parallel()

	const invalidSource = Source(99)
	err := invalidSource.Validate()
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("Source.Validate() error = %v, want ErrInvalidSource", err)
	}
	var sourceErr *InvalidSourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("Source.Validate() error = %T, want *InvalidSourceError", err)
	}
	if sourceErr.Value != invalidSource {
		t.Fatalf("InvalidSourceError.Value = %d, want %d", sourceErr.Value, invalidSource)
	}
}

func TestDiscoverModules_FiltersNonModuleFilesAndPreservesDiagnostics(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, invowkfile.InvowkfileName+".cue"), "cmds: []\n")
	moduleDir := filepath.Join(tmpDir, "tool.invowkmod")
	createTestModule(t, moduleDir, "tool", "run")
	seedDiagnostic := mustDiagnostic(SeverityWarning, CodeCommandNotFound, "seed diagnostic")

	d := newTestDiscovery(t, config.DefaultConfig(), tmpDir, WithInitialDiagnostics([]Diagnostic{seedDiagnostic}))
	result, err := d.DiscoverModules()
	if err != nil {
		t.Fatalf("DiscoverModules() error = %v", err)
	}

	if len(result.Modules) != 1 {
		t.Fatalf("DiscoverModules() returned %d modules, want 1: %#v", len(result.Modules), result.Modules)
	}
	moduleFile := result.Modules[0]
	if moduleFile == nil || moduleFile.Module == nil {
		t.Fatalf("DiscoverModules() module = %#v, want loaded module file", moduleFile)
	}
	if moduleFile.Module.Metadata.Module != "tool" {
		t.Fatalf("module ID = %q, want tool", moduleFile.Module.Metadata.Module)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code() != CodeCommandNotFound {
		t.Fatalf("diagnostics = %#v, want seed diagnostic preserved", result.Diagnostics)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("ModuleListResult.Validate() error = %v", err)
	}
}

func TestModuleListResult_ValidateSkipsNilModulesAndReportsInvalidDiagnostics(t *testing.T) {
	t.Parallel()

	valid := ModuleListResult{
		Modules: []*DiscoveredFile{
			nil,
			{Path: types.FilesystemPath(t.TempDir()), Source: SourceModule},
		},
		Diagnostics: []Diagnostic{mustDiagnostic(SeverityWarning, CodeCommandNotFound, "valid")},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid ModuleListResult.Validate() error = %v", err)
	}

	invalid := ModuleListResult{
		Modules:     []*DiscoveredFile{nil},
		Diagnostics: []Diagnostic{{severity: Severity("invalid"), code: CodeCommandNotFound}},
	}
	err := invalid.Validate()
	if !errors.Is(err, ErrInvalidDiagnostic) {
		t.Fatalf("invalid ModuleListResult.Validate() error = %v, want ErrInvalidDiagnostic", err)
	}
	var diagnosticErr *InvalidDiagnosticError
	if !errors.As(err, &diagnosticErr) {
		t.Fatalf("invalid ModuleListResult.Validate() error = %T, want *InvalidDiagnosticError", err)
	}
}

func TestDiscoverInDir_PrefersCueThenFallsBackToExtensionless(t *testing.T) {
	t.Parallel()

	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))

	withBoth := t.TempDir()
	writeTestFile(t, filepath.Join(withBoth, invowkfile.InvowkfileName), "legacy\n")
	writeTestFile(t, filepath.Join(withBoth, invowkfile.InvowkfileName+".cue"), "cmds: []\n")
	cueFile := d.discoverInDir(types.FilesystemPath(withBoth), SourceCurrentDir)
	if cueFile == nil {
		t.Fatal("discoverInDir() returned nil for directory with invowkfile.cue")
	}
	if filepath.Base(string(cueFile.Path)) != invowkfile.InvowkfileName+".cue" {
		t.Fatalf("discoverInDir() path = %s, want invowkfile.cue", cueFile.Path)
	}
	if cueFile.Source != SourceCurrentDir {
		t.Fatalf("discoverInDir() source = %v, want SourceCurrentDir", cueFile.Source)
	}

	moduleCueFile := d.discoverInDir(types.FilesystemPath(withBoth), SourceModule)
	if moduleCueFile == nil {
		t.Fatal("discoverInDir() returned nil for module directory with invowkfile.cue")
	}
	if moduleCueFile.Source != SourceModule {
		t.Fatalf("discoverInDir() module cue source = %v, want SourceModule", moduleCueFile.Source)
	}

	extensionlessOnly := t.TempDir()
	writeTestFile(t, filepath.Join(extensionlessOnly, invowkfile.InvowkfileName), "legacy\n")
	legacyFile := d.discoverInDir(types.FilesystemPath(extensionlessOnly), SourceModule)
	if legacyFile == nil {
		t.Fatal("discoverInDir() returned nil for directory with extensionless invowkfile")
	}
	if filepath.Base(string(legacyFile.Path)) != invowkfile.InvowkfileName {
		t.Fatalf("discoverInDir() path = %s, want extensionless invowkfile", legacyFile.Path)
	}
	if legacyFile.Source != SourceModule {
		t.Fatalf("discoverInDir() source = %v, want SourceModule", legacyFile.Source)
	}

	if file := d.discoverInDir(types.FilesystemPath(t.TempDir()), SourceCurrentDir); file != nil {
		t.Fatalf("discoverInDir() = %#v, want nil for directory without invowkfile", file)
	}
}

func TestLoadProvisionedModulesWithDiagnostics_AppliesEntryContracts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	moduleRoot := filepath.Join(tmpDir, "modules")
	createTestModule(t, filepath.Join(moduleRoot, "one.invowkmod"), "one", "one")
	createTestModule(t, filepath.Join(moduleRoot, "two.invowkmod"), "two", "two")
	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))

	files, diagnostics := d.loadProvisionedModulesWithDiagnostics(ProvisionedModuleEntries{
		{},
		{Path: types.FilesystemPath("   ")},
		{Path: types.FilesystemPath(moduleRoot), CommandNamespace: "tools"},
	}, true)

	if len(files) != 2 {
		t.Fatalf("loadProvisionedModulesWithDiagnostics() files = %d, want 2", len(files))
	}
	assertDiagnosticCodePresent(t, diagnostics, CodeModuleScanPathInvalid)
	for _, file := range files {
		if file == nil || file.Module == nil {
			t.Fatalf("provisioned file = %#v, want loaded module", file)
		}
		if !file.IsGlobalModule {
			t.Fatalf("provisioned module %s IsGlobalModule = false, want true", file.Path)
		}
		if file.CommandNamespace != "tools" {
			t.Fatalf("provisioned module %s namespace = %q, want tools", file.Path, file.CommandNamespace)
		}
	}
}

func TestLoadProvisionedModuleWithDiagnostics_PreservesDirectModuleAttributes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	moduleDir := filepath.Join(tmpDir, "direct.invowkmod")
	createTestModule(t, moduleDir, "direct", "run")
	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))

	file, diagnostics := d.loadProvisionedModuleWithDiagnostics(ProvisionedModuleEntry{
		Path:             types.FilesystemPath(moduleDir),
		CommandNamespace: "direct",
	}, true)
	if len(diagnostics) != 0 {
		t.Fatalf("loadProvisionedModuleWithDiagnostics() diagnostics = %#v, want none", diagnostics)
	}
	if file == nil || file.Module == nil {
		t.Fatalf("loadProvisionedModuleWithDiagnostics() file = %#v, want loaded module", file)
	}
	if file.Path != file.Module.InvowkfilePath() {
		t.Fatalf("file path = %s, want module invowkfile path %s", file.Path, file.Module.InvowkfilePath())
	}
	if file.Source != SourceModule {
		t.Fatalf("file source = %v, want SourceModule", file.Source)
	}
	if file.CommandNamespace != "direct" {
		t.Fatalf("file namespace = %q, want direct", file.CommandNamespace)
	}
	if !file.IsGlobalModule {
		t.Fatal("file IsGlobalModule = false, want true")
	}

	reservedPath := types.FilesystemPath(filepath.Join(tmpDir, "invowkfile.invowkmod"))
	_, reservedDiagnostics := d.loadProvisionedModuleWithDiagnostics(ProvisionedModuleEntry{Path: reservedPath}, false)
	assertDiagnosticCodePresent(t, reservedDiagnostics, CodeReservedModuleNameSkipped)
}

func TestApplyProvisionedModuleAttributes_SetsOnlyRequestedFields(t *testing.T) {
	t.Parallel()

	files := []*DiscoveredFile{{Source: SourceModule}, {Source: SourceModule}}
	applyProvisionedModuleAttributes(files, false, "")
	for _, file := range files {
		if file.IsGlobalModule {
			t.Fatal("IsGlobalModule changed when isGlobal=false")
		}
		if file.CommandNamespace != "" {
			t.Fatalf("CommandNamespace = %q, want empty", file.CommandNamespace)
		}
	}

	applyProvisionedModuleAttributes(files, true, "tools")
	for _, file := range files {
		if !file.IsGlobalModule {
			t.Fatal("IsGlobalModule = false, want true")
		}
		if file.CommandNamespace != "tools" {
			t.Fatalf("CommandNamespace = %q, want tools", file.CommandNamespace)
		}
	}
}

func TestDiscoverModulesInDirWithDiagnostics_SkipsInvalidEntriesAndReportsDiagnostics(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	createTestModule(t, filepath.Join(tmpDir, "valid.invowkmod"), "valid", "run")
	createTestModule(t, filepath.Join(tmpDir, "invowkfile.invowkmod"), "invowkfile", "reserved")
	mkdirTest(t, filepath.Join(tmpDir, "broken.invowkmod"))
	mkdirTest(t, filepath.Join(tmpDir, "plain-directory"))
	writeTestFile(t, filepath.Join(tmpDir, "file.invowkmod"), "not a directory\n")

	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))
	files, diagnostics := d.discoverModulesInDirWithDiagnostics(types.FilesystemPath(tmpDir))

	if len(files) != 1 {
		t.Fatalf("discoverModulesInDirWithDiagnostics() files = %d, want 1", len(files))
	}
	if files[0].Module == nil || files[0].Module.Metadata.Module != "valid" {
		t.Fatalf("discovered module = %#v, want valid", files[0])
	}
	assertDiagnosticCodePresent(t, diagnostics, CodeReservedModuleNameSkipped)
	assertDiagnosticCodePresent(t, diagnostics, CodeModuleLoadSkipped)
}

func TestDiscoverModulesInDirWithDiagnostics_MissingDirectoryIsQuiet(t *testing.T) {
	t.Parallel()

	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))
	files, diagnostics := d.discoverModulesInDirWithDiagnostics(types.FilesystemPath(filepath.Join(t.TempDir(), "missing")))

	if len(files) != 0 {
		t.Fatalf("discoverModulesInDirWithDiagnostics() files = %d, want 0", len(files))
	}
	if len(diagnostics) != 0 {
		t.Fatalf("discoverModulesInDirWithDiagnostics() diagnostics = %#v, want none", diagnostics)
	}
}

func TestLoadIncludesWithDiagnostics_NilConfigAndReservedInclude(t *testing.T) {
	t.Parallel()

	nilConfigDiscovery := New(nil, WithBaseDir(""), WithCommandsDir(""))
	files, diagnostics := nilConfigDiscovery.loadIncludesWithDiagnostics()
	if len(files) != 0 || len(diagnostics) != 0 {
		t.Fatalf("loadIncludesWithDiagnostics() with nil config = files %#v diagnostics %#v, want none", files, diagnostics)
	}

	tmpDir := t.TempDir()
	reservedModulePath := filepath.Join(tmpDir, "invowkfile.invowkmod")
	createTestModule(t, reservedModulePath, "invowkfile", "reserved")
	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(reservedModulePath)}}
	d := New(cfg, WithBaseDir(""), WithCommandsDir(""))

	files, diagnostics = d.loadIncludesWithDiagnostics()
	if len(files) != 0 {
		t.Fatalf("loadIncludesWithDiagnostics() files = %#v, want none for reserved include", files)
	}
	assertDiagnosticCodePresent(t, diagnostics, CodeIncludeReservedSkipped)
}

func TestLoadIncludesWithDiagnostics_SkippedIncludesDoNotStopLaterValidIncludes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nonModulePath := filepath.Join(tmpDir, "plain")
	mkdirTest(t, nonModulePath)

	reservedModulePath := filepath.Join(tmpDir, "invowkfile.invowkmod")
	createTestModule(t, reservedModulePath, "invowkfile", "reserved")

	brokenModulePath := filepath.Join(tmpDir, "broken.invowkmod")
	mkdirTest(t, brokenModulePath)
	writeTestFile(t, filepath.Join(brokenModulePath, "invowkmod.cue"), "module: [")
	writeTestFile(t, filepath.Join(brokenModulePath, "invowkfile.cue"), "cmds: []\n")

	validModulePath := filepath.Join(tmpDir, "valid.invowkmod")
	createTestModule(t, validModulePath, "valid", "run")

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(nonModulePath)},
		{Path: config.ModuleIncludePath(reservedModulePath)},
		{Path: config.ModuleIncludePath(brokenModulePath)},
		{Path: config.ModuleIncludePath(validModulePath)},
	}
	d := New(cfg, WithBaseDir(""), WithCommandsDir(""))

	files, diagnostics := d.loadIncludesWithDiagnostics()
	if len(files) != 1 {
		t.Fatalf("loadIncludesWithDiagnostics() files = %d, want 1 after skipped includes", len(files))
	}
	if files[0].Module == nil || files[0].Module.Metadata.Module != "valid" {
		t.Fatalf("loadIncludesWithDiagnostics() file = %#v, want valid module", files[0])
	}
	assertDiagnosticCodePresent(t, diagnostics, CodeIncludeNotModule)
	assertDiagnosticCodePresent(t, diagnostics, CodeIncludeReservedSkipped)
	assertDiagnosticCodePresent(t, diagnostics, CodeIncludeModuleLoadFailed)
}

func TestDiscoverAllWithDiagnostics_PropagatesVendoredIntegrityErrors(t *testing.T) {
	t.Parallel()

	t.Run("local module", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		parentDir := filepath.Join(tmpDir, "parent.invowkmod")
		createTestModule(t, parentDir, "parent", "run")
		childDir := createVendoredModule(t, parentDir, "child.invowkmod", "child", "run")
		corruptVendoredModuleHash(t, childDir)

		d := newTestDiscovery(t, config.DefaultConfig(), tmpDir)
		files, diagnostics, err := d.discoverAllWithDiagnostics()
		if !errors.Is(err, invowkmod.ErrContentHashMismatch) {
			t.Fatalf("discoverAllWithDiagnostics() error = %v, want ErrContentHashMismatch", err)
		}
		if files != nil {
			t.Fatalf("discoverAllWithDiagnostics() files = %#v, want nil after hard integrity error", files)
		}
		if len(diagnostics) != 0 {
			t.Fatalf("discoverAllWithDiagnostics() diagnostics = %#v, want none before integrity error", diagnostics)
		}
	})

	t.Run("configured include", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		parentDir := filepath.Join(tmpDir, "included.invowkmod")
		createTestModule(t, parentDir, "included", "run")
		childDir := createVendoredModule(t, parentDir, "child.invowkmod", "child", "run")
		corruptVendoredModuleHash(t, childDir)

		cfg := config.DefaultConfig()
		cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(parentDir)}}
		d := New(cfg, WithBaseDir(""), WithCommandsDir(""))

		files, diagnostics, err := d.discoverAllWithDiagnostics()
		if !errors.Is(err, invowkmod.ErrContentHashMismatch) {
			t.Fatalf("discoverAllWithDiagnostics() error = %v, want ErrContentHashMismatch", err)
		}
		if files != nil {
			t.Fatalf("discoverAllWithDiagnostics() files = %#v, want nil after hard integrity error", files)
		}
		if len(diagnostics) != 0 {
			t.Fatalf("discoverAllWithDiagnostics() diagnostics = %#v, want none before integrity error", diagnostics)
		}
	})
}

func TestDiscoverVendoredModulesWithDiagnostics_NilAndMissingInputsAreQuiet(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nilMetadataParentDir := filepath.Join(tmpDir, "nilmetadataparent.invowkmod")
	createTestModule(t, nilMetadataParentDir, "nilmetadataparent", "run")
	createTestModule(
		t,
		filepath.Join(nilMetadataParentDir, invowkmod.VendoredModulesDir, "child.invowkmod"),
		"child",
		"run",
	)

	missingVendorParentDir := filepath.Join(tmpDir, "missingvendorparent.invowkmod")
	createTestModule(t, missingVendorParentDir, "missingvendorparent", "run")
	parent := loadTestModule(t, missingVendorParentDir)
	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))

	for _, tt := range []struct {
		name   string
		parent *invowkmod.Module
	}{
		{"nil parent", nil},
		{"nil metadata", &invowkmod.Module{Path: types.FilesystemPath(nilMetadataParentDir)}},
		{"missing vendor directory", parent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files, diagnostics, err := d.discoverVendoredModulesWithDiagnostics(tt.parent)
			if err != nil {
				t.Fatalf("discoverVendoredModulesWithDiagnostics() error = %v", err)
			}
			if len(files) != 0 || len(diagnostics) != 0 {
				t.Fatalf("discoverVendoredModulesWithDiagnostics() = files %#v diagnostics %#v, want none", files, diagnostics)
			}
		})
	}
}

func TestVendoredCommandNamespace_UsesDeclaredLockCommandSource(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	childDir := filepath.Join(tmpDir, "child.invowkmod")
	createTestModule(t, childDir, "child", "run")
	child := loadTestModule(t, childDir)
	requirements := []invowkmod.ModuleRequirement{{
		GitURL:  "https://example.com/child.git",
		Version: "^1.0.0",
	}}
	lock := invowkmod.NewLockFile()
	lock.Modules["https://example.com/child.git"] = invowkmod.LockedModule{
		GitURL:          "https://example.com/child.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Namespace:       "tools",
		CommandSourceID: "tools",
		ModuleID:        "child",
	}

	if got := vendoredCommandNamespace(nil, lock, nil); got != "" {
		t.Fatalf("vendoredCommandNamespace(nil child) = %q, want empty", got)
	}
	if got := vendoredCommandNamespace(nil, lock, &invowkmod.Module{}); got != "" {
		t.Fatalf("vendoredCommandNamespace(child without metadata) = %q, want empty", got)
	}
	if got := vendoredCommandNamespace(requirements, nil, child); got != "" {
		t.Fatalf("vendoredCommandNamespace(nil lock) = %q, want empty", got)
	}
	if got := vendoredCommandNamespace(requirements, lock, child); got != "tools" {
		t.Fatalf("vendoredCommandNamespace() = %q, want tools", got)
	}
}

func TestVendoredTransitiveDiagnostic_ReportsOnlyMissingTransitiveDeps(t *testing.T) {
	t.Parallel()

	childPath := types.FilesystemPath(filepath.Join(t.TempDir(), "child.invowkmod"))
	child := &invowkmod.Module{
		Metadata: &invowkmod.Invowkmod{
			Module: "io.example.child",
			Requires: []invowkmod.ModuleRequirement{{
				GitURL:  "https://example.com/grandchild.git",
				Version: "^1.0.0",
			}},
		},
	}
	parent := &invowkmod.Module{
		Metadata: &invowkmod.Invowkmod{
			Module: "io.example.parent",
			Requires: []invowkmod.ModuleRequirement{{
				GitURL:  "https://example.com/child.git",
				Version: "^1.0.0",
			}},
		},
	}

	if _, ok := vendoredTransitiveDiagnostic(nil, child, childPath); ok {
		t.Fatal("vendoredTransitiveDiagnostic(nil parent) reported diagnostic")
	}
	if _, ok := vendoredTransitiveDiagnostic(&invowkmod.Module{}, child, childPath); ok {
		t.Fatal("vendoredTransitiveDiagnostic(parent without metadata) reported diagnostic")
	}
	if _, ok := vendoredTransitiveDiagnostic(parent, nil, childPath); ok {
		t.Fatal("vendoredTransitiveDiagnostic(nil child) reported diagnostic")
	}

	diag, ok := vendoredTransitiveDiagnostic(parent, child, childPath)
	if !ok {
		t.Fatal("vendoredTransitiveDiagnostic() did not report missing transitive dependency")
	}
	if diag.Code() != CodeVendoredTransitiveSkipped {
		t.Fatalf("diagnostic code = %s, want %s", diag.Code(), CodeVendoredTransitiveSkipped)
	}
	if diag.Path() != childPath {
		t.Fatalf("diagnostic path = %s, want %s", diag.Path(), childPath)
	}
	if !strings.Contains(diag.Message(), "grandchild.git") {
		t.Fatalf("diagnostic message = %q, want missing dependency key", diag.Message())
	}
}

func TestJoinModuleRefKeysAndGetModuleShortName(t *testing.T) {
	t.Parallel()

	keys := []invowkmod.ModuleRefKey{"https://example.com/b.git", "https://example.com/a.git"}
	if got := joinModuleRefKeys(keys); got != "https://example.com/b.git, https://example.com/a.git" {
		t.Fatalf("joinModuleRefKeys() = %q, want input order joined", got)
	}

	modulePath := types.FilesystemPath(filepath.Join(t.TempDir(), "tools.invowkmod"))
	if got := getModuleShortName(modulePath); got != "tools" {
		t.Fatalf("getModuleShortName() = %q, want tools", got)
	}
}

func TestDetectModuleShadowing_ReportsOnlyLocalModulesThatMatchGlobalIDs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local", "shared.invowkmod")
	globalDir := filepath.Join(tmpDir, "global", "shared.invowkmod")
	otherDir := filepath.Join(tmpDir, "other.invowkmod")
	createTestModule(t, localDir, "shared", "local")
	createTestModule(t, globalDir, "shared", "global")
	createTestModule(t, otherDir, "other", "other")
	local := loadTestModule(t, localDir)
	global := loadTestModule(t, globalDir)
	other := loadTestModule(t, otherDir)
	d := New(config.DefaultConfig(), WithBaseDir(""), WithCommandsDir(""))

	if diagnostics := d.detectModuleShadowing([]*DiscoveredFile{
		{Path: local.InvowkfilePath(), Source: SourceModule, Module: local},
		{Path: other.InvowkfilePath(), Source: SourceModule, Module: other},
		{Path: types.FilesystemPath(filepath.Join(tmpDir, "global-without-module")), Source: SourceModule, IsGlobalModule: true},
	}); diagnostics != nil {
		t.Fatalf("detectModuleShadowing() without globals = %#v, want nil", diagnostics)
	}

	diagnostics := d.detectModuleShadowing([]*DiscoveredFile{
		{Path: global.InvowkfilePath(), Source: SourceModule, Module: global, IsGlobalModule: true},
		{Path: local.InvowkfilePath(), Source: SourceModule, Module: local},
		{Path: other.InvowkfilePath(), Source: SourceModule, Module: other},
		{Path: types.FilesystemPath(filepath.Join(tmpDir, "nomodule")), Source: SourceModule},
	})
	if len(diagnostics) != 1 {
		t.Fatalf("detectModuleShadowing() diagnostics = %#v, want exactly one", diagnostics)
	}
	diag := diagnostics[0]
	if diag.Code() != CodeModuleShadowsGlobal {
		t.Fatalf("diagnostic code = %s, want %s", diag.Code(), CodeModuleShadowsGlobal)
	}
	if diag.Path() != local.InvowkfilePath() {
		t.Fatalf("diagnostic path = %s, want local module path %s", diag.Path(), local.InvowkfilePath())
	}
	if !strings.Contains(diag.Message(), string(global.InvowkfilePath())) {
		t.Fatalf("diagnostic message = %q, want global path", diag.Message())
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func mkdirTest(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

func loadTestModule(t *testing.T, moduleDir string) *invowkmod.Module {
	t.Helper()

	module, err := invowkmod.Load(types.FilesystemPath(moduleDir))
	if err != nil {
		t.Fatalf("failed to load module %s: %v", moduleDir, err)
	}
	return module
}

func corruptVendoredModuleHash(t *testing.T, moduleDir string) {
	t.Helper()

	writeTestFile(t, filepath.Join(moduleDir, "hash-drift.txt"), "changed after lock\n")
}

func assertDiagnosticCodePresent(t *testing.T, diagnostics []Diagnostic, code DiagnosticCode) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Code() == code {
			return
		}
	}
	t.Fatalf("diagnostics %#v missing code %s", diagnostics, code)
}
