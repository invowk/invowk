// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// buildEnvWithFile builds the environment of a command whose root-, command-
// or implementation-level env.files names file; modulePath is "" for a plain
// invowkfile.
func buildEnvWithFile(t *testing.T, dir, modulePath, level string, file invowkfile.DotenvFilePath) (map[string]string, error) {
	t.Helper()
	inv := &invowkfile.Invowkfile{
		FilePath:   invowkfile.FilesystemPath(filepath.Join(dir, "invowkfile.cue")),
		ModulePath: invowkfile.FilesystemPath(modulePath),
	}
	cmd := testCommandWithScript("test", "echo test", invowkfile.RuntimeNative)
	files := &invowkfile.EnvConfig{Files: []invowkfile.DotenvFilePath{file}}
	switch level {
	case "root":
		inv.Env = files
	case "command":
		cmd.Env = files
	case "implementation":
		cmd.Implementations[0].Env = files
	}
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	builder := &DefaultEnvBuilder{Environ: func() []string { return nil }}
	return builder.Build(ctx, invowkfile.EnvInheritNone)
}

// TestDefaultEnvBuilder_ModuleEnvFilesContained: in module context an env file
// that escapes through a link fails the build at every level, before the
// command runs; a contained one loads (SC-01, F17).
func TestDefaultEnvBuilder_ModuleEnvFilesContained(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	mod := filepath.Join(base, "m.invowkmod")
	writeEnvTestFile(t, filepath.Join(mod, "app.env"), "APP=1\n")
	writeEnvTestFile(t, filepath.Join(base, "outside.env"), "LEAKED=1\n")
	if err := os.MkdirAll(filepath.Join(mod, "invowk_modules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fstree.MustSymlink(t, filepath.Join(base, "outside.env"), filepath.Join(mod, "invowk_modules", "x"))

	for _, level := range []string{"root", "command", "implementation"} {
		env, err := buildEnvWithFile(t, mod, mod, level, "invowk_modules/x")
		if !errors.Is(err, ErrEnvFileOutsideModule) || env != nil {
			t.Errorf("%s level, escaping link: env = %v, err = %v; want ErrEnvFileOutsideModule", level, env, err)
		}
		env, err = buildEnvWithFile(t, mod, mod, level, "app.env")
		if err != nil || env["APP"] != "1" {
			t.Errorf("%s level, contained file: env = %v, err = %v; want APP=1", level, env, err)
		}
	}
}

// TestDefaultEnvBuilder_PlainInvowkfileSymlinkedEnv: a plain invowkfile's
// symlinked .env that points outside its directory keeps loading (decision 46
// item 5).
func TestDefaultEnvBuilder_PlainInvowkfileSymlinkedEnv(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	dir := filepath.Join(base, "project")
	writeEnvTestFile(t, filepath.Join(base, "secrets", ".env"), "SECRET=1\n")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fstree.MustSymlink(t, filepath.Join(base, "secrets", ".env"), filepath.Join(dir, ".env"))
	env, err := buildEnvWithFile(t, dir, "", "root", ".env")
	if err != nil || env["SECRET"] != "1" {
		t.Fatalf("env = %v, err = %v; want SECRET=1 through the link", env, err)
	}
}

// TestLoadEnvFile_ModuleAliasAndCase: the module env read is judged
// physically, so a module path given through an aliased parent (on macOS the
// unresolved os.TempDir() under /var), or in a different case on a
// case-insensitive filesystem, still loads a contained file.
func TestLoadEnvFile_ModuleAliasAndCase(t *testing.T) {
	t.Parallel()

	t.Run("alias", func(t *testing.T) {
		t.Parallel()
		var mod string
		if runtime.GOOS == "darwin" {
			raw := t.TempDir()
			if resolved, err := filepath.EvalSymlinks(raw); err != nil || resolved == raw {
				t.Fatalf("macOS temp dir %s is not aliased (resolved %q, %v)", raw, resolved, err)
			}
			mod = filepath.Join(raw, "mod")
		} else {
			fstree.Require(t, fstree.CapSymlink)
			base := fstree.TempRoot(t)
			if err := os.MkdirAll(filepath.Join(base, "real", "mod"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			fstree.MustSymlink(t, filepath.Join(base, "real"), filepath.Join(base, "alias"))
			mod = filepath.Join(base, "alias", "mod")
		}
		writeEnvTestFile(t, filepath.Join(mod, "app.env"), "APP=1\n")
		env := map[string]string{}
		if err := LoadEnvFile(env, "app.env", mod, mod); err != nil || env["APP"] != "1" {
			t.Fatalf("alias %s: env = %v, err = %v", mod, env, err)
		}
	})

	t.Run("case", func(t *testing.T) {
		t.Parallel()
		base := fstree.TempRoot(t)
		writeEnvTestFile(t, filepath.Join(base, "CaseProbe"), "")
		if _, err := os.Stat(filepath.Join(base, "caseprobe")); err != nil {
			t.Skipf("filesystem at %s is case-sensitive on %s; the case variant needs a case-insensitive one", base, runtime.GOOS)
		}
		writeEnvTestFile(t, filepath.Join(base, "CaseMod", "App.env"), "APP=1\n")
		mod := filepath.Join(base, "casemod")
		env := map[string]string{}
		if err := LoadEnvFile(env, "app.ENV", mod, mod); err != nil || env["APP"] != "1" {
			t.Fatalf("case: env = %v, err = %v", env, err)
		}
	})
}
