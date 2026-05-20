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

			if anchors["@home"] != "/home/user" {
				t.Fatalf("@home = %q, want /home/user", anchors["@home"])
			}
			if anchors["@tmp"] != "/tmp/invowk-test" {
				t.Fatalf("@tmp = %q, want /tmp/invowk-test", anchors["@tmp"])
			}
			if anchors["@work"] != "/work" {
				t.Fatalf("@work = %q, want /work", anchors["@work"])
			}
			for name, want := range tt.want {
				if got := anchors[name]; got != want {
					t.Fatalf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}
