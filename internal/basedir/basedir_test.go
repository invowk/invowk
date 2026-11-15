// SPDX-License-Identifier: MPL-2.0

package basedir

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

type baseKind struct {
	name    string
	resolve func(Dirs) (types.FilesystemPath, error)
}

func envLookup(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func baseKinds() []baseKind {
	return []baseKind{
		{"config", Dirs.ConfigHome},
		{"data", Dirs.DataHome},
		{"cache", Dirs.CacheHome},
		{"state", Dirs.StateHome},
	}
}

// TestBaseDirs covers each kind on linux, darwin and windows. Fixtures are
// absolute on the host OS: filepath.IsAbs follows the host, not the injected
// goos.
func TestBaseDirs(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	home := filepath.Join(base, "home")
	profile := filepath.Join(base, "profile")
	xdg := filepath.Join(base, "xdg")
	okHome := func() (string, error) { return home, nil }
	relHome := func() (string, error) { return "home", nil }
	failHome := func() (string, error) { return "", errors.New("no home here") }

	xdgVars := map[string]string{"config": envConfigHome, "data": envDataHome, "cache": envCacheHome, "state": envStateHome}
	xdgDefaults := map[string][]string{"config": {".config"}, "data": {".local", "share"}, "cache": {".cache"}, "state": {".local", "state"}}
	darwinDefaults := map[string][]string{
		"config": {"Library", "Application Support"}, "data": {"Library", "Application Support"},
		"cache": {"Library", "Caches"}, "state": {"Library", "Logs"},
	}
	winVars := map[string]string{"config": envAppData, "data": envLocalAppData, "cache": envLocalAppData, "state": envLocalAppData}
	winDefaults := map[string][]string{
		"config": {"AppData", "Roaming"}, "data": {"AppData", "Local"},
		"cache": {"AppData", "Local"}, "state": {"AppData", "Local"},
	}

	for _, kind := range baseKinds() {
		join := func(root string, elems []string) types.FilesystemPath {
			return types.FilesystemPath(filepath.Join(append([]string{root}, elems...)...))
		}
		tests := []struct {
			name      string
			goos      string
			env       map[string]string
			home      func() (string, error)
			want      types.FilesystemPath
			wantErr   []string
			wantNotAb bool
		}{
			{name: "linux absolute xdg", goos: "linux", env: map[string]string{xdgVars[kind.name]: xdg}, home: okHome, want: types.FilesystemPath(xdg)},
			{name: "linux relative xdg falls back", goos: "linux", env: map[string]string{xdgVars[kind.name]: "rel"}, home: okHome, want: join(home, xdgDefaults[kind.name])},
			{name: "linux empty xdg falls back", goos: "linux", env: nil, home: okHome, want: join(home, xdgDefaults[kind.name])},
			{name: "linux relative home rejected", goos: "linux", env: nil, home: relHome, wantErr: []string{"absolute", `"home"`}, wantNotAb: true},
			{name: "linux relative xdg and relative home rejected", goos: "linux", env: map[string]string{xdgVars[kind.name]: "."}, home: relHome, wantNotAb: true},
			{name: "linux failing home", goos: "linux", env: nil, home: failHome, wantErr: []string{"no home here"}},
			{name: "linux absolute xdg with relative home", goos: "linux", env: map[string]string{xdgVars[kind.name]: xdg}, home: relHome, want: types.FilesystemPath(xdg)},
			{name: "freebsd uses xdg", goos: "freebsd", env: map[string]string{xdgVars[kind.name]: xdg}, home: okHome, want: types.FilesystemPath(xdg)},
			{name: "darwin default", goos: "darwin", env: map[string]string{xdgVars[kind.name]: xdg}, home: okHome, want: join(home, darwinDefaults[kind.name])},
			{name: "darwin relative home rejected", goos: "darwin", env: nil, home: relHome, wantNotAb: true},
			{name: "darwin failing home", goos: "darwin", env: nil, home: failHome, wantErr: []string{"no home here"}},
			{name: "windows absolute appdata", goos: "windows", env: map[string]string{winVars[kind.name]: xdg}, home: failHome, want: types.FilesystemPath(xdg)},
			{name: "windows relative appdata falls back", goos: "windows", env: map[string]string{winVars[kind.name]: "rel", envUserProfile: profile}, home: failHome, want: join(profile, winDefaults[kind.name])},
			{name: "windows empty appdata falls back", goos: "windows", env: map[string]string{envUserProfile: profile}, home: failHome, want: join(profile, winDefaults[kind.name])},
			{name: "windows relative userprofile rejected", goos: "windows", env: map[string]string{envUserProfile: "profile"}, home: okHome, wantErr: []string{"absolute", envUserProfile}, wantNotAb: true},
			{name: "windows no appdata nor userprofile", goos: "windows", env: nil, home: okHome, wantErr: []string{winVars[kind.name], envUserProfile}},
		}
		for _, tt := range tests {
			t.Run(kind.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := kind.resolve(New(tt.goos, envLookup(tt.env), tt.home))
				if tt.want != "" {
					if err != nil || got != tt.want {
						t.Fatalf("got (%q, %v), want %q", got, err, tt.want)
					}
					return
				}
				if err == nil {
					t.Fatalf("got %q, want an error", got)
				}
				if got != "" {
					t.Errorf("got path %q alongside error", got)
				}
				for _, part := range tt.wantErr {
					if !strings.Contains(err.Error(), part) {
						t.Errorf("error %q does not contain %q", err, part)
					}
				}
				if errors.Is(err, ErrNotAbsolute) != tt.wantNotAb {
					t.Errorf("errors.Is(err, ErrNotAbsolute) = %v, want %v (err %v)", !tt.wantNotAb, tt.wantNotAb, err)
				}
			})
		}
	}
}

func TestHome(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	got, err := New("linux", nil, func() (string, error) { return home, nil }).Home()
	if err != nil || got != types.FilesystemPath(home) {
		t.Fatalf("Home() = (%q, %v), want %q", got, err, home)
	}
	if _, err := New("linux", nil, func() (string, error) { return ".", nil }).Home(); !errors.Is(err, ErrNotAbsolute) {
		t.Errorf("relative home: err = %v, want ErrNotAbsolute", err)
	}
	lookupErr := errors.New("lookup failed")
	if _, err := New("linux", nil, func() (string, error) { return "", lookupErr }).Home(); !errors.Is(err, lookupErr) || errors.Is(err, ErrNotAbsolute) {
		t.Errorf("failing lookup: err = %v, want the lookup error only", err)
	}
	if _, err := New("linux", nil, func() (string, error) { return "", nil }).Home(); !errors.Is(err, ErrNotAbsolute) {
		t.Errorf("empty home: err = %v, want ErrNotAbsolute", err)
	}
}

func TestRuntimeDir(t *testing.T) {
	t.Parallel()

	abs := t.TempDir()
	tests := []struct {
		name   string
		value  string
		want   types.FilesystemPath
		wantOK bool
	}{
		{name: "absolute", value: abs, want: types.FilesystemPath(abs), wantOK: true},
		{name: "relative is unavailable", value: "run"},
		{name: "unset is unavailable", value: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := New("linux", envLookup(map[string]string{envRuntimeDir: tt.value}), nil).RuntimeDir()
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("RuntimeDir() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestHomeFromEnv(t *testing.T) {
	t.Parallel()

	env := envLookup(map[string]string{envHome: "/h", envUserProfile: `C:\Users\me`, envPlan9Home: "/usr/me"})
	for goos, want := range map[string]string{"linux": "/h", "darwin": "/h", "windows": `C:\Users\me`, "plan9": "/usr/me"} {
		if got, err := HomeFromEnv(goos, env); err != nil || got != want {
			t.Errorf("HomeFromEnv(%s) = (%q, %v), want %q", goos, got, err, want)
		}
	}
	for goos, name := range map[string]string{"linux": envHome, "windows": envUserProfile, "plan9": envPlan9Home} {
		if _, err := HomeFromEnv(goos, envLookup(nil)); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("HomeFromEnv(%s) with no env: err = %v, want one naming %s", goos, err, name)
		}
	}
}

func TestZeroDirsBehavesAsOS(t *testing.T) {
	t.Parallel()

	var zero Dirs
	for _, kind := range baseKinds() {
		gotZero, errZero := kind.resolve(zero)
		gotOS, errOS := kind.resolve(OS())
		if gotZero != gotOS || (errZero == nil) != (errOS == nil) {
			t.Errorf("%s: zero Dirs = (%q, %v), OS() = (%q, %v)", kind.name, gotZero, errZero, gotOS, errOS)
		}
	}
}
