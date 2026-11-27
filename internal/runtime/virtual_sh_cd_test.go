// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

type virtualShTestRun struct {
	stdout string
	stderr string
	exit   int
}

// runVirtualShTest runs script in virtual-sh through the real ShRuntime, with
// work as the working directory and invowkfile directory, under access.
func runVirtualShTest(t *testing.T, work, script string, access invowkfile.VirtualFilesystemAccess) virtualShTestRun {
	t.Helper()
	inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(work, "invowkfile.cue"))}
	cmd := testCommandWithScript("virtual-sh-test", script, invowkfile.RuntimeVirtualSh)
	cmd.Implementations[0].Platforms = testPlatformsWithVirtualFilesystem(access, nil)
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.WorkDir = invowkfile.WorkDir(work)
	var stdout, stderr bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &stderr
	result := NewShRuntime(true).Execute(ctx)
	if result.Error != nil && result.ExitCode == 0 {
		t.Fatalf("virtual-sh run failed: %v", result.Error)
	}
	// A failing u-root built-in reports through the result's error, not
	// through the script's standard error.
	if result.Error != nil {
		stderr.WriteString(result.Error.Error())
	}
	return virtualShTestRun{stdout: stdout.String(), stderr: stderr.String(), exit: int(result.ExitCode)}
}

// outsideVirtualRootsDir creates a directory below the user's home directory,
// which is no implicit allowed root (the temporary directory is one), and
// removes it when the test ends.
func outsideVirtualRootsDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	dir, err := os.MkdirTemp(home, ".invowk-outside-*")
	if err != nil {
		t.Fatalf("MkdirTemp(home): %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// shQuote quotes s for a POSIX shell script.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func TestVirtualShCdDenialNamesTheDenial(t *testing.T) {
	t.Parallel()
	outside := outsideVirtualRootsDir(t)
	restricted := invowkfile.VirtualFilesystemAccessRestricted

	for _, tt := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "operand", script: "cd " + shQuote(outside), want: "cd: virtual path denied"},
		{name: "dashdash", script: "cd -- " + shQuote(outside), want: "cd: virtual path denied"},
		{name: "home", script: "HOME=" + shQuote(outside) + "; cd", want: "cd: virtual path denied"},
		{name: "oldpwd", script: "OLDPWD=" + shQuote(outside) + "; cd -", want: "cd: virtual path denied"},
		{name: "builtin_cd", script: "builtin cd " + shQuote(outside), want: "cd: virtual path denied"},
		{name: "command_cd", script: "command cd " + shQuote(outside), want: "cd: virtual path denied"},
		{name: "command_dashdash_cd", script: "command -- cd " + shQuote(outside), want: "cd: virtual path denied"},
		{name: "pushd_operand", script: "pushd " + shQuote(outside), want: "pushd: virtual path denied"},
		{name: "builtin_pushd", script: "builtin pushd " + shQuote(outside), want: "pushd: virtual path denied"},
		{name: "command_pushd", script: "command pushd " + shQuote(outside), want: "pushd: virtual path denied"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			work := t.TempDir()
			assertVirtualShCdSingleLineDenial(t, work, tt.script, tt.want, restricted)
		})
	}

	t.Run("stack_popd_after_pushd_n", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		assertVirtualShCdStackDenial(t, work, "pushd -n "+shQuote(outside)+"\npopd", "popd: virtual path denied", restricted)
	})

	t.Run("stack_pushd_swap_after_pushd_n", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		assertVirtualShCdStackDenial(t, work, "pushd -n "+shQuote(outside)+"\npushd", "pushd: virtual path denied", restricted)
	})
}

func assertVirtualShCdStackDenial(
	t *testing.T,
	work, script, want string,
	access invowkfile.VirtualFilesystemAccess,
) {
	t.Helper()
	run := runVirtualShTest(t, work, script+"\necho \"status=$?\"\npwd", access)
	if !strings.Contains(run.stdout, "status=1") || !strings.Contains(run.stderr, want) {
		t.Errorf("stack denial = (stdout %q, stderr %q), want status=1 and %q", run.stdout, run.stderr, want)
	}
	if !strings.Contains(run.stdout, work) {
		t.Errorf("the working directory changed: stdout %q, want %s", run.stdout, work)
	}
}

func assertVirtualShCdSingleLineDenial(
	t *testing.T,
	work, script, want string,
	access invowkfile.VirtualFilesystemAccess,
) {
	t.Helper()
	run := runVirtualShTest(t, work, script+"\necho \"status=$?\"\npwd", access)
	if !strings.Contains(run.stdout, "status=1") {
		t.Errorf("status: stdout %q, want status=1", run.stdout)
	}
	if !strings.Contains(run.stderr, want) {
		t.Errorf("stderr %q does not contain %q", run.stderr, want)
	}
	if strings.Contains(run.stderr, "no such file or directory") {
		t.Errorf("stderr %q unexpectedly included mvdan second error line", run.stderr)
	}
	if strings.Count(strings.TrimSpace(run.stderr), "\n") != 0 {
		t.Errorf("stderr %q has more than one line", run.stderr)
	}
	if !strings.Contains(run.stdout, work) {
		t.Errorf("the working directory changed: stdout %q, want %s", run.stdout, work)
	}
}

func TestVirtualShCdQuietCases(t *testing.T) {
	t.Parallel()
	outside := outsideVirtualRootsDir(t)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		run := runVirtualShTest(t, t.TempDir(), `cd ""`, invowkfile.VirtualFilesystemAccessRestricted)
		if run.exit != 1 {
			t.Errorf("cd \"\" exit = %d, want 1", run.exit)
		}
		if strings.TrimSpace(run.stderr) != "cd: empty directory path" {
			t.Errorf("stderr = %q, want only mvdan's empty-path message", run.stderr)
		}
	})
	t.Run("command_v_cd", func(t *testing.T) {
		t.Parallel()
		run := runVirtualShTest(t, t.TempDir(), "command -v cd", invowkfile.VirtualFilesystemAccessRestricted)
		if run.exit != 0 || strings.TrimSpace(run.stdout) != "cd" || run.stderr != "" {
			t.Errorf("command -v cd = (exit %d, stdout %q, stderr %q)", run.exit, run.stdout, run.stderr)
		}
	})
	t.Run("inside", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		mustMkdir(t, filepath.Join(work, "sub"))
		run := runVirtualShTest(t, work, "cd sub && pwd", invowkfile.VirtualFilesystemAccessRestricted)
		if run.exit != 0 || run.stderr != "" {
			t.Errorf("cd sub = (exit %d, stderr %q), want (0, \"\")", run.exit, run.stderr)
		}
		if !strings.HasSuffix(strings.TrimSpace(run.stdout), "sub") {
			t.Errorf("pwd after cd sub = %q", run.stdout)
		}
	})
	t.Run("full_cd_pushd_popd", func(t *testing.T) {
		t.Parallel()
		work := t.TempDir()
		run := runVirtualShTest(t, work, "pushd "+shQuote(outside)+" && popd && cd "+shQuote(outside)+" && pwd", invowkfile.VirtualFilesystemAccessFull)
		if run.exit != 0 || run.stderr != "" {
			t.Errorf("full-access cd/pushd/popd = (exit %d, stderr %q), want (0, \"\")", run.exit, run.stderr)
		}
	})
}
