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

func TestCheckCommandDependenciesExistRejectsPresentationAliasWithUnmatchedDiscoverySource(t *testing.T) {
	t.Parallel()

	req := invowkmod.ModuleRequirement{
		GitURL:  "https://github.com/example/tools.git",
		Version: "^1.0.0",
		Alias:   "allowed-tools",
	}
	moduleDir := t.TempDir()
	depID := invowkmod.ModuleID("io.example.tools")
	lock := invowkmod.NewLockFile()
	lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
		GitURL:          req.GitURL,
		Version:         req.Version,
		ResolvedVersion: "1.2.3",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Alias:           req.Alias,
		Namespace:       "allowed-tools",
		ModuleID:        depID,
		ContentHash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
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
		Commands: []*discovery.CommandInfo{
			{
				Name:     invowkfile.CommandName("allowed-tools helper"),
				SourceID: discovery.SourceID("allowed-tools"),
				ModuleID: &depID,
			},
			{
				Name:     invowkfile.CommandName("allowed-tools test"),
				SourceID: discovery.SourceID("other-tools"),
				ModuleID: &depID,
			},
		},
	}
	disc := &stubCommandSetProvider{
		result: discovery.CommandSetResult{Set: commandSet},
	}
	deps := &invowkfile.DependsOn{
		Commands: []invowkfile.CommandDependency{
			{Alternatives: []invowkfile.CommandName{"allowed-tools test"}},
		},
	}
	ctx := testDependencyExecutionContext(t, &invowkfile.Command{Name: "build"}, "")

	err := CheckCommandDependenciesExistWithLockProvider(disc, deps, callerInfo, ctx, staticCommandScopeLockProvider{lock: lock})
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
	if !strings.Contains(depErr.ForbiddenCommands[0].String(), "module 'other-tools' is not accessible") {
		t.Fatalf("ForbiddenCommands[0] = %q", depErr.ForbiddenCommands[0])
	}
}
