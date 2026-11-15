// SPDX-License-Identifier: MPL-2.0

//go:build linux

package commandadapters

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// envExpectedTUIPlanRow names the D2 row the engine of this job must select
// (set as a literal per CI job in .github/workflows/ci.yml).
const envExpectedTUIPlanRow = "INVOWK_TEST_TUI_PLAN_ROW"

var (
	invowkBinaryOnce sync.Once
	invowkBinaryPath string
	errInvowkBinary  error
)

type (
	// tuiRecordingContainerRuntime records the TUI context handed to the
	// container and runs onPrepare while the TUI server (if any) is up.
	// planOverride, when set, replaces the real planner.
	tuiRecordingContainerRuntime struct {
		*runtime.ContainerRuntime
		tuiCtx       runtime.TUIContext
		onPrepare    func(runtime.TUIContext)
		planOverride *containerplan.TUIExposurePlan
	}
)

func (r *tuiRecordingContainerRuntime) PlanTUIExposure(ctx *runtime.ExecutionContext) containerplan.TUIExposurePlan {
	if r.planOverride != nil {
		return *r.planOverride
	}
	return r.ContainerRuntime.PlanTUIExposure(ctx)
}

func (r *tuiRecordingContainerRuntime) PrepareInteractive(ctx *runtime.ExecutionContext) (*runtime.PreparedCommand, error) {
	r.tuiCtx = ctx.TUI
	if r.onPrepare != nil {
		r.onPrepare(ctx.TUI)
	}
	return r.ContainerRuntime.PrepareInteractive(ctx)
}

// buildInvowkBinary builds the invowk CLI once per test process so the
// provisioned image can run `invowk tui *` inside the container.
func buildInvowkBinary(t *testing.T) string {
	t.Helper()
	invowkBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "invowk-tui-exposure-bin-")
		if err != nil {
			errInvowkBinary = err
			return
		}
		invowkBinaryPath = filepath.Join(dir, "invowk")
		root, err := filepath.Abs(filepath.Join("..", "..", ".."))
		if err != nil {
			errInvowkBinary = err
			return
		}
		cmd := exec.CommandContext(t.Context(), "go", "build", "-o", invowkBinaryPath, ".")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
			errInvowkBinary = errors.New(string(out))
		}
	})
	if errInvowkBinary != nil {
		t.Fatalf("build invowk: %v", errInvowkBinary)
	}
	return invowkBinaryPath
}

// newIntegrationRuntime returns the production container runtime on the
// local engine, provisioning a freshly built invowk binary.
func newIntegrationRuntime(t *testing.T) *tuiRecordingContainerRuntime {
	t.Helper()
	engine, err := container.AutoDetectEngine()
	if err != nil {
		t.Skipf("no container engine available: %v", err)
	}
	rt, err := runtime.NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine(): %v", err)
	}
	cfg := provision.DefaultConfig()
	cfg.InvowkBinaryPath = types.FilesystemPath(buildInvowkBinary(t))
	if err := rt.SetProvisionConfig(cfg); err != nil {
		t.Fatalf("SetProvisionConfig(): %v", err)
	}
	return &tuiRecordingContainerRuntime{ContainerRuntime: rt}
}

func newIntegrationContext(t *testing.T, script string, persistent *invowkfile.RuntimePersistentConfig) (*runtime.ExecutionContext, *bytes.Buffer) {
	t.Helper()
	dir := testutil.ContainerSafeTempDir(t, "tui-exposure")
	inv := &invowkfile.Invowkfile{
		FilePath:   invowkfile.FilesystemPath(filepath.Join(dir, "invowkfile.cue")),
		ModulePath: invowkfile.FilesystemPath(dir),
	}
	cmd := &invowkfile.Command{
		Name: "tui-exposure",
		Implementations: []invowkfile.Implementation{{
			Script:    invowkfile.ImplementationScript{Content: invowkfile.ScriptContent(script), Interpreter: "bash"},
			Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim", Persistent: persistent}},
			Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
		}},
	}
	ctx := runtime.NewExecutionContext(testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout), cmd, inv)
	var stderr bytes.Buffer
	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &stderr
	return ctx, &stderr
}

// ptySession runs the prepared command under a PTY (container runs use -t)
// and appends its output to out, standing in for the Bubble Tea session.
func ptySession(out *bytes.Buffer) func(context.Context, tui.InteractiveOptions, *exec.Cmd) (*tui.InteractiveResult, error) {
	return func(_ context.Context, _ tui.InteractiveOptions, cmd *exec.Cmd) (*tui.InteractiveResult, error) {
		f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 120})
		if err != nil {
			return nil, err
		}
		_, _ = io.Copy(out, f) // EIO at child exit ends the copy.
		_ = f.Close()
		result := &tui.InteractiveResult{}
		if waitErr := cmd.Wait(); waitErr != nil {
			exitErr, ok := errors.AsType[*exec.ExitError](waitErr)
			if !ok {
				return nil, waitErr
			}
			result.ExitCode = types.ExitCode(exitErr.ExitCode())
		}
		return result, nil
	}
}

// expectedPlanRow reads INVOWK_TEST_TUI_PLAN_ROW; unset fails in CI and skips locally.
func expectedPlanRow(t *testing.T) containerplan.TUIPlanRow {
	t.Helper()
	raw := os.Getenv(envExpectedTUIPlanRow)
	if raw == "" {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s must be set in CI to the D2 row this job's engine selects", envExpectedTUIPlanRow)
		}
		t.Skipf("%s is not set; set it to the D2 row of this host's engine (8 rootful Docker, 11 rootless Podman with pasta)", envExpectedTUIPlanRow)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("%s=%q: %v", envExpectedTUIPlanRow, raw, err)
	}
	return containerplan.TUIPlanRow(n)
}

// assertRefusedOutsidePlan dials the server port on every local IPv4 address
// that is neither loopback nor in the plan, from the host side.
func assertRefusedOutsidePlan(t *testing.T, tuiCtx runtime.TUIContext, plan containerplan.TUIExposurePlan) {
	t.Helper()
	u, err := url.Parse(string(tuiCtx.ServerURL))
	if err != nil {
		t.Errorf("parse %s: %v", tuiCtx.ServerURL, err)
		return
	}
	ifaceAddrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Errorf("InterfaceAddrs(): %v", err)
		return
	}
	checked := 0
	for _, a := range ifaceAddrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.To4() == nil || ipNet.IP.IsLoopback() {
			continue
		}
		addr, _ := netip.AddrFromSlice(ipNet.IP.To4())
		if slices.Contains(plan.BindAddrs, addr) {
			continue
		}
		checked++
		dialer := net.Dialer{Timeout: time.Second}
		if conn, dialErr := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort(addr.String(), u.Port())); dialErr == nil {
			_ = conn.Close()
			t.Errorf("TUI server port %s accepts connections on non-plan address %s", u.Port(), addr)
		}
	}
	if checked == 0 {
		t.Log("SKIP host-side exposure check: no non-loopback, non-plan IPv4 address on this host")
	}
}

// TestInteractiveTUIExposureContainerIntegration runs the production path
// (real facts query, planner, tuiserver and ContainerRuntime.PrepareInteractive)
// on the local engine and checks the plan row pinned for this host or CI job.
func TestInteractiveTUIExposureContainerIntegration(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	rt := newIntegrationRuntime(t) // skips without an engine
	wantRow := expectedPlanRow(t)
	testutil.AcquireContainerSemaphore(t)

	// A second host loopback listener: under pasta the container must not reach it.
	var lc net.ListenConfig
	second, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(): %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	secondPort := strconv.Itoa(second.Addr().(*net.TCPAddr).Port)

	script := `set -u
echo "ADDR_SET=${INVOWK_TUI_ADDR:+yes}"
echo "TOKEN_SET=${INVOWK_TUI_TOKEN:+yes}"
if [ -n "${INVOWK_TUI_ADDR:-}" ]; then
  hp=${INVOWK_TUI_ADDR#http://}; h=${hp%:*}; p=${hp##*:}
  probe() {
    exec 3<>"/dev/tcp/$h/$p" || { echo "$1:CONNECT_FAIL"; return; }
    if [ -n "$2" ]; then printf 'GET /health HTTP/1.0\r\nAuthorization: Bearer %s\r\n\r\n' "$2" >&3
    else printf 'GET /health HTTP/1.0\r\n\r\n' >&3; fi
    read -r status <&3; echo "$1:${status%$'\r'}"; exec 3>&-
  }
  probe AUTH "$INVOWK_TUI_TOKEN"
  probe NOAUTH ""
  if timeout 3 bash -c "exec 4<>/dev/tcp/127.0.0.1/` + secondPort + `" 2>/dev/null; then echo SECOND=reachable; else echo SECOND=blocked; fi
fi
`
	ctx, stderr := newIntegrationContext(t, script, nil)
	plan := rt.PlanTUIExposure(ctx)
	if plan.Row != wantRow {
		t.Fatalf("selected D2 row %d (%s), want %d from %s", plan.Row, plan.Kind, wantRow, envExpectedTUIPlanRow)
	}
	rt.onPrepare = func(tuiCtx runtime.TUIContext) {
		if plan.Kind == containerplan.TUIPlanNarrow {
			assertRefusedOutsidePlan(t, tuiCtx, plan)
		}
	}

	var out bytes.Buffer
	result := InteractiveExecutor{runSession: ptySession(&out)}.Execute(ctx, invowkfile.CommandName("tui-exposure"), rt)
	if result.Error != nil || result.ExitCode != 0 {
		t.Fatalf("Execute() = exit %d, error %v\nstderr:\n%s\noutput:\n%s", result.ExitCode, result.Error, stderr, out.String())
	}
	output := out.String()
	t.Logf("plan row %d (%s), URL %q, output:\n%s", plan.Row, plan.Kind, rt.tuiCtx.ServerURL, output)

	switch plan.Kind {
	case containerplan.TUIPlanNarrow:
		for _, want := range []string{"ADDR_SET=yes", "AUTH:HTTP/1.0 200", "NOAUTH:HTTP/1.0 401"} {
			if !strings.Contains(output, want) {
				t.Errorf("container output lacks %q", want)
			}
		}
		if plan.PastaForward && !strings.Contains(output, "SECOND=blocked") {
			t.Error("under pasta the container reached a second host loopback port")
		}
		if strings.Contains(stderr.String(), "invowk: ") {
			t.Errorf("narrow plan printed an invowk warning to stderr:\n%s", stderr.String())
		}
	case containerplan.TUIPlanNone:
		assertNoServerRun(t, output, stderr.String())
	case containerplan.TUIPlanWide:
		t.Fatalf("restricted config selected the wide plan")
	}
}

func assertNoServerRun(t *testing.T, output, stderr string) {
	t.Helper()
	if strings.Contains(output, "ADDR_SET=yes") || strings.Contains(output, "TOKEN_SET=yes") {
		t.Errorf("no-server run injected INVOWK_TUI_* variables:\n%s", output)
	}
	if got := strings.Count(stderr, "interactive TUI overlay disabled"); got != 1 {
		t.Errorf("no-server warning printed %d times, want once:\n%s", got, stderr)
	}
}

// TestInteractiveNoServerRunRendersDirectly forces a no-route plan (row 7) on
// the local engine and checks that the container sees no INVOWK_TUI_*
// variables, that a delegating `invowk tui spin` renders directly and
// succeeds, that the exit code propagates and that the warning is printed once.
func TestInteractiveNoServerRunRendersDirectly(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	testutil.AcquireContainerSemaphore(t)

	rt := newIntegrationRuntime(t)
	plan := containerplan.SelectTUIExposure(containerplan.TUINetworkFacts{Container: true, Engine: containerplan.TUIEngineDocker, Rootless: true}, false)
	rt.planOverride = &plan

	script := `echo "ADDR_SET=${INVOWK_TUI_ADDR:+yes}"
echo "TOKEN_SET=${INVOWK_TUI_TOKEN:+yes}"
if invowk tui spin --title probing -- true; then echo SPIN=ok; else echo "SPIN=failed:$?"; fi
exit 7
`
	ctx, stderr := newIntegrationContext(t, script, nil)
	var out bytes.Buffer
	result := InteractiveExecutor{runSession: ptySession(&out)}.Execute(ctx, invowkfile.CommandName("tui-exposure"), rt)
	if result.Error != nil {
		t.Fatalf("Execute() error = %v\nstderr:\n%s\noutput:\n%s", result.Error, stderr, out.String())
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7\noutput:\n%s", result.ExitCode, out.String())
	}
	if !strings.Contains(out.String(), "SPIN=ok") {
		t.Errorf("invowk tui spin did not render directly:\n%s", out.String())
	}
	assertNoServerRun(t, out.String(), stderr.String())
	if rt.tuiCtx != (runtime.TUIContext{}) {
		t.Errorf("TUI context = %+v, want empty", rt.tuiCtx)
	}
}

// TestTUIClientUnreachableFromContainer runs a delegating `invowk tui confirm`
// in a container whose INVOWK_TUI_ADDR points at an address that refuses or
// drops the connection, and expects a bounded failure naming the opt-in.
func TestTUIClientUnreachableFromContainer(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	testutil.AcquireContainerSemaphore(t)

	rt := newIntegrationRuntime(t)
	// 127.0.0.1:1 is the container's own loopback (refused); 192.0.2.1 is
	// TEST-NET-1 (dropped or unroutable).
	script := `for addr in http://127.0.0.1:1 http://192.0.2.1:9; do
  start=$(date +%s)
  INVOWK_TUI_ADDR=$addr INVOWK_TUI_TOKEN=token invowk tui confirm probe 2>&1
  echo "EXIT[$addr]=$? SECS=$(( $(date +%s) - start ))"
done
`
	ctx, _ := newIntegrationContext(t, script, nil)
	var stdout, stderr bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &stderr
	result := rt.Execute(ctx)
	if result.Error != nil {
		t.Fatalf("Execute() error = %v\nstdout:\n%s\nstderr:\n%s", result.Error, stdout.String(), stderr.String())
	}
	output := stdout.String() + stderr.String()
	t.Logf("output:\n%s", output)
	if got := strings.Count(output, "container.tui_server_bind"); got < 2 {
		t.Errorf("error names container.tui_server_bind %d times, want once per address", got)
	}
	for _, addr := range []string{"http://127.0.0.1:1", "http://192.0.2.1:9"} {
		line := lineWithPrefix(output, "EXIT["+addr+"]=")
		if line == "" || strings.HasPrefix(line, "EXIT["+addr+"]=0 ") {
			t.Errorf("invowk tui confirm against %s: %q, want a non-zero exit", addr, line)
			continue
		}
		secs, _ := strconv.Atoi(line[strings.LastIndex(line, "=")+1:])
		if secs > 20 {
			t.Errorf("invowk tui confirm against %s took %ds, want the bounded dial timeout", addr, secs)
		}
	}
}

func lineWithPrefix(output, prefix string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// TestInteractivePersistentRootlessPodmanRunsWithoutServer uses the real
// planner against a persistent target (row 10). It runs only under rootless
// Podman, the CI Podman job's engine.
func TestInteractivePersistentRootlessPodmanRunsWithoutServer(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	rt := newIntegrationRuntime(t)
	facts, err := container.QueryEngineNetworkFacts(t.Context(), container.NewSandboxAwareEngine(mustEngine(t)), nil)
	if err != nil || facts.Engine != container.EngineTypePodman || !facts.Rootless || facts.RemoteEndpoint {
		t.Skipf("needs local rootless Podman (facts %+v, err %v)", facts, err)
	}
	testutil.AcquireContainerSemaphore(t)

	name := "invowk-tui-exposure-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	t.Cleanup(func() {
		_ = exec.CommandContext(context.WithoutCancel(t.Context()), "podman", "rm", "-f", name).Run() // Best-effort cleanup.
	})
	persistent := &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: invowkfile.ContainerName(name)}
	script := `echo "ADDR_SET=${INVOWK_TUI_ADDR:+yes}"
echo "TOKEN_SET=${INVOWK_TUI_TOKEN:+yes}"
exit 5
`
	ctx, stderr := newIntegrationContext(t, script, persistent)
	if plan := rt.PlanTUIExposure(ctx); plan.Row != containerplan.TUIRowPodmanPersistent {
		t.Fatalf("selected row %d, want %d (persistent rootless Podman)", plan.Row, containerplan.TUIRowPodmanPersistent)
	}
	var out bytes.Buffer
	result := InteractiveExecutor{runSession: ptySession(&out)}.Execute(ctx, invowkfile.CommandName("tui-exposure"), rt)
	if result.Error != nil || result.ExitCode != 5 {
		t.Fatalf("Execute() = exit %d, error %v\nstderr:\n%s\noutput:\n%s", result.ExitCode, result.Error, stderr, out.String())
	}
	assertNoServerRun(t, out.String(), stderr.String())
	if !strings.Contains(stderr.String(), "container.tui_server_bind") {
		t.Errorf("warning does not name the opt-in:\n%s", stderr.String())
	}
}

func mustEngine(t *testing.T) container.Engine {
	t.Helper()
	engine, err := container.AutoDetectEngine()
	if err != nil {
		t.Skipf("no container engine available: %v", err)
	}
	return engine
}
