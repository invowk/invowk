// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"path/filepath"
	"runtime/debug"
	"testing"
)

func TestDetectInstallMethod_LdflagsHint(t *testing.T) {
	// Not parallel: subtests mutate the package-level installMethodHint global.

	tests := []struct {
		name string
		hint string
		path string
		want InstallMethod
	}{
		{
			name: "homebrew hint overrides path heuristics",
			hint: "homebrew",
			path: "/usr/local/bin/invowk", // not a Homebrew path
			want: InstallMethodHomebrew,
		},
		{
			name: "goinstall hint",
			hint: "goinstall",
			path: "/usr/local/bin/invowk",
			want: InstallMethodGoInstall,
		},
		{
			name: "script hint",
			hint: "script",
			path: "/usr/local/bin/invowk",
			want: InstallMethodScript,
		},
		{
			name: "unknown hint value",
			hint: "manual",
			path: "/opt/homebrew/Cellar/invowk/1.0.0/bin/invowk", // Homebrew path, but hint overrides
			want: InstallMethodUnknown,
		},
		{
			name: "hint is case-insensitive",
			hint: "HOMEBREW",
			path: "/usr/local/bin/invowk",
			want: InstallMethodHomebrew,
		},
		{
			name: "empty hint falls through to path heuristics",
			hint: "",
			path: "/opt/homebrew/Cellar/invowk/1.0.0/bin/invowk",
			want: InstallMethodHomebrew,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: mutates package-level installMethodHint.

			saved := installMethodHint
			t.Cleanup(func() { installMethodHint = saved })
			installMethodHint = tt.hint

			got := DetectInstallMethod(tt.path)
			if got != tt.want {
				t.Errorf("DetectInstallMethod(%q) with hint=%q = %v, want %v", tt.path, tt.hint, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethod_HomebrewPaths(t *testing.T) {
	// Not parallel: mutates package-level installMethodHint.

	// Clear ldflags hint so path heuristics are used.
	saved := installMethodHint
	t.Cleanup(func() { installMethodHint = saved })
	installMethodHint = ""

	tests := []struct {
		name string
		path string
		want InstallMethod
	}{
		{
			name: "macOS ARM Homebrew",
			path: "/opt/homebrew/Cellar/invowk/1.0.0/bin/invowk",
			want: InstallMethodHomebrew,
		},
		{
			name: "macOS Intel Homebrew",
			path: "/usr/local/Cellar/invowk/1.0.0/bin/invowk",
			want: InstallMethodHomebrew,
		},
		{
			name: "Linux Homebrew",
			path: "/home/linuxbrew/.linuxbrew/Cellar/invowk/1.0.0/bin/invowk",
			want: InstallMethodHomebrew,
		},
		{
			name: "macOS ARM opt/homebrew bin",
			path: "/opt/homebrew/bin/invowk",
			want: InstallMethodHomebrew,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectInstallMethod(tt.path)
			if got != tt.want {
				t.Errorf("DetectInstallMethod(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethod_GoInstall(t *testing.T) {
	// Not parallel: subtests mutate package-level readBuildInfo and use t.Setenv.

	// Clear ldflags hint so path heuristics are used.
	savedHint := installMethodHint
	t.Cleanup(func() { installMethodHint = savedHint })
	installMethodHint = ""

	tests := []struct {
		name       string
		path       string
		modulePath string
		hasBuild   bool
		want       InstallMethod
	}{
		{
			name:       "path in GOPATH/bin with matching module path",
			path:       filepath.Join(t.TempDir(), "go", "bin", "invowk"),
			modulePath: "github.com/invowk/invowk",
			hasBuild:   true,
			want:       InstallMethodGoInstall,
		},
		{
			name:       "path in GOPATH/bin but wrong module path",
			path:       filepath.Join(t.TempDir(), "go", "bin", "invowk"),
			modulePath: "github.com/other/project",
			hasBuild:   true,
			want:       InstallMethodUnknown,
		},
		{
			name:       "path in GOPATH/bin but no build info",
			path:       filepath.Join(t.TempDir(), "go", "bin", "invowk"),
			modulePath: "",
			hasBuild:   false,
			want:       InstallMethodUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: mutates package-level readBuildInfo and uses t.Setenv.

			// Mock readBuildInfo for this subtest.
			savedReadBuildInfo := readBuildInfo
			t.Cleanup(func() { readBuildInfo = savedReadBuildInfo })

			if tt.hasBuild {
				modPath := tt.modulePath
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return &debug.BuildInfo{
						Path: modPath,
					}, true
				}
			} else {
				readBuildInfo = func() (*debug.BuildInfo, bool) {
					return nil, false
				}
			}

			// Set GOPATH to the parent of "go/bin" so the path detection works.
			// The test creates paths like <tmpdir>/go/bin/invowk, so GOPATH
			// should be <tmpdir>/go.
			gopath := filepath.Dir(filepath.Dir(tt.path)) // go/bin/invowk -> go/bin -> go
			t.Setenv("GOPATH", gopath)

			got := DetectInstallMethod(tt.path)
			if got != tt.want {
				t.Errorf("DetectInstallMethod(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethod_Script(t *testing.T) {
	// Not parallel: mutates package-level installMethodHint and readBuildInfo.

	savedHint := installMethodHint
	t.Cleanup(func() { installMethodHint = savedHint })
	installMethodHint = ""

	// Mock readBuildInfo to return no build info, so GOPATH detection
	// does not interfere with the script path check.
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() { readBuildInfo = savedReadBuildInfo })
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}

	tests := []struct {
		name string
		path string
		want InstallMethod
	}{
		{
			name: "standard script install location",
			path: "/home/user/.local/bin/invowk",
			want: InstallMethodScript,
		},
		{
			name: "root script install location",
			path: "/root/.local/bin/invowk",
			want: InstallMethodScript,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectInstallMethod(tt.path)
			if got != tt.want {
				t.Errorf("DetectInstallMethod(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethod_Unknown(t *testing.T) {
	// Not parallel: mutates package-level installMethodHint and readBuildInfo.

	savedHint := installMethodHint
	t.Cleanup(func() { installMethodHint = savedHint })
	installMethodHint = ""

	// Mock readBuildInfo to return no build info so GOPATH detection
	// does not interfere.
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() { readBuildInfo = savedReadBuildInfo })
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "system path",
			path: "/usr/local/bin/invowk",
		},
		{
			name: "custom path",
			path: "/opt/tools/invowk",
		},
		{
			name: "current directory",
			path: "./invowk",
		},
		{
			name: "absolute path in home but not .local/bin",
			path: "/home/user/bin/invowk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectInstallMethod(tt.path)
			if got != InstallMethodUnknown {
				t.Errorf("DetectInstallMethod(%q) = %v, want %v", tt.path, got, InstallMethodUnknown)
			}
		})
	}
}

func TestInstallMethod_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method InstallMethod
		want   string
	}{
		{
			name:   "unknown",
			method: InstallMethodUnknown,
			want:   "unknown",
		},
		{
			name:   "script",
			method: InstallMethodScript,
			want:   "script",
		},
		{
			name:   "homebrew",
			method: InstallMethodHomebrew,
			want:   "homebrew",
		},
		{
			name:   "goinstall",
			method: InstallMethodGoInstall,
			want:   "goinstall",
		},
		{
			name:   "out of range value",
			method: InstallMethod(99),
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.method.String()
			if got != tt.want {
				t.Errorf("InstallMethod(%d).String() = %q, want %q", tt.method, got, tt.want)
			}
		})
	}
}

func TestIsInGOPATHBin(t *testing.T) {
	// Not parallel: subtests use t.Setenv which mutates process-wide state.

	tests := []struct {
		name   string
		gopath string
		path   string
		want   bool
	}{
		{
			name:   "exact match in GOPATH/bin",
			gopath: "/home/user/go",
			path:   "/home/user/go/bin/invowk",
			want:   true,
		},
		{
			name:   "not in GOPATH/bin",
			gopath: "/home/user/go",
			path:   "/usr/local/bin/invowk",
			want:   false,
		},
		{
			name:   "similar prefix but not GOPATH/bin",
			gopath: "/home/user/go",
			path:   "/home/user/gobin/invowk",
			want:   false,
		},
		{
			name:   "subdirectory of GOPATH/bin",
			gopath: "/home/user/go",
			path:   "/home/user/go/bin/subdir/invowk",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: t.Setenv mutates process-wide state.
			t.Setenv("GOPATH", tt.gopath)

			got := isInGOPATHBin(tt.path)
			if got != tt.want {
				t.Errorf("isInGOPATHBin(%q) with GOPATH=%q = %v, want %v", tt.path, tt.gopath, got, tt.want)
			}
		})
	}
}

func TestIsInGOPATHBin_DefaultGOPATH(t *testing.T) {
	// Not parallel: t.Setenv mutates process-wide state.

	// When GOPATH is unset, the default is ~/go.
	t.Setenv("GOPATH", "")

	// We cannot predict the actual home directory in a portable way,
	// but we can verify that an arbitrary path outside any ~/go/bin
	// returns false.
	got := isInGOPATHBin("/opt/custom/bin/invowk")
	if got {
		t.Error("isInGOPATHBin should return false for paths outside default GOPATH/bin")
	}
}
