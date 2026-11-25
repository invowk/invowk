// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// workdirInteractiveCase is an invowkfile shape for the interactive launcher
// spec, and whether the spec must carry the module root.
type workdirInteractiveCase struct {
	name       string
	moduleRoot bool
	workdir    invowkfile.WorkDir
	module     bool
}

func workdirInteractiveCases() []workdirInteractiveCase {
	return []workdirInteractiveCase{
		{name: "module workdir outside the module", module: true, workdir: "../shared", moduleRoot: true},
		{name: "module default workdir", module: true},
		{name: "plain invowkfile workdir outside its directory", workdir: "../shared"},
	}
}

// workdirInteractiveContext builds the execution context of a command with
// the case's workdir, run with the given virtual runtime.
func workdirInteractiveContext(t *testing.T, tc workdirInteractiveCase, rt invowkfile.RuntimeMode) (ctx *ExecutionContext, module string) {
	t.Helper()
	base := fstree.TempRoot(t)
	module = mustMkdir(t, filepath.Join(base, "mod.invowkmod"))
	mustMkdir(t, filepath.Join(base, "shared"))
	script := invowkfile.ScriptContent("echo")
	if rt == invowkfile.RuntimeVirtualLua {
		script = "print('probe')"
	}
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(module, "invowkfile.cue")),
		Commands: []invowkfile.Command{{
			Name:    "probe",
			WorkDir: tc.workdir,
			Implementations: []invowkfile.Implementation{{
				Script:    invowkfile.ImplementationScript{Content: script},
				Runtimes:  []invowkfile.RuntimeConfig{{Name: rt}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}},
	}
	if tc.module {
		inv.ModulePath = invowkfile.FilesystemPath(module)
	}
	ctx = NewExecutionContext(t.Context(), &inv.Commands[0], inv)
	ctx.SelectedRuntime = rt
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]
	return ctx, module
}

// TestInteractiveSpecCarriesWorkDirModuleRoot checks that the launcher spec of
// both virtual runtimes carries the module root for a module-declared workdir
// and nil for a trusted one (virtual-runtime-sandbox, "Interactive mode makes
// the same decision").
func TestInteractiveSpecCarriesWorkDirModuleRoot(t *testing.T) {
	t.Parallel()
	for _, tc := range workdirInteractiveCases() {
		t.Run("virtual-sh "+tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, module := workdirInteractiveContext(t, tc, invowkfile.RuntimeVirtualSh)
			var got ShInteractiveCommandSpec
			factory := func(ctx context.Context, spec ShInteractiveCommandSpec) (*exec.Cmd, error) {
				got = spec
				return exec.CommandContext(ctx, "test-invowk", "virtual-launcher"), nil
			}
			prepared, err := NewShRuntime(false, WithInteractiveCommandFactory(factory)).PrepareInteractive(ctx)
			if err != nil {
				t.Fatalf("PrepareInteractive() error = %v", err)
			}
			t.Cleanup(prepared.Cleanup)
			expectSpecModuleRoot(t, got.WorkDirModuleRoot, tc.moduleRoot, module)
		})
		t.Run("virtual-lua "+tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, module := workdirInteractiveContext(t, tc, invowkfile.RuntimeVirtualLua)
			var got LuaInteractiveCommandSpec
			factory := func(ctx context.Context, spec LuaInteractiveCommandSpec) (*exec.Cmd, error) {
				got = spec
				return exec.CommandContext(ctx, "test-invowk", "lua-launcher"), nil
			}
			prepared, err := NewLuaRuntime(false, WithLuaInteractiveCommandFactory(factory)).PrepareInteractive(ctx)
			if err != nil {
				t.Fatalf("PrepareInteractive() error = %v", err)
			}
			t.Cleanup(prepared.Cleanup)
			expectSpecModuleRoot(t, got.WorkDirModuleRoot, tc.moduleRoot, module)
		})
	}
}

func expectSpecModuleRoot(t *testing.T, got *invowkfile.FilesystemPath, want bool, module string) {
	t.Helper()
	switch {
	case !want && got != nil:
		t.Fatalf("WorkDirModuleRoot = %q, want nil for a trusted workdir", *got)
	case want && got == nil:
		t.Fatal("WorkDirModuleRoot = nil, want the module root")
	case want && string(*got) != module:
		t.Fatalf("WorkDirModuleRoot = %q, want %q", *got, module)
	}
}

// interactiveWorkdirProbe returns a workdir at the filesystem root and a path
// below it that no other root encloses: the interactive runners are given
// that workdir, with and without a module root.
func interactiveWorkdirProbe(t *testing.T) (module, workdir, probe string) {
	t.Helper()
	base := fstree.TempRoot(t)
	module = mustMkdir(t, filepath.Join(base, "module"))
	workdir = filepath.VolumeName(base) + string(filepath.Separator)
	probe = filepath.Join(workdir, "etc", "hostname")
	if runtime.GOOS == goosWindows {
		probe = filepath.Join(workdir, "invowk-interactive-probe-"+filepath.Base(base), "data.txt")
	}
	return module, workdir, filepath.ToSlash(probe)
}

// TestRunShScriptWorkDirModuleRoot checks that the interactive virtual-sh
// runner denies a read below a module-declared workdir outside the module
// when the module root is given, and does not when it is not.
func TestRunShScriptWorkDirModuleRoot(t *testing.T) {
	t.Parallel()
	module, workdir, probe := interactiveWorkdirProbe(t)
	for _, moduleRoot := range []string{module, ""} {
		stderr, err := os.CreateTemp(t.TempDir(), "stderr-*")
		if err != nil {
			t.Fatalf("create stderr file: %v", err)
		}
		t.Cleanup(func() { _ = stderr.Close() })
		runErr := RunShScript(t.Context(), ShScriptOptions{
			Script:            "read line < " + probe,
			ScriptName:        "probe.sh",
			WorkDir:           workdir,
			WorkDirModuleRoot: moduleRoot,
			ScriptBasePath:    module,
			FilesystemAccess:  invowkfile.VirtualFilesystemAccessRestricted,
			Stderr:            stderr,
		})
		output, readErr := os.ReadFile(stderr.Name())
		if readErr != nil {
			t.Fatalf("read stderr: %v", readErr)
		}
		denied := strings.Contains(string(output), "virtual path denied")
		if moduleRoot != "" && (!denied || runErr == nil) {
			t.Fatalf("module root %q: read of %s = %v, stderr %q, want virtual path denied", moduleRoot, probe, runErr, output)
		}
		if moduleRoot == "" && denied {
			t.Fatalf("trusted workdir: read of %s was denied: %s", probe, output)
		}
	}
}

// TestRunLuaScriptWorkDirModuleRoot is TestRunShScriptWorkDirModuleRoot for
// the interactive virtual-lua runner.
func TestRunLuaScriptWorkDirModuleRoot(t *testing.T) {
	t.Parallel()
	module, workdir, probe := interactiveWorkdirProbe(t)
	for _, moduleRoot := range []string{module, ""} {
		var stdout bytes.Buffer
		err := RunLuaScript(t.Context(), LuaScriptOptions{
			Script:            `local f, err = io.open("` + probe + `", "r"); print(tostring(err)); if f ~= nil then f:close() end`,
			ScriptName:        "probe.lua",
			WorkDir:           workdir,
			WorkDirModuleRoot: moduleRoot,
			ScriptBasePath:    module,
			FilesystemAccess:  invowkfile.VirtualFilesystemAccessRestricted,
			Stdout:            &stdout,
			Stderr:            &bytes.Buffer{},
		})
		if err != nil {
			t.Fatalf("module root %q: RunLuaScript() error = %v", moduleRoot, err)
		}
		denied := strings.Contains(stdout.String(), "virtual path denied")
		if moduleRoot != "" && !denied {
			t.Fatalf("module root %q: io.open(%s) printed %q, want virtual path denied", moduleRoot, probe, stdout.String())
		}
		if moduleRoot == "" && denied {
			t.Fatalf("trusted workdir: io.open(%s) was denied: %s", probe, stdout.String())
		}
	}
}
