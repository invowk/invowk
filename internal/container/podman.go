// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// podmanBinaryNames lists Podman binary names to try in order of preference.
// "podman" is preferred; "podman-remote" is the fallback for immutable distros
// like Fedora Silverblue/Kinoite.
var podmanBinaryNames = []string{"podman", "podman-remote"}

// PodmanEngine implements the Engine interface using Podman CLI.
// It embeds BaseCLIEngine for common CLI operations.
type PodmanEngine struct {
	*BaseCLIEngine
}

// findPodmanBinary searches for an available Podman binary.
// Returns the full path to the first found binary, or empty string if none found.
func findPodmanBinary() string {
	for _, name := range podmanBinaryNames {
		if path, err := exec.LookPath(name); err == nil {
			slog.Debug("found podman binary", "name", name, "path", path)
			return path
		}
	}
	return ""
}

// NewPodmanEngine creates a new Podman engine.
// On Linux with SELinux enabled, volume mounts are automatically labeled with :z.
// For rootless Podman compatibility, --userns=keep-id is automatically added to run commands.
func NewPodmanEngine(opts ...BaseCLIEngineOption) *PodmanEngine {
	path := findPodmanBinary()

	// Podman needs SELinux volume labels on Linux (prepend to user options)
	// Use the default SELinux check unless overridden by options
	selinuxLabelAdder := makeSELinuxLabelAdder(isSELinuxPresent)
	usernsKeepIDAdder := makeUsernsKeepIDAdder()
	allOpts := append(
		[]BaseCLIEngineOption{
			WithVolumeFormatter(selinuxLabelAdder),
			WithRunArgsTransformer(usernsKeepIDAdder),
		},
		opts...,
	)

	return &PodmanEngine{
		BaseCLIEngine: NewBaseCLIEngine(path, allOpts...),
	}
}

// NewPodmanEngineWithSELinuxCheck creates a Podman engine with a custom SELinux check function.
// This is primarily useful for testing SELinux labeling behavior on non-SELinux systems.
// For rootless Podman compatibility, --userns=keep-id is automatically added to run commands.
func NewPodmanEngineWithSELinuxCheck(selinuxCheck SELinuxCheckFunc, opts ...BaseCLIEngineOption) *PodmanEngine {
	path := findPodmanBinary()

	// Use the provided SELinux check function
	selinuxLabelAdder := makeSELinuxLabelAdder(selinuxCheck)
	usernsKeepIDAdder := makeUsernsKeepIDAdder()
	allOpts := append(
		[]BaseCLIEngineOption{
			WithVolumeFormatter(selinuxLabelAdder),
			WithRunArgsTransformer(usernsKeepIDAdder),
		},
		opts...,
	)

	return &PodmanEngine{
		BaseCLIEngine: NewBaseCLIEngine(path, allOpts...),
	}
}

// Name returns the engine name.
func (e *PodmanEngine) Name() string {
	return string(EngineTypePodman)
}

// Available checks if Podman is available.
func (e *PodmanEngine) Available() bool {
	if e.BinaryPath() == "" {
		return false
	}
	cmd := e.CreateCommand(context.Background(), "version", "--format", "{{.Version}}")
	return cmd.Run() == nil
}

// Version returns the Podman version.
func (e *PodmanEngine) Version(ctx context.Context) (string, error) {
	out, err := e.RunCommandWithOutput(ctx, "version", "--format", "{{.Version}}")
	if err != nil {
		return "", fmt.Errorf("failed to get podman version: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// Build builds an image from a Dockerfile.
func (e *PodmanEngine) Build(ctx context.Context, opts BuildOptions) error {
	args := e.BuildArgs(opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		return buildContainerError("podman", opts, err)
	}

	return nil
}

// Run runs a command in a container.
// Volume mounts are automatically labeled with SELinux labels if needed.
func (e *PodmanEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	args := e.RunArgs(opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// Remove removes a container.
func (e *PodmanEngine) Remove(ctx context.Context, containerID string, force bool) error {
	args := e.RemoveArgs(containerID, force)
	return e.RunCommandStatus(ctx, args...)
}

// ImageExists checks if an image exists.
func (e *PodmanEngine) ImageExists(ctx context.Context, image string) (bool, error) {
	err := e.RunCommandStatus(ctx, "image", "exists", image)
	return err == nil, nil
}

// RemoveImage removes an image.
func (e *PodmanEngine) RemoveImage(ctx context.Context, image string, force bool) error {
	args := e.RemoveImageArgs(image, force)
	return e.RunCommandStatus(ctx, args...)
}

// Exec runs a command in a running container.
func (e *PodmanEngine) Exec(ctx context.Context, containerID string, command []string, opts RunOptions) (*RunResult, error) {
	args := e.ExecArgs(containerID, command, opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{ContainerID: containerID}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// InspectImage returns information about an image.
func (e *PodmanEngine) InspectImage(ctx context.Context, image string) (string, error) {
	return e.RunCommandWithOutput(ctx, "image", "inspect", image)
}

// isSELinuxPresent checks if SELinux labeling should be applied.
// Returns true on Linux systems because the :z label is:
//   - Required when SELinux is enforcing
//   - Still needed when SELinux is present but disabled (for rootless Podman)
//   - Harmlessly ignored when SELinux is completely absent
//
// This is more robust than checking /sys/fs/selinux/enforce because:
// 1. The enforce file may not exist when SELinux is disabled
// 2. Podman still needs :z for proper volume access in rootless mode
// 3. The :z label is a no-op when SELinux isn't present
func isSELinuxPresent() bool {
	// Check if SELinux filesystem exists (selinuxfs mount point)
	// This is more reliable than checking enforce status
	_, err := os.Stat("/sys/fs/selinux")
	return err == nil
}

// makeSELinuxLabelAdder creates a volume formatter function that adds SELinux labels.
// The selinuxCheck function is called to determine if SELinux labeling should be applied.
// This factory pattern allows injection of custom SELinux check functions for testing.
func makeSELinuxLabelAdder(selinuxCheck SELinuxCheckFunc) VolumeFormatFunc {
	return func(volume string) string {
		return addSELinuxLabelWithCheck(volume, selinuxCheck)
	}
}

// addSELinuxLabelWithCheck adds the :z label to a volume mount if SELinux is enabled
// and the volume doesn't already have an SELinux label (:z or :Z).
// The selinuxCheck function is called to determine if SELinux labeling should be applied.
func addSELinuxLabelWithCheck(volume string, selinuxCheck SELinuxCheckFunc) string {
	if !selinuxCheck() {
		return volume
	}

	// Parse the volume string to check if it already has SELinux labels
	// Volume format: host_path:container_path[:options]
	// Options can include: ro, rw, z, Z, and others
	parts := strings.Split(volume, ":")

	// Need at least host:container
	if len(parts) < 2 {
		return volume
	}

	// Check if options already contain SELinux label
	if len(parts) >= 3 {
		options := parts[len(parts)-1]
		// Check for :z or :Z in options
		for opt := range strings.SplitSeq(options, ",") {
			if opt == "z" || opt == "Z" {
				// Already has SELinux label
				return volume
			}
		}
		// Append :z to existing options
		return volume + ",z"
	}

	// No options specified, add :z
	return volume + ":z"
}

// makeUsernsKeepIDAdder creates a transformer that adds --userns=keep-id to run commands.
// This preserves host user UID/GID in rootless Podman, preventing permission
// issues with volume mounts. The flag is harmless in rootful mode.
func makeUsernsKeepIDAdder() RunArgsTransformer {
	return func(args []string) []string {
		if len(args) == 0 || args[0] != "run" {
			return args // Only transform run commands
		}

		// Find image position (first non-flag argument after "run")
		// We need to insert --userns=keep-id before the image name.
		imagePos := -1
		skipNext := false
		for i := 1; i < len(args); i++ {
			if skipNext {
				skipNext = false
				continue
			}
			arg := args[i]
			// Flags that take a separate argument value
			if arg == "-w" || arg == "-e" || arg == "-v" || arg == "-p" ||
				arg == "--name" || arg == "--add-host" {
				skipNext = true
				continue
			}
			// Skip flags that start with - (including --rm, -i, -t, --userns=..., etc.)
			if strings.HasPrefix(arg, "-") {
				continue
			}
			// Found the image name
			imagePos = i
			break
		}

		if imagePos == -1 {
			// No image found, append at the end (unusual case)
			return append(args, "--userns=keep-id")
		}

		// Insert --userns=keep-id before the image name
		result := make([]string, 0, len(args)+1)
		result = append(result, args[:imagePos]...)
		result = append(result, "--userns=keep-id")
		result = append(result, args[imagePos:]...)
		return result
	}
}

// BuildRunArgs builds the argument slice for a 'run' command without executing.
// Returns the full argument slice including 'run' and all options.
// This is used for interactive mode where the command needs to be attached to a PTY.
// Note: Volume mounts are automatically labeled with SELinux labels if needed
// (via the volume formatter set in the constructor).
func (e *PodmanEngine) BuildRunArgs(opts RunOptions) []string {
	return e.RunArgs(opts)
}
