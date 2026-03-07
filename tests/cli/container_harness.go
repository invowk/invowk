// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkfile"

	"github.com/rogpeppe/go-internal/testscript"
)

const (
	containerHarnessStatusSkip  containerHarnessStatus = 0
	containerHarnessStatusReady containerHarnessStatus = 1
	containerHarnessStatusFail  containerHarnessStatus = 2
	containerSmokeTimeout                              = 30 * time.Second
)

var containerHarness = sync.OnceValue(resolveContainerSuiteHarness)

type (
	containerHarnessStatus int

	engineProbeStatus struct {
		present    bool
		healthy    bool
		reason     string
		binaryPath string
	}

	containerSuiteHarness struct {
		status     containerHarnessStatus
		engineType container.EngineType
		binaryPath string
		reason     string
	}
)

func currentContainerHarness() containerSuiteHarness {
	return containerHarness()
}

func containerCLISuiteSupportedHost(host invowkfile.PlatformType) bool {
	return host == invowkfile.PlatformLinux
}

func resolveContainerSuiteHarness() containerSuiteHarness {
	preferred := container.EngineType(config.DefaultConfig().ContainerEngine)
	statuses := map[container.EngineType]engineProbeStatus{
		container.EngineTypePodman: probeEngineStatus(container.EngineTypePodman),
		container.EngineTypeDocker: probeEngineStatus(container.EngineTypeDocker),
	}

	return decideContainerSuiteHarnessForHost(
		invowkfile.CurrentPlatform(),
		strings.TrimSpace(os.Getenv("INVOWK_TEST_CONTAINER_ENGINE")),
		preferred,
		statuses,
	)
}

func decideContainerSuiteHarnessForHost(
	host invowkfile.PlatformType,
	explicit string,
	preferred container.EngineType,
	statuses map[container.EngineType]engineProbeStatus,
) containerSuiteHarness {
	if !containerCLISuiteSupportedHost(host) {
		return containerSuiteHarness{
			status: containerHarnessStatusSkip,
			reason: "container CLI suite requires a Linux host for Linux container runtime coverage",
		}
	}

	return decideContainerSuiteHarness(explicit, preferred, statuses)
}

func decideContainerSuiteHarness(
	explicit string,
	preferred container.EngineType,
	statuses map[container.EngineType]engineProbeStatus,
) containerSuiteHarness {
	if explicit != "" {
		engineType := container.EngineType(explicit)
		if engineType != container.EngineTypePodman && engineType != container.EngineTypeDocker {
			return containerSuiteHarness{
				status: containerHarnessStatusFail,
				reason: fmt.Sprintf("invalid INVOWK_TEST_CONTAINER_ENGINE value %q (expected docker or podman)", explicit),
			}
		}
		return harnessForSelectedEngine(engineType, statuses[engineType], true)
	}

	preferredStatus := statuses[preferred]
	if preferredStatus.present {
		return harnessForSelectedEngine(preferred, preferredStatus, false)
	}

	fallback := container.EngineTypeDocker
	if preferred == container.EngineTypeDocker {
		fallback = container.EngineTypePodman
	}

	fallbackStatus := statuses[fallback]
	if fallbackStatus.present {
		return harnessForSelectedEngine(fallback, fallbackStatus, false)
	}

	return containerSuiteHarness{
		status: containerHarnessStatusSkip,
		reason: "no installed container engine is available for the container CLI suite",
	}
}

func harnessForSelectedEngine(
	engineType container.EngineType,
	status engineProbeStatus,
	explicit bool,
) containerSuiteHarness {
	if status.healthy {
		return containerSuiteHarness{
			status:     containerHarnessStatusReady,
			engineType: engineType,
			binaryPath: status.binaryPath,
		}
	}

	if status.present {
		prefix := "selected"
		if explicit {
			prefix = "explicit"
		}
		return containerSuiteHarness{
			status: containerHarnessStatusFail,
			reason: fmt.Sprintf("%s container engine %q is installed but unhealthy for the container CLI suite: %s", prefix, engineType, status.reason),
		}
	}

	return containerSuiteHarness{
		status: containerHarnessStatusSkip,
		reason: fmt.Sprintf("container engine %q is not installed for the container CLI suite", engineType),
	}
}

func probeEngineStatus(engineType container.EngineType) engineProbeStatus {
	engine := newSpecificEngine(engineType)
	if engine == nil {
		return engineProbeStatus{reason: fmt.Sprintf("unsupported engine %q", engineType)}
	}
	defer func() {
		closeErr := container.CloseEngine(engine)
		_ = closeErr
	}()

	binaryPath := engine.BinaryPath()
	if binaryPath == "" {
		return engineProbeStatus{reason: "binary not found on PATH"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), containerSmokeTimeout)
	defer cancel()

	if _, err := engine.Version(ctx); err != nil {
		return engineProbeStatus{
			present:    true,
			reason:     fmt.Sprintf("version probe failed: %v", err),
			binaryPath: binaryPath,
		}
	}

	if err := smokeRunEngine(ctx, engine); err != nil {
		return engineProbeStatus{
			present:    true,
			reason:     fmt.Sprintf("run smoke failed: %v", err),
			binaryPath: binaryPath,
		}
	}

	if err := smokeBuildEngine(ctx, engine); err != nil {
		return engineProbeStatus{
			present:    true,
			reason:     fmt.Sprintf("build smoke failed: %v", err),
			binaryPath: binaryPath,
		}
	}

	return engineProbeStatus{
		present:    true,
		healthy:    true,
		binaryPath: binaryPath,
	}
}

func newSpecificEngine(engineType container.EngineType) container.Engine {
	switch engineType {
	case container.EngineTypePodman:
		return container.NewSandboxAwareEngine(container.NewPodmanEngine())
	case container.EngineTypeDocker:
		return container.NewSandboxAwareEngine(container.NewDockerEngine())
	case container.EngineTypeAny:
		return nil
	default:
		return nil
	}
}

func smokeRunEngine(ctx context.Context, engine container.Engine) error {
	result, err := engine.Run(ctx, container.RunOptions{
		Image:   "debian:stable-slim",
		Command: []string{"echo", "ok"},
		Remove:  true,
		Stdout:  io.Discard,
		Stderr:  io.Discard,
	})
	if err != nil {
		return err
	}
	if result == nil {
		return errors.New("run returned nil result")
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("exit code %d", result.ExitCode)
	}
	return nil
}

func smokeBuildEngine(ctx context.Context, engine container.Engine) error {
	tmpDir, err := os.MkdirTemp("", "invowk-container-smoke-*")
	if err != nil {
		return fmt.Errorf("create smoke build dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM debian:stable-slim\nRUN echo smoke >/tmp/smoke\n"), 0o600); err != nil {
		return err
	}

	tag := container.ImageTag(fmt.Sprintf("invowk-test-smoke:%d", time.Now().UnixNano()))
	if err := engine.Build(ctx, container.BuildOptions{
		ContextDir: container.HostFilesystemPath(tmpDir),       //goplint:ignore -- temp dir for smoke build
		Dockerfile: container.HostFilesystemPath("Dockerfile"), //goplint:ignore -- deterministic smoke dockerfile name
		Tag:        tag,
		Stdout:     io.Discard,
		Stderr:     io.Discard,
	}); err != nil {
		return err
	}

	removeErr := engine.RemoveImage(ctx, tag, true)
	_ = removeErr
	return nil
}

func ensureContainerSuiteConfig(env *testscript.Env) error {
	harness := currentContainerHarness()
	if harness.status != containerHarnessStatusReady {
		return nil
	}

	env.Setenv("INVOWK_TEST_CONTAINER_ENGINE", harness.engineType.String())

	configDir := containerSuiteConfigDir(env)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.cue")
	configBody := fmt.Sprintf("container_engine: %q\n", harness.engineType)
	return os.WriteFile(configPath, []byte(configBody), 0o600)
}

func containerSuiteConfigDir(env *testscript.Env) string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(env.Getenv("APPDATA"), config.AppName)
	case "darwin":
		return filepath.Join(env.Getenv("HOME"), "Library", "Application Support", config.AppName)
	default:
		if xdg := env.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, config.AppName)
		}
		return filepath.Join(env.Getenv("HOME"), ".config", config.AppName)
	}
}

func cleanupTestContainersForHarness(prefix string, harness containerSuiteHarness) {
	if harness.status != containerHarnessStatusReady || harness.binaryPath == "" {
		return
	}

	enginePath := harness.binaryPath
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	listCmd := exec.CommandContext(ctx, enginePath, "ps", "-a", "-q", "--filter", "name="+prefix)
	output, err := listCmd.Output()
	cancel()
	if err != nil || len(output) == 0 {
		return
	}

	for containerID := range strings.FieldsSeq(strings.TrimSpace(string(output))) {
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 5*time.Second)
		rmCmd := exec.CommandContext(rmCtx, enginePath, "rm", "-f", containerID)
		runErr := rmCmd.Run()
		_ = runErr
		rmCancel()
	}
}
