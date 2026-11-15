// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

func stubTUIProbe(facts container.EngineNetworkFacts, err error, calls *int) tuiExposureProbe {
	return tuiExposureProbe{
		queryFacts: func(context.Context, container.NamedEngine) (container.EngineNetworkFacts, error) {
			*calls++
			return facts, err
		},
		sandbox:        func() platform.SandboxType { return platform.SandboxNone },
		flatpakNetwork: func() bool { return false },
		interfaceAddrs: func() ([]netip.Prefix, error) {
			return []netip.Prefix{netip.MustParsePrefix("127.0.0.1/8"), netip.MustParsePrefix("172.17.0.1/16")}, nil
		},
	}
}

func TestContainerRuntimePlanTUIExposure(t *testing.T) {
	t.Parallel()

	gw := netip.MustParseAddr("172.17.0.1")
	podman := container.EngineNetworkFacts{Engine: container.EngineTypePodman, Rootless: true, PastaAvailable: true}
	tests := []struct {
		name       string
		facts      container.EngineNetworkFacts
		err        error
		bind       config.TUIServerBind
		persistent bool
		wantRow    containerplan.TUIPlanRow
		wantKind   containerplan.TUIPlanKind
	}{
		{name: "rootful docker", facts: container.EngineNetworkFacts{Engine: container.EngineTypeDocker, BridgeGateway: gw}, wantRow: 8, wantKind: containerplan.TUIPlanNarrow},
		{name: "gateway not local", facts: container.EngineNetworkFacts{Engine: container.EngineTypeDocker, BridgeGateway: netip.MustParseAddr("10.9.0.1")}, wantRow: 13, wantKind: containerplan.TUIPlanRefused},
		{name: "pasta", facts: podman, wantRow: 11, wantKind: containerplan.TUIPlanNarrow},
		{name: "persistent podman", facts: podman, persistent: true, wantRow: 10, wantKind: containerplan.TUIPlanRefused},
		{name: "persistent podman with opt-in", facts: podman, persistent: true, bind: config.TUIServerBindAllInterfaces, wantRow: 10, wantKind: containerplan.TUIPlanWide},
		{name: "query failure", err: errors.New("info failed"), wantRow: 4, wantKind: containerplan.TUIPlanRefused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			rt := newPersistentTestRuntime(t, NewMockEngine())
			rt.tuiProbe = stubTUIProbe(tt.facts, tt.err, &calls)
			cfg := config.DefaultConfig()
			if tt.bind != "" {
				cfg.Container.TUIServerBind = tt.bind
			}
			rt.cfg = cfg
			ctx := newPersistentExecutionContext(t, nil)
			if tt.persistent {
				ctx.ContainerNameOverride = "dev"
			}
			plan := rt.PlanTUIExposure(ctx)
			if plan.Row() != tt.wantRow || plan.Kind() != tt.wantKind {
				t.Fatalf("plan = row %d kind %s, want row %d kind %s", plan.Row(), plan.Kind(), tt.wantRow, tt.wantKind)
			}
			_ = rt.PlanTUIExposure(ctx)
			if calls != 1 {
				t.Fatalf("facts queried %d times, want once (cached)", calls)
			}
		})
	}
}

// TestContainerRuntimeConstructionQueriesNoFacts pins D11: building a runtime
// and running non-interactively never asks for TUI facts.
func TestContainerRuntimeConstructionQueriesNoFacts(t *testing.T) {
	t.Parallel()

	calls := 0
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	rt.tuiProbe = stubTUIProbe(container.EngineNetworkFacts{}, nil, &calls)
	_ = rt.Execute(newPersistentExecutionContext(t, nil))
	if calls != 0 {
		t.Fatalf("non-interactive Execute queried TUI facts %d times, want 0", calls)
	}
	if len(engine.RunCalls) > 0 && engine.RunCalls[0].NetworkMode != "" {
		t.Fatalf("non-interactive run got network mode %q", engine.RunCalls[0].NetworkMode)
	}
}

// TestContainerPrepareCommandTUINetworkMode pins that the plan's network mode
// reaches a new container run, and that an empty TUI context injects no TUI
// variables and no host-gateway mapping.
func TestContainerPrepareCommandTUINetworkMode(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, nil)
	ctx.TUI = TUIContext{ServerURL: "http://127.0.0.1:43123", ServerToken: "tok", NetworkMode: "pasta:-T,43123"}
	prepared, err := rt.PrepareCommand(ctx)
	if err != nil {
		t.Fatalf("PrepareCommand(): %v", err)
	}
	prepared.Cleanup()

	noServer := newPersistentExecutionContext(t, nil)
	prepared, err = rt.PrepareCommand(noServer)
	if err != nil {
		t.Fatalf("PrepareCommand(no server): %v", err)
	}
	prepared.Cleanup()

	if len(engine.PrepareRunCalls) != 2 {
		t.Fatalf("PrepareRunCommand calls = %d, want 2", len(engine.PrepareRunCalls))
	}
	if got := engine.PrepareRunCalls[0].NetworkMode; got != "pasta:-T,43123" {
		t.Fatalf("NetworkMode = %q, want pasta:-T,43123", got)
	}
	second := engine.PrepareRunCalls[1]
	if second.NetworkMode != "" || len(second.ExtraHosts) != 0 {
		t.Fatalf("empty-TUI run got network %q and hosts %v, want neither", second.NetworkMode, second.ExtraHosts)
	}
	for _, key := range []string{"INVOWK_TUI_ADDR", "INVOWK_TUI_TOKEN"} {
		if _, ok := second.Env[key]; ok {
			t.Fatalf("empty-TUI run injected %s", key)
		}
	}
}

// TestContainerRuntimeTUIOptInFollowsConfigSource loads each config-source
// kind through the production provider (S1 source selection) and plans a
// persistent rootless Podman run (row 10): the opt-in applies from the
// selected source, and the user config file is not read under an explicit one.
func TestContainerRuntimeTUIOptInFollowsConfigSource(t *testing.T) {
	t.Parallel()

	const optIn = "container: {tui_server_bind: \"all-interfaces\"}\n"
	tests := []struct {
		name     string
		userFile string // content of <config dir>/config.cue; empty = none
		explicit string // content of the explicit file; empty = none
		viaFlag  bool   // select the explicit file with --ivk-config instead of INVOWK_CONFIG
		wantKind containerplan.TUIPlanKind
	}{
		{name: "user file opt-in", userFile: optIn, wantKind: containerplan.TUIPlanWide},
		{name: "INVOWK_CONFIG opt-in", userFile: "\n", explicit: optIn, wantKind: containerplan.TUIPlanWide},
		{name: "--ivk-config opt-in", userFile: "\n", explicit: optIn, viaFlag: true, wantKind: containerplan.TUIPlanWide},
		{name: "user file ignored under INVOWK_CONFIG", userFile: optIn, explicit: "\n", wantKind: containerplan.TUIPlanRefused},
		{name: "user file ignored under --ivk-config", userFile: optIn, explicit: "\n", viaFlag: true, wantKind: containerplan.TUIPlanRefused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfgDir := t.TempDir()
			if tt.userFile != "" {
				writeTestFile(t, filepath.Join(cfgDir, config.ConfigFileName+"."+config.ConfigFileExt), tt.userFile)
			}
			opts := config.LoadOptions{ConfigDirPath: types.FilesystemPath(cfgDir)}
			env := map[string]string{}
			if tt.explicit != "" {
				explicitPath := filepath.Join(t.TempDir(), "explicit.cue")
				writeTestFile(t, explicitPath, tt.explicit)
				if tt.viaFlag {
					opts.ConfigFilePath = types.FilesystemPath(explicitPath)
				} else {
					env[config.EnvConfigPath] = explicitPath
				}
			}
			loaded, err := config.NewProviderWithEnv(func(k string) string { return env[k] }).LoadWithSource(t.Context(), opts)
			if err != nil {
				t.Fatalf("LoadWithSource(): %v", err)
			}

			calls := 0
			rt := newPersistentTestRuntime(t, NewMockEngine())
			rt.tuiProbe = stubTUIProbe(container.EngineNetworkFacts{Engine: container.EngineTypePodman, Rootless: true, PastaAvailable: true}, nil, &calls)
			rt.cfg = loaded.Config
			ctx := newPersistentExecutionContext(t, nil)
			ctx.ContainerNameOverride = "dev"
			plan := rt.PlanTUIExposure(ctx)
			if plan.Row() != containerplan.TUIRowPodmanPersistent || plan.Kind() != tt.wantKind {
				t.Fatalf("plan = row %d kind %s (source %s), want row 10 kind %s", plan.Row(), plan.Kind(), loaded.Source, tt.wantKind)
			}
		})
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
