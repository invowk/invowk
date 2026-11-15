// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// spyPlannerRuntime is an interactive runtime with a fixed TUI exposure plan.
	// PrepareInteractive records the TUI context and, when probe is set, runs it
	// while the server (if any) is still up.
	spyPlannerRuntime struct {
		plan        containerplan.TUIExposurePlan
		planCalls   int
		prepareTUI  runtime.TUIContext
		prepareRuns int
		probe       func(runtime.TUIContext)
	}

	// fakeSession returns exitCode and records whether a TUI bridge was wired.
	fakeSession struct {
		exitCode     types.ExitCode
		bridgeWired  bool
		sessionCalls int
	}

	// plainInteractiveRuntime has no planner, like the native and virtual runtimes.
	plainInteractiveRuntime struct{ tuiCtx runtime.TUIContext }
)

func (s *spyPlannerRuntime) Validate(*runtime.ExecutionContext) error { return nil }

func (s *spyPlannerRuntime) PlanTUIExposure(*runtime.ExecutionContext) containerplan.TUIExposurePlan {
	s.planCalls++
	return s.plan
}

func (s *spyPlannerRuntime) PrepareInteractive(ctx *runtime.ExecutionContext) (*runtime.PreparedCommand, error) {
	s.prepareRuns++
	s.prepareTUI = ctx.TUI
	if s.probe != nil {
		s.probe(ctx.TUI)
	}
	return &runtime.PreparedCommand{Cmd: exec.CommandContext(context.Background(), "true")}, nil
}

func (f *fakeSession) run(_ context.Context, opts tui.InteractiveOptions, _ *exec.Cmd) (*tui.InteractiveResult, error) {
	f.sessionCalls++
	f.bridgeWired = opts.OnProgramReady != nil
	return &tui.InteractiveResult{ExitCode: f.exitCode}, nil
}

func newExposureTestContext(t *testing.T, stderr *bytes.Buffer) *runtime.ExecutionContext {
	t.Helper()
	return &runtime.ExecutionContext{
		Context: t.Context(),
		IO:      runtime.IOContext{Stdout: &bytes.Buffer{}, Stderr: stderr},
		// A stale value from an earlier run must not leak into a refused run.
		TUI: runtime.TUIContext{ServerURL: "http://stale:1", ServerToken: "stale"},
	}
}

func authedHealth(t *testing.T, tuiCtx runtime.TUIContext, host string) int {
	t.Helper()
	u, err := url.Parse(string(tuiCtx.ServerURL))
	if err != nil {
		t.Fatalf("parse %s: %v", tuiCtx.ServerURL, err)
	}
	target := "http://" + net.JoinHostPort(host, u.Port()) + "/health"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+string(tuiCtx.ServerToken))
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestInteractiveExecutorRefusedPlan(t *testing.T) {
	t.Parallel()

	plan := containerplan.SelectTUIExposure(containerplan.TUINetworkFacts{
		Container: true, Engine: containerplan.TUIEnginePodman, Rootless: true, PastaAvailable: true, PersistentTarget: true,
	}, false)
	rt := &spyPlannerRuntime{plan: plan}
	session := &fakeSession{exitCode: 7}
	var stderr bytes.Buffer
	ctx := newExposureTestContext(t, &stderr)

	result := InteractiveExecutor{runSession: session.run}.Execute(ctx, invowkfile.CommandName("probe"), rt)
	if result.ExitCode != 1 || !errors.Is(result.Error, containerplan.ErrTUIExposureRefused) {
		t.Fatalf("result = %+v, want exit 1 with ErrTUIExposureRefused", result)
	}
	if !strings.Contains(result.Error.Error(), containerplan.TUIOptInSetting) || !strings.Contains(result.Error.Error(), "without -i") {
		t.Fatalf("error %q does not name the opt-in and the run without -i", result.Error)
	}
	if rt.prepareRuns != 0 || session.sessionCalls != 0 {
		t.Fatalf("prepare=%d session=%d, want neither (no container may start)", rt.prepareRuns, session.sessionCalls)
	}
	if ctx.TUI != (runtime.TUIContext{}) {
		t.Fatalf("TUI context = %+v, want the stale value cleared", ctx.TUI)
	}
	if stderr.Len() != 0 {
		t.Fatalf("refused plan wrote %q to stderr; the error travels in the result", stderr.String())
	}
}

// TestInteractiveExecutorRefusesInvalidPlan pins that the adapter validates
// the plan before any listener opens. Outside containerplan the only invalid
// plan that can exist is the zero value: the constructors reject the other
// shapes (TestTUIExposurePlanConstructorsValidate), and the fields are
// unexported.
func TestInteractiveExecutorRefusesInvalidPlan(t *testing.T) {
	t.Parallel()

	rt := &spyPlannerRuntime{plan: containerplan.TUIExposurePlan{}}
	session := &fakeSession{}
	result := InteractiveExecutor{runSession: session.run}.Execute(newExposureTestContext(t, &bytes.Buffer{}), invowkfile.CommandName("probe"), rt)
	if result.ExitCode != 1 || !errors.Is(result.Error, containerplan.ErrInvalidTUIExposurePlan) {
		t.Fatalf("result = %+v, want exit 1 with ErrInvalidTUIExposurePlan", result)
	}
	if rt.prepareRuns != 0 || session.sessionCalls != 0 {
		t.Fatalf("prepare=%d session=%d, want neither", rt.prepareRuns, session.sessionCalls)
	}
}

func TestInteractiveExecutorPreflight(t *testing.T) {
	t.Parallel()

	refused := &spyPlannerRuntime{plan: containerplan.SelectTUIExposure(containerplan.TUINetworkFacts{
		Container: true, Engine: containerplan.TUIEngineDocker, Rootless: true,
	}, false)}
	ctx := newExposureTestContext(t, &bytes.Buffer{})
	if err := (InteractiveExecutor{}).PreflightInteractive(ctx, refused); !errors.Is(err, containerplan.ErrTUIExposureRefused) {
		t.Fatalf("PreflightInteractive(refused) = %v, want ErrTUIExposureRefused", err)
	}
	if err := (InteractiveExecutor{}).PreflightInteractive(ctx, &spyPlannerRuntime{}); !errors.Is(err, containerplan.ErrInvalidTUIExposurePlan) {
		t.Fatalf("PreflightInteractive(zero plan) = %v, want ErrInvalidTUIExposurePlan", err)
	}
	if err := (InteractiveExecutor{}).PreflightInteractive(ctx, &plainInteractiveRuntime{}); err != nil {
		t.Fatalf("PreflightInteractive(host runtime) = %v, want nil", err)
	}
	if refused.prepareRuns != 0 {
		t.Fatal("PreflightInteractive prepared the command")
	}
}

func TestInteractiveExecutorPastaPlan(t *testing.T) {
	t.Parallel()

	plan := containerplan.SelectTUIExposure(containerplan.TUINetworkFacts{
		Container: true, Engine: containerplan.TUIEnginePodman, Rootless: true, PastaAvailable: true,
	}, false)
	rt := &spyPlannerRuntime{plan: plan}
	rt.probe = func(tuiCtx runtime.TUIContext) {
		if code := authedHealth(t, tuiCtx, "127.0.0.1"); code != http.StatusOK {
			t.Errorf("authenticated /health on loopback = %d, want 200", code)
		}
	}
	var stderr bytes.Buffer
	session := &fakeSession{}
	result := InteractiveExecutor{runSession: session.run}.Execute(newExposureTestContext(t, &stderr), invowkfile.CommandName("probe"), rt)
	if result.Error != nil {
		t.Fatalf("Execute(): %v", result.Error)
	}
	u, err := url.Parse(string(rt.prepareTUI.ServerURL))
	if err != nil || u.Hostname() != "127.0.0.1" {
		t.Fatalf("ServerURL = %q, want host 127.0.0.1", rt.prepareTUI.ServerURL)
	}
	if want := "pasta:-T," + u.Port(); rt.prepareTUI.NetworkMode.String() != want {
		t.Fatalf("NetworkMode = %q, want %q", rt.prepareTUI.NetworkMode, want)
	}
	if !session.bridgeWired {
		t.Fatal("narrow plan did not wire the TUI bridge")
	}
	if stderr.Len() != 0 {
		t.Fatalf("narrow plan wrote %q to stderr, want nothing", stderr.String())
	}
}

func TestInteractiveExecutorOptInWidePlan(t *testing.T) {
	t.Parallel()

	facts := []containerplan.TUINetworkFacts{
		{Container: true, Engine: containerplan.TUIEnginePodman, Rootless: true, PastaAvailable: true, PersistentTarget: true},
		{Container: true, Engine: containerplan.TUIEngineDocker, DockerDesktop: true},
	}
	wantHosts := []string{"host.containers.internal", "host.docker.internal"}
	nonLoopback := firstNonLoopbackIPv4(t)
	for i, f := range facts {
		plan := containerplan.SelectTUIExposure(f, true)
		rt := &spyPlannerRuntime{plan: plan}
		rt.probe = func(tuiCtx runtime.TUIContext) {
			if nonLoopback == "" {
				t.Log("no non-loopback IPv4 address; wildcard reachability check skipped")
				return
			}
			if code := authedHealth(t, tuiCtx, nonLoopback); code != http.StatusOK {
				t.Errorf("wide plan /health on %s = %d, want 200 (0.0.0.0 listener)", nonLoopback, code)
			}
		}
		var stderr bytes.Buffer
		result := InteractiveExecutor{runSession: (&fakeSession{}).run}.Execute(newExposureTestContext(t, &stderr), invowkfile.CommandName("probe"), rt)
		if result.Error != nil {
			t.Fatalf("Execute(): %v", result.Error)
		}
		u, err := url.Parse(string(rt.prepareTUI.ServerURL))
		if err != nil || u.Hostname() != wantHosts[i] {
			t.Fatalf("ServerURL = %q, want host %s", rt.prepareTUI.ServerURL, wantHosts[i])
		}
		if !strings.Contains(stderr.String(), "0.0.0.0") || strings.Count(stderr.String(), "\n") != 1 {
			t.Fatalf("stderr = %q, want one exposure warning line", stderr.String())
		}
	}

	// A verified narrow plan stays narrow under the opt-in and warns nothing.
	gw := netip.MustParseAddr("172.17.0.1")
	narrow := containerplan.SelectTUIExposure(containerplan.TUINetworkFacts{Container: true, Engine: containerplan.TUIEngineDocker, BridgeGateway: gw, GatewayLocal: true}, true)
	if narrow.Kind() != containerplan.TUIPlanNarrow || narrow.Warning() != "" || narrow.DialHost().String() != "172.17.0.1" {
		t.Fatalf("row 8 under the opt-in = %+v, want narrow, gateway URL, no warning", narrow)
	}
}

func TestInteractiveExecutorHostRuntimeUsesLoopback(t *testing.T) {
	t.Parallel()

	rt := &plainInteractiveRuntime{}
	result := InteractiveExecutor{runSession: (&fakeSession{}).run}.Execute(newExposureTestContext(t, &bytes.Buffer{}), invowkfile.CommandName("probe"), rt)
	if result.Error != nil {
		t.Fatalf("Execute(): %v", result.Error)
	}
	if !strings.HasPrefix(string(rt.tuiCtx.ServerURL), "http://127.0.0.1:") {
		t.Fatalf("ServerURL = %q, want loopback", rt.tuiCtx.ServerURL)
	}
}

func (*plainInteractiveRuntime) Validate(*runtime.ExecutionContext) error { return nil }

func (p *plainInteractiveRuntime) PrepareInteractive(ctx *runtime.ExecutionContext) (*runtime.PreparedCommand, error) {
	p.tuiCtx = ctx.TUI
	return &runtime.PreparedCommand{Cmd: exec.CommandContext(context.Background(), "true")}, nil
}

func firstNonLoopbackIPv4(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
			return ipNet.IP.String()
		}
	}
	return ""
}
