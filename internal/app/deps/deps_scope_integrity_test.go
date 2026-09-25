// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// TestCheckCommandDependenciesExistComparesCallerLockedContent replays
// LockIntegrity.f6CheckCallerHash through the real dependency check: the
// target's commands were discovered in a copy (for instance a sibling module's
// vendored version) that differs from what the caller locked, so the call is
// forbidden; a copy of the locked content is admitted.
func TestCheckCommandDependenciesExistComparesCallerLockedContent(t *testing.T) {
	t.Parallel()

	depID := invowkmod.ModuleID("io.example.tools")
	req := invowkmod.ModuleRequirement{GitURL: "https://github.com/example/tools.git", Version: "^1.0.0"}
	locked := writeScopeModuleDir(t, "cmds: {} // the version the caller locked")
	other := writeScopeModuleDir(t, "cmds: {} // a sibling's pinned version")
	lockedHash, err := invowkmod.ComputeModuleHash(locked)
	if err != nil {
		t.Fatalf("ComputeModuleHash() error = %v", err)
	}
	lock := invowkmod.NewLockFile()
	lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
		GitURL:          req.GitURL,
		Version:         req.Version,
		ResolvedVersion: "1.2.3",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Namespace:       "io.example.tools@1.2.3",
		ModuleID:        depID,
		CommandSourceID: "io.example.tools",
		ContentHash:     lockedHash,
	}
	callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:   "io.example.caller",
		Version:  "1.0.0",
		Requires: []invowkmod.ModuleRequirement{req},
	})
	callerInfo := &discovery.CommandInfo{
		Name:       invowkfile.CommandName("build"),
		Command:    &invowkfile.Command{Name: "build"},
		Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(t.TempDir()), Metadata: callerMeta},
	}
	deps := &invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{
		{Alternatives: []invowkfile.CommandDependencyRef{"@io.example.tools helper"}},
	}}
	check := func(targetDir string) error {
		target := &discovery.CommandInfo{
			Name:       invowkfile.CommandName("io.example.tools helper"),
			SimpleName: "helper",
			SourceID:   discovery.SourceID("io.example.tools"),
			ModuleID:   &depID,
			Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(targetDir)},
		}
		disc := &stubCommandSetProvider{result: discovery.CommandSetResult{
			Set: &discovery.DiscoveredCommandSet{Commands: []*discovery.CommandInfo{target}},
		}}
		ctx := testDependencyExecutionContext(t, &invowkfile.Command{Name: "build"}, "")
		return CheckCommandDependenciesExistWithLockProvider(disc, deps, callerInfo, ctx, staticCommandScopeLockProvider{lock: lock})
	}

	if lockedErr := check(locked); lockedErr != nil {
		t.Fatalf("copy of the locked content: CheckCommandDependenciesExist() = %v, want nil", lockedErr)
	}
	err = check(other)
	var depErr *DependencyError
	if !errors.As(err, &depErr) || len(depErr.ForbiddenCommands) != 1 {
		t.Fatalf("copy with other content: CheckCommandDependenciesExist() = %v, want one forbidden command", err)
	}
}

func writeScopeModuleDir(t *testing.T, invowkfileBody string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte(invowkfileBody), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return dir
}
