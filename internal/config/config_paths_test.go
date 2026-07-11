// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestConfigDir(t *testing.T) {
	t.Parallel()

	fakeHomeDir := filepath.Join("fake", "home")
	fakeHome := func() (string, error) { return fakeHomeDir, nil }
	failHome := func() (string, error) { return "", errors.New("no home") }

	t.Run("linux with XDG_CONFIG_HOME set", func(t *testing.T) {
		t.Parallel()
		xdgPath := filepath.Join("custom", "xdg")
		getenv := func(key string) string {
			if key == "XDG_CONFIG_HOME" {
				return xdgPath
			}
			return ""
		}
		dir, err := configDirFrom("linux", getenv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(xdgPath, AppName))
		if dir != expected {
			t.Errorf("configDirFrom() = %s, want %s", dir, expected)
		}
	})

	t.Run("linux without XDG_CONFIG_HOME", func(t *testing.T) {
		t.Parallel()
		noEnv := func(string) string { return "" }
		dir, err := configDirFrom("linux", noEnv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(fakeHomeDir, ".config", AppName))
		if dir != expected {
			t.Errorf("configDirFrom() = %s, want %s", dir, expected)
		}
	})

	t.Run("linux home dir error", func(t *testing.T) {
		t.Parallel()
		noEnv := func(string) string { return "" }
		_, err := configDirFrom("linux", noEnv, failHome)
		if err == nil {
			t.Fatal("expected error when home dir fails")
		}
	})

	t.Run("darwin", func(t *testing.T) {
		t.Parallel()
		noEnv := func(string) string { return "" }
		dir, err := configDirFrom("darwin", noEnv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(fakeHomeDir, "Library", "Application Support", AppName))
		if dir != expected {
			t.Errorf("configDirFrom() = %s, want %s", dir, expected)
		}
	})

	t.Run("darwin home dir error", func(t *testing.T) {
		t.Parallel()
		noEnv := func(string) string { return "" }
		_, err := configDirFrom("darwin", noEnv, failHome)
		if err == nil {
			t.Fatal("expected error when home dir fails")
		}
	})

	t.Run("windows with APPDATA", func(t *testing.T) {
		t.Parallel()
		appDataPath := filepath.Join("C", "Users", "test", "AppData", "Roaming")
		getenv := func(key string) string {
			if key == "APPDATA" {
				return appDataPath
			}
			return ""
		}
		dir, err := configDirFrom("windows", getenv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(appDataPath, AppName))
		if dir != expected {
			t.Errorf("configDirFrom() = %s, want %s", dir, expected)
		}
	})

	t.Run("windows without APPDATA", func(t *testing.T) {
		t.Parallel()
		userProfile := filepath.Join("C", "Users", "test")
		getenv := func(key string) string {
			if key == "USERPROFILE" {
				return userProfile
			}
			return ""
		}
		dir, err := configDirFrom("windows", getenv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(userProfile, "AppData", "Roaming", AppName))
		if dir != expected {
			t.Errorf("configDirFrom() = %s, want %s", dir, expected)
		}
	})

	t.Run("windows without APPDATA or USERPROFILE", func(t *testing.T) {
		t.Parallel()
		noEnv := func(string) string { return "" }
		_, err := configDirFrom("windows", noEnv, fakeHome)
		if err == nil {
			t.Fatal("configDirFrom() should return error when both APPDATA and USERPROFILE are empty")
		}
		if !strings.Contains(err.Error(), "APPDATA") || !strings.Contains(err.Error(), "USERPROFILE") {
			t.Errorf("error should mention both env vars, got: %v", err)
		}
	})
}

func TestCommandsDir(t *testing.T) {
	t.Parallel()
	dir, err := CommandsDir()
	if err != nil {
		t.Fatalf("CommandsDir() returned error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := types.FilesystemPath(filepath.Join(home, ".invowk", "cmds"))
	if dir != expected {
		t.Errorf("CommandsDir() = %s, want %s", dir, expected)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	t.Parallel()
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	err := EnsureConfigDir(types.FilesystemPath(configDir))
	if err != nil {
		t.Fatalf("EnsureConfigDir() returned error: %v", err)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("EnsureConfigDir() did not create directory %s", configDir)
	}
}

func TestConfigDirWithOverride(t *testing.T) {
	t.Parallel()

	t.Run("explicit path returned as-is", func(t *testing.T) {
		t.Parallel()
		want := types.FilesystemPath(filepath.Join(t.TempDir(), "explicit-config"))
		dir, err := configDirWithOverride(want)
		if err != nil {
			t.Fatalf("configDirWithOverride() error: %v", err)
		}
		if dir != want {
			t.Errorf("configDirWithOverride() = %q, want %q", dir, want)
		}
	})

	t.Run("invalid explicit path rejected", func(t *testing.T) {
		t.Parallel()
		_, err := configDirWithOverride("   ")
		if !errors.Is(err, types.ErrInvalidFilesystemPath) {
			t.Fatalf("configDirWithOverride() error = %v, want %v", err, types.ErrInvalidFilesystemPath)
		}
	})

	t.Run("empty falls through to ConfigDir", func(t *testing.T) {
		t.Parallel()
		dir, err := configDirWithOverride("")
		if err != nil {
			t.Fatalf("configDirWithOverride() error: %v", err)
		}
		expected, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() error: %v", err)
		}
		if dir != expected {
			t.Errorf("configDirWithOverride(\"\") = %q, want ConfigDir() = %q", dir, expected)
		}
	})
}

func TestCommandsDirWithOverride(t *testing.T) {
	t.Parallel()

	t.Run("explicit path returned as-is", func(t *testing.T) {
		t.Parallel()
		want := types.FilesystemPath(filepath.Join(t.TempDir(), "explicit-cmds"))
		dir, err := commandsDirWithOverride(want)
		if err != nil {
			t.Fatalf("commandsDirWithOverride() error: %v", err)
		}
		if dir != want {
			t.Errorf("commandsDirWithOverride() = %q, want %q", dir, want)
		}
	})

	t.Run("invalid explicit path rejected", func(t *testing.T) {
		t.Parallel()
		_, err := commandsDirWithOverride("   ")
		if !errors.Is(err, types.ErrInvalidFilesystemPath) {
			t.Fatalf("commandsDirWithOverride() error = %v, want %v", err, types.ErrInvalidFilesystemPath)
		}
	})

	t.Run("empty falls through to CommandsDir", func(t *testing.T) {
		t.Parallel()
		dir, err := commandsDirWithOverride("")
		if err != nil {
			t.Fatalf("commandsDirWithOverride() error: %v", err)
		}
		expected, err := CommandsDir()
		if err != nil {
			t.Fatalf("CommandsDir() error: %v", err)
		}
		if dir != expected {
			t.Errorf("commandsDirWithOverride(\"\") = %q, want CommandsDir() = %q", dir, expected)
		}
	})
}

// TestConfigDirFrom_UnknownGOOS verifies that an unrecognized GOOS value
// falls through to the default (Linux) case: $XDG_CONFIG_HOME if set,
// otherwise ~/.config.
func TestConfigDirFrom_UnknownGOOS(t *testing.T) {
	t.Parallel()

	fakeHomeDir := filepath.Join("fake", "home")
	fakeHome := func() (string, error) { return fakeHomeDir, nil }

	t.Run("unknown GOOS with XDG_CONFIG_HOME", func(t *testing.T) {
		t.Parallel()
		xdgPath := filepath.Join("custom", "xdg")
		getenv := func(key string) string {
			if key == "XDG_CONFIG_HOME" {
				return xdgPath
			}
			return ""
		}
		dir, err := configDirFrom("freebsd", getenv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(xdgPath, AppName))
		if dir != expected {
			t.Errorf("configDirFrom(freebsd) = %s, want %s", dir, expected)
		}
	})

	t.Run("unknown GOOS without XDG falls back to ~/.config", func(t *testing.T) {
		t.Parallel()
		noEnv := func(string) string { return "" }
		dir, err := configDirFrom("freebsd", noEnv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(fakeHomeDir, ".config", AppName))
		if dir != expected {
			t.Errorf("configDirFrom(freebsd) = %s, want %s", dir, expected)
		}
	})
}

func TestEnsureCommandsDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cmdsDir := filepath.Join(tmpDir, "cmds")

	err := EnsureCommandsDir(types.FilesystemPath(cmdsDir))
	if err != nil {
		t.Fatalf("EnsureCommandsDir() returned error: %v", err)
	}

	if _, err := os.Stat(cmdsDir); os.IsNotExist(err) {
		t.Errorf("EnsureCommandsDir() did not create directory %s", cmdsDir)
	}
}
