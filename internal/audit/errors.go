// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"errors"
	"fmt"

	"github.com/invowk/invowk/pkg/types"
)

const (
	scanContextBuildErrMsg = "failed to build scan context"
	checkerFailedErrMsg    = "checker failed"
	noScanTargetsErrMsg    = "no invowkfiles or modules found"
)

var (
	// ErrScanContextBuild is the sentinel for scan context build failures.
	ErrScanContextBuild = errors.New(scanContextBuildErrMsg)
	// ErrCheckerFailed is the sentinel for individual checker failures.
	ErrCheckerFailed = errors.New(checkerFailedErrMsg)
	// ErrNoScanTargets is returned when no invowkfiles or modules are found at the scan path.
	ErrNoScanTargets = errors.New(noScanTargetsErrMsg)
)

type (
	// ScanContextBuildError is returned when the scanner fails to discover or parse
	// invowkfiles and modules at the target path.
	ScanContextBuildError struct {
		Path types.FilesystemPath
		Err  error
	}

	// CheckerFailedError is returned when an individual checker encounters an error.
	// The scanner continues with partial results when this occurs.
	CheckerFailedError struct {
		CheckerName string
		Err         error
	}
)

// Error implements the error interface.
func (e *ScanContextBuildError) Error() string {
	return fmt.Sprintf("%s at %q: %v", scanContextBuildErrMsg, e.Path, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As chains.
func (e *ScanContextBuildError) Unwrap() error { return e.Err }

// Is reports whether the target matches ErrScanContextBuild.
func (e *ScanContextBuildError) Is(target error) bool {
	return target == ErrScanContextBuild
}

// Error implements the error interface.
func (e *CheckerFailedError) Error() string {
	return fmt.Sprintf("checker %q failed: %v", e.CheckerName, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As chains.
func (e *CheckerFailedError) Unwrap() error { return e.Err }

// Is reports whether the target matches ErrCheckerFailed.
func (e *CheckerFailedError) Is(target error) bool {
	return target == ErrCheckerFailed
}
