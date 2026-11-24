// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// legacyStandardVirtualAnchorsForOS is standardVirtualAnchorsForOS as it was
// on main before require-absolute-base-dirs (home passed in, "" on a lookup
// error). It is the zero-change oracle (design D5 row a) and must not be
// edited.
func legacyStandardVirtualAnchorsForOS(goos, workDir, home, tempDir string, getenv func(string) string) map[string]string {
	first := func(a, b string) string {
		if a != "" {
			return a
		}
		return b
	}
	anchors := map[string]string{virtualAnchorHome: home, virtualAnchorTmp: tempDir, virtualAnchorWork: workDir}
	switch goos {
	case goosWindows:
		roaming := first(getenv("APPDATA"), filepath.Join(home, "AppData", "Roaming"))
		local := first(getenv("LOCALAPPDATA"), filepath.Join(home, "AppData", "Local"))
		anchors[virtualAnchorConfig] = filepath.Join(roaming, "invowk", "config")
		anchors[virtualAnchorData] = filepath.Join(local, "invowk", "data")
		anchors[virtualAnchorCache] = filepath.Join(local, "invowk", "cache")
		anchors[virtualAnchorState] = filepath.Join(local, "invowk", "state")
	case "darwin":
		anchors[virtualAnchorConfig] = filepath.Join(home, "Library", "Application Support", "invowk")
		anchors[virtualAnchorData] = filepath.Join(home, "Library", "Application Support", "invowk")
		anchors[virtualAnchorCache] = filepath.Join(home, "Library", "Caches", "invowk")
		anchors[virtualAnchorState] = filepath.Join(home, "Library", "Logs", "invowk")
	default:
		anchors[virtualAnchorConfig] = filepath.Join(first(getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config")), "invowk")
		anchors[virtualAnchorData] = filepath.Join(first(getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share")), "invowk")
		anchors[virtualAnchorCache] = filepath.Join(first(getenv("XDG_CACHE_HOME"), filepath.Join(home, ".cache")), "invowk")
		anchors[virtualAnchorState] = filepath.Join(first(getenv("XDG_STATE_HOME"), filepath.Join(home, ".local", "state")), "invowk")
	}
	return anchors
}

// TestStandardVirtualAnchorsMatchLegacy proves the zero-change invariant for
// the anchors: on linux, darwin and windows, with the home absolute or unset
// and each XDG/AppData variable absolute or unset, every anchor is the same
// as before. The only allowed difference is the documented BREAKING row: with
// the home unset, an anchor the legacy code made relative (cwd-relative,
// such as ".config/invowk") is now "".
func TestStandardVirtualAnchorsMatchLegacy(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	home := filepath.Join(base, "home")
	work := filepath.Join(base, "work")
	tmp := filepath.Join(base, "tmp")
	baseVars := map[string][]string{
		"linux":   {"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"},
		"darwin":  {"XDG_CONFIG_HOME"},
		"windows": {"APPDATA", "LOCALAPPDATA"},
	}
	for goos, vars := range baseVars {
		for _, homeSet := range []bool{true, false} {
			// Each subset of the base variables set to absolute values.
			for mask := range 1 << len(vars) {
				env := map[string]string{}
				for i, name := range vars {
					if mask&(1<<i) != 0 {
						env[name] = filepath.Join(base, "set", name)
					}
				}
				homeValue, homeErr := home, error(nil)
				if homeSet {
					if goos == goosWindows {
						env["USERPROFILE"] = home
					}
				} else {
					homeValue, homeErr = "", errors.New("home is not defined")
				}
				legacyHome := homeValue
				got := standardVirtualAnchorsForOS(goos, work, tmp, anchorDirs(goos, env, homeValue, homeErr))
				want := legacyStandardVirtualAnchorsForOS(goos, work, legacyHome, tmp, func(k string) string { return env[k] })
				for name, wantPath := range want {
					breaking := !homeSet && !filepath.IsAbs(wantPath) && wantPath != ""
					if breaking {
						if got[name] != "" {
							t.Errorf("%s home unset env %v: %s = %q, want \"\" (BREAKING row; legacy %q)", goos, env, name, got[name], wantPath)
						}
						continue
					}
					if got[name] != wantPath {
						t.Errorf("%s home set=%v env %v: %s = %q, legacy %q", goos, homeSet, env, name, got[name], wantPath)
					}
				}
			}
		}
	}
}

func TestVirtualPathMappingOnUnavailableAnchorFails(t *testing.T) {
	t.Parallel()

	anchors := standardVirtualAnchorsForOS("linux", t.TempDir(), t.TempDir(), anchorDirs("linux", nil, ".", nil))
	paths := invowkfile.VirtualFilesystemPaths{"DOTFILES": "@home/.dotfiles"}
	_, err := resolveVirtualFilesystemPaths(paths, t.TempDir(), anchors, invowkfile.VirtualFilesystemAccessRestricted)
	if err == nil || !strings.Contains(err.Error(), `"@home" is unavailable`) {
		t.Fatalf("resolveVirtualFilesystemPaths() error = %v, want @home unavailable", err)
	}
}
