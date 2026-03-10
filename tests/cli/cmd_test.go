// SPDX-License-Identifier: MPL-2.0

// Package cli contains CLI integration tests using testscript.
//
// These tests verify invowk command-line behavior with deterministic
// output capture, replacing the flaky VHS-based tests.
//
// Container tests are separated into TestContainerCLI (cmd_container_test.go)
// and pinned to a single verified engine for deterministic execution. The
// runtime retry logic still protects individual container runs, but the test
// harness no longer treats "any healthy engine" as sufficient.
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

	"github.com/invowk/invowk/pkg/platform"

	"github.com/rogpeppe/go-internal/testscript"
)

var (
	// binaryPath is the path to the built invowk binary.
	binaryPath string
	// projectRoot is the path to the invowk project root.
	projectRoot string
)

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

	// Set GOCOVERDIR for coverage instrumentation (Go 1.20+).
	// When the binary is built with -cover, it writes coverage data to this directory.
	// Each test gets its own subdirectory under a shared GOCOVERDIR root.
	// Reuses testSuffix (already computed above) to avoid redundant SHA-256.
	if coverDir := os.Getenv("GOCOVERDIR"); coverDir != "" {
		testCoverDir := filepath.Join(coverDir, testSuffix)
		if err := os.MkdirAll(testCoverDir, 0o755); err != nil {
			return fmt.Errorf("failed to create coverage directory: %w", err)
		}
		env.Setenv("GOCOVERDIR", testCoverDir)
	}

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
		// "container-available" means the test suite resolved a specific healthy
		// engine for container CLI execution. It intentionally does not mean
		// "some engine works" — the suite must run on the same pinned engine
		// that invowk will use via its test-scoped config.
		return currentContainerHarness().status == containerHarnessStatusReady, nil
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

	// Build invowk. When GOCOVERDIR is set (e.g. via make test-cli-cover),
	// build with -cover so the binary emits coverage data to that directory.
	// Without GOCOVERDIR, build normally to avoid the Go runtime's
	// "warning: GOCOVERDIR not set" stderr noise that breaks ! stderr assertions.
	buildArgs := []string{"build"}
	if os.Getenv("GOCOVERDIR") != "" {
		buildArgs = append(buildArgs, "-cover")
	}
	buildArgs = append(buildArgs, "-o", binaryPath, ".")
	cmd := exec.CommandContext(context.Background(), "go", buildArgs...)
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
