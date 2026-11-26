// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// cwdDiscoveryOutcome is what one discovery run loaded and reported.
	cwdDiscoveryOutcome struct {
		files []*DiscoveredFile
		diags []Diagnostic
		err   error
	}
)

// discoverFromBase loads every discovered file with baseDir as the current
// directory and no user commands directory.
func discoverFromBase(t *testing.T, cfg *config.Config, baseDir string, opts ...Option) cwdDiscoveryOutcome {
	t.Helper()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	defaults := []Option{
		WithBaseDir(types.FilesystemPath(baseDir)),
		WithCommandsDir(""),
	}
	d := New(cfg, append(defaults, opts...)...)
	files, diags, err := d.loadAllWithDiagnostics()
	return cwdDiscoveryOutcome{files: files, diags: diags, err: err}
}

// moduleNames lists the module names of the loaded files, in order.
func (o cwdDiscoveryOutcome) moduleNames() []string {
	var names []string
	for _, f := range o.files {
		if f.Module != nil {
			names = append(names, string(f.Module.Name()))
		}
	}
	return names
}

func (o cwdDiscoveryOutcome) module(name string) *DiscoveredFile {
	for _, f := range o.files {
		if f.Module != nil && string(f.Module.Name()) == name {
			return f
		}
	}
	return nil
}

func (o cwdDiscoveryOutcome) plainFiles() []*DiscoveredFile {
	var plain []*DiscoveredFile
	for _, f := range o.files {
		if f.Module == nil {
			plain = append(plain, f)
		}
	}
	return plain
}

func (o cwdDiscoveryOutcome) hasCode(code DiagnosticCode) bool {
	return slices.ContainsFunc(o.diags, func(d Diagnostic) bool { return d.Code() == code })
}

// requireNothingLoaded fails when the run loaded any file or failed.
func (o cwdDiscoveryOutcome) requireNothingLoaded(t *testing.T, leg string) {
	t.Helper()
	if o.err != nil {
		t.Fatalf("%s: discovery error: %v", leg, o.err)
	}
	if len(o.files) != 0 {
		t.Fatalf("%s: loaded %d file(s) (modules %v), want none", leg, len(o.files), o.moduleNames())
	}
}

func (o cwdDiscoveryOutcome) requireCode(t *testing.T, leg string, code DiagnosticCode) {
	t.Helper()
	if !o.hasCode(code) {
		t.Fatalf("%s: missing %s diagnostic; got %v", leg, code, o.diags)
	}
}

func writeTreeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func commandSetFromBase(t *testing.T, cfg *config.Config, baseDir string) *DiscoveredCommandSet {
	t.Helper()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	d := New(cfg, WithBaseDir(types.FilesystemPath(baseDir)), WithCommandsDir(""))
	result, err := d.DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error: %v", err)
	}
	return result.Set
}

func TestDiscovery_CurrentDirModule_IdentityAttached(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		out := discoverFromBase(t, nil, base)
		if out.err != nil {
			t.Fatalf("%s: error: %v", leg, out.err)
		}
		if got := out.moduleNames(); !slices.Equal(got, []string{"tools"}) {
			t.Fatalf("%s: modules = %v, want [tools]", leg, got)
		}
		if plain := out.plainFiles(); len(plain) != 0 {
			t.Fatalf("%s: plain invowkfiles = %d, want 0", leg, len(plain))
		}
		file := out.module("tools")
		if file.Invowkfile == nil {
			t.Fatalf("%s: module invowkfile not parsed (error %v)", leg, file.Error)
		}
		if got := string(file.Invowkfile.ModulePath); got != moduleDir {
			t.Errorf("%s: ModulePath = %q, want %q", leg, got, moduleDir)
		}
		if file.Invowkfile.Metadata == nil {
			t.Errorf("%s: Metadata is nil", leg)
		}
		if !file.Invowkfile.IsFromModule() {
			t.Errorf("%s: IsFromModule() = false", leg)
		}

		set := commandSetFromBase(t, nil, base)
		if len(set.Commands) != 1 {
			t.Fatalf("%s: commands = %d, want 1", leg, len(set.Commands))
		}
		cmd := set.Commands[0]
		if cmd.SourceID != "tools" || cmd.SimpleName != "hello" || cmd.Name != "tools hello" {
			t.Errorf("%s: command = %q (simple %q, source %q), want \"tools hello\" from tools", leg, cmd.Name, cmd.SimpleName, cmd.SourceID)
		}
	}
}

func TestDiscovery_CurrentDirModule_LoadFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		needs   fstree.Capability
		prepare func(t *testing.T, moduleDir, outside string)
	}{
		{
			name:  "inner symbolic link",
			needs: fstree.CapSymlink,
			prepare: func(t *testing.T, moduleDir, outside string) {
				t.Helper()
				createTestModule(t, moduleDir, "tools", "hello")
				writeTreeFile(t, filepath.Join(outside, "secret.env"), "SECRET=1\n")
				fstree.MustSymlink(t, filepath.Join(outside, "secret.env"), filepath.Join(moduleDir, "link.env"))
			},
		},
		{
			name: "nested module",
			prepare: func(t *testing.T, moduleDir, _ string) {
				t.Helper()
				createTestModule(t, moduleDir, "tools", "hello")
				createTestModule(t, filepath.Join(moduleDir, "inner.invowkmod"), "inner", "nhello")
			},
		},
		{
			name: "module id does not match the directory",
			prepare: func(t *testing.T, moduleDir, _ string) {
				t.Helper()
				createTestModule(t, moduleDir, "com.example.other", "hello")
			},
		},
		{
			name: "missing invowkmod.cue",
			prepare: func(t *testing.T, moduleDir, _ string) {
				t.Helper()
				writeTreeFile(t, filepath.Join(moduleDir, "invowkfile.cue"), `cmds: []`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.needs != "" {
				fstree.Require(t, tt.needs)
			}
			parent := t.TempDir()
			moduleDir := filepath.Join(parent, "tools.invowkmod")
			tt.prepare(t, moduleDir, t.TempDir())

			for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
				out := discoverFromBase(t, nil, base)
				out.requireNothingLoaded(t, leg)
				out.requireCode(t, leg, CodeModuleLoadSkipped)
			}
		})
	}
}

// TestDiscovery_CurrentDirModule_NotLoadableName covers names the parent scan
// never loads as a module: from the parent they are ignored, and from inside
// they load nothing and are reported.
func TestDiscovery_CurrentDirModule_NotLoadableName(t *testing.T) {
	t.Parallel()

	for _, dirName := range []string{"1bad.invowkmod", "Tools.INVOWKMOD"} {
		t.Run(dirName, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			moduleDir := filepath.Join(parent, dirName)
			createTestModule(t, moduleDir, "tools", "hello")

			discoverFromBase(t, nil, parent).requireNothingLoaded(t, "parent")
			inside := discoverFromBase(t, nil, moduleDir)
			inside.requireNothingLoaded(t, "module dir")
			inside.requireCode(t, "module dir", CodeModuleLoadSkipped)
		})
	}
}

func TestDiscovery_CurrentDirModule_ReservedName(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "invowkfile.invowkmod")
	createTestModule(t, moduleDir, "invowkfile", "hello")

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		out := discoverFromBase(t, nil, base)
		out.requireNothingLoaded(t, leg)
		out.requireCode(t, leg, CodeReservedModuleNameSkipped)
	}
}

func TestDiscovery_CurrentDirModule_ExtensionlessInvowkfileNotRead(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	writeTreeFile(t, filepath.Join(moduleDir, "invowkmod.cue"), "module: \"tools\"\nversion: \"1.0.0\"\n")
	writeTreeFile(t, filepath.Join(moduleDir, "invowkfile"),
		`cmds: [{name: "plain", implementations: [{script: {content: "echo plain"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}]}]}]`)

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		if plain := discoverFromBase(t, nil, base).plainFiles(); len(plain) != 0 {
			t.Fatalf("%s: read plain invowkfile %s", leg, plain[0].Path)
		}
		if set := commandSetFromBase(t, nil, base); len(set.Commands) != 0 {
			t.Fatalf("%s: commands = %d, want 0", leg, len(set.Commands))
		}
	}
}

func TestDiscovery_CurrentDirModule_LibraryOnly(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "lib.invowkmod")
	writeTreeFile(t, filepath.Join(moduleDir, "invowkmod.cue"), "module: \"lib\"\nversion: \"1.0.0\"\n")

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		out := discoverFromBase(t, nil, base)
		if got := out.moduleNames(); !slices.Equal(got, []string{"lib"}) {
			t.Fatalf("%s: modules = %v, want [lib]", leg, got)
		}
		if out.hasCode(CodeInvowkfileParseSkipped) {
			t.Fatalf("%s: unexpected %s: %v", leg, CodeInvowkfileParseSkipped, out.diags)
		}
		if set := commandSetFromBase(t, nil, base); len(set.Commands) != 0 {
			t.Fatalf("%s: commands = %d, want 0", leg, len(set.Commands))
		}
	}
}

func TestDiscovery_CurrentDirModule_SymlinkedModuleDir(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	target := filepath.Join(t.TempDir(), "real.invowkmod")
	createTestModule(t, target, "real", "hello")
	parent := t.TempDir()
	link := filepath.Join(parent, "link.invowkmod")
	fstree.MustSymlink(t, target, link)

	for leg, base := range map[string]string{"parent": parent, "module dir": link} {
		out := discoverFromBase(t, nil, base)
		out.requireNothingLoaded(t, leg)
		out.requireCode(t, leg, CodeModuleSymlinkSkipped)
	}
}

func TestDiscovery_CurrentDirModule_SymlinkWithoutSuffix(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	target := filepath.Join(t.TempDir(), "tools.invowkmod")
	createTestModule(t, target, "tools", "hello")
	parent := t.TempDir()
	link := filepath.Join(parent, "tools")
	fstree.MustSymlink(t, target, link)

	discoverFromBase(t, nil, parent).requireNothingLoaded(t, "parent")
	inside := discoverFromBase(t, nil, link)
	inside.requireNothingLoaded(t, "module dir")
	inside.requireCode(t, "module dir", CodeModuleSymlinkSkipped)
}

func TestDiscovery_CurrentDirModule_VendoredPolicy(t *testing.T) {
	t.Parallel()

	t.Run("undeclared", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		moduleDir := filepath.Join(parent, "tools.invowkmod")
		createTestModule(t, moduleDir, "tools", "hello")
		createTestModule(t, filepath.Join(moduleDir, "invowk_modules", "vend.invowkmod"), "vend", "vhello")

		for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
			out := discoverFromBase(t, nil, base)
			if got := out.moduleNames(); !slices.Equal(got, []string{"tools"}) {
				t.Fatalf("%s: modules = %v, want [tools]", leg, got)
			}
			out.requireCode(t, leg, CodeVendoredUndeclaredSkipped)
		}
	})

	t.Run("declared and locked", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		moduleDir := filepath.Join(parent, "tools.invowkmod")
		createTestModule(t, moduleDir, "tools", "hello")
		createVendoredModule(t, moduleDir, "vend.invowkmod", "vend", "vhello")

		for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
			out := discoverFromBase(t, nil, base)
			if out.err != nil {
				t.Fatalf("%s: error: %v", leg, out.err)
			}
			if got := out.moduleNames(); !slices.Equal(got, []string{"tools", "vend"}) {
				t.Fatalf("%s: modules = %v, want [tools vend]", leg, got)
			}
			if vend := out.module("vend"); vend.ParentModule == nil || vend.ParentModule.Name() != "tools" {
				t.Fatalf("%s: vend ParentModule = %v, want tools", leg, vend.ParentModule)
			}
		}
	})
}

func TestDiscovery_CurrentDirModule_EmptyBaseDir(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	createTestModule(t, filepath.Join(parent, "tools.invowkmod"), "tools", "hello")

	// An empty base directory (--ivk-no-cwd, or an unavailable working
	// directory) loads nothing from the current directory.
	discoverFromBase(t, nil, "").requireNothingLoaded(t, "empty base")
}

func TestDiscovery_CurrentDirModule_PlainDirWithInvowkmodCue(t *testing.T) {
	t.Parallel()

	project := filepath.Join(t.TempDir(), "project")
	writeTreeFile(t, filepath.Join(project, "invowkmod.cue"), "module: \"project\"\nversion: \"1.0.0\"\n")
	writeTreeFile(t, filepath.Join(project, "invowkfile.cue"),
		`cmds: [{name: "build", implementations: [{script: {content: "echo build"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}]}]}]`)

	out := discoverFromBase(t, nil, project)
	if out.err != nil {
		t.Fatalf("error: %v", out.err)
	}
	plain := out.plainFiles()
	if len(plain) != 1 || plain[0].Source != SourceCurrentDir {
		t.Fatalf("plain files = %v, want the current-directory invowkfile", plain)
	}
	if names := out.moduleNames(); len(names) != 0 {
		t.Fatalf("modules = %v, want none", names)
	}
	set := commandSetFromBase(t, nil, project)
	if len(set.Commands) != 1 || set.Commands[0].SourceID != SourceIDInvowkfile {
		t.Fatalf("commands = %v, want build from %s", set.Commands, SourceIDInvowkfile)
	}
}

func TestDiscovery_CurrentDirModule_PrecedesIncludes(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")
	otherDir := filepath.Join(t.TempDir(), "other.invowkmod")
	createTestModule(t, otherDir, "other", "hello")

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(otherDir)}}

	for leg, base := range map[string]string{"parent": parent, "module dir": moduleDir} {
		out := discoverFromBase(t, cfg, base)
		if out.err != nil {
			t.Fatalf("%s: error: %v", leg, out.err)
		}
		if got := out.moduleNames(); !slices.Equal(got, []string{"tools", "other"}) {
			t.Fatalf("%s: modules = %v, want [tools other]", leg, got)
		}
	}
}

// TestDiscovery_CurrentDirModule_AncestorLinkStillModule: a link in an
// ancestor directory does not count, only the module directory's own entry.
func TestDiscovery_CurrentDirModule_AncestorLinkStillModule(t *testing.T) {
	t.Parallel()

	t.Run("unresolved temp dir", func(t *testing.T) {
		t.Parallel()
		// On macOS t.TempDir() runs through /var -> /private/var.
		moduleDir := filepath.Join(t.TempDir(), "tools.invowkmod")
		createTestModule(t, moduleDir, "tools", "hello")
		if got := discoverFromBase(t, nil, moduleDir).moduleNames(); !slices.Equal(got, []string{"tools"}) {
			t.Fatalf("modules = %v, want [tools]", got)
		}
	})

	t.Run("linked ancestor", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)
		root := fstree.TempRoot(t)
		realDir := filepath.Join(root, "real")
		createTestModule(t, filepath.Join(realDir, "tools.invowkmod"), "tools", "hello")
		link := filepath.Join(root, "link")
		fstree.MustSymlink(t, realDir, link)

		out := discoverFromBase(t, nil, filepath.Join(link, "tools.invowkmod"))
		if out.err != nil {
			t.Fatalf("error: %v", out.err)
		}
		if got := out.moduleNames(); !slices.Equal(got, []string{"tools"}) {
			t.Fatalf("modules = %v, want [tools]", got)
		}
	})
}

func TestDiscovery_CurrentDirModule_CaseVariantCwd(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")
	variant := filepath.Join(parent, "TOOLS.INVOWKMOD")

	inside := discoverFromBase(t, nil, variant)
	if _, err := os.Lstat(variant); err != nil {
		// Case-sensitive volume: the variant does not exist.
		inside.requireNothingLoaded(t, "case variant")
		inside.requireCode(t, "case variant", CodeModuleLoadSkipped)
		return
	}

	fromParent := discoverFromBase(t, nil, parent)
	if inside.err != nil || fromParent.err != nil {
		t.Fatalf("errors: inside %v, parent %v", inside.err, fromParent.err)
	}
	if got, want := inside.moduleNames(), fromParent.moduleNames(); !slices.Equal(got, want) || !slices.Equal(got, []string{"tools"}) {
		t.Fatalf("modules inside = %v, from parent = %v, want [tools]", got, want)
	}
	if got := string(inside.module("tools").Module.Path); got != moduleDir {
		t.Fatalf("module path = %q, want the stored name %q", got, moduleDir)
	}
	if len(inside.diags) != len(fromParent.diags) {
		t.Fatalf("diagnostics inside = %v, from parent = %v", inside.diags, fromParent.diags)
	}
}

func TestDiscovery_CurrentDirModule_JunctionModuleDir(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)

	target := filepath.Join(t.TempDir(), "real.invowkmod")
	createTestModule(t, target, "real", "hello")
	parent := t.TempDir()
	junction := filepath.Join(parent, "j.invowkmod")
	fstree.MustJunction(t, target, junction)

	discoverFromBase(t, nil, parent).requireNothingLoaded(t, "parent")
	inside := discoverFromBase(t, nil, junction)
	inside.requireNothingLoaded(t, "module dir")
	inside.requireCode(t, "module dir", CodeModuleSymlinkSkipped)
}

func TestDiscovery_CurrentDirModule_JunctionWithoutSuffix(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)

	target := filepath.Join(t.TempDir(), "tools.invowkmod")
	createTestModule(t, target, "tools", "hello")
	junction := filepath.Join(t.TempDir(), "tools")
	fstree.MustJunction(t, target, junction)

	inside := discoverFromBase(t, nil, junction)
	inside.requireNothingLoaded(t, "module dir")
	inside.requireCode(t, "module dir", CodeModuleSymlinkSkipped)
}

func TestDiscoverCurrentDirModule_UnreadableEntry(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "gone.invowkmod")
	isModuleDir, files, diags := discoverCurrentDirModule(types.FilesystemPath(missing))
	if !isModuleDir || len(files) != 0 {
		t.Fatalf("missing candidate: isModuleDir=%v files=%d, want true and none", isModuleDir, len(files))
	}
	if len(diags) != 1 || diags[0].Code() != CodeModuleLoadSkipped {
		t.Fatalf("diagnostics = %v, want one %s", diags, CodeModuleLoadSkipped)
	}
	if !errors.Is(diags[0].Cause(), os.ErrNotExist) {
		t.Fatalf("cause = %v, want not-exist", diags[0].Cause())
	}

	plainMissing := filepath.Join(t.TempDir(), "gone")
	if isModuleDir, files, diags := discoverCurrentDirModule(types.FilesystemPath(plainMissing)); isModuleDir || len(files) != 0 || len(diags) != 0 {
		t.Fatalf("missing plain dir: isModuleDir=%v files=%d diags=%v, want false and nothing", isModuleDir, len(files), diags)
	}
}

// TestDiscovery_CurrentDirModule_SymlinkChainWithoutSuffix: a chain of links
// without the suffix that ends at a module directory is judged by the end of
// the chain.
func TestDiscovery_CurrentDirModule_SymlinkChainWithoutSuffix(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	root := fstree.TempRoot(t)
	target := filepath.Join(root, "real", "tools.invowkmod")
	createTestModule(t, target, "tools", "hello")
	mid := filepath.Join(root, "mid")
	fstree.MustSymlink(t, target, mid)
	link := filepath.Join(root, "tools")
	fstree.MustSymlink(t, mid, link)

	inside := discoverFromBase(t, nil, link)
	inside.requireNothingLoaded(t, "module dir")
	inside.requireCode(t, "module dir", CodeModuleSymlinkSkipped)
}

// TestDiscovery_CurrentDirModule_JunctionChainWithoutSuffix: filepath.EvalSymlinks
// does not follow junctions, so a junction to a junction to a module directory
// is followed hop by hop.
func TestDiscovery_CurrentDirModule_JunctionChainWithoutSuffix(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)

	target := filepath.Join(t.TempDir(), "tools.invowkmod")
	createTestModule(t, target, "tools", "hello")
	links := t.TempDir()
	mid := filepath.Join(links, "mid")
	fstree.MustJunction(t, target, mid)
	junction := filepath.Join(links, "tools")
	fstree.MustJunction(t, mid, junction)

	inside := discoverFromBase(t, nil, junction)
	inside.requireNothingLoaded(t, "module dir")
	inside.requireCode(t, "module dir", CodeModuleSymlinkSkipped)
}

func TestLinkChainTargets(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	t.Run("every hop is a target", func(t *testing.T) {
		t.Parallel()
		root := fstree.TempRoot(t)
		end := filepath.Join(root, "end.invowkmod")
		if err := os.Mkdir(end, 0o750); err != nil {
			t.Fatal(err)
		}
		hop := filepath.Join(root, "hop")
		fstree.MustSymlink(t, end, hop)

		targets, err := linkChainTargets(types.FilesystemPath(hop))
		if err != nil {
			t.Fatalf("linkChainTargets: %v", err)
		}
		if !slices.ContainsFunc(targets, func(p types.FilesystemPath) bool { return filepath.Base(string(p)) == "hop" }) ||
			!anyModuleSuffixFold(targets) {
			t.Fatalf("targets = %v, want the hop and the module-named end", targets)
		}
	})

	t.Run("dangling end", func(t *testing.T) {
		t.Parallel()
		root := fstree.TempRoot(t)
		hop := filepath.Join(root, "hop")
		fstree.MustSymlink(t, filepath.Join(root, "missing"), hop)

		targets, err := linkChainTargets(types.FilesystemPath(hop))
		if err != nil {
			t.Fatalf("linkChainTargets: %v", err)
		}
		if anyModuleSuffixFold(targets) {
			t.Fatalf("targets = %v, want no module name", targets)
		}
	})

	t.Run("loop", func(t *testing.T) {
		t.Parallel()
		root := fstree.TempRoot(t)
		a := filepath.Join(root, "a")
		b := filepath.Join(root, "b")
		fstree.MustSymlink(t, b, a)
		fstree.MustSymlink(t, a, b)

		if _, err := linkChainTargets(types.FilesystemPath(a)); err == nil {
			t.Fatal("linkChainTargets of a loop: want an error")
		}
	})
}

func TestUnstoredEntryVerdict(t *testing.T) {
	t.Parallel()

	path := types.FilesystemPath(filepath.Join(t.TempDir(), "dir"))
	evalErr := errors.New("resolve links: unsupported volume")
	tests := []struct {
		name       string
		candidate  bool
		volumeRoot bool
		evalErr    error
		wantModule bool
	}{
		{name: "candidate", candidate: true, wantModule: true},
		{name: "candidate volume root", candidate: true, volumeRoot: true, wantModule: true},
		{name: "plain, physical name known", wantModule: false},
		{name: "plain, physical name unknown", evalErr: evalErr, wantModule: true},
		{name: "volume root, physical name unknown", volumeRoot: true, evalErr: evalErr, wantModule: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isModuleDir, files, diags := unstoredEntryVerdict(path, tt.candidate, tt.volumeRoot, tt.evalErr)
			if isModuleDir != tt.wantModule || len(files) != 0 {
				t.Fatalf("isModuleDir=%v files=%d, want %v and none", isModuleDir, len(files), tt.wantModule)
			}
			if tt.wantModule && (len(diags) != 1 || diags[0].Code() != CodeModuleLoadSkipped) {
				t.Fatalf("diagnostics = %v, want one %s", diags, CodeModuleLoadSkipped)
			}
			if !tt.wantModule && len(diags) != 0 {
				t.Fatalf("diagnostics = %v, want none", diags)
			}
		})
	}
}

func TestDiscovery_CurrentDirModule_InsideSubdirectoriesFailsClosed(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	modDir := filepath.Join(ws, "tools.invowkmod")
	createTestModule(t, modDir, "tools", "hello")

	// Plain invowkfile and child module inside a subdirectory must never load
	// when running directly inside that subdirectory.
	const plainSubCue = `cmds: [{name: "subcmd", implementations: [{script: "echo SUB", runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`
	subDir := filepath.Join(modDir, "sub")
	writeTreeFile(t, filepath.Join(subDir, "invowkfile.cue"), plainSubCue)
	childMod := filepath.Join(subDir, "child.invowkmod")
	createTestModule(t, childMod, "child", "childcmd")

	vendRoot := filepath.Join(modDir, "invowk_modules")
	vendMod := filepath.Join(vendRoot, "vend.invowkmod")
	createTestModule(t, vendMod, "vend", "vendcmd")

	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "subdir", dir: subDir},
		{name: "nested child module", dir: childMod},
		{name: "invowk_modules", dir: vendRoot},
		{name: "vendored module root", dir: vendMod},
		{name: "nonexistent subdir", dir: filepath.Join(modDir, "missing")},
	} {
		out := discoverFromBase(t, nil, tc.dir)
		out.requireNothingLoaded(t, tc.name)
		out.requireCode(t, tc.name, CodeModuleLoadSkipped)
	}

	// Uppercase ancestor suffix also fails closed.
	upperMod := filepath.Join(ws, "UPPER.INVOWKMOD")
	upperSub := filepath.Join(upperMod, "sub")
	writeTreeFile(t, filepath.Join(upperSub, "invowkfile.cue"), plainSubCue)
	upperOut := discoverFromBase(t, nil, upperSub)
	upperOut.requireNothingLoaded(t, "uppercase ancestor")
	upperOut.requireCode(t, "uppercase ancestor", CodeModuleLoadSkipped)

	// Control: a module root directory and an unrelated sibling plain dir still load normally.
	cleanMod := filepath.Join(ws, "clean.invowkmod")
	createTestModule(t, cleanMod, "clean", "hello")
	rootOut := discoverFromBase(t, nil, cleanMod)
	if rootOut.err != nil || !slices.Equal(rootOut.moduleNames(), []string{"clean"}) {
		t.Fatalf("module root load failed: err=%v modules=%v diags=%v", rootOut.err, rootOut.moduleNames(), rootOut.diags)
	}
	plainSub := filepath.Join(ws, "plain", "sub")
	writeTreeFile(t, filepath.Join(plainSub, "invowkfile.cue"), plainSubCue)
	plainOut := discoverFromBase(t, nil, plainSub)
	if plainOut.err != nil || len(plainOut.plainFiles()) != 1 {
		t.Fatalf("plain subdir load failed: err=%v plain=%d", plainOut.err, len(plainOut.plainFiles()))
	}
}

func TestDiscovery_CurrentDirModule_LinkIntoModuleSubdirFailsClosed(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	ws := fstree.TempRoot(t)
	modDir := filepath.Join(ws, "tools.invowkmod")
	createTestModule(t, modDir, "tools", "hello")
	subDir := filepath.Join(modDir, "sub")
	writeTreeFile(t, filepath.Join(subDir, "invowkfile.cue"), `cmds: [{name: "subcmd", implementations: [{script: "echo SUB", runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`)
	vendMod := filepath.Join(modDir, "invowk_modules", "vend.invowkmod")
	createTestModule(t, vendMod, "vend", "vendcmd")

	links := fstree.TempRoot(t)
	modAlias := filepath.Join(links, "mod_alias")
	fstree.MustSymlink(t, modDir, modAlias)
	subLink := filepath.Join(links, "sub_link")
	fstree.MustSymlink(t, subDir, subLink)
	vendLink := filepath.Join(links, "vend_link")
	fstree.MustSymlink(t, vendMod, vendLink)

	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "symlink ancestor sub", dir: filepath.Join(modAlias, "sub")},
		{name: "symlink ancestor missing", dir: filepath.Join(modAlias, "missing")},
		{name: "direct symlink to sub", dir: subLink},
		{name: "direct symlink to vendored module", dir: vendLink},
	} {
		out := discoverFromBase(t, nil, tc.dir)
		out.requireNothingLoaded(t, tc.name)
		out.requireCode(t, tc.name, CodeModuleLoadSkipped)
	}
}

func TestDiscovery_CurrentDirModule_JunctionAncestorFailsClosed(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)

	ws := t.TempDir()
	modDir := filepath.Join(ws, "tools.invowkmod")
	createTestModule(t, modDir, "tools", "hello")
	subDir := filepath.Join(modDir, "sub")
	writeTreeFile(t, filepath.Join(subDir, "invowkfile.cue"), `cmds: [{name: "subcmd", implementations: [{script: "echo SUB", runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`)

	links := t.TempDir()
	modJunc := filepath.Join(links, "mod_junc")
	fstree.MustJunction(t, modDir, modJunc)
	subJunc := filepath.Join(links, "sub_junc")
	fstree.MustJunction(t, subDir, subJunc)

	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "junction ancestor sub", dir: filepath.Join(modJunc, "sub")},
		{name: "direct junction to sub", dir: subJunc},
	} {
		out := discoverFromBase(t, nil, tc.dir)
		out.requireNothingLoaded(t, tc.name)
		out.requireCode(t, tc.name, CodeModuleLoadSkipped)
	}
}
