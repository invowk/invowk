// SPDX-License-Identifier: MPL-2.0

// Package cli contains CLI integration tests using testscript.
//
// These tests verify invowk command-line behavior with deterministic
// output capture, replacing the flaky VHS-based tests.
package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"invowk-cli/internal/container"

	"github.com/rogpeppe/go-internal/testscript"
)

var (
	// binaryPath is the path to the built invowk binary.
	binaryPath string
	// projectRoot is the path to the invowk project root.
	projectRoot string
	// containerAvailable checks if a functional container runtime (Docker or Podman) is available.
	// This goes beyond just checking for the binary - it verifies the runtime is actually usable.
	containerAvailable = func() bool {
		engine, err := container.AutoDetectEngine()
		if err != nil {
			return false
		}
		return engine.Available()
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

// TestCLI runs all testscript tests in the testdata directory.
func TestCLI(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(env *testscript.Env) error {
			// Add the binary directory to PATH
			binDir := filepath.Dir(binaryPath)
			env.Setenv("PATH", binDir+string(os.PathListSeparator)+env.Getenv("PATH"))

			// Set INVOWK_ROOT to the project root where invkfile.cue is located
			env.Setenv("INVOWK_ROOT", projectRoot)

			// Ensure we're running from the project root for invkfile.cue discovery
			env.Cd = projectRoot

			return nil
		},
		// Custom conditions for testscript
		Condition: func(cond string) (bool, error) {
			// "container-available" returns true if a functional container runtime is available
			// Use [!container-available] to skip tests when no container runtime works
			if cond == "container-available" {
				return containerAvailable, nil
			}
			// Return false with no error for unknown conditions - let testscript handle them
			return false, nil
		},
		// Continue running all tests even if one fails
		ContinueOnError: true,
	})
}
