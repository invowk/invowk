// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestValidateRuntimeDependenciesNonContainerNoop(t *testing.T) {
	t.Parallel()

	cmd := runtimeDependencyCommand("build")
	err := ValidateRuntimeDependencies(
		&stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}}},
		runtimeDependencyCommandInfo(cmd),
		nil,
		testDependencyExecutionContext(t, cmd, invowkfile.RuntimeVirtualSh),
		nil,
	)
	if err != nil {
		t.Fatalf("ValidateRuntimeDependencies() = %v", err)
	}
}

func TestValidateRuntimeDependenciesContainerDelegatesToProbe(t *testing.T) {
	t.Parallel()

	cmd := runtimeDependencyCommand("build")
	probe := runtimeDependencyProbe("check-cmd 'build'")

	err := ValidateRuntimeDependencies(
		&stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{Commands: []*discovery.CommandInfo{{Name: "build"}}}}},
		runtimeDependencyCommandInfo(cmd),
		probe,
		testDependencyExecutionContext(t, cmd, invowkfile.RuntimeContainer),
		nil,
	)
	if err != nil {
		t.Fatalf("ValidateRuntimeDependencies() = %v", err)
	}
}

func TestValidateRuntimeDependenciesContainerUsesModuleCommandName(t *testing.T) {
	t.Parallel()

	moduleID := invowkmod.ModuleID("io.example.mod")
	moduleMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:  moduleID,
		Version: "1.0.0",
	})
	moduleCmd := runtimeDependencyCommand("build")
	callerInfo := &discovery.CommandInfo{
		Name:       "mod build",
		SourceID:   "mod",
		ModuleID:   &moduleID,
		Command:    moduleCmd,
		Invowkfile: &invowkfile.Invowkfile{Metadata: moduleMeta},
	}
	disc := &stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{
		Commands: []*discovery.CommandInfo{{
			Name:       "mod build",
			SourceID:   "mod",
			ModuleID:   &moduleID,
			Invowkfile: &invowkfile.Invowkfile{Metadata: moduleMeta},
		}},
	}}}
	var scripts []string
	probe := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			scripts = append(scripts, string(ctx.SelectedImpl.Script.Content))
			if strings.Contains(string(ctx.SelectedImpl.Script.Content), "check-cmd 'mod build'") {
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1}
		},
	}

	err := ValidateRuntimeDependencies(
		disc,
		callerInfo,
		probe,
		testDependencyExecutionContext(t, moduleCmd, invowkfile.RuntimeContainer),
		nil,
	)
	if err != nil {
		t.Fatalf("ValidateRuntimeDependencies() = %v", err)
	}
	if len(scripts) != 1 || !strings.Contains(scripts[0], "check-cmd 'mod build'") {
		t.Fatalf("probe scripts = %v, want resolved module command", scripts)
	}
}

func TestValidateRuntimeDependenciesChecksScopeBeforeProbe(t *testing.T) {
	t.Parallel()

	req := invowkmod.ModuleRequirement{
		GitURL:  "https://github.com/example/tools.git",
		Version: "^1.0.0",
		Alias:   "allowed-tools",
	}
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
	moduleCmd := runtimeDependencyCommand("@other-tools test")
	callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:   "io.example.caller",
		Version:  "1.0.0",
		Requires: []invowkmod.ModuleRequirement{req},
	})
	callerInfo := &discovery.CommandInfo{
		Name:       moduleCmd.Name,
		Command:    moduleCmd,
		Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(t.TempDir()), Metadata: callerMeta},
	}
	disc := &stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{
		Commands: []*discovery.CommandInfo{{
			Name:       "other-tools test",
			SimpleName: "test",
			SourceID:   "other-tools",
			ModuleID:   &depID,
		}},
	}}}
	probe := &filepathStubRuntime{
		execFn: func(*runtimepkg.ExecutionContext) *runtimepkg.Result {
			t.Fatal("container probe should not run after command scope denial")
			return &runtimepkg.Result{ExitCode: 0}
		},
	}

	err := ValidateRuntimeDependencies(
		disc,
		callerInfo,
		probe,
		testDependencyExecutionContext(t, moduleCmd, invowkfile.RuntimeContainer),
		staticCommandScopeLockProvider{lock: lock},
	)
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("errors.As(*DependencyError) = false for %T", err)
	}
	if len(depErr.ForbiddenCommands) != 1 {
		t.Fatalf("len(ForbiddenCommands) = %d, want 1", len(depErr.ForbiddenCommands))
	}
}

func runtimeDependencyCommand(refs ...invowkfile.CommandDependencyRef) *invowkfile.Command {
	return &invowkfile.Command{
		Name: "build",
		Implementations: []invowkfile.Implementation{{
			Script: invowkfile.ImplementationScript{Content: "echo hello"},
			Runtimes: []invowkfile.RuntimeConfig{{
				Name: invowkfile.RuntimeContainer,
				DependsOn: &invowkfile.DependsOn{
					Commands: []invowkfile.CommandDependency{{Alternatives: refs}},
				},
			}},
		}},
	}
}

func runtimeDependencyCommandInfo(cmd *invowkfile.Command) *discovery.CommandInfo {
	return &discovery.CommandInfo{
		Name:       cmd.Name,
		Command:    cmd,
		Invowkfile: &invowkfile.Invowkfile{},
	}
}

func runtimeDependencyProbe(scriptFragment string) *filepathStubRuntime {
	return &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			if strings.Contains(string(ctx.SelectedImpl.Script.Content), scriptFragment) {
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1}
		},
	}
}
