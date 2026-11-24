// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// These tests run the u-root built-ins through the real ShRuntime and
// LuaRuntime under restricted access with operands that start with "-"
// (design D4, D8): each operand is classified the way the built-in parses it
// and validated, whatever it starts with.

// dashOperand returns a relative operand that starts with "-" and names
// target from work: "-", then enough ".." to reach the volume root, then
// target's path below its volume root. Cleaning drops the "-" component.
func dashOperand(t *testing.T, work, target string) string {
	t.Helper()
	sep := string(filepath.Separator)
	workRest := strings.TrimPrefix(work, filepath.VolumeName(work))
	depth := len(strings.FieldsFunc(workRest, func(r rune) bool { return os.IsPathSeparator(uint8(r)) }))
	targetRest := strings.TrimLeft(strings.TrimPrefix(target, filepath.VolumeName(target)), `/\`)
	operand := "-" + sep + strings.Repeat(".."+sep, depth+1) + targetRest
	if got := filepath.Clean(filepath.Join(work, operand)); !strings.EqualFold(got, filepath.Clean(target)) {
		t.Fatalf("dash operand %q from %s names %s, want %s", operand, work, got, target)
	}
	return operand
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("%s is gone: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s exists (lstat error %v), want it missing", path, err)
	}
}

func TestVirtualPathDashOperandsDenied(t *testing.T) {
	t.Parallel()
	outside := outsideVirtualRootsDir(t)
	secret := filepath.Join(outside, "secret")
	writeTestFile(t, secret, "outside-secret-content")
	restricted := invowkfile.VirtualFilesystemAccessRestricted

	t.Run("rm", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		writeTestFile(t, filepath.Join(work, "keep"), "keep")
		run := runVirtualShTest(t, work, "rm keep "+shQuote(dashOperand(t, work, secret)), restricted)
		if run.exit == 0 || !strings.Contains(run.stderr, "virtual path denied") {
			t.Errorf("rm = (exit %d, stderr %q), want the denial", run.exit, run.stderr)
		}
		assertPathExists(t, filepath.Join(work, "keep"))
		assertPathExists(t, secret)
	})
	t.Run("mv", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		writeTestFile(t, filepath.Join(work, "a"), "a")
		mustMkdir(t, filepath.Join(work, "b"))
		run := runVirtualShTest(t, work, "mv a b "+shQuote(dashOperand(t, work, outside)), restricted)
		if run.exit == 0 || !strings.Contains(run.stderr, "virtual path denied") {
			t.Errorf("mv = (exit %d, stderr %q), want the denial", run.exit, run.stderr)
		}
		assertPathExists(t, filepath.Join(work, "a"))
		assertPathMissing(t, filepath.Join(outside, "a"))
		assertPathMissing(t, filepath.Join(outside, "b"))
	})
	t.Run("cat", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		writeTestFile(t, filepath.Join(work, "a"), "a-content\n")
		run := runVirtualShTest(t, work, "cat a "+shQuote(dashOperand(t, work, secret)), restricted)
		if run.exit == 0 || !strings.Contains(run.stderr, "virtual path denied") {
			t.Errorf("cat = (exit %d, stderr %q), want the denial", run.exit, run.stderr)
		}
		if strings.Contains(run.stdout, "outside-secret-content") {
			t.Errorf("cat printed the outside file: %q", run.stdout)
		}
	})
	t.Run("touch", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		created := filepath.Join(outside, "touched")
		run := runVirtualShTest(t, work, "touch a "+shQuote(dashOperand(t, work, created)), restricted)
		if run.exit == 0 || !strings.Contains(run.stderr, "virtual path denied") {
			t.Errorf("touch = (exit %d, stderr %q), want the denial", run.exit, run.stderr)
		}
		assertPathMissing(t, created)
		assertPathMissing(t, filepath.Join(work, "a"))
	})
	t.Run("lua cat", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		writeTestFile(t, filepath.Join(work, "a"), "a-content\n")
		script := fmt.Sprintf(`
local code, err = invowk.cmd.cat("a", %q)
print("code=" .. tostring(code))
print("err=" .. tostring(err))
`, dashOperand(t, work, secret))
		run := runVirtualLuaTest(t, work, script, restricted)
		if strings.Contains(run.stdout, "outside-secret-content") {
			t.Errorf("invowk.cmd.cat printed the outside file: %q", run.stdout)
		}
		if !strings.Contains(run.stdout+run.stderr, "virtual path denied") || strings.Contains(run.stdout, "code=0") {
			t.Errorf("invowk.cmd.cat = stdout %q, stderr %q; want the denial", run.stdout, run.stderr)
		}
	})
}

func TestVirtualPathDashOperandsDoubleDash(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	writeTestFile(t, filepath.Join(work, "-x"), "x")
	writeTestFile(t, filepath.Join(work, "keep"), "keep")
	run := runVirtualShTest(t, work, "rm -- -x", invowkfile.VirtualFilesystemAccessRestricted)
	if run.exit != 0 {
		t.Fatalf("rm -- -x = (exit %d, stderr %q)", run.exit, run.stderr)
	}
	assertPathMissing(t, filepath.Join(work, "-x"))
	assertPathExists(t, filepath.Join(work, "keep"))
	if entries, err := os.ReadDir(work); err != nil || len(entries) != 1 {
		t.Errorf("rm -- -x left %v (%v), want only keep", entries, err)
	}
}

func TestVirtualPathDashOperandsGzipSuffixSeparator(t *testing.T) {
	t.Parallel()
	for _, access := range []invowkfile.VirtualFilesystemAccess{
		invowkfile.VirtualFilesystemAccessRestricted,
		invowkfile.VirtualFilesystemAccessFull,
	} {
		t.Run(string(access), func(t *testing.T) {
			t.Parallel()
			work := t.TempDir()
			writeTestFile(t, filepath.Join(work, "f"), "f")
			mustMkdir(t, filepath.Join(work, "fx"))
			run := runVirtualShTest(t, work, "gzip -k -S x/y f", access)
			if run.exit == 0 || !strings.Contains(run.stderr, "-S") {
				t.Errorf("gzip -k -S x/y f = (exit %d, stderr %q), want the refusal naming -S", run.exit, run.stderr)
			}
			for _, name := range []string{"fx/y", "f.gz"} {
				assertPathMissing(t, filepath.Join(work, filepath.FromSlash(name)))
			}
			if entries, err := os.ReadDir(filepath.Join(work, "fx")); err != nil || len(entries) != 0 {
				t.Errorf("gzip created %v below fx (%v)", entries, err)
			}
		})
	}
}

// runVirtualLuaTest runs script in virtual-lua, with its built-ins enabled,
// through the real LuaRuntime, with work as the working directory and
// invowkfile directory, under access.
func runVirtualLuaTest(t *testing.T, work, script string, access invowkfile.VirtualFilesystemAccess) virtualShTestRun {
	t.Helper()
	inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(work, "invowkfile.cue"))}
	cmd := testCommandWithScript("virtual-lua-test", script, invowkfile.RuntimeVirtualLua)
	cmd.Implementations[0].Platforms = testPlatformsWithVirtualFilesystem(access, nil)
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.WorkDir = invowkfile.WorkDir(work)
	ctx.SelectedRuntime = invowkfile.RuntimeVirtualLua
	ctx.SelectedImpl = &cmd.Implementations[0]
	var stdout, stderr strings.Builder
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &stderr
	result := NewLuaRuntime(true).Execute(ctx)
	return virtualShTestRun{stdout: stdout.String(), stderr: stderr.String(), exit: int(result.ExitCode)}
}
