// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/basedir"
)

// anchorDirs builds a resolver over goos with an injected environment and
// home. Fixtures must be absolute on the host OS, because the resolver checks
// them with filepath.IsAbs, which follows the host, not goos.
func anchorDirs(goos string, env map[string]string, home string, homeErr error) basedir.Dirs {
	return basedir.New(goos, func(key string) string { return env[key] }, func() (string, error) { return home, homeErr })
}

func TestStandardVirtualAnchorsForOS(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	home := filepath.Join(base, "home", "user")
	xdg := filepath.Join(base, "xdg")
	work := filepath.Join(base, "work")
	tmp := filepath.Join(base, "tmp")
	roaming := filepath.Join(base, "Roaming")
	local := filepath.Join(base, "Local")

	tests := []struct {
		name     string
		goos     string
		env      map[string]string
		home     string
		homeErr  error
		wantHome string
		want     map[string]string
	}{
		{
			name: "linux xdg", goos: "linux", home: home, wantHome: home,
			env: map[string]string{
				"XDG_CONFIG_HOME": filepath.Join(xdg, "config"), "XDG_DATA_HOME": filepath.Join(xdg, "data"),
				"XDG_CACHE_HOME": filepath.Join(xdg, "cache"), "XDG_STATE_HOME": filepath.Join(xdg, "state"),
			},
			want: map[string]string{
				"@config": filepath.Join(xdg, "config", "invowk"), "@data": filepath.Join(xdg, "data", "invowk"),
				"@cache": filepath.Join(xdg, "cache", "invowk"), "@state": filepath.Join(xdg, "state", "invowk"),
			},
		},
		{
			name: "linux relative xdg falls back to home", goos: "linux", home: home, wantHome: home,
			env: map[string]string{"XDG_CONFIG_HOME": "config", "XDG_DATA_HOME": "data", "XDG_CACHE_HOME": ".", "XDG_STATE_HOME": "state"},
			want: map[string]string{
				"@config": filepath.Join(home, ".config", "invowk"), "@data": filepath.Join(home, ".local", "share", "invowk"),
				"@cache": filepath.Join(home, ".cache", "invowk"), "@state": filepath.Join(home, ".local", "state", "invowk"),
			},
		},
		{
			name: "linux relative home makes home-derived anchors unavailable", goos: "linux", home: ".", wantHome: "",
			want: map[string]string{"@config": "", "@data": "", "@cache": "", "@state": ""},
		},
		{
			name: "linux unset home keeps an absolute xdg anchor", goos: "linux", homeErr: errors.New("$HOME is not defined"), wantHome: "",
			env:  map[string]string{"XDG_CONFIG_HOME": filepath.Join(xdg, "config")},
			want: map[string]string{"@config": filepath.Join(xdg, "config", "invowk"), "@data": "", "@cache": "", "@state": ""},
		},
		{
			name: "macos library", goos: "darwin", home: home, wantHome: home,
			want: map[string]string{
				"@config": filepath.Join(home, "Library", "Application Support", "invowk"),
				"@data":   filepath.Join(home, "Library", "Application Support", "invowk"),
				"@cache":  filepath.Join(home, "Library", "Caches", "invowk"),
				"@state":  filepath.Join(home, "Library", "Logs", "invowk"),
			},
		},
		{
			name: "windows appdata", goos: "windows", home: home, wantHome: home,
			env: map[string]string{"APPDATA": roaming, "LOCALAPPDATA": local, "USERPROFILE": home},
			want: map[string]string{
				"@config": filepath.Join(roaming, "invowk", "config"), "@data": filepath.Join(local, "invowk", "data"),
				"@cache": filepath.Join(local, "invowk", "cache"), "@state": filepath.Join(local, "invowk", "state"),
			},
		},
		{
			name: "windows relative LOCALAPPDATA falls back to USERPROFILE", goos: "windows", home: home, wantHome: home,
			env: map[string]string{"APPDATA": roaming, "LOCALAPPDATA": `AppData\Local`, "USERPROFILE": home},
			want: map[string]string{
				"@config": filepath.Join(roaming, "invowk", "config"),
				"@cache":  filepath.Join(home, "AppData", "Local", "invowk", "cache"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			anchors := standardVirtualAnchorsForOS(tt.goos, work, tmp, anchorDirs(tt.goos, tt.env, tt.home, tt.homeErr))

			if anchors[virtualAnchorHome] != tt.wantHome {
				t.Fatalf("%s = %q, want %q", virtualAnchorHome, anchors[virtualAnchorHome], tt.wantHome)
			}
			if anchors[virtualAnchorTmp] != tmp || anchors[virtualAnchorWork] != work {
				t.Fatalf("@tmp/@work = %q/%q, want %q/%q", anchors[virtualAnchorTmp], anchors[virtualAnchorWork], tmp, work)
			}
			assertExpectedVirtualAnchors(t, anchors, tt.want)
		})
	}
}

func TestExpandVirtualAnchorPathUnavailable(t *testing.T) {
	t.Parallel()

	anchors := map[string]string{virtualAnchorHome: "", virtualAnchorConfig: filepath.Join(t.TempDir(), "invowk")}
	_, err := expandVirtualAnchorPath("@home/.dotfiles", anchors)
	if err == nil || err.Error() != `virtual path anchor "@home" is unavailable: its base directory is relative or unset` {
		t.Errorf("known empty anchor: err = %v, want the unavailable error", err)
	}
	_, err = expandVirtualAnchorPath("@nope/x", anchors)
	if err == nil || err.Error() != `unknown virtual path anchor "@nope"` {
		t.Errorf("unknown anchor: err = %v, want the unknown error", err)
	}
	if got, err := expandVirtualAnchorPath("@config/x", anchors); err != nil || got != filepath.Join(anchors[virtualAnchorConfig], "x") {
		t.Errorf("available anchor: (%q, %v)", got, err)
	}
}

func assertExpectedVirtualAnchors(t *testing.T, anchors, want map[string]string) {
	t.Helper()

	for name, expected := range want {
		if got := anchors[name]; got != expected {
			t.Fatalf("%s = %q, want %q", name, got, expected)
		}
	}
}
