// SPDX-License-Identifier: MPL-2.0

//go:build linux

package container

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// createSysctlOverrideTempFile creates a temporary file containing the
// containers.conf override that disables default_sysctls. The file is written
// to os.TempDir() and referenced via its filesystem path in
// CONTAINERS_CONF_OVERRIDE. Each Podman subprocess opens the path
// independently, avoiding the fd-inheritance issues that break memfd-based
// approaches (Podman's internal process tree reuses low fd numbers).
//
// The caller is responsible for removing the file when the engine is closed.
func createSysctlOverrideTempFile() (string, error) {
	f, err := os.CreateTemp("", "invowk-containers-conf-*.toml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if _, writeErr := f.WriteString("[containers]\ndefault_sysctls = []\n"); writeErr != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write: %w", writeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close: %w", closeErr)
	}

	return f.Name(), nil
}

// isRemotePodman reports whether the binary at binaryPath is podman-remote.
// On Fedora Silverblue/toolbox and similar immutable distros, only podman-remote
// exists — it communicates with the host Podman service via a Unix socket.
// CONTAINERS_CONF_OVERRIDE set on the client doesn't reach the service (which is
// what calls crun and triggers the ping_group_range sysctl write), so the temp
// file approach is ineffective.
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
// via a temporary CONTAINERS_CONF_OVERRIDE file. Returns nil when the override is
// not applicable (podman-remote, temp file failure) — the retry mechanism in
// runWithRetry and the run mutex handle transient errors as a fallback.
func sysctlOverrideOpts(binaryPath string) []BaseCLIEngineOption {
	if isRemotePodman(binaryPath) {
		slog.Debug("podman-remote detected, sysctl override not applicable",
			"binary", binaryPath)
		return nil
	}

	tempPath, err := createSysctlOverrideTempFile()
	if err != nil {
		slog.Debug("sysctl temp file unavailable, relying on run-level retry", "error", err)
		return nil
	}
	return []BaseCLIEngineOption{
		WithCmdEnvOverride("CONTAINERS_CONF_OVERRIDE", tempPath),
		WithSysctlOverridePath(tempPath),
		WithSysctlOverrideActive(true),
	}
}
