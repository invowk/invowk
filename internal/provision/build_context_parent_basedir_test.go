// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/pkg/types"
)

const (
	parentCache parentKind = "cache"
	parentHome  parentKind = "home invowk-build"
	parentTemp  parentKind = "temp"
	parentCwd   parentKind = "cwd .invowk-build"
)

type parentKind string

func buildParentDirs(home string, err error) basedir.Dirs {
	return basedir.New("linux", func(string) string { return "" }, func() (string, error) { return home, err })
}

func classifyParent(t *testing.T, parent types.FilesystemPath, cacheDir, home, cwd string) parentKind {
	t.Helper()
	switch p := string(parent); {
	case cacheDir != "" && p == cacheDir:
		return parentCache
	case home != "" && p == filepath.Join(home, "invowk-build"):
		return parentHome
	case p == filepath.Join(cwd, ".invowk-build"):
		return parentCwd
	case strings.HasPrefix(filepath.Base(p), "invowk-build-") && filepath.Dir(p) == filepath.Clean(os.TempDir()):
		return parentTemp
	default:
		t.Fatalf("unclassified build-context parent %q", p)
		return ""
	}
}

// TestResolveBuildContextParentNeverUsesCwd changes the working directory,
// so it does not run in parallel.
func TestResolveBuildContextParentNeverUsesCwd(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	work := t.TempDir()
	t.Chdir(work)

	tests := map[string]basedir.Dirs{
		"relative home":         buildParentDirs(".", nil),
		"absolute missing home": buildParentDirs(filepath.Join(t.TempDir(), "missing"), nil),
	}
	for name, dirs := range tests { //nolint:paralleltest // subtests share the t.Chdir working directory.
		t.Run(name, func(t *testing.T) {
			p := &LayerProvisioner{config: &Config{BaseDirs: dirs}}
			parent, cleanup, err := p.resolveBuildContextParent()
			if err != nil {
				t.Fatalf("resolveBuildContextParent() error: %v", err)
			}
			if cleanup == nil {
				t.Fatal("temp parent has no cleanup")
			}
			defer cleanup()
			if kind := classifyParent(t, parent, "", "", work); kind != parentTemp {
				t.Errorf("parent %q is %s, want a temp parent", parent, kind)
			}
			entries, readErr := os.ReadDir(work)
			if readErr != nil || len(entries) != 0 {
				t.Errorf("working directory entries = %v (err %v), want none", entries, readErr)
			}
		})
	}
}

// legacyResolveBuildContextParentKind is the branch choice of
// resolveBuildContextParent as it was on main before
// require-absolute-base-dirs, with the home lookup injected. It is the
// zero-change oracle (design D5 row d) and must not be edited.
func legacyResolveBuildContextParentKind(cacheDir string, userHomeDir func() (string, error)) parentKind {
	if cacheDir != "" {
		return parentCache
	}
	if home, homeErr := userHomeDir(); homeErr == nil {
		if _, statErr := os.Stat(home); statErr == nil {
			return parentHome
		}
	}
	if _, cwdErr := os.Getwd(); cwdErr == nil {
		return parentCwd
	}
	return parentTemp
}

// TestResolveBuildContextParentMatchesLegacy: with a cache dir set or the
// home absolute or unset, the parent is unchanged, except where the legacy
// code chose <cwd>/.invowk-build. Those rows are the documented BREAKING
// difference (decision 5) and must now be a temp parent.
func TestResolveBuildContextParentMatchesLegacy(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")
	cacheDir := filepath.Join(t.TempDir(), "cache")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	homes := map[string]struct {
		home string
		err  error
	}{
		"absolute existing home": {home: existing},
		"absolute missing home":  {home: missing},
		"home unset":             {err: errors.New("$HOME is not defined")},
	}
	for homeName, h := range homes {
		for _, cache := range []string{"", cacheDir} {
			t.Run(homeName+"/cache="+cache, func(t *testing.T) {
				t.Parallel()
				lookup := func() (string, error) { return h.home, h.err }
				p := &LayerProvisioner{config: &Config{CacheDir: types.FilesystemPath(cache), BaseDirs: basedir.New("linux", nil, lookup)}}
				parent, cleanup, resolveErr := p.resolveBuildContextParent()
				if resolveErr != nil {
					t.Fatalf("resolveBuildContextParent() error: %v", resolveErr)
				}
				if cleanup != nil {
					defer cleanup()
				}
				got := classifyParent(t, parent, cache, h.home, cwd)
				want := legacyResolveBuildContextParentKind(cache, lookup)
				if want == parentCwd {
					want = parentTemp // BREAKING row: the cwd fallback is removed.
				}
				if got != want {
					t.Errorf("parent kind = %s, want %s", got, want)
				}
			})
		}
	}
}
