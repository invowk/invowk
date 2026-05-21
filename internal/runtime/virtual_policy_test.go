// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"path/filepath"
	"testing"
)

func TestStandardVirtualAnchorsForOS(t *testing.T) {
	t.Parallel()
	fromSlash := filepath.FromSlash

	tests := []struct {
		name string
		goos string
		env  map[string]string
		want map[string]string
	}{
		{
			name: "linux xdg",
			goos: "linux",
			env: map[string]string{
				"XDG_CONFIG_HOME": "/xdg/config",
				"XDG_DATA_HOME":   "/xdg/data",
				"XDG_CACHE_HOME":  "/xdg/cache",
				"XDG_STATE_HOME":  "/xdg/state",
			},
			want: map[string]string{
				"@config": fromSlash("/xdg/config/invowk"),
				"@data":   fromSlash("/xdg/data/invowk"),
				"@cache":  fromSlash("/xdg/cache/invowk"),
				"@state":  fromSlash("/xdg/state/invowk"),
			},
		},
		{
			name: "macos library",
			goos: "darwin",
			want: map[string]string{
				"@config": fromSlash("/home/user/Library/Application Support/invowk"),
				"@data":   fromSlash("/home/user/Library/Application Support/invowk"),
				"@cache":  fromSlash("/home/user/Library/Caches/invowk"),
				"@state":  fromSlash("/home/user/Library/Logs/invowk"),
			},
		},
		{
			name: "windows appdata",
			goos: "windows",
			env: map[string]string{
				"APPDATA":      filepath.Join("C:", "Users", "user", "Roaming"),
				"LOCALAPPDATA": filepath.Join("C:", "Users", "user", "Local"),
			},
			want: map[string]string{
				"@config": filepath.Join("C:", "Users", "user", "Roaming", "invowk", "config"),
				"@data":   filepath.Join("C:", "Users", "user", "Local", "invowk", "data"),
				"@cache":  filepath.Join("C:", "Users", "user", "Local", "invowk", "cache"),
				"@state":  filepath.Join("C:", "Users", "user", "Local", "invowk", "state"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			anchors := standardVirtualAnchorsForOS(tt.goos, "/work", "/home/user", "/tmp/invowk-test", func(key string) string {
				return tt.env[key]
			})

			assertStandardVirtualBaseAnchors(t, anchors)
			assertExpectedVirtualAnchors(t, anchors, tt.want)
		})
	}
}

func assertStandardVirtualBaseAnchors(t *testing.T, anchors map[string]string) {
	t.Helper()

	if anchors[virtualAnchorHome] != "/home/user" {
		t.Fatalf("%s = %q, want /home/user", virtualAnchorHome, anchors[virtualAnchorHome])
	}
	if anchors[virtualAnchorTmp] != "/tmp/invowk-test" {
		t.Fatalf("%s = %q, want /tmp/invowk-test", virtualAnchorTmp, anchors[virtualAnchorTmp])
	}
	if anchors[virtualAnchorWork] != "/work" {
		t.Fatalf("%s = %q, want /work", virtualAnchorWork, anchors[virtualAnchorWork])
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
