// SPDX-License-Identifier: MPL-2.0

package basedir

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// TestWindowsPathForms runs only on Windows, because filepath.IsAbs follows
// the host: a Windows path is absolute only with a volume and a root, or as a
// UNC path (design D4).
func TestWindowsPathForms(t *testing.T) {
	t.Parallel()

	profileHome := func(profile string) func() (string, error) {
		return func() (string, error) { return profile, nil }
	}
	tests := []struct {
		name      string
		env       map[string]string
		resolve   func(Dirs) (types.FilesystemPath, error)
		want      types.FilesystemPath
		wantNotAb bool
	}{
		{name: "root-relative profile", env: map[string]string{envUserProfile: `\Users\me`}, resolve: Dirs.ConfigHome, wantNotAb: true},
		{name: "drive-relative profile", env: map[string]string{envUserProfile: `C:Users\me`}, resolve: Dirs.DataHome, wantNotAb: true},
		{name: "relative APPDATA falls back", env: map[string]string{envAppData: `AppData\Roaming`, envUserProfile: `C:\Users\me`}, resolve: Dirs.ConfigHome, want: `C:\Users\me\AppData\Roaming`},
		{name: "relative LOCALAPPDATA falls back", env: map[string]string{envLocalAppData: `AppData\Local`, envUserProfile: `C:\Users\me`}, resolve: Dirs.CacheHome, want: `C:\Users\me\AppData\Local`},
		{name: "relative USERPROFILE rejected", env: map[string]string{envUserProfile: `Users\me`}, resolve: Dirs.StateHome, wantNotAb: true},
		{name: "UNC profile accepted", env: map[string]string{envUserProfile: `\\server\share\me`}, resolve: Dirs.ConfigHome, want: `\\server\share\me\AppData\Roaming`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dirs := New("windows", envLookup(tt.env), profileHome(tt.env[envUserProfile]))
			got, err := tt.resolve(dirs)
			if tt.wantNotAb {
				if !errors.Is(err, ErrNotAbsolute) {
					t.Fatalf("got (%q, %v), want ErrNotAbsolute", got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got (%q, %v), want %q", got, err, tt.want)
			}
		})
	}

	for _, profile := range []string{`\Users\me`, `C:Users\me`, `Users\me`} {
		if _, err := New("windows", nil, profileHome(profile)).Home(); !errors.Is(err, ErrNotAbsolute) {
			t.Errorf("Home() with %q: err = %v, want ErrNotAbsolute", profile, err)
		}
	}
}
