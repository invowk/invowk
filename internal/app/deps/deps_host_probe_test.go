// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestValidateHostDependenciesWithHostProbeUsesInjectedProbe(t *testing.T) {
	t.Parallel()

	invowkfilePath := filepath.Join(t.TempDir(), "work", "invowkfile.cue")
	expectedFilepath := filepath.Join(filepath.Dir(invowkfilePath), "data", "input.txt")

	cmd := &invowkfile.Command{
		Name: "build",
		DependsOn: &invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{{
				Alternatives: []invowkfile.BinaryName{"tool-a"},
			}},
			Filepaths: []invowkfile.FilepathDependency{{
				Alternatives: []invowkfile.FilesystemPath{"data/input.txt"},
				Readable:     true,
			}},
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Name:   "custom",
				Script: invowkfile.CustomCheckScript{Content: "exit 0"},
			}},
		},
		Implementations: []invowkfile.Implementation{{
			Script:   invowkfile.ImplementationScript{Content: "echo ok"},
			Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
		}},
	}
	cmdInfo := &discovery.CommandInfo{
		Name:       cmd.Name,
		Command:    cmd,
		Invowkfile: &invowkfile.Invowkfile{FilePath: types.FilesystemPath(invowkfilePath)},
	}
	execCtx := testDependencyExecutionContext(t, cmd, invowkfile.RuntimeNative)
	probe := &recordingHostProbe{}

	err := ValidateHostDependenciesWithHostProbe(
		&stubCommandSetProvider{result: discovery.CommandSetResult{Set: &discovery.DiscoveredCommandSet{}}},
		cmdInfo,
		execCtx,
		map[string]string{},
		nil,
		probe,
	)
	if err != nil {
		t.Fatalf("ValidateHostDependenciesWithHostProbe() = %v", err)
	}
	if len(probe.tools) != 1 || probe.tools[0] != "tool-a" {
		t.Fatalf("probe tools = %v, want [tool-a]", probe.tools)
	}
	if len(probe.filepaths) != 1 || probe.filepaths[0] != types.FilesystemPath(expectedFilepath) {
		t.Fatalf("probe filepaths = %v, want resolved path", probe.filepaths)
	}
	if len(probe.checks) != 1 || probe.checks[0] != "custom" {
		t.Fatalf("probe checks = %v, want [custom]", probe.checks)
	}
}
