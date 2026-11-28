// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestMountTrustTierValidate(t *testing.T) {
	t.Parallel()

	for _, valid := range []MountTrustTier{"", MountTrustTierContained, MountTrustTierLocalCwd, MountTrustTierInclude} {
		if err := valid.Validate(); err != nil {
			t.Errorf("MountTrustTier(%q).Validate() = %v, want nil", valid, err)
		}
	}
	if err := MountTrustTier("bad").Validate(); !errors.Is(err, ErrInvalidMountTrustTier) {
		t.Fatalf("Validate(bad) = %v, want ErrInvalidMountTrustTier", err)
	}
}

func TestResolveContainerMountPlan_ThreeTierContainmentAndSymlinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := filepath.Join(root, "repo")
	modDir := filepath.Join(cwd, "nested", "org.local.invowkmod")
	vendoredModDir := filepath.Join(cwd, "invowk_modules", "org.vendored.invowkmod")
	sharedParent := filepath.Join(root, "shared")
	includeModDir := filepath.Join(sharedParent, "sub", "org.inc.invowkmod")
	outsideDir := filepath.Join(root, "outside-secret")

	for _, dir := range []string{
		filepath.Join(cwd, "data"),
		filepath.Join(modDir, "scripts"),
		filepath.Join(vendoredModDir, "sub"),
		includeModDir,
		outsideDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	makeCtx := func(dir string, tier MountTrustTier, ceiling string, rtCfg invowkfile.RuntimeConfig, cmdWorkDir invowkfile.WorkDir, cliWorkDir invowkfile.WorkDir) *ExecutionContext {
		inv := &invowkfile.Invowkfile{
			FilePath: invowkfile.FilesystemPath(filepath.Join(dir, "invowkfile.cue")),
		}
		if strings.HasSuffix(dir, ".invowkmod") {
			inv.ModulePath = invowkfile.FilesystemPath(dir)
		}
		cmd := &invowkfile.Command{
			Name:    "test",
			WorkDir: cmdWorkDir,
			Implementations: []invowkfile.Implementation{{
				Script:    invowkfile.ImplementationScript{Content: "pwd"},
				Runtimes:  []invowkfile.RuntimeConfig{rtCfg},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}
		execCtx := NewExecutionContext(t.Context(), cmd, inv)
		execCtx.SelectedRuntime = invowkfile.RuntimeContainer
		execCtx.SelectedImpl = &cmd.Implementations[0]
		execCtx.MountTrustTier = tier
		execCtx.MountCeiling = types.FilesystemPath(ceiling)
		execCtx.WorkDir = cliWorkDir
		return execCtx
	}

	t.Run("Layer 1 contained blocks parent escape in workdir root and relative volume", func(t *testing.T) {
		t.Parallel()

		ctx := makeCtx(
			vendoredModDir,
			MountTrustTierContained,
			"",
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
			},
			"../..",
			"",
		)
		if _, err := ResolveContainerMountPlan(ctx); !errors.Is(err, ErrContainerMountPolicy) {
			t.Fatalf("ResolveContainerMountPlan() error = %v, want ErrContainerMountPolicy", err)
		}

		volCtx := makeCtx(
			vendoredModDir,
			MountTrustTierContained,
			"",
			invowkfile.RuntimeConfig{
				Name:    invowkfile.RuntimeContainer,
				Image:   "debian:stable-slim",
				Volumes: []invowkfile.VolumeMountSpec{"../..:/escape"},
			},
			"",
			"",
		)
		if _, err := ResolveContainerMountPlan(volCtx); !errors.Is(err, ErrContainerMountPolicy) {
			t.Fatalf("ResolveContainerMountPlan() relative escape error = %v, want ErrContainerMountPolicy", err)
		}
	})

	t.Run("Layer 2 local-cwd allows ancestor within cwd but blocks above cwd", func(t *testing.T) {
		t.Parallel()

		okCtx := makeCtx(
			modDir,
			MountTrustTierLocalCwd,
			cwd,
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
				Volumes: []invowkfile.VolumeMountSpec{"../../data:/data:ro"},
			},
			"../..",
			"",
		)
		plan, err := ResolveContainerMountPlan(okCtx)
		if err != nil {
			t.Fatalf("ResolveContainerMountPlan() unexpected error: %v", err)
		}
		if string(plan.WorkspaceHostRoot) == "" || string(plan.WorkDir) != "/workspace" {
			t.Fatalf("plan = %+v, want mounted /workspace", plan)
		}
		if len(plan.ResolvedVolumes) != 1 || !strings.Contains(string(plan.ResolvedVolumes[0].ResolvedSpec), "/data:ro") {
			t.Fatalf("ResolvedVolumes = %+v, want resolved data mount", plan.ResolvedVolumes)
		}

		badCtx := makeCtx(
			modDir,
			MountTrustTierLocalCwd,
			cwd,
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
			},
			"../../../outside-secret",
			"",
		)
		if _, err := ResolveContainerMountPlan(badCtx); !errors.Is(err, ErrContainerMountPolicy) {
			t.Fatalf("escape above cwd error = %v, want ErrContainerMountPolicy", err)
		}

		// When cwd is a subdirectory of the declaring module (e.g. <mod>/scripts),
		// paths inside the module root itself remain valid alongside cwd.
		subCwdCtx := makeCtx(
			modDir,
			MountTrustTierLocalCwd,
			filepath.Join(modDir, "scripts"),
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
				Volumes: []invowkfile.VolumeMountSpec{"./scripts:/scripts:ro"},
			},
			".",
			"",
		)
		if _, err := ResolveContainerMountPlan(subCwdCtx); err != nil {
			t.Fatalf("module invoked from subdirectory unexpected error: %v", err)
		}
	})

	t.Run("Layer 3 include allows within workspace_root ceiling when set, module root when unset", func(t *testing.T) {
		t.Parallel()

		noRootCtx := makeCtx(
			includeModDir,
			MountTrustTierInclude,
			"",
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
			},
			"..",
			"",
		)
		if _, err := ResolveContainerMountPlan(noRootCtx); !errors.Is(err, ErrContainerMountPolicy) {
			t.Fatalf("include without workspace_root escape error = %v, want ErrContainerMountPolicy", err)
		}

		withRootCtx := makeCtx(
			includeModDir,
			MountTrustTierInclude,
			sharedParent,
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
			},
			"../..",
			"",
		)
		plan, err := ResolveContainerMountPlan(withRootCtx)
		if err != nil {
			t.Fatalf("include within workspace_root unexpected error: %v", err)
		}
		if string(plan.WorkDir) != "/workspace" {
			t.Fatalf("WorkDir = %q, want /workspace", plan.WorkDir)
		}
	})

	t.Run("CLI --ivk-workdir bypasses tier ceiling", func(t *testing.T) {
		t.Parallel()

		cliCtx := makeCtx(
			vendoredModDir,
			MountTrustTierContained,
			"",
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
			},
			"",
			invowkfile.WorkDir(outsideDir),
		)
		plan, err := ResolveContainerMountPlan(cliCtx)
		if err != nil {
			t.Fatalf("CLI workdir override unexpected error: %v", err)
		}
		if string(plan.WorkDir) != "/workspace" {
			t.Fatalf("WorkDir = %q, want /workspace", plan.WorkDir)
		}
	})

	t.Run("symlink escape inside module is blocked by physical resolution", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires elevated privileges on some Windows hosts")
		}
		t.Parallel()

		escapeLink := filepath.Join(vendoredModDir, "escape-link")
		if err := os.Symlink(outsideDir, escapeLink); err != nil {
			t.Fatalf("Symlink(): %v", err)
		}
		symCtx := makeCtx(
			vendoredModDir,
			MountTrustTierContained,
			"",
			invowkfile.RuntimeConfig{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
			},
			"escape-link",
			"",
		)
		if _, err := ResolveContainerMountPlan(symCtx); !errors.Is(err, ErrContainerMountPolicy) {
			t.Fatalf("symlink escape error = %v, want ErrContainerMountPolicy", err)
		}
	})

	t.Run("module absolute host volume rejected, named volume allowed, standalone absolute allowed", func(t *testing.T) {
		t.Parallel()

		modAbs := makeCtx(
			modDir,
			MountTrustTierLocalCwd,
			cwd,
			invowkfile.RuntimeConfig{
				Name:    invowkfile.RuntimeContainer,
				Image:   "debian:stable-slim",
				Volumes: []invowkfile.VolumeMountSpec{invowkfile.VolumeMountSpec(filepath.ToSlash(outsideDir) + ":/data:ro")},
			},
			"",
			"",
		)
		if _, err := ResolveContainerMountPlan(modAbs); !errors.Is(err, ErrContainerModuleAbsoluteVolume) {
			t.Fatalf("module abs volume error = %v, want ErrContainerModuleAbsoluteVolume", err)
		}

		modNamed := makeCtx(
			modDir,
			MountTrustTierLocalCwd,
			cwd,
			invowkfile.RuntimeConfig{
				Name:    invowkfile.RuntimeContainer,
				Image:   "debian:stable-slim",
				Volumes: []invowkfile.VolumeMountSpec{"my-cache-vol:/cache:rw"},
			},
			"",
			"",
		)
		namedPlan, err := ResolveContainerMountPlan(modNamed)
		if err != nil {
			t.Fatalf("module named volume error = %v", err)
		}
		if namedPlan.ResolvedVolumes[0].SourceKind != containerVolumeKindNamed || namedPlan.ResolvedVolumes[0].ResolvedSpec != "my-cache-vol:/cache:rw" {
			t.Fatalf("named volume = %+v", namedPlan.ResolvedVolumes[0])
		}

		standaloneAbs := makeCtx(
			cwd,
			MountTrustTierLocalCwd,
			cwd,
			invowkfile.RuntimeConfig{
				Name:    invowkfile.RuntimeContainer,
				Image:   "debian:stable-slim",
				Volumes: []invowkfile.VolumeMountSpec{invowkfile.VolumeMountSpec(filepath.ToSlash(outsideDir) + ":/data:ro")},
			},
			"",
			"",
		)
		if _, err := ResolveContainerMountPlan(standaloneAbs); err != nil {
			t.Fatalf("standalone absolute volume error = %v", err)
		}
	})

	t.Run("workspace_mount disabled omits workspace mount and omits -w for default workdir or fails closed for unmounted host dir", func(t *testing.T) {
		t.Parallel()

		disabledDefault := makeCtx(
			cwd,
			MountTrustTierLocalCwd,
			cwd,
			invowkfile.RuntimeConfig{
				Name:           invowkfile.RuntimeContainer,
				Image:          "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{Enabled: new(false)},
			},
			"",
			"",
		)
		plan, err := ResolveContainerMountPlan(disabledDefault)
		if err != nil {
			t.Fatalf("disabled default unexpected error: %v", err)
		}
		if plan.WorkspaceEnabled || plan.WorkDir != "" {
			t.Fatalf("plan = %+v, want WorkspaceEnabled=false and empty WorkDir", plan)
		}

		unmountedHost := makeCtx(
			cwd,
			MountTrustTierLocalCwd,
			cwd,
			invowkfile.RuntimeConfig{
				Name:           invowkfile.RuntimeContainer,
				Image:          "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{Enabled: new(false)},
			},
			invowkfile.WorkDir(outsideDir),
			"",
		)
		if _, err := ResolveContainerMountPlan(unmountedHost); !errors.Is(err, ErrContainerWorkDirNotMounted) {
			t.Fatalf("unmounted host workdir error = %v, want ErrContainerWorkDirNotMounted", err)
		}
	})
}

func TestClassifyVolumeSource_WindowsDriveVsNamedVolume(t *testing.T) {
	t.Parallel()

	cases := []struct {
		spec     string
		wantKind string
		wantTgt  string
		wantOpts string
	}{
		{"my-vol:/data", containerVolumeKindNamed, "/data", ""},
		{"my_vol.1:/data:ro", containerVolumeKindNamed, "/data", "ro"},
		{"./rel:/data:rw", containerVolumeKindRelative, "/data", "rw"},
		{"../rel:/data", containerVolumeKindRelative, "/data", ""},
		{`C:\work:/workspace:ro`, containerVolumeKindAbsolute, "/workspace", "ro"},
		{`D:/work:/workspace`, containerVolumeKindAbsolute, "/workspace", ""},
		{`\\server\share\dir:/mnt`, containerVolumeKindAbsolute, "/mnt", ""},
		{`/abs/path:/mnt:ro,z`, containerVolumeKindAbsolute, "/mnt", "ro,z"},
	}

	for _, tc := range cases {
		parsed, err := parseVolumeMountSpec(tc.spec)
		if err != nil {
			t.Fatalf("parseVolumeMountSpec(%q) error = %v", tc.spec, err)
		}
		if parsed.sourceKind != tc.wantKind || parsed.containerTarget != tc.wantTgt || parsed.options != tc.wantOpts {
			t.Errorf("parseVolumeMountSpec(%q) = %+v, want kind=%q tgt=%q opts=%q", tc.spec, parsed, tc.wantKind, tc.wantTgt, tc.wantOpts)
		}
	}
}

func TestContainerRuntime_ScriptDecouplingAndSELinuxOptions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	scriptFile := filepath.Join(scriptDir, "run.py")
	if err := os.WriteFile(scriptFile, []byte("#!/usr/bin/env python3\nprint('hi')\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	t.Run("workspace disabled decouples file script into readonly /.invowk/scripts mount and preserves SELinux auto on tmp", func(t *testing.T) {
		t.Parallel()

		engine := NewMockEngine()
		cfg := config.DefaultConfig()
		cfg.Container.SELinuxLabeling = config.SELinuxLabelingEnabled
		rt, err := NewContainerRuntimeWithEngine(engine, WithContainerConfig(cfg))
		if err != nil {
			t.Fatalf("NewContainerRuntimeWithEngine(): %v", err)
		}

		sf := invowkfile.ScriptFilePath("scripts/run.py")
		inv := &invowkfile.Invowkfile{
			FilePath:   invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
			ModulePath: invowkfile.FilesystemPath(tmpDir),
		}
		cmd := &invowkfile.Command{
			Name: "decoupled-file",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{File: &sf, Interpreter: "python3"},
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:  invowkfile.RuntimeContainer,
					Image: "python:3-slim",
					WorkspaceMount: &invowkfile.WorkspaceMountConfig{
						Enabled:      new(false),
						SELinuxLabel: invowkfile.WorkspaceMountSELinuxNone,
					},
				}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}

		ctx := NewExecutionContext(t.Context(), cmd, inv)
		ctx.SelectedRuntime = invowkfile.RuntimeContainer
		ctx.SelectedImpl = &cmd.Implementations[0]

		res := rt.ExecuteCapture(ctx)
		if res.Error != nil || res.ExitCode != 0 {
			t.Fatalf("ExecuteCapture() = (%d, %v)", res.ExitCode, res.Error)
		}
		if len(engine.RunCalls) != 1 {
			t.Fatalf("RunCalls = %d, want 1", len(engine.RunCalls))
		}
		runOpts := engine.RunCalls[0]
		if runOpts.WorkDir != "" {
			t.Errorf("WorkDir = %q, want empty when workspace disabled", runOpts.WorkDir)
		}
		if runOpts.DisableSELinuxLabeling {
			t.Error("DisableSELinuxLabeling = true, want false")
		}
		if !slices.Contains(runOpts.Command, "/.invowk/scripts/run.py") {
			t.Errorf("Command = %v, want /.invowk/scripts/run.py", runOpts.Command)
		}
		resolvedScriptFile, err := filepath.EvalSymlinks(scriptFile)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q): %v", scriptFile, err)
		}
		expectedScriptVol := container.VolumeMountSpec(filepath.ToSlash(resolvedScriptFile) + ":/.invowk/scripts/run.py:ro")
		if !slices.Contains(runOpts.Volumes, expectedScriptVol) {
			t.Errorf("Volumes = %v, want %q", runOpts.Volumes, expectedScriptVol)
		}
		if mode := runOpts.VolumeSELinuxModes[expectedScriptVol]; mode != container.VolumeMountSELinuxAuto {
			t.Errorf("decoupled script SELinux mode = %q, want auto", mode)
		}
	})

	t.Run("persistent container uses in-container temp script delivery instead of random host tmp mount", func(t *testing.T) {
		t.Parallel()

		engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
			ContainerID: "persist-123",
			Name:        "persist-py",
			Running:     true,
		})
		rt, err := NewContainerRuntimeWithEngine(engine)
		if err != nil {
			t.Fatalf("NewContainerRuntimeWithEngine(): %v", err)
		}

		inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}
		cmd := &invowkfile.Command{
			Name: "persist-inline-py",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{
					Content:     "print('persist')",
					Interpreter: "python3",
				},
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:       invowkfile.RuntimeContainer,
					Image:      "python:3-slim",
					Persistent: &invowkfile.RuntimePersistentConfig{Name: "persist-py"},
				}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}

		ctx := NewExecutionContext(t.Context(), cmd, inv)
		ctx.SelectedRuntime = invowkfile.RuntimeContainer
		ctx.SelectedImpl = &cmd.Implementations[0]
		ctx.ContainerNameOverride = "persist-py"

		res := rt.ExecuteCapture(ctx)
		if res.Error != nil || res.ExitCode != 0 {
			t.Fatalf("ExecuteCapture() = (%d, %v)", res.ExitCode, res.Error)
		}
		if len(engine.ExecCommands) != 1 {
			t.Fatalf("ExecCommands = %d, want 1", len(engine.ExecCommands))
		}
		execCmd := engine.ExecCommands[0]
		if len(execCmd) < 4 || execCmd[0] != "/bin/sh" || execCmd[1] != "-c" {
			t.Fatalf("ExecCommands[0] = %v, want /bin/sh -c in-container temp script delivery", execCmd)
		}
	})

	t.Run("operator selinux_labeling disabled propagates DisableSELinuxLabeling", func(t *testing.T) {
		t.Parallel()

		engine := NewMockEngine()
		cfg := config.DefaultConfig()
		cfg.Container.SELinuxLabeling = config.SELinuxLabelingDisabled
		rt, err := NewContainerRuntimeWithEngine(engine, WithContainerConfig(cfg))
		if err != nil {
			t.Fatalf("NewContainerRuntimeWithEngine(): %v", err)
		}

		inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}
		cmd := &invowkfile.Command{
			Name: "selinux-disabled",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{Content: "echo ok"},
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:  invowkfile.RuntimeContainer,
					Image: "debian:stable-slim",
					WorkspaceMount: &invowkfile.WorkspaceMountConfig{
						Readonly:     true,
						SELinuxLabel: invowkfile.WorkspaceMountSELinuxPrivate,
					},
				}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}

		ctx := NewExecutionContext(t.Context(), cmd, inv)
		ctx.SelectedRuntime = invowkfile.RuntimeContainer
		ctx.SelectedImpl = &cmd.Implementations[0]

		res := rt.ExecuteCapture(ctx)
		if res.Error != nil || res.ExitCode != 0 {
			t.Fatalf("ExecuteCapture() = (%d, %v)", res.ExitCode, res.Error)
		}
		if !engine.RunCalls[0].DisableSELinuxLabeling {
			t.Fatal("DisableSELinuxLabeling = false, want true")
		}
	})
}

func TestGetContainerWorkDir_GuardAndError(t *testing.T) {
	t.Parallel()

	rt, err := NewContainerRuntimeWithEngine(NewMockEngine(), WithContainerConfig(config.DefaultConfig()))
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine(): %v", err)
	}

	tmpDir := t.TempDir()
	cwd := filepath.Join(tmpDir, "repo")
	modDir := filepath.Join(cwd, "org.local.invowkmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", modDir, err)
	}

	if got, err := rt.getContainerWorkDir(nil, modDir); err == nil || got != "" {
		t.Fatalf("getContainerWorkDir(nil) = (%q, %v), want (\"\", non-nil error)", got, err)
	}
	if got, err := rt.getContainerWorkDir(&ExecutionContext{}, modDir); err == nil || got != "" {
		t.Fatalf("getContainerWorkDir(nil Invowkfile) = (%q, %v), want (\"\", non-nil error)", got, err)
	}

	inv := &invowkfile.Invowkfile{
		FilePath:   invowkfile.FilesystemPath(filepath.Join(modDir, "invowkfile.cue")),
		ModulePath: invowkfile.FilesystemPath(modDir),
	}
	cmd := &invowkfile.Command{
		Name:    "escape",
		WorkDir: "../..",
		Implementations: []invowkfile.Implementation{{
			Script: invowkfile.ImplementationScript{Content: "pwd"},
			Runtimes: []invowkfile.RuntimeConfig{{
				Name:  invowkfile.RuntimeContainer,
				Image: "debian:stable-slim",
				WorkspaceMount: &invowkfile.WorkspaceMountConfig{
					Root: invowkfile.WorkspaceMountRootWorkdir,
				},
			}},
			Platforms: invowkfile.AllPlatformConfigs(),
		}},
	}
	outOfBoundsCtx := NewExecutionContext(t.Context(), cmd, inv)
	outOfBoundsCtx.SelectedRuntime = invowkfile.RuntimeContainer
	outOfBoundsCtx.SelectedImpl = &cmd.Implementations[0]
	outOfBoundsCtx.MountTrustTier = MountTrustTierLocalCwd
	outOfBoundsCtx.MountCeiling = types.FilesystemPath(cwd)

	if got, err := rt.getContainerWorkDir(outOfBoundsCtx, modDir); !errors.Is(err, ErrContainerMountPolicy) || got != "" {
		t.Fatalf("getContainerWorkDir(outOfBounds) = (%q, %v), want (\"\", ErrContainerMountPolicy)", got, err)
	}
}
