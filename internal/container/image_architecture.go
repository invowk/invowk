// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const (
	// ImageArchitectureUnknown means the engine could not report an image's
	// architecture: the image is not local, the engine cannot run queries, or
	// the query failed.
	ImageArchitectureUnknown ImageArchitecture = "unknown"

	imageArchitectureFmt = "{{.Architecture}}"
)

// ErrInvalidImageArchitecture is the sentinel error wrapped by InvalidImageArchitectureError.
var ErrInvalidImageArchitecture = errors.New("invalid image architecture")

type (
	// ImageArchitecture is a Go/OCI architecture name as container engines
	// report it for an image ("amd64", "arm64"), or ImageArchitectureUnknown.
	ImageArchitecture string

	// InvalidImageArchitectureError is returned when an ImageArchitecture is
	// empty or contains whitespace or control characters.
	InvalidImageArchitectureError struct {
		Value ImageArchitecture
	}
)

// String returns the architecture name.
func (a ImageArchitecture) String() string { return string(a) }

// Validate returns an error if the architecture is empty or contains
// whitespace or control characters.
func (a ImageArchitecture) Validate() error {
	if a == "" {
		return &InvalidImageArchitectureError{Value: a}
	}
	for _, r := range string(a) {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return &InvalidImageArchitectureError{Value: a}
		}
	}
	return nil
}

// IsUnknown reports whether the architecture is ImageArchitectureUnknown.
func (a ImageArchitecture) IsUnknown() bool { return a == ImageArchitectureUnknown }

// Error implements the error interface for InvalidImageArchitectureError.
func (e *InvalidImageArchitectureError) Error() string {
	return fmt.Sprintf("invalid image architecture %q: must be non-empty and contain no whitespace or control characters", e.Value)
}

// Unwrap returns ErrInvalidImageArchitecture for errors.Is() compatibility.
func (e *InvalidImageArchitectureError) Unwrap() error { return ErrInvalidImageArchitecture }

// QueryImageArchitecture asks the engine for the architecture it recorded for
// image, with the read-only `image inspect --format '{{.Architecture}}'`
// query that Docker and Podman both accept. It never pulls the image or starts
// a container. An error, empty output or a malformed name gives
// ImageArchitectureUnknown, so callers decide what an unknown architecture
// means.
func QueryImageArchitecture(ctx context.Context, runner EngineQueryRunner, image ImageTag) ImageArchitecture {
	if runner == nil || image.Validate() != nil {
		return ImageArchitectureUnknown
	}
	out, err := runner.RunQuery(ctx, "image", "inspect", containerArgFormat, imageArchitectureFmt, string(image))
	if err != nil {
		return ImageArchitectureUnknown
	}
	arch := ImageArchitecture(strings.TrimSpace(string(out))) //goplint:ignore -- validated immediately below.
	if arch.Validate() != nil {
		return ImageArchitectureUnknown
	}
	return arch
}
