// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// DockerEngine implements the Engine interface using Docker CLI.
// It embeds BaseCLIEngine for common CLI operations.
type DockerEngine struct {
	*BaseCLIEngine
}

// NewDockerEngine creates a new Docker engine.
//
// When the Docker daemon reports SELinux among its security options, volume
// mounts are labeled with :z, as the Podman engine does. The daemon is asked
// rather than the local host because it may run elsewhere (a toolbox talking to
// the host daemon, or a Docker Desktop VM). The answer is cached per engine.
func NewDockerEngine(opts ...BaseCLIEngineOption) *DockerEngine {
	engine := &DockerEngine{}
	daemonSELinux := sync.OnceValue(engine.daemonReportsSELinux)
	return newDockerEngine(engine, daemonSELinux, opts...)
}

// NewDockerEngineWithSELinuxCheck creates a Docker engine with a custom SELinux
// check, for testing volume labeling without a Docker daemon.
func NewDockerEngineWithSELinuxCheck(selinuxCheck SELinuxCheckFunc, opts ...BaseCLIEngineOption) *DockerEngine {
	return newDockerEngine(&DockerEngine{}, selinuxCheck, opts...)
}

func newDockerEngine(engine *DockerEngine, selinuxCheck SELinuxCheckFunc, opts ...BaseCLIEngineOption) *DockerEngine {
	path, _ := exec.LookPath("docker")
	allOpts := []BaseCLIEngineOption{
		WithName(string(EngineTypeDocker)),
		WithImageExistsSubCmd("inspect"),
		WithVolumeFormatter(makeSELinuxLabelAdder(selinuxCheck)),
	}
	allOpts = append(allOpts, opts...)
	// Binary path may be empty if Docker is not installed — validated later via Available().
	engine.BaseCLIEngine = NewBaseCLIEngine(HostFilesystemPath(path), allOpts...) //goplint:ignore -- validated by Available() guard
	return engine
}

// Available checks if Docker is available.
// Uses an internal timeout to prevent indefinite hangs when the daemon is unresponsive.
func (e *DockerEngine) Available() bool {
	if e.BinaryPath() == "" {
		return false
	}
	return probeEngineAvailability(func(ctx context.Context) error {
		cmd := e.CreateCommand(ctx, containerCommandVersion, containerArgFormat, "{{.Server.Version}}")
		return cmd.Run()
	})
}

// Version returns the Docker version.
//
//plint:render
func (e *DockerEngine) Version(ctx context.Context) (string, error) {
	out, err := e.RunCommandWithOutput(ctx, containerCommandVersion, containerArgFormat, "{{.Server.Version}}")
	if err != nil {
		return "", fmt.Errorf("failed to get docker version: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ImageExists checks if an image exists.
// Docker uses "image inspect" which returns detailed JSON on success.
func (e *DockerEngine) ImageExists(ctx context.Context, image ImageTag) (bool, error) {
	err := e.RunCommandStatus(ctx, "image", "inspect", string(image))
	return err == nil, nil
}

// daemonReportsSELinux asks the Docker daemon whether it runs containers with
// SELinux confinement. Any failure (no binary, unreachable daemon) reports
// false, which keeps volume mounts unlabeled as before.
func (e *DockerEngine) daemonReportsSELinux() bool {
	if e.BinaryPath() == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), availabilityTimeout)
	defer cancel()
	out, err := e.RunCommandWithOutput(ctx, "info", containerArgFormat, "{{json .SecurityOptions}}")
	if err != nil {
		return false
	}
	return securityOptionsReportSELinux(out)
}

// securityOptionsReportSELinux reports whether `docker info` security options
// (for example ["name=seccomp,profile=builtin","name=selinux"]) include SELinux.
func securityOptionsReportSELinux(securityOptions string) bool {
	var options []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(securityOptions)), &options); err != nil {
		return false
	}
	for _, option := range options {
		for field := range strings.SplitSeq(option, ",") {
			if field == "name=selinux" {
				return true
			}
		}
	}
	return false
}
