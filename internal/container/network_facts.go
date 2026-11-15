// SPDX-License-Identifier: MPL-2.0

package container

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strings"

	"github.com/invowk/invowk/pkg/platform"
)

const (
	dockerDesktopOS        = "Docker Desktop"
	dockerRootlessSecOpt   = "name=rootless"
	flatpakInfoPath        = "/.flatpak-info"
	dockerInfoNetworkFmt   = "{{.OperatingSystem}}|{{json .SecurityOptions}}"
	dockerBridgeGatewayFmt = "{{range .IPAM.Config}}{{.Gateway}} {{end}}"
	dockerContextHostFmt   = "{{.Endpoints.docker.Host}}"
	podmanInfoNetworkFmt   = "{{.Host.Security.Rootless}}|{{.Host.RootlessNetworkCmd}}|{{.Host.ServiceIsRemote}}|{{.Host.Pasta.Executable}}"
)

// ErrNetworkFactsUnavailable is returned when an engine cannot report its network facts.
var ErrNetworkFactsUnavailable = errors.New("container engine network facts unavailable")

type (
	// EngineQueryRunner runs a read-only engine CLI query and returns stdout.
	// Both BaseCLIEngine and SandboxAwareEngine implement it, so a sandboxed
	// invowk queries the host engine through its spawn wrapper.
	EngineQueryRunner interface {
		RunQuery(ctx context.Context, args ...string) ([]byte, error)
	}

	// NamedEngine is the part of an engine the facts query needs besides
	// EngineQueryRunner.
	NamedEngine interface {
		Name() string
	}

	// EngineNetworkFacts are the engine facts behind a TUI exposure plan
	// (restrict-tui-server-bind D2).
	EngineNetworkFacts struct {
		// Engine is "docker" or "podman".
		Engine EngineType
		// DockerDesktop reports OperatingSystem == "Docker Desktop".
		DockerDesktop bool
		// Rootless reports a rootless engine.
		Rootless bool
		// RemoteEndpoint reports a remote Docker endpoint or Podman service.
		RemoteEndpoint bool
		// BridgeGateway is Docker's default bridge gateway, when reported.
		BridgeGateway netip.Addr
		// PastaAvailable reports a non-empty Podman Pasta.Executable.
		PastaAvailable bool
	}
)

// Validate checks the engine type.
func (f EngineNetworkFacts) Validate() error {
	return f.Engine.Validate()
}

// QueryEngineNetworkFacts asks engine for its network facts. getenv supplies
// DOCKER_HOST; nil uses os.Getenv.
func QueryEngineNetworkFacts(ctx context.Context, engine NamedEngine, getenv func(string) string) (EngineNetworkFacts, error) {
	runner, ok := engine.(EngineQueryRunner)
	if !ok {
		return EngineNetworkFacts{}, fmt.Errorf("%w: engine %T cannot run queries", ErrNetworkFactsUnavailable, engine)
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	switch EngineType(engine.Name()) {
	case EngineTypeDocker:
		return queryDockerNetworkFacts(ctx, runner, getenv)
	case EngineTypePodman:
		return queryPodmanNetworkFacts(ctx, runner)
	case EngineTypeAny:
		return EngineNetworkFacts{}, fmt.Errorf("%w: engine type %q is not concrete", ErrNetworkFactsUnavailable, engine.Name())
	default:
		return EngineNetworkFacts{}, fmt.Errorf("%w: unknown engine %q", ErrNetworkFactsUnavailable, engine.Name())
	}
}

func queryDockerNetworkFacts(ctx context.Context, runner EngineQueryRunner, getenv func(string) string) (EngineNetworkFacts, error) {
	facts := EngineNetworkFacts{Engine: EngineTypeDocker}
	out, err := runner.RunQuery(ctx, "info", containerArgFormat, dockerInfoNetworkFmt)
	if err != nil {
		return facts, fmt.Errorf("%w: docker info: %w", ErrNetworkFactsUnavailable, err)
	}
	osName, secOptsJSON, found := strings.Cut(strings.TrimSpace(string(out)), "|")
	if !found {
		return facts, fmt.Errorf("%w: unexpected docker info output %q", ErrNetworkFactsUnavailable, strings.TrimSpace(string(out)))
	}
	var secOpts []string
	if jsonErr := json.Unmarshal([]byte(secOptsJSON), &secOpts); jsonErr != nil {
		return facts, fmt.Errorf("%w: docker info security options: %w", ErrNetworkFactsUnavailable, jsonErr)
	}
	facts.DockerDesktop = strings.TrimSpace(osName) == dockerDesktopOS
	facts.Rootless = slices.Contains(secOpts, dockerRootlessSecOpt)

	endpoint := strings.TrimSpace(getenv("DOCKER_HOST"))
	if endpoint == "" {
		ctxOut, ctxErr := runner.RunQuery(ctx, "context", "inspect", containerArgFormat, dockerContextHostFmt)
		if ctxErr != nil {
			return facts, fmt.Errorf("%w: docker context inspect: %w", ErrNetworkFactsUnavailable, ctxErr)
		}
		endpoint = strings.TrimSpace(string(ctxOut))
	}
	facts.RemoteEndpoint = isRemoteDockerEndpoint(endpoint)
	if facts.RemoteEndpoint || facts.DockerDesktop {
		return facts, nil
	}

	gwOut, gwErr := runner.RunQuery(ctx, "network", "inspect", "bridge", containerArgFormat, dockerBridgeGatewayFmt)
	if gwErr != nil {
		return facts, fmt.Errorf("%w: docker network inspect bridge: %w", ErrNetworkFactsUnavailable, gwErr)
	}
	for field := range strings.FieldsSeq(string(gwOut)) {
		if addr, parseErr := netip.ParseAddr(field); parseErr == nil && addr.Is4() {
			facts.BridgeGateway = addr
			break
		}
	}
	return facts, nil
}

func queryPodmanNetworkFacts(ctx context.Context, runner EngineQueryRunner) (EngineNetworkFacts, error) {
	facts := EngineNetworkFacts{Engine: EngineTypePodman}
	out, err := runner.RunQuery(ctx, "info", containerArgFormat, podmanInfoNetworkFmt)
	if err != nil {
		return facts, fmt.Errorf("%w: podman info: %w", ErrNetworkFactsUnavailable, err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) != 4 {
		return facts, fmt.Errorf("%w: unexpected podman info output %q", ErrNetworkFactsUnavailable, strings.TrimSpace(string(out)))
	}
	facts.Rootless = parts[0] == "true"
	facts.RemoteEndpoint = parts[2] == "true"
	facts.PastaAvailable = strings.TrimSpace(parts[3]) != ""
	return facts, nil
}

// isRemoteDockerEndpoint reports tcp:// and ssh:// endpoints; unix:// and
// npipe:// (and an empty endpoint, the default socket) are local.
//
//goplint:ignore -- raw DOCKER_HOST / docker context output, classified only.
func isRemoteDockerEndpoint(endpoint string) bool {
	return strings.HasPrefix(endpoint, "tcp://") || strings.HasPrefix(endpoint, "ssh://")
}

// FlatpakSharesNetwork reports whether the running Flatpak's /.flatpak-info
// lists "network" under [Context] shared. It returns false outside Flatpak or
// when the file cannot be read.
func FlatpakSharesNetwork() bool {
	if platform.DetectSandbox() != platform.SandboxFlatpak {
		return false
	}
	data, err := os.ReadFile(flatpakInfoPath)
	if err != nil {
		return false
	}
	return flatpakInfoSharesNetwork(string(data))
}

//goplint:ignore -- raw /.flatpak-info keyfile text, parsed only.
func flatpakInfoSharesNetwork(info string) bool {
	scanner := bufio.NewScanner(strings.NewReader(info))
	inContext := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inContext = line == "[Context]"
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !inContext || !found || strings.TrimSpace(key) != "shared" {
			continue
		}
		for item := range strings.SplitSeq(value, ";") {
			if strings.TrimSpace(item) == "network" {
				return true
			}
		}
	}
	return false
}
