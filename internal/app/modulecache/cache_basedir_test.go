// SPDX-License-Identifier: MPL-2.0

package modulecache

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/pkg/types"
)

func envOf(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestDefaultDirFrom(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	absHome := basedir.New("linux", nil, func() (string, error) { return home, nil })
	relHome := basedir.New("linux", nil, func() (string, error) { return ".", nil })
	explicit := filepath.Join(t.TempDir(), "modules")

	t.Run("relative env rejected", func(t *testing.T) {
		t.Parallel()
		_, err := DefaultDirFrom(envOf(map[string]string{ModuleCachePathEnv: ".modules"}), absHome)
		var typed *ModulesPathNotAbsoluteError
		if !errors.As(err, &typed) || !errors.Is(err, basedir.ErrNotAbsolute) {
			t.Fatalf("err = %v, want *ModulesPathNotAbsoluteError wrapping ErrNotAbsolute", err)
		}
		if msg := err.Error(); !strings.Contains(msg, ModuleCachePathEnv) || !strings.Contains(msg, "absolute") {
			t.Errorf("error %q should name %s and ask for an absolute path", msg, ModuleCachePathEnv)
		}
	})
	t.Run("relative home rejected", func(t *testing.T) {
		t.Parallel()
		if got, err := DefaultDirFrom(envOf(nil), relHome); !errors.Is(err, basedir.ErrNotAbsolute) {
			t.Fatalf("DefaultDirFrom() = (%q, %v), want ErrNotAbsolute", got, err)
		}
	})
	t.Run("absolute env unchanged", func(t *testing.T) {
		t.Parallel()
		got, err := DefaultDirFrom(envOf(map[string]string{ModuleCachePathEnv: explicit}), relHome)
		if err != nil || got != types.FilesystemPath(explicit) {
			t.Fatalf("DefaultDirFrom() = (%q, %v), want %q", got, err, explicit)
		}
	})
	t.Run("absolute home default", func(t *testing.T) {
		t.Parallel()
		got, err := DefaultDirFrom(envOf(nil), absHome)
		if want := types.FilesystemPath(filepath.Join(home, ".invowk", DefaultModulesDir)); err != nil || got != want {
			t.Fatalf("DefaultDirFrom() = (%q, %v), want %q", got, err, want)
		}
	})
}

// legacyDefaultDirWith is DefaultDirWith as it was on main before
// require-absolute-base-dirs, with the home lookup injected. It is the
// zero-change oracle (design D5) and must not be edited.
func legacyDefaultDirWith(getenv func(string) string, userHomeDir func() (string, error)) (types.FilesystemPath, error) {
	if envPath := getenv(ModuleCachePathEnv); envPath != "" {
		cachePath := types.FilesystemPath(envPath)
		if err := cachePath.Validate(); err != nil {
			return "", fmt.Errorf("module cache path from %s: %w", ModuleCachePathEnv, err)
		}
		return cachePath, nil
	}

	homeDir, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	cachePath := types.FilesystemPath(filepath.Join(homeDir, ".invowk", DefaultModulesDir))
	if err := cachePath.Validate(); err != nil {
		return "", fmt.Errorf("module cache path: %w", err)
	}
	return cachePath, nil
}

// TestDefaultDirFromMatchesLegacy proves the zero-change invariant (design
// D5 row f): with INVOWK_MODULES_PATH and the home absolute or unset, the
// result and the error text are the same as before.
func TestDefaultDirFromMatchesLegacy(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")
	explicit := filepath.Join(t.TempDir(), "modules")
	homes := map[string]func() (string, error){
		"absolute existing home": func() (string, error) { return existing, nil },
		"absolute missing home":  func() (string, error) { return missing, nil },
		"home unset":             func() (string, error) { return "", errors.New("$HOME is not defined") },
	}
	envs := map[string]map[string]string{
		"INVOWK_MODULES_PATH unset":    nil,
		"INVOWK_MODULES_PATH absolute": {ModuleCachePathEnv: explicit},
	}
	for homeName, home := range homes {
		for envName, env := range envs {
			t.Run(homeName+"/"+envName, func(t *testing.T) {
				t.Parallel()
				want, wantErr := legacyDefaultDirWith(envOf(env), home)
				got, gotErr := DefaultDirFrom(envOf(env), basedir.New("linux", envOf(env), home))
				if got != want || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
					t.Errorf("DefaultDirFrom() = (%q, %v), legacy = (%q, %v)", got, gotErr, want, wantErr)
				}
			})
		}
	}
}
