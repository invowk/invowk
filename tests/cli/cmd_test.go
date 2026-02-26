// SPDX-License-Identifier: MPL-2.0

// Package cli contains CLI integration tests using testscript.
//
// These tests verify invowk command-line behavior with deterministic
// output capture, replacing the flaky VHS-based tests.
//
// Container tests are separated into TestContainerCLI (cmd_container_test.go)
// and run in parallel. Transient rootless Podman errors are handled by
// run-level retry in the container runtime. See
// .claude/docs/podman-parallel-tests.md for details.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/platform"

	"github.com/rogpeppe/go-internal/testscript"
)

const (
	// maxRetries is the number of attempts for the container smoke test.
	// Retry handles transient OCI errors on rootless Podman.
	maxRetries = 3
)

var (
	// binaryPath is the path to the built invowk binary.
	binaryPath string
	// projectRoot is the path to the invowk project root.
	projectRoot string
	// containerAvailable checks if a functional container runtime (Docker or Podman) is available.
	// This goes beyond just checking for the binary - it verifies the runtime can actually run
	// Linux containers by performing a smoke test with retry logic.
	//
	// The retry handles transient OCI runtime errors that occur on rootless Podman when
	// multiple containers start simultaneously (ping_group_range race condition).
	containerAvailable = func() bool {
		engine, err := container.AutoDetectEngine()
		if err != nil {
			return false
		}
		if !engine.Available() {
			return false
		}

		// Smoke test with retry: actually run a minimal container.
		// This catches scenarios where the CLI responds but Linux containers can't run:
		// - Windows Docker Desktop in Windows container mode
		// - Docker daemon not actually running
		// - Permission issues
		//
		// Retry logic handles transient OCI errors on rootless Podman.
		for attempt := range maxRetries {
			// Intentional: package-level probe has no *testing.T to derive context from.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			result, err := engine.Run(ctx, container.RunOptions{
				Image:   "debian:stable-slim",
				Command: []string{"echo", "ok"},
				Remove:  true,
			})
			cancel()

			if err == nil && result.ExitCode == 0 {
				return true
			}

			// Check if this is a transient OCI error worth retrying
			if !isTransientOCIError(err) {
				return false
			}

			// Exponential backoff: 500ms, 1s, 1.5s
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(500*(attempt+1)) * time.Millisecond)
			}
		}
		return false
	}()
)

// isTransientOCIError checks if an error is a transient container engine error
// that can be retried. Delegates to the shared classifier in the container package
// which covers rootless Podman races, exit code 125, network errors, and more.
func isTransientOCIError(err error) bool {
	return container.IsTransientError(err)
}

// commonSetup provides the shared testscript setup function for all CLI tests.
// This is used by both TestCLI and TestContainerCLI to ensure consistent environment.
func commonSetup(env *testscript.Env) error {
	// Add the binary directory to PATH so tests can run 'invowk'
	binDir := filepath.Dir(binaryPath)
	env.Setenv("PATH", binDir+string(os.PathListSeparator)+env.Getenv("PATH"))

	// Set PROJECT_ROOT for tests that need to run against the project's invowkfile.
	// Tests with embedded invowkfile.cue use 'cd $WORK', while tests that rely on
	// the project's invowkfile.cue should use 'cd $PROJECT_ROOT'.
	env.Setenv("PROJECT_ROOT", projectRoot)

	// Set HOME to $WORK directory for container build tests.
	// Docker/Podman CLI requires a valid HOME to store configuration in ~/.docker/
	// or ~/.config/containers/. By default, testscript sets HOME=/no-home which
	// causes "mkdir /no-home: permission denied" errors during docker build.
	// Using WorkDir ensures HOME exists and is writable for the test duration.
	env.Setenv("HOME", env.WorkDir)

	// Windows-specific: set APPDATA and USERPROFILE so config.ConfigDir() and
	// config.CommandsDir() resolve to test-scoped paths instead of the real
	// system directories. Without this, all Windows tests share the same
	// %APPDATA%\invowk config directory, causing cross-test contamination.
	if runtime.GOOS == platform.Windows {
		env.Setenv("APPDATA", filepath.Join(env.WorkDir, "appdata"))
		env.Setenv("USERPROFILE", env.WorkDir)
	}

	// Set a unique tag suffix for container image provisioning.
	// This prevents race conditions when parallel tests compete to build
	// the same provisioned image (e.g., invowk-provisioned:abc123).
	// Each test gets a unique suffix based on its WorkDir, producing tags
	// like invowk-provisioned:abc123-a1b2c3d4.
	testSuffix := generateTestSuffix(env.WorkDir)
	env.Setenv("INVOWK_PROVISION_TAG_SUFFIX", testSuffix)

	// IMPORTANT: Do NOT set env.Cd here. Each test file controls its own working
	// directory. Tests that need the project root should use 'cd $PROJECT_ROOT'.
	// Setting env.Cd = projectRoot globally caused container tests with embedded
	// invowkfile.cue files to fail because invowk discovered commands from the
	// project root instead of the test's $WORK directory.

	return nil
}

// commonCondition provides the shared testscript condition function for all CLI tests.
// This is used by both TestCLI and TestContainerCLI to ensure consistent conditions.
func commonCondition(cond string) (bool, error) {
	switch cond {
	case "container-available":
		// "container-available" returns true if a functional container runtime is available
		// Use [!container-available] to skip tests when no container runtime works
		return containerAvailable, nil
	case "in-sandbox":
		// "in-sandbox" returns true if running inside a Flatpak or Snap sandbox
		// Use [in-sandbox] to skip tests that require host filesystem permissions
		return platform.IsInSandbox(), nil
	default:
		// Fail loudly on unknown conditions to catch typos and invalid syntax.
		// Built-in conditions ([windows], [linux], [darwin], [short], [net], [exec:...])
		// are handled by the testscript engine before reaching this callback.
		return false, fmt.Errorf("unknown testscript condition: %q", cond)
	}
}

func TestMain(m *testing.M) {
	// Find project root (where go.mod is located)
	wd, err := os.Getwd()
	if err != nil {
		panic("failed to get working directory: " + err.Error())
	}

	// Walk up to find go.mod
	projectRoot = wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			panic("could not find project root (go.mod)")
		}
		projectRoot = parent
	}

	// Build the binary
	binDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		panic("failed to create bin directory: " + err.Error())
	}

	binaryName := "invowk"
	if runtime.GOOS == "windows" {
		binaryName = "invowk.exe"
	}
	binaryPath = filepath.Join(binDir, binaryName)

	// Build invowk
	// Intentional: TestMain has no *testing.T/*testing.B, so t.Context()/b.Context() are unavailable.
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", binaryPath, ".")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build invowk: " + err.Error())
	}

	os.Exit(m.Run())
}

// generateTestSuffix creates a short hash from the test's WorkDir.
// This ensures each parallel test gets a unique container image tag suffix,
// preventing race conditions when multiple tests try to build the same
// provisioned image simultaneously.
func generateTestSuffix(workDir string) string {
	h := sha256.Sum256([]byte(workDir))
	return hex.EncodeToString(h[:])[:8]
}

// TestCLI runs all non-container testscript tests in the testdata directory.
//
// Container tests (container_*.txtar) are excluded and run separately in
// TestContainerCLI to avoid rootless Podman race conditions. See
// .claude/docs/podman-parallel-tests.md for details.
func TestCLI(t *testing.T) {
	// Find all non-container test files
	testdataDir := "testdata"
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("failed to read testdata directory: %v", err)
	}

	var nonContainerTests []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txtar") {
			// Exclude container tests - they run in TestContainerCLI
			if !strings.HasPrefix(entry.Name(), "container_") {
				nonContainerTests = append(nonContainerTests, filepath.Join(testdataDir, entry.Name()))
			}
		}
	}

	if len(nonContainerTests) == 0 {
		t.Skip("no non-container tests found")
	}

	testscript.Run(t, testscript.Params{
		Files:           nonContainerTests,
		Setup:           commonSetup,
		Condition:       commonCondition,
		ContinueOnError: true,
	})
}
