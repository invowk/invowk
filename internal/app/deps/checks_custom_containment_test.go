// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestCustomCheckScriptFileThroughEscapingLinkFails: a custom check whose file
// resolves outside the module through an invowk_modules/ link fails with the
// script path traversal error; the probe never runs and the reader is never
// asked for the file (SC-01, F13).
func TestCustomCheckScriptFileThroughEscapingLinkFails(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	mod := filepath.Join(base, "m.invowkmod")
	if err := os.MkdirAll(filepath.Join(mod, "invowk_modules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(base, "outside.sh")
	if err := os.WriteFile(outside, []byte("echo OUTSIDE-SENTINEL"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	fstree.MustSymlink(t, outside, filepath.Join(mod, "invowk_modules", "x"))

	reads := 0
	ctx := ExecutionContext{
		CommandName:      "build",
		Context:          t.Context(),
		SourceModulePath: customCheckModulePath(mod),
		ReadScriptFile: func(p string) ([]byte, error) {
			reads++
			return os.ReadFile(p)
		},
	}
	deps := &invowkfile.DependsOn{CustomChecks: []invowkfile.CustomCheckDependency{{
		Name:   "escaping-check",
		Script: customCheckFileScript("invowk_modules/x"),
	}}}
	probe := &recordingHostProbe{}
	err := CheckHostCustomCheckDependenciesWithProbe(deps, ctx, probe)
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("err = %v, want a *DependencyError", err)
	}
	if len(depErr.FailedCustomChecks) != 1 || !strings.Contains(depErr.FailedCustomChecks[0].String(), "script path escapes module boundary") {
		t.Fatalf("FailedCustomChecks = %v, want the traversal error", depErr.FailedCustomChecks)
	}
	if strings.Contains(err.Error(), "OUTSIDE-SENTINEL") {
		t.Fatalf("error leaks the outside content: %v", err)
	}
	if len(probe.checks) != 0 || reads != 0 {
		t.Fatalf("probe ran %d checks and the reader was called %d times, want none", len(probe.checks), reads)
	}
}
