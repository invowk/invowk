// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

// legacyConfigDirFrom, legacyAbsoluteEnv, legacyAbsoluteHomeDir and
// legacyCommandsDirFrom are configDirFrom and its helpers as they were on main
// (3760a4ba) before require-absolute-base-dirs, copied verbatim except for the
// names. They are the zero-change oracle (design D5 row g) and must not be
// edited.
func legacyConfigDirFrom(goos string, getenv func(string) string, userHomeDir func() (string, error)) (types.FilesystemPath, error) {
	var configDir string

	switch goos {
	case platform.Windows:
		configDir = legacyAbsoluteEnv(getenv, "APPDATA")
		if configDir == "" {
			userProfile := getenv("USERPROFILE")
			if userProfile == "" {
				return "", errors.New("neither APPDATA nor USERPROFILE environment variable is set")
			}
			if !filepath.IsAbs(userProfile) {
				return "", fmt.Errorf("%w: USERPROFILE=%q", ErrBaseDirNotAbsolute, userProfile)
			}
			configDir = filepath.Join(userProfile, "AppData", "Roaming")
		}
	case "darwin":
		home, err := legacyAbsoluteHomeDir(userHomeDir)
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		configDir = legacyAbsoluteEnv(getenv, "XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := legacyAbsoluteHomeDir(userHomeDir)
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(home, ".config")
		}
	}

	return types.FilesystemPath(filepath.Join(configDir, AppName)), nil
}

func legacyAbsoluteEnv(getenv func(string) string, name string) string {
	if value := getenv(name); filepath.IsAbs(value) {
		return value
	}
	return ""
}

func legacyAbsoluteHomeDir(userHomeDir func() (string, error)) (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("%w: home directory %q", ErrBaseDirNotAbsolute, home)
	}
	return home, nil
}

func legacyCommandsDirFrom(userHomeDir func() (string, error)) (types.FilesystemPath, error) {
	home, err := legacyAbsoluteHomeDir(userHomeDir)
	if err != nil {
		return "", err
	}
	return types.FilesystemPath(filepath.Join(home, ".invowk", "cmds")), nil
}

// TestConfigPathsMatchLegacy proves the zero-change invariant for the config
// adopter (design D5 row g): with every base absolute or unset, configDirFrom
// and commandsDirFrom return the same path and the same error text as before,
// on every goos branch. Fixtures are host-absolute, as in the S1 tests.
func TestConfigPathsMatchLegacy(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")
	base := t.TempDir()
	homes := map[string]func() (string, error){
		"absolute existing home": func() (string, error) { return existing, nil },
		"absolute missing home":  func() (string, error) { return missing, nil },
		"home unset":             func() (string, error) { return "", errors.New("$HOME is not defined") },
	}
	envs := map[string]map[string]string{
		"bases unset":              nil,
		"XDG_CONFIG_HOME absolute": {"XDG_CONFIG_HOME": filepath.Join(base, "xdg")},
		"APPDATA absolute":         {"APPDATA": filepath.Join(base, "appdata")},
		"USERPROFILE absolute":     {"USERPROFILE": filepath.Join(base, "profile")},
		"APPDATA and USERPROFILE absolute": {
			"APPDATA":     filepath.Join(base, "appdata"),
			"USERPROFILE": filepath.Join(base, "profile"),
		},
	}
	for _, goos := range []string{"linux", "darwin", platform.Windows} {
		for homeName, home := range homes {
			for envName, env := range envs {
				t.Run(goos+"/"+homeName+"/"+envName, func(t *testing.T) {
					t.Parallel()
					getenv := func(name string) string { return env[name] }
					want, wantErr := legacyConfigDirFrom(goos, getenv, home)
					got, gotErr := configDirFrom(goos, getenv, home)
					if got != want || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
						t.Errorf("configDirFrom() = (%q, %v), legacy = (%q, %v)", got, gotErr, want, wantErr)
					}
				})
			}
		}
	}
	for homeName, home := range homes {
		t.Run("commands/"+homeName, func(t *testing.T) {
			t.Parallel()
			want, wantErr := legacyCommandsDirFrom(home)
			got, gotErr := commandsDirFrom(home)
			if got != want || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
				t.Errorf("commandsDirFrom() = (%q, %v), legacy = (%q, %v)", got, gotErr, want, wantErr)
			}
		})
	}
}
