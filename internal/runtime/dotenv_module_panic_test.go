// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
)

// TestRootCall_RecoversPanic: a panic inside an os.Root method (Go 1.27 on
// Windows, a link resolving to its own directory) becomes an error wrapping
// errRootResolverPanic with a zero value; a normal result passes through.
func TestRootCall_RecoversPanic(t *testing.T) {
	t.Parallel()
	content, err := rootCall(func(string) ([]byte, error) { panic("index out of range [1] with length 1") }, "a/x")
	if !errors.Is(err, errRootResolverPanic) || content != nil {
		t.Fatalf("panicking call: content=%q err=%v, want nil and errRootResolverPanic", content, err)
	}
	if !strings.Contains(err.Error(), "index out of range") {
		t.Errorf("err = %v, want the panic value in the message", err)
	}
	var gotRel string
	content, err = rootCall(func(rel string) ([]byte, error) { gotRel = rel; return []byte("K=V"), nil }, "a/x")
	if err != nil || string(content) != "K=V" || gotRel != "a/x" {
		t.Fatalf("plain call: content=%q err=%v rel=%q, want K=V for a/x", content, err, gotRel)
	}
}

// TestReadModuleEnvFile_PanicIsReadError: a recovered os.Root panic is a read
// error, never absent (so an optional file is not skipped) and never an escape.
func TestReadModuleEnvFile_PanicIsReadError(t *testing.T) {
	t.Parallel()
	_, mod := moduleEnvFixture(t)
	root, err := os.OpenRoot(mod)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	_, absent, err := readModuleEnvFileWith(root, "app.env",
		func(string) (fs.FileInfo, error) { panic("boom") }, root.ReadFile)
	if !errors.Is(err, errRootResolverPanic) || errors.Is(err, errEnvFileEscapes) || absent {
		t.Fatalf("Stat panic: absent=%v err=%v, want a read error wrapping the panic", absent, err)
	}
	_, absent, err = readModuleEnvFileWith(root, "app.env",
		root.Stat, func(string) ([]byte, error) { panic("boom") })
	if !errors.Is(err, errRootResolverPanic) || errors.Is(err, errEnvFileEscapes) || absent {
		t.Fatalf("ReadFile panic: absent=%v err=%v, want a read error wrapping the panic", absent, err)
	}
}

// TestLoadEnvFile_ModuleLinkToOwnDirectory: sub/self -> "." names a directory
// inside the module. Loading it is a read error on every platform (on Windows
// through the recovered os.Root panic), never containment, and an optional
// one is not skipped.
func TestLoadEnvFile_ModuleLinkToOwnDirectory(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	_, mod := moduleEnvFixture(t)
	fstree.MustSymlink(t, ".", filepath.Join(mod, "sub", "self"))
	for _, path := range []string{"sub/self", "sub/self?"} {
		env := map[string]string{}
		err := LoadEnvFile(env, path, mod, mod)
		if err == nil || errors.Is(err, ErrEnvFileOutsideModule) {
			t.Errorf("%s: err = %v, want a read error", path, err)
		}
		if len(env) != 0 {
			t.Errorf("%s: env = %v, want nothing merged", path, env)
		}
	}
}
