// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gossh "golang.org/x/crypto/ssh"

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/pkg/types"
)

// unroutableSSHURL points at TEST-NET-1 (RFC 5737), so a dial can only time
// out; the gate must refuse before that.
const unroutableSSHURL GitURL = "ssh://git@192.0.2.1/x.git"

func gitEnv(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func homeLookup(home string, err error) func() (string, error) {
	return func() (string, error) { return home, err }
}

func writeTestSSHKey(t *testing.T, dir string) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := gossh.MarshalPrivateKey(key, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
}

// TestTrySSHAuthIgnoresRelativeHome changes the working directory, so it
// does not run in parallel.
func TestTrySSHAuthIgnoresRelativeHome(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	work := t.TempDir()
	writeTestSSHKey(t, work)
	t.Chdir(work)

	relative := newGitFetcherWith(types.FilesystemPath(t.TempDir()), basedir.New("linux", nil, homeLookup(".", nil)), gitEnv(nil))
	if relative.auth != nil {
		t.Fatalf("relative home loaded SSH auth %T from the current directory", relative.auth)
	}
	// The same key under an absolute home is loaded, so the negative above
	// is not vacuous.
	absolute := newGitFetcherWith(types.FilesystemPath(t.TempDir()), basedir.New("linux", nil, homeLookup(work, nil)), gitEnv(nil))
	if absolute.auth == nil {
		t.Fatal("absolute home with .ssh/id_ed25519 loaded no SSH auth")
	}
}

// TestHTTPSAuthWithRelativeHome covers base-directory-resolution "HTTPS fetch
// with a relative home still works": with HOME=. and a key under the current
// directory, an https:// URL passes the gate and token authentication is
// chosen, not SSH keys from the current directory.
func TestHTTPSAuthWithRelativeHome(t *testing.T) { //nolint:paralleltest // t.Chdir mutates process cwd.
	work := t.TempDir()
	writeTestSSHKey(t, work)
	t.Chdir(work)

	f := newGitFetcherWith(types.FilesystemPath(t.TempDir()), basedir.New("linux", nil, homeLookup(".", nil)),
		gitEnv(map[string]string{"GIT_TOKEN": "tok"}))
	if err := f.requireTrustedSSHFiles("https://192.0.2.1/x.git"); err != nil {
		t.Fatalf("requireTrustedSSHFiles(https) = %v, want nil", err)
	}
	basic, ok := f.auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("auth = %T, want *http.BasicAuth from GIT_TOKEN", f.auth)
	}
	if basic.Username != "git" || basic.Password != "tok" {
		t.Errorf("auth = %q/%q, want git/tok", basic.Username, basic.Password)
	}
}

func TestRequireTrustedSSHFiles(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	knownHosts := filepath.Join(home, "known_hosts")
	tests := []struct {
		name       string
		url        GitURL
		home       func() (string, error)
		env        map[string]string
		wantRefuse bool
		wantParts  []string
	}{
		{name: "relative home", url: unroutableSSHURL, home: homeLookup(".", nil), wantRefuse: true, wantParts: []string{"home directory", "absolute"}},
		{name: "unavailable home", url: "git@192.0.2.1:x.git", home: homeLookup("", errors.New("$HOME is not defined")), env: map[string]string{sshKnownHostsEnv: knownHosts}, wantRefuse: true, wantParts: []string{"home directory", "not defined"}},
		{name: "relative SSH_KNOWN_HOSTS", url: unroutableSSHURL, home: homeLookup(home, nil), env: map[string]string{sshKnownHostsEnv: "known_hosts"}, wantRefuse: true, wantParts: []string{sshKnownHostsEnv, `"known_hosts"`}},
		{name: "one relative entry in a list", url: unroutableSSHURL, home: homeLookup(home, nil), env: map[string]string{sshKnownHostsEnv: knownHosts + string(filepath.ListSeparator) + "rel"}, wantRefuse: true, wantParts: []string{`"rel"`}},
		{name: "absolute home and SSH_KNOWN_HOSTS", url: unroutableSSHURL, home: homeLookup(home, nil), env: map[string]string{sshKnownHostsEnv: knownHosts}},
		{name: "absolute home without SSH_KNOWN_HOSTS", url: unroutableSSHURL, home: homeLookup(home, nil)},
		{name: "https with a relative home", url: "https://192.0.2.1/x.git", home: homeLookup(".", nil), env: map[string]string{sshKnownHostsEnv: "rel"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newGitFetcherWith(types.FilesystemPath(t.TempDir()), basedir.New("linux", nil, tt.home), gitEnv(tt.env))
			err := f.requireTrustedSSHFiles(tt.url)
			if !tt.wantRefuse {
				if err != nil {
					t.Fatalf("requireTrustedSSHFiles() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, ErrSSHTrustFilesNotAbsolute) {
				t.Fatalf("requireTrustedSSHFiles() = %v, want ErrSSHTrustFilesNotAbsolute", err)
			}
			for _, part := range tt.wantParts {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("error %q does not contain %q", err, part)
				}
			}
		})
	}
}

// TestSSHGateRunsBeforeDial calls every public transport method with an
// unroutable endpoint and a relative home: each must return the typed error,
// never a dial timeout.
func TestSSHGateRunsBeforeDial(t *testing.T) {
	t.Parallel()

	f := newGitFetcherWith(types.FilesystemPath(t.TempDir()), basedir.New("linux", nil, homeLookup(".", nil)), gitEnv(nil))
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	calls := map[string]func() error{
		"ListVersions": func() error { _, err := f.ListVersions(ctx, unroutableSSHURL); return err },
		"Fetch":        func() error { _, _, err := f.Fetch(ctx, unroutableSSHURL, "v1.0.0"); return err },
		"GetCommitForTag": func() error {
			_, err := f.GetCommitForTag(ctx, unroutableSSHURL, "v1.0.0")
			return err
		},
		"CloneShallow": func() error {
			_, err := f.CloneShallow(ctx, unroutableSSHURL, "v1.0.0", "")
			return err
		},
		"ListTagsWithCommits": func() error { _, err := f.ListTagsWithCommits(ctx, unroutableSSHURL); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrSSHTrustFilesNotAbsolute) {
			t.Errorf("%s() = %v, want ErrSSHTrustFilesNotAbsolute before any dial", name, err)
		}
	}
	if !f.IsPrivateRepo(ctx, unroutableSSHURL) {
		t.Error("IsPrivateRepo() = false, want true without connecting")
	}
	if ctx.Err() != nil {
		t.Errorf("context expired (%v): a transport call dialled before the gate", ctx.Err())
	}
}

// TestSSHGateMatchesLegacy proves the zero-change invariant for the gate
// (design D5 row e). Before require-absolute-base-dirs no gate existed, so the
// legacy verdict is always "proceeds to the transport". With an absolute home
// and an absolute or unset SSH_KNOWN_HOSTS the gate must agree. The unset-home
// rows are the documented BREAKING difference (design decision 18).
func TestSSHGateMatchesLegacy(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")
	knownHosts := filepath.Join(existing, "known_hosts")
	homes := map[string]func() (string, error){
		"absolute existing home": homeLookup(existing, nil),
		"absolute missing home":  homeLookup(missing, nil),
		"home unset":             homeLookup("", errors.New("$HOME is not defined")),
	}
	envs := map[string]map[string]string{
		"SSH_KNOWN_HOSTS unset":    nil,
		"SSH_KNOWN_HOSTS absolute": {sshKnownHostsEnv: knownHosts},
	}
	for homeName, home := range homes {
		for envName, env := range envs {
			for _, url := range []GitURL{unroutableSSHURL, "git@192.0.2.1:x.git", "https://192.0.2.1/x.git"} {
				t.Run(homeName+"/"+envName+"/"+string(url), func(t *testing.T) {
					t.Parallel()
					const legacyProceeds = true
					breaking := homeName == "home unset" && !strings.HasPrefix(string(url), "https://")
					f := newGitFetcherWith(types.FilesystemPath(t.TempDir()), basedir.New("linux", nil, home), gitEnv(env))
					proceeds := f.requireTrustedSSHFiles(url) == nil
					if breaking {
						if proceeds {
							t.Error("unset home: gate proceeds, want the documented BREAKING refusal")
						}
						return
					}
					if proceeds != legacyProceeds {
						t.Errorf("gate proceeds = %v, legacy = %v", proceeds, legacyProceeds)
					}
				})
			}
		}
	}
}
