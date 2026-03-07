// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/invowk/invowk/internal/testutil"
)

const (
	// containerTestTimeout is the maximum time for a single container test.
	// This provides defense against container operations (image pulls, startup, network issues)
	// that hang indefinitely. The timeout is generous enough for normal operations
	// (image pull, container start, script execution) while failing fast on true hangs.
	containerTestTimeout = 3 * time.Minute

	// containerNamePrefix is the prefix used for test container names.
	// This allows cleanup functions to identify and remove orphaned test containers.
	containerNamePrefix = "invowk-test-"
)

// containerSetup extends commonSetup with container-specific cleanup and
// engine pinning. Every container txtar uses the exact same verified engine
// via a test-scoped config file.
func containerSetup(env *testscript.Env) error {
	if err := commonSetup(env); err != nil {
		return err
	}
	if err := ensureContainerSuiteConfig(env); err != nil {
		return err
	}

	// Generate a unique container name prefix for this test.
	// This uses the same hash-based suffix as INVOWK_PROVISION_TAG_SUFFIX.
	testSuffix := generateTestSuffix(env.WorkDir)
	containerPrefix := containerNamePrefix + testSuffix

	// Register cleanup to run after test completes (pass, fail, or timeout).
	// This prevents resource leaks when tests hang and are terminated by the deadline.
	env.Defer(func() {
		cleanupTestContainersForHarness(containerPrefix, currentContainerHarness())
	})

	return nil
}

// TestContainerCLI runs container-related testscript tests using a single
// pinned engine under a suite-scoped cross-process lock. This intentionally
// trades some throughput for deterministic behavior in full-suite runs.
func TestContainerCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container tests in short mode")
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

	// Run each container test serially within the suite. This avoids testscript-
	// level build/run contention while the rest of the repo can stay parallel.
	for _, testFile := range containerTests {
		testName := strings.TrimSuffix(filepath.Base(testFile), ".txtar")
		t.Run(testName, func(t *testing.T) {
			// Set a per-test deadline to prevent indefinite hangs.
			// Container operations (image pulls, startup, network issues) can hang forever
			// without an explicit timeout. This ensures tests fail fast with a clear error.
			deadline := time.Now().Add(containerTestTimeout)

			testscript.Run(t, testscript.Params{
				Files:           []string{testFile},
				Setup:           containerSetup,
				Condition:       commonCondition,
				ContinueOnError: false,
				Deadline:        deadline,
			})
		})
	}
}
