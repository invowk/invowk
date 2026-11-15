// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/pkg/types"
)

func provisionHome(home string, err error) basedir.Dirs {
	return basedir.New("linux", func(string) string { return "" }, func() (string, error) { return home, err })
}

func defaultedCacheDir(dirs basedir.Dirs) types.FilesystemPath {
	cfg := provision.DefaultConfig()
	cfg.BaseDirs = dirs
	applyHostProvisionDefaults(cfg)
	return cfg.CacheDir
}

func TestApplyHostProvisionDefaultsCacheDir(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name string
		dirs basedir.Dirs
		want types.FilesystemPath
	}{
		{name: "relative home leaves it empty", dirs: provisionHome(".", nil)},
		{name: "failing home lookup leaves it empty", dirs: provisionHome("", errors.New("no home"))},
		{name: "absolute missing home keeps the default", dirs: provisionHome(missing, nil), want: types.FilesystemPath(filepath.Join(missing, ".cache", "invowk", "provision"))},
		{name: "existing absolute home", dirs: provisionHome(existing, nil), want: types.FilesystemPath(filepath.Join(existing, ".cache", "invowk", "provision"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := defaultedCacheDir(tt.dirs); got != tt.want {
				t.Errorf("CacheDir = %q, want %q", got, tt.want)
			}
		})
	}
}

// legacyDefaultCacheDir is the CacheDir default of applyHostProvisionDefaults
// as it was on main before require-absolute-base-dirs, with the home lookup
// injected. It is the zero-change oracle (design D5 row c) and must not be
// edited.
func legacyDefaultCacheDir(userHomeDir func() (string, error)) types.FilesystemPath {
	if home, err := userHomeDir(); err == nil {
		return types.FilesystemPath(filepath.Join(home, ".cache", "invowk", "provision"))
	}
	return ""
}

// TestDefaultCacheDirMatchesLegacy: with the home absolute (existing or
// missing) or unset, the default CacheDir is unchanged (decision 15).
func TestDefaultCacheDirMatchesLegacy(t *testing.T) {
	t.Parallel()

	homes := map[string]func() (string, error){
		"absolute existing home": func() (string, error) { return t.TempDir(), nil },
		"absolute missing home":  func() (string, error) { return filepath.Join(t.TempDir(), "missing"), nil },
		"home unset":             func() (string, error) { return "", errors.New("$HOME is not defined") },
	}
	for name, lookup := range homes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home, err := lookup()
			fixed := func() (string, error) { return home, err }
			got := defaultedCacheDir(basedir.New("linux", func(string) string { return "" }, fixed))
			if want := legacyDefaultCacheDir(fixed); got != want {
				t.Errorf("CacheDir = %q, legacy %q", got, want)
			}
		})
	}
}
