// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestNetworkModeValidate(t *testing.T) {
	t.Parallel()

	for _, ok := range []NetworkMode{"", "pasta:-T,43123", "bridge", "host"} {
		if err := ok.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v", ok, err)
		}
	}
	for _, bad := range []NetworkMode{"-T", "pasta: -T,1", "pasta:-T,1 --privileged", "Pasta"} {
		if err := bad.Validate(); !errors.Is(err, ErrInvalidNetworkMode) {
			t.Errorf("Validate(%q) = %v, want ErrInvalidNetworkMode", bad, err)
		}
	}
	if _, err := PastaForwardNetworkMode(types.ListenPort(0)); err == nil {
		t.Error("PastaForwardNetworkMode(0) succeeded")
	}
	mode, err := PastaForwardNetworkMode(types.ListenPort(43123))
	if err != nil || mode != "pasta:-T,43123" {
		t.Fatalf("PastaForwardNetworkMode(43123) = %q, %v", mode, err)
	}
}

// TestPodmanRunArgsNetworkModeBeforeImage pins that --network=pasta:-T,<port>
// is one argument, so the --userns=keep-id transformer still finds the image.
func TestPodmanRunArgsNetworkModeBeforeImage(t *testing.T) {
	t.Parallel()

	engine := NewPodmanEngine()
	opts := RunOptions{
		Image:       "debian:stable-slim",
		Command:     []string{"true"},
		Env:         map[string]string{"A": "1"},
		Ports:       []PortMappingSpec{"127.0.0.1:8080:80"},
		ExtraHosts:  []HostMapping{"host.docker.internal:host-gateway"},
		NetworkMode: "pasta:-T,43123",
		Remove:      true,
	}
	if err := opts.Validate(); err != nil {
		t.Fatalf("RunOptions.Validate(): %v", err)
	}
	args := engine.RunArgs(opts)
	imagePos := slices.Index(args, "debian:stable-slim")
	netPos := slices.Index(args, "--network=pasta:-T,43123")
	usernsPos := slices.Index(args, "--userns=keep-id")
	if imagePos < 0 || netPos < 0 || usernsPos < 0 {
		t.Fatalf("args = %v, want image, --network=… and --userns=keep-id", args)
	}
	if netPos > imagePos || usernsPos > imagePos {
		t.Fatalf("args = %v: --network and --userns must precede the image", args)
	}
	if slices.Contains(args, "pasta:-T,43123") {
		t.Fatalf("args = %v: network mode must not be a separate argument", args)
	}

	execArgs := engine.ExecArgs("abc123", []string{"true"}, opts)
	createArgs := engine.CreateArgs(CreateOptions{Image: "debian:stable-slim", Name: "persist"})
	for _, a := range append(execArgs, createArgs...) {
		if strings.HasPrefix(a, "--network") {
			t.Fatalf("exec/create args contain %q: exec=%v create=%v", a, execArgs, createArgs)
		}
	}
}

func TestIsLocalInterfaceAddr(t *testing.T) {
	t.Parallel()

	addrs := func() ([]netip.Prefix, error) {
		return []netip.Prefix{netip.MustParsePrefix("127.0.0.1/8"), netip.MustParsePrefix("172.17.0.1/16")}, nil
	}
	if !IsLocalInterfaceAddr(netip.MustParseAddr("172.17.0.1"), addrs) {
		t.Error("172.17.0.1 not detected as local")
	}
	if IsLocalInterfaceAddr(netip.MustParseAddr("172.18.0.1"), addrs) {
		t.Error("172.18.0.1 detected as local")
	}
	if IsLocalInterfaceAddr(netip.Addr{}, addrs) {
		t.Error("invalid address detected as local")
	}
}
