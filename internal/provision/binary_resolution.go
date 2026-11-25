// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"context"
	"errors"
	"fmt"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// architectureUnknown: the engine did not report the image and nothing
	// was assumed; the machine comparison is skipped.
	architectureUnknown architectureSource = 0
	// architectureReported: the engine reported the base image's architecture.
	architectureReported architectureSource = 1
	// architectureAssumed: the engine did not report the image, and the
	// Linux host's architecture stands in for it.
	architectureAssumed architectureSource = 2

	defaultPullEngine = "docker"
)

// ErrUnusableInvowkBinary is the sentinel error wrapped by UnusableInvowkBinaryError.
var ErrUnusableInvowkBinary = errors.New("unusable invowk binary")

type (
	// architectureSource says where a container architecture came from.
	architectureSource int

	// containerArchitecture is the architecture a candidate binary is
	// checked against, with its source.
	containerArchitecture struct {
		arch   container.ImageArchitecture
		source architectureSource
	}

	// binaryResolution is the outcome of resolving the invowk binary for one
	// request: the path to install (empty for none) and an optional warning
	// for a skipped default binary.
	binaryResolution struct {
		path    types.FilesystemPath //goplint:ignore -- optional: empty means no binary is installed.
		warning *Warning
	}

	// UnusableInvowkBinaryError is returned when the configured
	// container.auto_provision.binary_path is not a Linux executable for the
	// container architecture. Runtimes never soften it into a fallback.
	UnusableInvowkBinaryError struct {
		// Path is the configured binary.
		Path types.FilesystemPath
		// Reason says why the file cannot run in the container.
		Reason BinaryRejection
		// ArchitectureAssumed is true when the only failure is a machine
		// mismatch against the host architecture, assumed because the
		// engine did not have the base image.
		ArchitectureAssumed bool
		// Image is the base image the binary was checked for.
		Image container.ImageTag
		// Engine names the engine CLI for the recovery hint ("docker", "podman").
		Engine container.EngineType
	}
)

// String returns the source's name.
func (s architectureSource) String() string {
	switch s {
	case architectureUnknown:
		return "unknown"
	case architectureReported:
		return "reported"
	case architectureAssumed:
		return "assumed"
	default:
		return fmt.Sprintf("source(%d)", int(s))
	}
}

// Validate returns an error when the source is not one of the declared sources.
func (s architectureSource) Validate() error {
	if s < architectureUnknown || s > architectureAssumed {
		return fmt.Errorf("invalid architecture source %d", int(s))
	}
	return nil
}

// Validate returns an error when the architecture or its source is invalid.
func (a containerArchitecture) Validate() error {
	return errors.Join(a.arch.Validate(), a.source.Validate())
}

// Validate returns an error when the resolved path or warning is invalid.
func (r binaryResolution) Validate() error {
	var errs []error
	if r.path != "" {
		errs = append(errs, r.path.Validate())
	}
	if r.warning != nil {
		errs = append(errs, r.warning.Validate())
	}
	return errors.Join(errs...)
}

// warnings returns the resolution's warning as a slice.
func (r binaryResolution) warnings() []Warning {
	if r.warning == nil {
		return nil
	}
	return []Warning{*r.warning}
}

// Error implements the error interface for UnusableInvowkBinaryError.
func (e *UnusableInvowkBinaryError) Error() string {
	if e.ArchitectureAssumed {
		engine := string(e.Engine)
		if engine == "" || e.Engine == container.EngineTypeAny {
			engine = defaultPullEngine
		}
		host := e.Reason.ContainerArchitecture
		return fmt.Sprintf("container.auto_provision.binary_path %s is built for %s, but the engine does not have image %s yet, "+
			"so invowk assumed the host architecture %s. If the image is for another platform, make it local first "+
			"(%s pull %s, or build its Containerfile) and run again; otherwise set binary_path to a Linux %s build",
			e.Path, e.Reason.BinaryArchitecture, e.Image, host, engine, e.Image, host)
	}
	return fmt.Sprintf("container.auto_provision.binary_path %s cannot run in the container: %s", e.Path, e.Reason)
}

// Unwrap returns ErrUnusableInvowkBinary for errors.Is() compatibility.
func (e *UnusableInvowkBinaryError) Unwrap() error { return ErrUnusableInvowkBinary }

// resolveInvowkBinary decides which invowk binary, if any, a request installs
// into an image built on baseImage. The configured InvowkBinaryPath wins over
// DefaultInvowkBinaryPath. A configured binary that is not a Linux executable
// for the container architecture is an *UnusableInvowkBinaryError; such a
// default binary is skipped with a warning. assumeHost allows the Linux-host
// fallback when the engine cannot report the base image: the persistent
// identity step passes false, so it never rejects a binary on an assumption
// that the later image build may disprove. The shared Config is not mutated.
func (p *LayerProvisioner) resolveInvowkBinary(ctx context.Context, baseImage container.ImageTag, assumeHost bool) (binaryResolution, error) {
	candidate, explicit := p.config.InvowkBinaryPath, true
	if candidate == "" {
		candidate, explicit = p.config.DefaultInvowkBinaryPath, false
	}
	if candidate == "" {
		return binaryResolution{}, nil
	}

	arch := p.containerArchitecture(ctx, baseImage, assumeHost)
	rejection := checkLinuxBinary(candidate, arch.arch)
	if rejection == nil {
		return binaryResolution{path: candidate}, nil
	}
	if explicit {
		return binaryResolution{}, &UnusableInvowkBinaryError{
			Path:                candidate,
			Reason:              *rejection,
			ArchitectureAssumed: rejection.Kind == BinaryRejectionMachine && arch.source == architectureAssumed,
			Image:               baseImage,
			Engine:              p.engineType(),
		}
	}
	return binaryResolution{warning: skippedBinaryWarning(candidate, *rejection, arch)}, nil
}

// containerArchitecture returns the base image's architecture as the engine
// reports it; otherwise, when assumeHost is set and the host is Linux, the
// host's architecture; otherwise unknown. A reported name invowk does not
// know is kept as reported: the host is never assumed in its place.
func (p *LayerProvisioner) containerArchitecture(ctx context.Context, baseImage container.ImageTag, assumeHost bool) containerArchitecture {
	if reported := p.queryImageArchitecture(ctx, baseImage); !reported.IsUnknown() {
		return containerArchitecture{arch: reported, source: architectureReported}
	}
	if assumeHost && p.hostLinux {
		return containerArchitecture{arch: p.hostArch, source: architectureAssumed}
	}
	return containerArchitecture{arch: container.ImageArchitectureUnknown, source: architectureUnknown}
}

// queryImageArchitecture asks the engine for the image's architecture through
// its read-only query runner, when the engine has one.
func (p *LayerProvisioner) queryImageArchitecture(ctx context.Context, image container.ImageTag) container.ImageArchitecture {
	if p.imageArchitecture != nil {
		return p.imageArchitecture(ctx, image)
	}
	runner, ok := p.engine.(container.EngineQueryRunner)
	if !ok {
		return container.ImageArchitectureUnknown
	}
	return container.QueryImageArchitecture(ctx, runner, image)
}

// engineType names the engine for recovery hints, when it can say.
func (p *LayerProvisioner) engineType() container.EngineType {
	named, ok := p.engine.(container.NamedEngine)
	if !ok {
		return ""
	}
	return container.EngineType(named.Name()) //goplint:ignore -- display name for the recovery hint only.
}

// skippedBinaryWarning is the actionable warning for a default binary that is
// not installed.
func skippedBinaryWarning(path types.FilesystemPath, rejection BinaryRejection, arch containerArchitecture) *Warning {
	advice := "a Linux build of invowk for the image's architecture, which the engine did not report"
	if !arch.arch.IsUnknown() {
		advice = fmt.Sprintf("a Linux %s build of invowk", arch.arch)
	}
	return &Warning{Message: WarningMessage(fmt.Sprintf(
		"invowk binary %s was not installed in the container: %s. Nested invowk commands inside the container will not work. "+
			"Set container.auto_provision.binary_path to %s",
		path, rejection, advice))}
}
