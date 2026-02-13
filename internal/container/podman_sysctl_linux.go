// SPDX-License-Identifier: MPL-2.0

//go:build linux

package container

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// createSysctlOverrideMemfd creates an anonymous in-memory file containing the
// containers.conf override that disables default_sysctls. The memfd has no
// filesystem entry — it is passed to Podman subprocesses via ExtraFiles[0]
// (fd 3 in the child) and referenced as CONTAINERS_CONF_OVERRIDE=/dev/fd/3.
//
// Multiple concurrent child processes can safely read the same memfd because
// Podman opens the /dev/fd/3 path via os.Open(), creating a new file description
// with an independent read offset each time.
func createSysctlOverrideMemfd() (*os.File, error) {
	fd, err := unix.MemfdCreate("invowk-containers-conf", unix.MFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("memfd_create: %w", err)
	}

	f := os.NewFile(uintptr(fd), "containers-no-sysctls.conf")
	if _, writeErr := f.WriteString("[containers]\ndefault_sysctls = []\n"); writeErr != nil {
		f.Close()
		return nil, fmt.Errorf("write: %w", writeErr)
	}
	if _, seekErr := f.Seek(0, 0); seekErr != nil {
		f.Close()
		return nil, fmt.Errorf("seek: %w", seekErr)
	}

	return f, nil
}

// isRemotePodman reports whether the binary at binaryPath is podman-remote.
// On Fedora Silverblue/toolbox and similar immutable distros, only podman-remote
// exists — it communicates with the host Podman service via a Unix socket.
// CONTAINERS_CONF_OVERRIDE set on the client doesn't reach the service (which is
// what calls crun and triggers the ping_group_range sysctl write), so the memfd
// approach is ineffective.
func isRemotePodman(binaryPath string) bool {
	// Direct name check (handles the podman-remote binary case)
	if strings.Contains(filepath.Base(binaryPath), "remote") {
		return true
	}
	// Follow symlinks to detect podman -> podman-remote
	resolved, err := filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return false
	}
	return strings.Contains(filepath.Base(resolved), "remote")
}

// sysctlOverrideOpts returns BaseCLIEngine options that disable default_sysctls
// via an in-memory CONTAINERS_CONF_OVERRIDE. Returns nil when the override is not
// applicable (podman-remote, memfd failure) — the retry mechanism in runWithRetry
// and the run mutex handle transient errors as a fallback.
func sysctlOverrideOpts(binaryPath string) []BaseCLIEngineOption {
	if isRemotePodman(binaryPath) {
		slog.Debug("podman-remote detected, sysctl memfd override not applicable",
			"binary", binaryPath)
		return nil
	}

	memfd, err := createSysctlOverrideMemfd()
	if err != nil {
		slog.Debug("sysctl memfd unavailable, relying on run-level retry", "error", err)
		return nil
	}
	return []BaseCLIEngineOption{
		WithCmdExtraFile(memfd),
		WithCmdEnvOverride("CONTAINERS_CONF_OVERRIDE", "/dev/fd/3"),
		WithSysctlOverrideActive(true),
	}
}
