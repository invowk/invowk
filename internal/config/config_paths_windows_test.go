// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// TestConfigDirWindowsRelativeBases covers the config-source-resolution
// Windows scenarios on a real Windows host, where filepath.IsAbs applies the
// Windows rules. It uses the process environment, so it does not run in
// parallel.
func TestConfigDirWindowsRelativeBases(t *testing.T) { //nolint:paralleltest // t.Setenv mutates the process environment.
	profile := t.TempDir()

	t.Run("relative APPDATA falls back to USERPROFILE", func(t *testing.T) {
		t.Setenv("APPDATA", "Roaming")
		t.Setenv("USERPROFILE", profile)
		dir, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() error: %v", err)
		}
		if want := types.FilesystemPath(filepath.Join(profile, "AppData", "Roaming", AppName)); dir != want {
			t.Errorf("ConfigDir() = %s, want %s", dir, want)
		}
	})

	t.Run("relative USERPROFILE is rejected and loading reports it", func(t *testing.T) {
		t.Setenv("APPDATA", "")
		t.Setenv("USERPROFILE", `Users\me`)
		if _, err := ConfigDir(); !errors.Is(err, ErrBaseDirNotAbsolute) || !strings.Contains(err.Error(), "USERPROFILE") {
			t.Fatalf("ConfigDir() error = %v, want ErrBaseDirNotAbsolute naming USERPROFILE", err)
		}
		// The implicit load path surfaces the same error; commandsvc turns
		// it into the built-in defaults plus a config_load_failed diagnostic.
		_, _, _, err := loadWithOptions(t.Context(), LoadOptions{}, SourceSelection{Kind: ConfigSourceImplicit})
		if !errors.Is(err, ErrBaseDirNotAbsolute) {
			t.Fatalf("loadWithOptions() error = %v, want ErrBaseDirNotAbsolute", err)
		}
	})
}
