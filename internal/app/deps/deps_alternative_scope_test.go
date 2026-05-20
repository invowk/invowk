// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestCommandDependencyScopeAlternatives(t *testing.T) {
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
	callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:   "io.example.caller",
		Version:  "1.0.0",
		Requires: []invowkmod.ModuleRequirement{req},
	})
	commandSet := &discovery.DiscoveredCommandSet{
		Commands: []*discovery.CommandInfo{
			{
				Name:       invowkfile.CommandName("other-tools test"),
				SimpleName: "test",
				SourceID:   discovery.SourceID("other-tools"),
				ModuleID:   &depID,
			},
			{
				Name:       invowkfile.CommandName("allowed-tools test"),
				SimpleName: "test",
				SourceID:   discovery.SourceID("allowed-tools"),
				ModuleID:   &depID,
			},
		},
	}
	disc := &stubCommandSetProvider{
		result: discovery.CommandSetResult{Set: commandSet},
	}

	t.Run("host accepts later accessible alternative after forbidden candidate", func(t *testing.T) {
		t.Parallel()

		callerInfo := &discovery.CommandInfo{
			Name:       invowkfile.CommandName("build"),
			Command:    &invowkfile.Command{Name: "build"},
			Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(t.TempDir()), Metadata: callerMeta},
		}
		deps := &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []invowkfile.CommandDependencyRef{"@other-tools test", "@allowed-tools test"}},
			},
		}

		err := CheckCommandDependenciesExistWithLockProvider(
			disc,
			deps,
			callerInfo,
			testDependencyExecutionContext(t, callerInfo.Command, invowkfile.RuntimeNative),
			staticCommandScopeLockProvider{lock: lock},
		)
		if err != nil {
			t.Fatalf("CheckCommandDependenciesExist() = %v", err)
		}
	})

	t.Run("container probes later accessible alternative after forbidden candidate", func(t *testing.T) {
		t.Parallel()

		moduleCmd := &invowkfile.Command{
			Name: "build",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{Content: "echo hello"},
				Runtimes: []invowkfile.RuntimeConfig{{
					Name: invowkfile.RuntimeContainer,
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{{
							Alternatives: []invowkfile.CommandDependencyRef{"@other-tools test", "@allowed-tools test"},
						}},
					},
				}},
			}},
		}
		callerInfo := &discovery.CommandInfo{
			Name:       moduleCmd.Name,
			Command:    moduleCmd,
			Invowkfile: &invowkfile.Invowkfile{ModulePath: types.FilesystemPath(t.TempDir()), Metadata: callerMeta},
		}

		var scripts []string
		probe := &filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				scripts = append(scripts, string(ctx.SelectedImpl.Script.Content))
				if strings.Contains(string(ctx.SelectedImpl.Script.Content), "check-cmd 'allowed-tools test'") {
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
			staticCommandScopeLockProvider{lock: lock},
		)
		if err != nil {
			t.Fatalf("ValidateRuntimeDependencies() = %v", err)
		}
		if len(scripts) != 1 || !strings.Contains(scripts[0], "check-cmd 'allowed-tools test'") {
			t.Fatalf("probe scripts = %v, want allowed alternative", scripts)
		}
	})
}
