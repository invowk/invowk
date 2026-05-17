// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package container

// sysctlOverrideOpts is a no-op on non-Linux platforms. On macOS/Windows, Podman
// runs inside a Linux VM (podman machine/WSL2) — CONTAINERS_CONF_OVERRIDE set on
// the host doesn't reach the VM's container runtime. BaseCLIEngine serializes
// Podman runs with the best available host-side primitive, and the runtime retry
// loop handles transient errors from the VM.
func sysctlOverrideOpts(_ string) []BaseCLIEngineOption {
	return nil
}
