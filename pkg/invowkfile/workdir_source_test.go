// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

// workDirLevels holds one workdir value per precedence level for the drift test.
type workDirLevels struct {
	cli, impl, cmd, root WorkDir
}

func (l workDirLevels) build(base string) (inv *Invowkfile, cmd *Command, impl *Implementation, cliOverride WorkDir) {
	inv = &Invowkfile{FilePath: FilesystemPath(filepath.Join(base, "invowkfile.cue")), WorkDir: l.root}
	cmd = &Command{Name: "test", WorkDir: l.cmd}
	impl = &Implementation{Script: ImplementationScript{Content: "echo test"}, WorkDir: l.impl}
	return inv, cmd, impl, l.cli
}

// only keeps the level named by source and clears every other one.
func (l workDirLevels) only(source WorkDirSource) workDirLevels {
	switch source {
	case WorkDirSourceCLIOverride:
		return workDirLevels{cli: l.cli}
	case WorkDirSourceImplementation:
		return workDirLevels{impl: l.impl}
	case WorkDirSourceCommand:
		return workDirLevels{cmd: l.cmd}
	case WorkDirSourceRoot:
		return workDirLevels{root: l.root}
	case WorkDirSourceDefault:
		return workDirLevels{}
	default:
		return workDirLevels{}
	}
}

// TestResolveEffectiveWorkDir is the drift test between GetEffectiveWorkDir
// and effectiveWorkDirSource: for all 16 combinations of the four declared
// levels, the level named by Source must reproduce Path on its own, and Source
// must be the highest level that is set.
func TestResolveEffectiveWorkDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	all := workDirLevels{cli: "cli-dir", impl: "impl-dir", cmd: "cmd-dir", root: "root-dir"}
	for mask := range 16 {
		levels := workDirLevels{}
		if mask&1 != 0 {
			levels.cli = all.cli
		}
		if mask&2 != 0 {
			levels.impl = all.impl
		}
		if mask&4 != 0 {
			levels.cmd = all.cmd
		}
		if mask&8 != 0 {
			levels.root = all.root
		}
		want := WorkDirSourceDefault
		switch {
		case levels.cli != "":
			want = WorkDirSourceCLIOverride
		case levels.impl != "":
			want = WorkDirSourceImplementation
		case levels.cmd != "":
			want = WorkDirSourceCommand
		case levels.root != "":
			want = WorkDirSourceRoot
		}

		t.Run(fmt.Sprintf("mask=%04b", mask), func(t *testing.T) {
			t.Parallel()

			inv, cmd, impl, cli := levels.build(base)
			got := inv.ResolveEffectiveWorkDir(cmd, impl, cli)
			if got.Source != want {
				t.Fatalf("Source = %q, want %q", got.Source, want)
			}
			if path := inv.GetEffectiveWorkDir(cmd, impl, cli); got.Path != path {
				t.Fatalf("Path = %q, GetEffectiveWorkDir = %q", got.Path, path)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}

			soloInv, soloCmd, soloImpl, soloCLI := levels.only(got.Source).build(base)
			if solo := soloInv.GetEffectiveWorkDir(soloCmd, soloImpl, soloCLI); solo != got.Path {
				t.Fatalf("level %q alone resolves to %q, want %q", got.Source, solo, got.Path)
			}
		})
	}
}

// TestResolveEffectiveWorkDir_NilCommandAndImplementation checks that a nil
// command or implementation is skipped, as GetEffectiveWorkDir skips it.
func TestResolveEffectiveWorkDir_NilCommandAndImplementation(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	inv := &Invowkfile{FilePath: FilesystemPath(filepath.Join(base, "invowkfile.cue")), WorkDir: "root-dir"}
	got := inv.ResolveEffectiveWorkDir(nil, nil, "")
	if got.Source != WorkDirSourceRoot {
		t.Fatalf("Source = %q, want %q", got.Source, WorkDirSourceRoot)
	}
	if want := FilesystemPath(filepath.Join(base, "root-dir")); got.Path != want {
		t.Fatalf("Path = %q, want %q", got.Path, want)
	}
}

// TestResolveEffectiveWorkDir_InvalidPathFallback covers the one case where the
// two can disagree: an invalid workdir value falls back to the invowkfile
// directory while Source still names its level (design D1). The fallback is
// the script base, so the root decision is the same either way.
func TestResolveEffectiveWorkDir_InvalidPathFallback(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	inv := &Invowkfile{FilePath: FilesystemPath(filepath.Join(base, "invowkfile.cue"))}
	cmd := &Command{Name: "test", WorkDir: "   "}
	got := inv.ResolveEffectiveWorkDir(cmd, nil, "")
	if got.Source != WorkDirSourceCommand {
		t.Fatalf("Source = %q, want %q", got.Source, WorkDirSourceCommand)
	}
	if got.Path != inv.GetScriptBasePath() {
		t.Fatalf("Path = %q, want the script base %q", got.Path, inv.GetScriptBasePath())
	}
}

func TestWorkDirSource_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source   WorkDirSource
		declared bool
	}{
		{WorkDirSourceCLIOverride, false},
		{WorkDirSourceImplementation, true},
		{WorkDirSourceCommand, true},
		{WorkDirSourceRoot, true},
		{WorkDirSourceDefault, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			t.Parallel()

			if err := tt.source.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if got := tt.source.DeclaredByInvowkfile(); got != tt.declared {
				t.Fatalf("DeclaredByInvowkfile() = %v, want %v", got, tt.declared)
			}
			if got := tt.source.String(); got != string(tt.source) {
				t.Fatalf("String() = %q, want %q", got, tt.source)
			}
		})
	}

	for _, invalid := range []WorkDirSource{"", "module", "CLI-OVERRIDE"} {
		t.Run(fmt.Sprintf("invalid %q", invalid), func(t *testing.T) {
			t.Parallel()

			err := invalid.Validate()
			if !errors.Is(err, ErrInvalidWorkDirSource) {
				t.Fatalf("Validate() = %v, want ErrInvalidWorkDirSource", err)
			}
			var typed *InvalidWorkDirSourceError
			if !errors.As(err, &typed) || typed.Value != invalid {
				t.Fatalf("Validate() = %v, want *InvalidWorkDirSourceError{%q}", err, invalid)
			}
			if invalid.DeclaredByInvowkfile() {
				t.Fatal("DeclaredByInvowkfile() = true for an invalid source")
			}
		})
	}
}

func TestEffectiveWorkDir_Validate(t *testing.T) {
	t.Parallel()

	if err := (EffectiveWorkDir{Path: "dir", Source: WorkDirSourceRoot}).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	err := (EffectiveWorkDir{Path: " ", Source: "bogus"}).Validate()
	if !errors.Is(err, ErrInvalidFilesystemPath) || !errors.Is(err, ErrInvalidWorkDirSource) {
		t.Fatalf("Validate() = %v, want both the path and the source errors", err)
	}
}
