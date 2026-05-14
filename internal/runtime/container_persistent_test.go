// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestContainerRuntimeExecuteCreatesManagedPersistentContainerWhenMissing(t *testing.T) {
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true})

	result := rt.Execute(ctx)

	if result.Error != nil {
		t.Fatalf("Execute() error = %v", result.Error)
	}
	if len(engine.RunCalls) != 0 {
		t.Fatalf("RunCalls = %d, want 0 for persistent exec path", len(engine.RunCalls))
	}
	if len(engine.InspectCalls) != 2 {
		t.Fatalf("InspectCalls = %d, want 2", len(engine.InspectCalls))
	}
	gotName := engine.InspectCalls[0]
	if !strings.HasPrefix(string(gotName), "invowk-") {
		t.Fatalf("derived name = %q, want invowk- prefix", gotName)
	}
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1", len(engine.CreateCalls))
	}
	create := engine.CreateCalls[0]
	if create.Name != gotName {
		t.Fatalf("created name = %q, want %q", create.Name, gotName)
	}
	if create.Labels[persistentContainerLabelManaged] != persistentContainerManagedLabelTrue {
		t.Fatalf("managed label = %q, want %q", create.Labels[persistentContainerLabelManaged], persistentContainerManagedLabelTrue)
	}
	if create.Labels[persistentContainerLabelSpecHash] == "" {
		t.Fatal("create options missing persistent spec hash label")
	}
	if len(engine.StartCalls) != 1 || engine.StartCalls[0] != "created-container" {
		t.Fatalf("StartCalls = %v, want [created-container]", engine.StartCalls)
	}
	if len(engine.ExecCalls) != 1 {
		t.Fatalf("ExecCalls = %d, want 1", len(engine.ExecCalls))
	}
	if !slices.Equal(engine.ExecCommands[0], []string{"/bin/sh", "-c", "echo persistent"}) {
		t.Fatalf("Exec command = %v", engine.ExecCommands[0])
	}
}

func TestDerivePersistentContainerName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace invowkfile.CommandName
		wantSlug  string
	}{
		{
			name:      "root command namespace",
			namespace: "build assets",
			wantSlug:  "invowk-build-assets-",
		},
		{
			name:      "module command namespace",
			namespace: "io.invowk.tools build",
			wantSlug:  "invowk-io.invowk.tools-build-",
		},
		{
			name:      "punctuation is normalized",
			namespace: "Build:Release/All",
			wantSlug:  "invowk-build-release-all-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := &ExecutionContext{
				CommandFullName: tt.namespace,
				Invowkfile: &invowkfile.Invowkfile{
					FilePath: invowkfile.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue")),
				},
			}
			name := derivePersistentContainerName(ctx)
			if !strings.HasPrefix(string(name), tt.wantSlug) {
				t.Fatalf("derived name = %q, want prefix %q", name, tt.wantSlug)
			}
			if err := name.Validate(); err != nil {
				t.Fatalf("derived name Validate() = %v", err)
			}
			again := derivePersistentContainerName(ctx)
			if again != name {
				t.Fatalf("derived name is not deterministic: %q then %q", name, again)
			}
		})
	}
}

func TestContainerRuntimeExecuteUsesCLIContainerNameAsExistingTarget(t *testing.T) {
	engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
		ContainerID: "external-container",
		Name:        "existing-dev",
		Running:     true,
		Labels:      map[string]string{},
	})
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, nil)
	ctx.ContainerNameOverride = "existing-dev"

	result := rt.Execute(ctx)

	if result.Error != nil {
		t.Fatalf("Execute() error = %v", result.Error)
	}
	if len(engine.CreateCalls) != 0 {
		t.Fatalf("CreateCalls = %d, want 0 for existing CLI target", len(engine.CreateCalls))
	}
	if len(engine.StartCalls) != 0 {
		t.Fatalf("StartCalls = %d, want 0 for running external target", len(engine.StartCalls))
	}
	if len(engine.ExecCalls) != 1 {
		t.Fatalf("ExecCalls = %d, want 1", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteSkipsForceRebuildForExistingPersistentTarget(t *testing.T) {
	engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
		ContainerID: "external-container",
		Name:        "existing-dev",
		Running:     true,
		Labels:      map[string]string{},
	})
	provisionCalls := 0
	rt, err := NewContainerRuntimeWithEngine(
		engine,
		WithContainerProvisioner(
			fakeProvisioner{
				result: &provision.Result{
					ImageTag: "invowk-provisioned:test",
					EnvVars:  map[string]string{"INVOWK_MODULE_PATH": "/invowk/modules"},
				},
				calls: &provisionCalls,
			},
			&provision.Config{Enabled: true},
		),
	)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
	}
	ctx := newPersistentExecutionContext(t, nil)
	ctx.ContainerNameOverride = "existing-dev"
	ctx.ForceRebuild = true

	result := rt.Execute(ctx)

	if result.Error != nil {
		t.Fatalf("Execute() error = %v", result.Error)
	}
	if provisionCalls != 0 {
		t.Fatalf("provision calls = %d, want 0 for existing persistent target", provisionCalls)
	}
	if len(engine.BuildCalls) != 0 {
		t.Fatalf("BuildCalls = %d, want 0 for existing persistent target", len(engine.BuildCalls))
	}
	if len(engine.ExecCalls) != 1 {
		t.Fatalf("ExecCalls = %d, want 1", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteDoesNotCreateMissingCLIContainerByDefault(t *testing.T) {
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, nil)
	ctx.ContainerNameOverride = "typo-target"

	result := rt.Execute(ctx)

	if result.Error == nil {
		t.Fatal("Execute() error = nil, want missing persistent container error")
	}
	if !strings.Contains(result.Error.Error(), "does not exist") {
		t.Fatalf("Execute() error = %v, want does not exist message", result.Error)
	}
	if len(engine.CreateCalls) != 0 {
		t.Fatalf("CreateCalls = %d, want 0", len(engine.CreateCalls))
	}
	if len(engine.ExecCalls) != 0 {
		t.Fatalf("ExecCalls = %d, want 0", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteRejectsMissingTargetBeforeImagePreparation(t *testing.T) {
	t.Parallel()

	provisionCalls := 0
	tagCalls := 0
	engine := NewMockEngine().WithImageExists(false)
	rt, err := NewContainerRuntimeWithEngine(
		engine,
		WithContainerProvisioner(
			fakeProvisioner{
				result:   &provision.Result{ImageTag: "invowk-provisioned:test"},
				calls:    &provisionCalls,
				tagCalls: &tagCalls,
			},
			&provision.Config{Enabled: true},
		),
	)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
	}
	ctx := newPersistentExecutionContext(t, nil)
	ctx.ContainerNameOverride = "typo-target"
	ctx.SelectedImpl.Runtimes[0].Image = ""
	ctx.SelectedImpl.Runtimes[0].Containerfile = "Containerfile"

	result := rt.Execute(ctx)

	if result.Error == nil {
		t.Fatal("Execute() error = nil, want missing persistent container error")
	}
	if !strings.Contains(result.Error.Error(), "does not exist") {
		t.Fatalf("Execute() error = %v, want missing persistent container message", result.Error)
	}
	if tagCalls != 0 {
		t.Fatalf("GetProvisionedImageTag calls = %d, want 0 before missing-target failure", tagCalls)
	}
	if provisionCalls != 0 {
		t.Fatalf("Provision calls = %d, want 0 before missing-target failure", provisionCalls)
	}
	if len(engine.BuildCalls) != 0 {
		t.Fatalf("BuildCalls = %d, want 0 before missing-target failure", len(engine.BuildCalls))
	}
}

func TestContainerRuntimeExecuteLabelsFallbackImageForManagedPersistentContainer(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	const desiredImage = "invowk-provisioned:desired"
	rt, err := NewContainerRuntimeWithEngine(
		engine,
		WithContainerProvisioner(
			fakeProvisioner{
				err:         errors.New("provision failed"),
				resolvedTag: desiredImage,
			},
			&provision.Config{Enabled: true, Strict: false},
		),
	)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
	}
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "managed-dev"})
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)

	if result.Error != nil {
		t.Fatalf("Execute() error = %v", result.Error)
	}
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1", len(engine.CreateCalls))
	}
	create := engine.CreateCalls[0]
	got := create.Labels[persistentContainerLabelSpecHash]
	want := persistentContainerSpecHash(&containerExecPrep{
		image:      "debian:stable-slim",
		volumes:    create.Volumes,
		ports:      create.Ports,
		extraHosts: create.ExtraHosts,
	})
	wrongProvisioned := persistentContainerSpecHash(&containerExecPrep{
		image:      desiredImage,
		volumes:    create.Volumes,
		ports:      create.Ports,
		extraHosts: create.ExtraHosts,
	})
	if got != want {
		t.Fatalf("created spec hash = %q, want fallback base-image hash %q", got, want)
	}
	if got == wrongProvisioned {
		t.Fatal("created spec hash used the desired provisioned image after fallback")
	}
}

func TestContainerRuntimeExecuteRejectsUnmanagedConfigTarget(t *testing.T) {
	engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
		ContainerID: "external-container",
		Name:        "managed-dev",
		Running:     true,
		Labels:      map[string]string{},
	})
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "managed-dev"})

	result := rt.Execute(ctx)

	if result.Error == nil {
		t.Fatal("Execute() error = nil, want unmanaged config target error")
	}
	if !strings.Contains(result.Error.Error(), "not managed by invowk") {
		t.Fatalf("Execute() error = %v, want unmanaged conflict message", result.Error)
	}
	if len(engine.ExecCalls) != 0 {
		t.Fatalf("ExecCalls = %d, want 0", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteRestartsStoppedManagedPersistentContainer(t *testing.T) {
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "managed-dev"})

	first := rt.Execute(ctx)
	if first.Error != nil {
		t.Fatalf("first Execute() error = %v", first.Error)
	}
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1", len(engine.CreateCalls))
	}

	labels := engine.CreateCalls[0].Labels
	engine.WithInspectInfo(&container.ContainerInfo{
		ContainerID: "managed-container",
		Name:        "managed-dev",
		Running:     false,
		Labels:      labels,
	})

	second := rt.Execute(ctx)
	if second.Error != nil {
		t.Fatalf("second Execute() error = %v", second.Error)
	}
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want still 1", len(engine.CreateCalls))
	}
	if got := engine.StartCalls[len(engine.StartCalls)-1]; got != "managed-container" {
		t.Fatalf("last StartCall = %q, want managed-container", got)
	}
}

func TestContainerRuntimeExecuteKeepsDynamicStateOnPersistentExec(t *testing.T) {
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "managed-dev"})
	ctx.WorkDir = "build"
	ctx.Env.RuntimeEnvVars = map[string]string{"RUN_ONLY": "secret"}

	result := rt.Execute(ctx)

	if result.Error != nil {
		t.Fatalf("Execute() error = %v", result.Error)
	}
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1", len(engine.CreateCalls))
	}
	create := engine.CreateCalls[0]
	if create.WorkDir != "" {
		t.Fatalf("create WorkDir = %q, want empty because workdir is exec-time state", create.WorkDir)
	}
	if len(create.Env) != 0 {
		t.Fatalf("create Env = %v, want empty because environment is exec-time state", create.Env)
	}
	if len(engine.ExecCalls) != 1 {
		t.Fatalf("ExecCalls = %d, want 1", len(engine.ExecCalls))
	}
	exec := engine.ExecCalls[0]
	if exec.WorkDir != "/workspace/build" {
		t.Fatalf("exec WorkDir = %q, want /workspace/build", exec.WorkDir)
	}
	if exec.Env["RUN_ONLY"] != "secret" {
		t.Fatalf("exec RUN_ONLY env = %q, want secret", exec.Env["RUN_ONLY"])
	}
}

func TestContainerRuntimeExecuteRejectsManagedPersistentDrift(t *testing.T) {
	engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
		ContainerID: "managed-container",
		Name:        "managed-dev",
		Running:     true,
		Labels: map[string]string{
			persistentContainerLabelManaged:    persistentContainerManagedLabelTrue,
			persistentContainerLabelPersistent: persistentContainerManagedLabelTrue,
			persistentContainerLabelSpecHash:   "different",
		},
	})
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "managed-dev"})

	result := rt.Execute(ctx)

	if result.Error == nil {
		t.Fatal("Execute() error = nil, want drift error")
	}
	if !strings.Contains(result.Error.Error(), "different runtime configuration") {
		t.Fatalf("Execute() error = %v, want drift message", result.Error)
	}
	if len(engine.ExecCalls) != 0 {
		t.Fatalf("ExecCalls = %d, want 0", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteReinspectsAfterNameConflict(t *testing.T) {
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "race-dev"})
	invowkDir := filepath.Dir(string(ctx.Invowkfile.FilePath))
	prep := &containerExecPrep{
		image: "debian:stable-slim",
		volumes: []container.VolumeMountSpec{
			container.VolumeMountSpec(filepath.ToSlash(invowkDir) + ":/workspace"),
		},
	}
	target := persistentContainerTarget{
		name:            "race-dev",
		nameSource:      persistentContainerNameSourceConfig,
		createIfMissing: true,
	}
	labels := persistentContainerLabels(ctx, prep, target)
	engine.createErr = &container.ContainerNameConflictError{Name: "race-dev"}
	engine.WithInspectSequence(
		mockInspectResult{err: &container.ContainerNotFoundError{Name: "race-dev"}},
		mockInspectResult{err: &container.ContainerNotFoundError{Name: "race-dev"}},
		mockInspectResult{info: &container.ContainerInfo{
			ContainerID: "created-by-race",
			Name:        "race-dev",
			Running:     true,
			Labels:      labels,
		}},
	)

	result := rt.Execute(ctx)

	if result.Error != nil {
		t.Fatalf("Execute() error = %v", result.Error)
	}
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1", len(engine.CreateCalls))
	}
	if len(engine.InspectCalls) != 3 {
		t.Fatalf("InspectCalls = %d, want 3 including prep, lifecycle, and conflict re-inspect", len(engine.InspectCalls))
	}
	if len(engine.ExecCalls) != 1 {
		t.Fatalf("ExecCalls = %d, want 1", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteConcurrentMissingPersistentContainerCreatesOnce(t *testing.T) {
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctxs := []*ExecutionContext{
		newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "race-dev"}),
		newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "race-dev"}),
	}
	ctxs[1].Invowkfile = ctxs[0].Invowkfile

	start := make(chan struct{})
	results := make(chan *Result, len(ctxs))
	var wg sync.WaitGroup
	for _, ctx := range ctxs {
		wg.Go(func() {
			<-start
			results <- rt.Execute(ctx)
		})
	}

	close(start)
	wg.Wait()
	close(results)

	for result := range results {
		if result.Error != nil {
			t.Fatalf("Execute() error = %v", result.Error)
		}
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1 for concurrent first use", len(engine.CreateCalls))
	}
	if len(engine.StartCalls) != 1 {
		t.Fatalf("StartCalls = %d, want 1 for concurrent first use", len(engine.StartCalls))
	}
	if len(engine.ExecCalls) != 2 {
		t.Fatalf("ExecCalls = %d, want 2", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteRejectsStoppedExternalCLIContainer(t *testing.T) {
	engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
		ContainerID: "external-container",
		Name:        "existing-dev",
		Running:     false,
		Labels:      map[string]string{},
	})
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, nil)
	ctx.ContainerNameOverride = "existing-dev"

	result := rt.Execute(ctx)

	if result.Error == nil {
		t.Fatal("Execute() error = nil, want stopped external target error")
	}
	if !strings.Contains(result.Error.Error(), "not running") {
		t.Fatalf("Execute() error = %v, want not running message", result.Error)
	}
	if len(engine.StartCalls) != 0 {
		t.Fatalf("StartCalls = %d, want 0 for external target", len(engine.StartCalls))
	}
	if len(engine.ExecCalls) != 0 {
		t.Fatalf("ExecCalls = %d, want 0", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteCaptureUsesPersistentExecPath(t *testing.T) {
	engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
		ContainerID: "external-container",
		Name:        "existing-dev",
		Running:     true,
		Labels:      map[string]string{},
	})
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, nil)
	ctx.ContainerNameOverride = "existing-dev"

	result := rt.ExecuteCapture(ctx)

	if result.Error != nil {
		t.Fatalf("ExecuteCapture() error = %v", result.Error)
	}
	if len(engine.RunCalls) != 0 {
		t.Fatalf("RunCalls = %d, want 0", len(engine.RunCalls))
	}
	if len(engine.ExecCalls) != 1 {
		t.Fatalf("ExecCalls = %d, want 1", len(engine.ExecCalls))
	}
}

func TestContainerRuntimeExecuteCaptureDisablesPersistentStdin(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
		ContainerID: "external-container",
		Name:        "existing-dev",
		Running:     true,
		Labels:      map[string]string{},
	})
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, nil)
	ctx.ContainerNameOverride = "existing-dev"
	ctx.IO.Stdin = strings.NewReader("terminal input")

	result := rt.ExecuteCapture(ctx)

	if result.Error != nil {
		t.Fatalf("ExecuteCapture() error = %v", result.Error)
	}
	if len(engine.ExecCalls) != 1 {
		t.Fatalf("ExecCalls = %d, want 1", len(engine.ExecCalls))
	}
	call := engine.ExecCalls[0]
	if call.Stdin != nil {
		t.Fatal("persistent capture exec inherited stdin, want nil")
	}
	if call.Interactive {
		t.Fatal("persistent capture exec is interactive, want non-interactive")
	}
}

func newPersistentTestRuntime(t *testing.T, engine *MockEngine) *ContainerRuntime {
	t.Helper()
	rt, err := NewContainerRuntimeWithEngine(engine, WithContainerProvisioner(nil, nil))
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
	}
	return rt
}

func newPersistentExecutionContext(t *testing.T, persistent *invowkfile.RuntimePersistentConfig) *ExecutionContext {
	t.Helper()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue")),
	}
	cmd := &invowkfile.Command{
		Name: "persistent",
		Implementations: []invowkfile.Implementation{{
			Script: "echo persistent",
			Runtimes: []invowkfile.RuntimeConfig{{
				Name:       invowkfile.RuntimeContainer,
				Image:      "debian:stable-slim",
				Persistent: persistent,
			}},
			Platforms: invowkfile.AllPlatformConfigs(),
		}},
	}
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.CommandFullName = "io.invowk.sample persistent"
	ctx.SelectedRuntime = invowkfile.RuntimeContainer
	ctx.SelectedImpl = &cmd.Implementations[0]
	return ctx
}
