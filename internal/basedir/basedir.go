// SPDX-License-Identifier: MPL-2.0

package basedir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

const (
	envHome         = "HOME"
	envPlan9Home    = "home"
	envUserProfile  = "USERPROFILE"
	envAppData      = "APPDATA"
	envLocalAppData = "LOCALAPPDATA"
	envConfigHome   = "XDG_CONFIG_HOME"
	envDataHome     = "XDG_DATA_HOME"
	envCacheHome    = "XDG_CACHE_HOME"
	envStateHome    = "XDG_STATE_HOME"
	envRuntimeDir   = "XDG_RUNTIME_DIR"

	goosPlan9 = "plan9"

	errMsgHomeDir = "failed to get home directory: %w"
)

// ErrNotAbsolute marks a relative home directory or USERPROFILE. invowk never
// derives a directory from one, because it would resolve under the current
// directory. The text is the one S1 (harden-config-trust) introduced for the
// config directory, which re-exports this sentinel.
var ErrNotAbsolute = errors.New("base directory must be an absolute path")

type (
	// Dirs resolves the per-user base directories from injectable inputs.
	// The zero value behaves as [OS], so struct fields that hold a Dirs need
	// no constructor change. Build one with [New] or [OS].
	//
	//goplint:ignore -- constructed by New (injected inputs) and OS; the zero value is valid.
	Dirs struct {
		goos        string //goplint:ignore -- raw runtime.GOOS value at the OS boundary.
		getenv      func(string) string
		userHomeDir func() (string, error)
	}

	// windowsBase names one Windows AppData base: the variable that sets it
	// and the USERPROFILE-relative default.
	windowsBase struct {
		env      string   //goplint:ignore -- fixed environment variable name.
		fallback []string //goplint:ignore -- fixed path elements under USERPROFILE.
	}
)

// New returns a resolver over the given OS name, environment lookup and home
// lookup. A nil getenv or userHomeDir, or an empty goos, takes the value the
// real process would use.
//
//goplint:ignore -- goos and the lookups are raw OS-boundary inputs, injected for tests.
func New(goos string, getenv func(string) string, userHomeDir func() (string, error)) Dirs {
	return Dirs{goos: goos, getenv: getenv, userHomeDir: userHomeDir}
}

// OS returns the resolver for the real process: runtime.GOOS, os.Getenv and
// os.UserHomeDir.
func OS() Dirs {
	return Dirs{}
}

// HomeFromEnv reproduces os.UserHomeDir's choice of variable (USERPROFILE on
// Windows, home on Plan 9, HOME elsewhere) over an environment lookup, for
// harnesses that only have an environment map. It does not check that the
// value is absolute; [Dirs.Home] does.
//
//goplint:ignore -- raw OS name, lookup and home string at the OS boundary.
func HomeFromEnv(goos string, getenv func(string) string) (string, error) {
	name := envHome
	switch goos {
	case platform.Windows:
		name = envUserProfile
	case goosPlan9:
		name = envPlan9Home
	}
	if value := getenv(name); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("$%s is not defined", name)
}

// Home returns the absolute home directory. A failing lookup is wrapped, and
// a relative home is an [ErrNotAbsolute] error.
func (d Dirs) Home() (types.FilesystemPath, error) {
	home, err := d.home()
	if err != nil {
		return "", err
	}
	return types.FilesystemPath(home), nil //goplint:ignore -- checked absolute by home().
}

// ConfigHome returns the base of per-user config directories:
// XDG_CONFIG_HOME or ~/.config on Linux and other Unix systems,
// ~/Library/Application Support on macOS, and APPDATA or
// %USERPROFILE%\AppData\Roaming on Windows.
func (d Dirs) ConfigHome() (types.FilesystemPath, error) {
	return d.base(envConfigHome, []string{".config"},
		[]string{"Library", "Application Support"},
		windowsBase{env: envAppData, fallback: []string{"AppData", "Roaming"}})
}

// DataHome returns the base of per-user data directories: XDG_DATA_HOME or
// ~/.local/share, ~/Library/Application Support on macOS, and LOCALAPPDATA or
// %USERPROFILE%\AppData\Local on Windows.
func (d Dirs) DataHome() (types.FilesystemPath, error) {
	return d.base(envDataHome, []string{".local", "share"},
		[]string{"Library", "Application Support"}, localAppData())
}

// CacheHome returns the base of per-user cache directories: XDG_CACHE_HOME
// or ~/.cache, ~/Library/Caches on macOS, and LOCALAPPDATA or
// %USERPROFILE%\AppData\Local on Windows.
func (d Dirs) CacheHome() (types.FilesystemPath, error) {
	return d.base(envCacheHome, []string{".cache"},
		[]string{"Library", "Caches"}, localAppData())
}

// StateHome returns the base of per-user state directories: XDG_STATE_HOME
// or ~/.local/state, ~/Library/Logs on macOS, and LOCALAPPDATA or
// %USERPROFILE%\AppData\Local on Windows.
func (d Dirs) StateHome() (types.FilesystemPath, error) {
	return d.base(envStateHome, []string{".local", "state"},
		[]string{"Library", "Logs"}, localAppData())
}

// RuntimeDir returns XDG_RUNTIME_DIR when it is absolute. The runtime dir has
// no default, so a relative or unset value reports false and the caller uses
// its own fallback.
func (d Dirs) RuntimeDir() (types.FilesystemPath, bool) {
	dir := d.absoluteEnv(envRuntimeDir)
	if dir == "" {
		return "", false
	}
	return types.FilesystemPath(dir), true //goplint:ignore -- checked absolute by absoluteEnv.
}

func localAppData() windowsBase {
	return windowsBase{env: envLocalAppData, fallback: []string{"AppData", "Local"}}
}

// base resolves one base directory for the resolver's OS.
//
//goplint:ignore -- variable names and path elements are fixed OS-boundary literals.
func (d Dirs) base(xdgEnv string, xdgFallback, darwinFallback []string, win windowsBase) (types.FilesystemPath, error) {
	var dir string
	switch d.os() {
	case platform.Windows:
		dir = d.absoluteEnv(win.env)
		if dir == "" {
			profile, err := d.userProfile(win.env)
			if err != nil {
				return "", err
			}
			dir = filepath.Join(append([]string{profile}, win.fallback...)...)
		}
	case platform.Darwin:
		home, err := d.home()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(append([]string{home}, darwinFallback...)...)
	default:
		dir = d.absoluteEnv(xdgEnv)
		if dir == "" {
			home, err := d.home()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(append([]string{home}, xdgFallback...)...)
		}
	}
	return types.FilesystemPath(dir), nil //goplint:ignore -- built from an absolute base and fixed elements.
}

// userProfile returns the absolute USERPROFILE for a Windows AppData
// fallback. It keeps S1's two error texts.
//
//goplint:ignore -- raw environment variable name and value at the OS boundary.
func (d Dirs) userProfile(primaryEnv string) (string, error) {
	profile := d.env(envUserProfile)
	if profile == "" {
		return "", fmt.Errorf("neither %s nor %s environment variable is set", primaryEnv, envUserProfile)
	}
	if !filepath.IsAbs(profile) {
		return "", fmt.Errorf("%w: %s=%q", ErrNotAbsolute, envUserProfile, profile)
	}
	return profile, nil
}

// home returns the absolute home directory string.
//
//goplint:ignore -- raw OS home directory string at the OS boundary.
func (d Dirs) home() (string, error) {
	lookup := d.userHomeDir
	if lookup == nil {
		lookup = os.UserHomeDir
	}
	home, err := lookup()
	if err != nil {
		return "", fmt.Errorf(errMsgHomeDir, err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("%w: home directory %q", ErrNotAbsolute, home)
	}
	return home, nil
}

// absoluteEnv returns the value of the variable name, or "" when it is unset
// or not an absolute path.
//
//goplint:ignore -- raw environment variable name and value at the OS boundary.
func (d Dirs) absoluteEnv(name string) string {
	if value := d.env(name); filepath.IsAbs(value) {
		return value
	}
	return ""
}

//goplint:ignore -- raw environment variable name and value at the OS boundary.
func (d Dirs) env(name string) string {
	if d.getenv == nil {
		return os.Getenv(name)
	}
	return d.getenv(name)
}

//goplint:ignore -- raw runtime.GOOS value at the OS boundary.
func (d Dirs) os() string {
	if d.goos == "" {
		return runtime.GOOS
	}
	return d.goos
}
