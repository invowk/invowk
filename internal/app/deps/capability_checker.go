// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"golang.org/x/term"
)

type (
	// CapabilityChecker checks host capabilities for dependency validation.
	CapabilityChecker interface {
		Check(context.Context, runtime.IOContext, invowkfile.CapabilityName) error
	}

	hostCapabilityChecker struct{}
)

func newHostCapabilityChecker() CapabilityChecker {
	return hostCapabilityChecker{}
}

// Check validates that a system capability is available.
func (hostCapabilityChecker) Check(ctx context.Context, ioCtx runtime.IOContext, capability invowkfile.CapabilityName) error {
	if ctx == nil {
		ctx = context.Background()
	}

	switch capability {
	case invowkfile.CapabilityLocalAreaNetwork:
		return checkLocalAreaNetwork()
	case invowkfile.CapabilityInternet:
		return checkInternet(ctx)
	case invowkfile.CapabilityContainers:
		return checkContainers(ctx)
	case invowkfile.CapabilityTTY:
		return checkTTY(ioCtx)
	default:
		return &invowkfile.CapabilityError{
			Capability: capability,
			Message:    "unknown capability",
		}
	}
}

// checkLocalAreaNetwork checks for non-loopback, routable network interfaces.
func checkLocalAreaNetwork() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return &invowkfile.CapabilityError{
			Capability: invowkfile.CapabilityLocalAreaNetwork,
			Message:    fmt.Sprintf("failed to list network interfaces: %v", err),
		}
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			return nil
		}
	}

	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityLocalAreaNetwork,
		Message:    "no active network interface with routable IP address found",
	}
}

// checkInternet checks for working internet connectivity with lightweight DNS probes.
func checkInternet(parentCtx context.Context) error {
	if checkLocalAreaNetwork() != nil {
		return &invowkfile.CapabilityError{
			Capability: invowkfile.CapabilityInternet,
			Message:    "no local network available",
		}
	}

	dnsServers := []string{
		"8.8.8.8:53",
		"1.1.1.1:53",
		"208.67.222.222:53",
	}

	dialer := &net.Dialer{Timeout: invowkfile.DefaultCapabilityTimeout}
	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(parentCtx, invowkfile.DefaultCapabilityTimeout)
	defer cancel()

	var lastErr error
	for _, server := range dnsServers {
		conn, err := dialer.DialContext(ctx, "udp", server)
		if err != nil {
			lastErr = err
			continue
		}
		_ = conn.Close()

		if _, err := resolver.LookupHost(ctx, "dns.google"); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	msg := "unable to reach internet DNS servers"
	if lastErr != nil {
		msg = fmt.Sprintf("unable to reach internet: %v", lastErr)
	}
	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityInternet,
		Message:    msg,
	}
}

// checkContainers checks if Docker or Podman is available and ready.
func checkContainers(parentCtx context.Context) error {
	foundEngine := false
	var lastErr error
	for _, engine := range []config.ContainerEngine{config.ContainerEnginePodman, config.ContainerEngineDocker} {
		path, err := exec.LookPath(string(engine))
		if err != nil {
			continue
		}
		foundEngine = true

		ctx, cancel := context.WithTimeout(parentCtx, invowkfile.DefaultCapabilityTimeout)
		cmd := exec.CommandContext(ctx, path, containerCapabilityProbeArgs(engine)...)
		err = cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if !foundEngine {
		return &invowkfile.CapabilityError{
			Capability: invowkfile.CapabilityContainers,
			Message:    "no container engine (podman or docker) found in PATH",
		}
	}

	msg := "container engine is not ready"
	if lastErr != nil {
		msg = fmt.Sprintf("container engine is not ready: %v", lastErr)
	}
	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityContainers,
		Message:    msg,
	}
}

//goplint:ignore -- exec.CommandContext requires argv as primitive strings.
func containerCapabilityProbeArgs(engine config.ContainerEngine) []string {
	switch engine {
	case config.ContainerEnginePodman:
		return []string{"version", "--format", "{{.Version}}"}
	case config.ContainerEngineDocker:
		return []string{"version", "--format", "{{.Server.Version}}"}
	default:
		return []string{"version"}
	}
}

// checkTTY checks whether invowk is running in an interactive terminal.
func checkTTY(ioCtx runtime.IOContext) error {
	stdin, stdinOK := ioCtx.Stdin.(*os.File)
	stdout, stdoutOK := ioCtx.Stdout.(*os.File)
	if stdinOK && stdoutOK && term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd())) {
		return nil
	}

	return &invowkfile.CapabilityError{
		Capability: invowkfile.CapabilityTTY,
		Message:    "not running in an interactive TTY (stdin/stdout)",
	}
}
