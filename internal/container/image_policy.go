// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrWindowsContainerImage is returned when a Windows container image is used.
	// The container runtime only supports Linux containers.
	ErrWindowsContainerImage = errors.New("windows container images are not supported")

	// ErrAlpineContainerImage is returned when an Alpine-based container image is used.
	// Alpine images are intentionally unsupported due to musl compatibility issues.
	ErrAlpineContainerImage = errors.New("alpine-based container images are not supported")
)

// ValidateSupportedRuntimeImage enforces Invowk's supported container image policy.
func ValidateSupportedRuntimeImage(image ImageTag) error {
	if isWindowsContainerImage(string(image)) {
		return fmt.Errorf("%w; the container runtime requires Linux-based images (e.g., debian:stable-slim); see https://invowk.io/docs/runtime-modes/container for details", ErrWindowsContainerImage)
	}
	if isAlpineContainerImage(string(image)) {
		return fmt.Errorf("%w; use a Debian-based image (e.g., debian:stable-slim) for reliable execution; see https://invowk.io/docs/runtime-modes/container for details", ErrAlpineContainerImage)
	}

	return nil
}

// isWindowsContainerImage detects if an image is Windows-based by name convention.
// The container runtime only supports Linux containers. Windows container images
// are not supported because runtime scripts execute through Linux shell tooling.
func isWindowsContainerImage(image string) bool {
	imageLower := strings.ToLower(image)
	windowsPatterns := []string{
		"mcr.microsoft.com/windows/",
		"mcr.microsoft.com/powershell:",
		"microsoft/windowsservercore",
		"microsoft/nanoserver",
	}
	for _, pattern := range windowsPatterns {
		if strings.Contains(imageLower, pattern) {
			return true
		}
	}
	return false
}

// isAlpineContainerImage detects Alpine-based image references by repository name.
// Detection is segment-aware: only the last path segment of the image name is checked.
func isAlpineContainerImage(image string) bool {
	imageLower := strings.ToLower(strings.TrimSpace(image))
	if imageLower == "" {
		return false
	}

	name := imageLower
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		name = name[:idx]
	}
	if idx := strings.LastIndex(name, "@"); idx != -1 {
		name = name[:idx]
	}

	return name == "alpine" || strings.HasSuffix(name, "/alpine")
}
