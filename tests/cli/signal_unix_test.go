// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/testutil"
)

// signalTestEnv builds an isolated subprocess environment, giving each child its own
// GOCOVERDIR subdirectory when coverage collection is active so parallel subtests do not
// race renaming covmeta files on exit.
func signalTestEnv(t *testing.T, workDir string, extra ...string) []string {
	t.Helper()
	baseEnv := os.Environ()
	coverDir := os.Getenv("GOCOVERDIR")
	env := make([]string, 0, len(baseEnv)+len(extra)+1)
	for _, entry := range baseEnv {
		if coverDir != "" && strings.HasPrefix(entry, "GOCOVERDIR=") {
			continue
		}
		env = append(env, entry)
	}
	if coverDir != "" {
		subCoverDir := filepath.Join(coverDir, "sig-"+generateTestSuffix(workDir))
		if err := os.MkdirAll(subCoverDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", subCoverDir, err)
		}
		env = append(env, "GOCOVERDIR="+subCoverDir)
	}
	return append(env, extra...)
}

func TestSignalForwarding_NativeExitCodesAndCleanup(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name         string
		sig          syscall.Signal
		wantExitCode int
	}{
		{name: "SIGHUP exits 129", sig: syscall.SIGHUP, wantExitCode: 129},
		{name: "SIGINT exits 130", sig: syscall.SIGINT, wantExitCode: 130},
		{name: "SIGTERM exits 143", sig: syscall.SIGTERM, wantExitCode: 143},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workDir := t.TempDir()
			tmpDir := filepath.Join(workDir, "tmp")
			if err := os.MkdirAll(tmpDir, 0o755); err != nil {
				t.Fatalf("MkdirAll(tmp) error = %v", err)
			}
			pidFile := filepath.Join(workDir, "grandchild.pid")
			readyFile := filepath.Join(workDir, "ready")

			cue := fmt.Sprintf(`cmds: [{
	name: "slow"
	implementations: [{
		script: {
			interpreter: "sh"
			content: """
				sleep 30 &
				child=$!
				printf '%%s\n' "$child" > %q
				touch %q
				wait "$child"
			"""
		}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`, pidFile, readyFile)
			if err := os.WriteFile(filepath.Join(workDir, "invowkfile.cue"), []byte(cue), 0o600); err != nil {
				t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
			}

			devNull, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatalf("Open(%s) error = %v", os.DevNull, err)
			}
			t.Cleanup(func() { _ = devNull.Close() })

			var stdout, stderr bytes.Buffer
			cmd := exec.CommandContext(t.Context(), binaryPath, "cmd", "slow")
			cmd.Dir = workDir
			cmd.Stdin = devNull
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Env = signalTestEnv(t, workDir, "NO_COLOR=1", "TMPDIR="+tmpDir)
			cmd.SysProcAttr = noTerminalProcAttr()
			cmd.WaitDelay = 5 * time.Second

			if startErr := cmd.Start(); startErr != nil {
				t.Fatalf("Start() error = %v", startErr)
			}

			if !waitForPath(readyFile, 15*time.Second) {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatalf("command did not start; stderr:\n%s", stderr.String())
			}

			rawPID, readErr := os.ReadFile(pidFile)
			if readErr != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatalf("ReadFile(pidFile) error = %v", readErr)
			}
			grandchildPID, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID)))
			if parseErr != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatalf("Atoi(%q) error = %v", string(rawPID), parseErr)
			}

			if sigErr := cmd.Process.Signal(tt.sig); sigErr != nil {
				t.Fatalf("Signal(%v) error = %v", tt.sig, sigErr)
			}

			waitErr := cmd.Wait()
			exitErr, ok := errors.AsType[*exec.ExitError](waitErr)
			if !ok || exitErr.ExitCode() != tt.wantExitCode {
				t.Fatalf("exit = %v, want exit code %d; stdout=%q stderr=%q",
					waitErr, tt.wantExitCode, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty (no redundant ERROR box)", stderr.String())
			}

			testutil.RequirePollUntil(t, 5*time.Second, 20*time.Millisecond, "grandchild exit", func() bool {
				return errors.Is(syscall.Kill(grandchildPID, 0), syscall.ESRCH)
			})

			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatalf("ReadDir(tmpDir) error = %v", err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "invowk-script-") {
					t.Fatalf("leftover temp script %q after signal termination", entry.Name())
				}
			}
		})
	}
}

func TestSignalForwarding_TUISpinSIGTERMStaysExitOne(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	workDir := t.TempDir()
	pidPath := filepath.Join(workDir, "pid")
	cmd := exec.CommandContext(t.Context(), binaryPath, "tui", "spin", "--",
		"sh", "-c", `echo started; echo $$ > "$1"; exec sleep 30`, "sh", pidPath)
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devNull.Close() })

	var stdout, stderr bytes.Buffer
	cmd.Stdin = devNull
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = signalTestEnv(t, workDir, "NO_COLOR=1")
	cmd.SysProcAttr = noTerminalProcAttr()
	cmd.WaitDelay = 5 * time.Second
	if err = cmd.Start(); err != nil {
		t.Fatalf("start invowk: %v", err)
	}

	if !waitForPath(pidPath, 20*time.Second) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("wrapped command did not start; stderr:\n%s", stderr.String())
	}
	if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	err = cmd.Wait()
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("spin error = %v, want exit status 1; stdout %q, stderr:\n%s", err, stdout.String(), stderr.String())
	}
}

func TestSignalForwarding_ContainerCleanup(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	releaseSuiteLock := testutil.AcquireContainerSuiteLock(t)
	defer releaseSuiteLock()

	harness := currentContainerHarness()
	switch harness.status {
	case containerHarnessStatusSkip:
		t.Skipf("skipping: %s", harness.reason)
	case containerHarnessStatusFail:
		t.Fatalf("container CLI infrastructure unavailable: %s", harness.reason)
	case containerHarnessStatusReady:
	}

	workDir := t.TempDir()
	configPath := filepath.Join(workDir, "config.cue")
	if err := os.WriteFile(configPath,
		[]byte(fmt.Sprintf("container_engine: %q\n", harness.engineType)), 0o600); err != nil {
		t.Fatalf("WriteFile(config.cue) error = %v", err)
	}

	readyFile := filepath.Join(workDir, "container.ready")
	cue := `cmds: [{
	name: "container-slow"
	implementations: [{
		script: {content: "touch /workspace/container.ready && sleep 30"}
		runtimes: [{name: "container", image: "debian:stable-slim"}]
		platforms: [{name: "linux"}]
	}]
}]
`
	if err := os.WriteFile(filepath.Join(workDir, "invowkfile.cue"), []byte(cue), 0o600); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("Open(%s) error = %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devNull.Close() })

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binaryPath, "cmd", "--ivk-from", "invowkfile", "container-slow")
	cmd.Dir = workDir
	cmd.Stdin = devNull
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Keep host HOME so rootless Podman uses the same storage as harness.binaryPath
	// and does not create subuid-owned overlay files inside t.TempDir().
	cmd.Env = signalTestEnv(
		t,
		workDir,
		"NO_COLOR=1",
		"INVOWK_CONFIG="+configPath,
		"INVOWK_PROVISION_TAG_SUFFIX="+generateTestSuffix(workDir),
	)
	cmd.SysProcAttr = noTerminalProcAttr()
	cmd.WaitDelay = 15 * time.Second

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !waitForPath(readyFile, 60*time.Second) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("container did not start; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal(SIGTERM) error = %v", err)
	}

	waitErr := cmd.Wait()
	exitErr, ok := errors.AsType[*exec.ExitError](waitErr)
	if !ok || exitErr.ExitCode() != 143 {
		t.Fatalf("exit = %v, want exit code 143; stdout=%q stderr=%q",
			waitErr, stdout.String(), stderr.String())
	}

	runningOut, runningErr := exec.CommandContext(t.Context(), harness.binaryPath,
		"ps", "-q", "--filter", "name=invowk-ephem-", "--filter", "status=running").Output()
	if runningErr != nil {
		t.Fatalf("ps running ephemeral containers error = %v", runningErr)
	}
	if running := strings.TrimSpace(string(runningOut)); running != "" {
		t.Fatalf("ephemeral container(s) still running after SIGTERM cleanup: %q", running)
	}

	testutil.RequirePollUntil(t, 5*time.Second, 50*time.Millisecond, "ephemeral container removal", func() bool {
		psOut, psErr := exec.CommandContext(t.Context(), harness.binaryPath,
			"ps", "-a", "-q", "--filter", "name=invowk-ephem-").Output()
		return psErr == nil && strings.TrimSpace(string(psOut)) == ""
	})
}
