// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/internal/config"
)

// TestContainerSuiteConfigDir checks that the harness writes the container
// config where the binary will read it: a relative XDG_CONFIG_HOME or APPDATA
// is ignored, as the shared resolver does.
func TestContainerSuiteConfigDir(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	env := &testscript.Env{Vars: []string{
		"HOME=" + home, "USERPROFILE=" + home,
		"XDG_CONFIG_HOME=.config", "APPDATA=Roaming",
	}}
	got, err := containerSuiteConfigDir(env)
	if err != nil {
		t.Fatalf("containerSuiteConfigDir() error: %v", err)
	}
	var want string
	switch runtime.GOOS {
	case "windows":
		want = filepath.Join(home, "AppData", "Roaming", config.AppName)
	case "darwin":
		want = filepath.Join(home, "Library", "Application Support", config.AppName)
	default:
		want = filepath.Join(home, ".config", config.AppName)
	}
	if got != want {
		t.Errorf("containerSuiteConfigDir() = %q, want %q", got, want)
	}

	relative := &testscript.Env{Vars: []string{"HOME=.", "USERPROFILE=."}}
	if _, err := containerSuiteConfigDir(relative); !errors.Is(err, basedir.ErrNotAbsolute) {
		t.Errorf("relative home: error = %v, want ErrNotAbsolute", err)
	}
}
