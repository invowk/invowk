// SPDX-License-Identifier: MPL-2.0

// Package cli contains CLI integration tests using testscript.
//
// These tests verify invowk command-line behavior with deterministic
// output capture, replacing the flaky VHS-based tests.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"invowk-cli/internal/container"
	"invowk-cli/pkg/platform"

	"github.com/rogpeppe/go-internal/testscript"
)

var (
	// binaryPath is the path to the built invowk binary.
	binaryPath string
	// projectRoot is the path to the invowk project root.
	projectRoot string
	// containerAvailable checks if a functional container runtime (Docker or Podman) is available.
	// This goes beyond just checking for the binary - it verifies the runtime can actually run
	// Linux containers by performing a smoke test.
	containerAvailable = func() bool {
		engine, err := container.AutoDetectEngine()
		if err != nil {
			return false
		}
		if !engine.Available() {
			return false
		}

		// Smoke test: actually run a minimal container.
		// This catches scenarios where the CLI responds but Linux containers can't run:
		// - Windows Docker Desktop in Windows container mode
		// - Docker daemon not actually running
		// - Permission issues
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := engine.Run(ctx, container.RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"echo", "ok"},
			Remove:  true,
		})
		if err != nil {
			return false
		}
		return result.ExitCode == 0
	}()
)

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

// TestCLI runs all testscript tests in the testdata directory.
func TestCLI(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(env *testscript.Env) error {
			// Add the binary directory to PATH so tests can run 'invowk'
			binDir := filepath.Dir(binaryPath)
			env.Setenv("PATH", binDir+string(os.PathListSeparator)+env.Getenv("PATH"))

			// Set PROJECT_ROOT for tests that need to run against the project's invkfile.
			// Tests with embedded invkfile.cue use 'cd $WORK', while tests that rely on
			// the project's invkfile.cue should use 'cd $PROJECT_ROOT'.
			env.Setenv("PROJECT_ROOT", projectRoot)

			// Set HOME to $WORK directory for container build tests.
			// Docker/Podman CLI requires a valid HOME to store configuration in ~/.docker/
			// or ~/.config/containers/. By default, testscript sets HOME=/no-home which
			// causes "mkdir /no-home: permission denied" errors during docker build.
			// Using WorkDir ensures HOME exists and is writable for the test duration.
			env.Setenv("HOME", env.WorkDir)

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
			// invkfile.cue files to fail because invowk discovered commands from the
			// project root instead of the test's $WORK directory.

			return nil
		},
		// Custom conditions for testscript
		Condition: func(cond string) (bool, error) {
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
				// Return false with no error for unknown conditions - let testscript handle them
				return false, nil
			}
		},
		// Continue running all tests even if one fails
		ContinueOnError: true,
	})
}
