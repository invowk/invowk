// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestCheckCommandDependenciesDeclaredRequireWithoutLockEntryPointsToSync(t *testing.T) {
	t.Parallel()

	req := invowkmod.ModuleRequirement{
		GitURL:  "https://github.com/example/tools.git",
		Version: "^1.0.0",
	}
	moduleDir := t.TempDir()
	depID := invowkmod.ModuleID("io.example.tools")
	callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:   "io.example.caller",
		Version:  "1.0.0",
		Requires: []invowkmod.ModuleRequirement{req},
	})
	callerInfo := &discovery.CommandInfo{
		Name:       invowkfile.CommandName("build"),
		Command:    &invowkfile.Command{Name: "build"},
		Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(moduleDir), Metadata: callerMeta},
	}
	commandSet := &discovery.DiscoveredCommandSet{
		Commands: []*discovery.CommandInfo{{
			Name:       invowkfile.CommandName("tools test"),
			SimpleName: "test",
			SourceID:   discovery.SourceID("tools"),
			ModuleID:   &depID,
		}},
	}
	disc := &stubCommandSetProvider{
		result: discovery.CommandSetResult{Set: commandSet},
	}
	deps := &invowkfile.DependsOn{
		Commands: []invowkfile.CommandDependency{
			{Alternatives: []invowkfile.CommandDependencyRef{"@tools test"}},
		},
	}

	ctx := testDependencyExecutionContext(t, &invowkfile.Command{Name: "build"}, "")
	err := CheckCommandDependenciesExistWithLockProvider(disc, deps, callerInfo, ctx, staticCommandScopeLockProvider{lock: invowkmod.NewLockFile()})
	if err == nil {
		t.Fatal("CheckCommandDependenciesExist() error = nil, want forbidden dependency")
	}
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("errors.As(*DependencyError) = false for %T", err)
	}
	if len(depErr.ForbiddenCommands) != 1 {
		t.Fatalf("len(depErr.ForbiddenCommands) = %d, want 1", len(depErr.ForbiddenCommands))
	}
	detail := depErr.ForbiddenCommands[0].String()
	if !strings.Contains(detail, "invowkmod.lock.cue") || !strings.Contains(detail, "invowk module sync") {
		t.Fatalf("ForbiddenCommands[0] = %q, want lock/sync remediation", detail)
	}
	if strings.Contains(detail, "Add 'tools'") {
		t.Fatalf("ForbiddenCommands[0] = %q, should not tell users to add a command source to requires", detail)
	}
}
