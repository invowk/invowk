// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestContainerCLI runs container-related testscript tests sequentially.
//
// Container tests are separated from TestCLI because rootless Podman has a known
// race condition when multiple containers start simultaneously, causing sporadic
// failures with "ping_group_range" OCI runtime errors.
//
// By running container tests in a separate function without calling t.Parallel(),
// we ensure sequential execution while allowing non-container tests to run in parallel.
//
// See .claude/docs/podman-parallel-tests.md for details on the underlying issue.
func TestContainerCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container tests in short mode")
	}

	if !containerAvailable {
		t.Skip("skipping: no functional container runtime available")
	}

	// Find all container test files
	testdataDir := "testdata"
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("failed to read testdata directory: %v", err)
	}

	var containerTests []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "container_") && strings.HasSuffix(entry.Name(), ".txtar") {
			containerTests = append(containerTests, filepath.Join(testdataDir, entry.Name()))
		}
	}

	if len(containerTests) == 0 {
		t.Skip("no container tests found")
	}

	// Run each container test sequentially to avoid Podman race conditions.
	// We use testscript.Params.Files instead of Dir to run one file at a time,
	// preventing testscript's internal t.Parallel() from running them concurrently.
	for _, testFile := range containerTests {
		testName := strings.TrimSuffix(filepath.Base(testFile), ".txtar")
		t.Run(testName, func(t *testing.T) {
			// NOTE: Do NOT call t.Parallel() here - sequential execution is intentional
			testscript.Run(t, testscript.Params{
				Files:           []string{testFile},
				Setup:           commonSetup,
				Condition:       commonCondition,
				ContinueOnError: true,
			})
		})
	}
}
