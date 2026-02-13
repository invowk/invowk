// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"invowk-cli/internal/testutil"
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

// containerSetup extends commonSetup with container-specific cleanup.
// It registers a deferred cleanup function that removes any orphaned containers
// if the test times out or fails unexpectedly.
func containerSetup(env *testscript.Env) error {
	if err := commonSetup(env); err != nil {
		return err
	}

	// Generate a unique container name prefix for this test.
	// This uses the same hash-based suffix as INVOWK_PROVISION_TAG_SUFFIX.
	testSuffix := generateTestSuffix(env.WorkDir)
	containerPrefix := containerNamePrefix + testSuffix

	// Register cleanup to run after test completes (pass, fail, or timeout).
	// This prevents resource leaks when tests hang and are terminated by the deadline.
	env.Defer(func() {
		cleanupTestContainers(containerPrefix)
	})

	return nil
}

// cleanupTestContainers removes any containers with the given name prefix.
// This handles cleanup after test timeout or unexpected failure, preventing resource leaks.
//
// The function tries both Docker and Podman since we don't know which engine
// was used to create the containers. All errors are silently ignored since
// cleanup is best-effort and should not fail tests.
func cleanupTestContainers(prefix string) {
	// Try both Docker and Podman - we don't know which engine was used
	engines := []string{"docker", "podman"}

	for _, engine := range engines {
		enginePath, err := exec.LookPath(engine)
		if err != nil {
			continue // Engine not found, try next
		}

		// List containers matching the prefix (including stopped containers)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		listCmd := exec.CommandContext(ctx, enginePath, "ps", "-a", "-q",
			"--filter", "name="+prefix)
		output, err := listCmd.Output()
		cancel()

		if err != nil || len(output) == 0 {
			continue // No containers found or error, try next engine
		}

		// Remove found containers (force remove to handle running containers)
		for containerID := range strings.FieldsSeq(strings.TrimSpace(string(output))) {
			rmCtx, rmCancel := context.WithTimeout(context.Background(), 5*time.Second)
			rmCmd := exec.CommandContext(rmCtx, enginePath, "rm", "-f", containerID)
			_ = rmCmd.Run() // Best effort - ignore errors
			rmCancel()
		}

		return // Only use one engine (the one that found containers)
	}
}

// TestContainerCLI runs container-related testscript tests in parallel.
//
// Container tests are separated from TestCLI for organizational purposes.
// Transient rootless Podman errors (ping_group_range race, exit code 125,
// overlay mount races) are handled by run-level retry in the container
// runtime (runWithRetry in internal/runtime/container_exec.go), enabling
// safe parallel execution.
//
// See .claude/docs/podman-parallel-tests.md for details on the underlying issue.
func TestContainerCLI(t *testing.T) {
	t.Parallel()

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

	// Run each container test in parallel. Transient rootless Podman errors
	// (ping_group_range race, exit code 125) are absorbed by run-level retry
	// in the container runtime, making parallel execution safe.
	for _, testFile := range containerTests {
		testName := strings.TrimSuffix(filepath.Base(testFile), ".txtar")
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// Acquire a slot from the process-wide container semaphore to limit
			// concurrent container operations. This prevents Podman resource
			// exhaustion on constrained CI runners where too many concurrent
			// operations cause indefinite hangs.
			sem := testutil.ContainerSemaphore()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Set a per-test deadline to prevent indefinite hangs.
			// Container operations (image pulls, startup, network issues) can hang forever
			// without an explicit timeout. This ensures tests fail fast with a clear error.
			deadline := time.Now().Add(containerTestTimeout)

			testscript.Run(t, testscript.Params{
				Files:           []string{testFile},
				Setup:           containerSetup,
				Condition:       commonCondition,
				ContinueOnError: true,
				Deadline:        deadline,
			})
		})
	}
}
