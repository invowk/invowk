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

	for name, script := range map[string]string{
		"operand": "cd " + shQuote(outside),
		"home":    "HOME=" + shQuote(outside) + "; cd",
		"oldpwd":  "OLDPWD=" + shQuote(outside) + "; cd -",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			work := t.TempDir()
			run := runVirtualShTest(t, work, script+"\necho \"status=$?\"\npwd", restricted)
			if !strings.Contains(run.stdout, "status=1") {
				t.Errorf("cd status: stdout %q, want status=1", run.stdout)
			}
			if !strings.Contains(run.stderr, "cd: virtual path denied") {
				t.Errorf("stderr %q does not contain %q", run.stderr, "cd: virtual path denied")
			}
			if !strings.Contains(run.stdout, work) {
				t.Errorf("the working directory changed: stdout %q, want %s", run.stdout, work)
			}
		})
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
		if got := strings.TrimSpace(run.stderr); got != "cd: empty directory path" {
			t.Errorf("stderr = %q, want only mvdan's empty-path message", run.stderr)
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
	t.Run("full", func(t *testing.T) {
		t.Parallel()
		run := runVirtualShTest(t, t.TempDir(), "cd "+shQuote(outside)+" && pwd", invowkfile.VirtualFilesystemAccessFull)
		if run.exit != 0 || run.stderr != "" {
			t.Errorf("full-access cd = (exit %d, stderr %q), want (0, \"\")", run.exit, run.stderr)
		}
	})
}
