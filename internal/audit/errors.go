// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/invowk/invowk/pkg/types"
)

const (
	scanContextBuildErrMsg       = "failed to build scan context"
	checkerFailedErrMsg          = "checker failed"
	noScanTargetsErrMsg          = "no invowkfiles or modules found"
	artifactEntryLimitErrMsg     = "artifact entry limit exceeded"
	invalidArtifactEntryLimitMsg = "invalid artifact entry limit"
)

var (
	// ErrScanContextBuild is the sentinel for scan context build failures.
	ErrScanContextBuild = errors.New(scanContextBuildErrMsg)
	// ErrCheckerFailed is the sentinel for individual checker failures.
	ErrCheckerFailed = errors.New(checkerFailedErrMsg)
	// ErrNoScanTargets is returned when no invowkfiles or modules are found at the scan path.
	ErrNoScanTargets = errors.New(noScanTargetsErrMsg)
	// ErrArtifactEntryLimitExceeded is returned when filesystem artifact traversal
	// exceeds its configured scan-wide entry budget.
	ErrArtifactEntryLimitExceeded = errors.New(artifactEntryLimitErrMsg)
	// ErrInvalidArtifactEntryLimit is returned when an artifact entry limit is not positive.
	ErrInvalidArtifactEntryLimit = errors.New(invalidArtifactEntryLimitMsg)
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

	// ArtifactEntryLimitError reports a fail-closed filesystem traversal that
	// exceeded the configured scan-wide budget for one artifact class.
	ArtifactEntryLimitError struct {
		Kind  ArtifactKind
		Path  *types.FilesystemPath
		Limit ArtifactEntryLimit
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

// Error implements the error interface.
func (e *ArtifactEntryLimitError) Error() string {
	path := types.FilesystemPath("")
	if e.Path != nil {
		path = *e.Path
	}
	return fmt.Sprintf("%s for %s at %q: limit %s", artifactEntryLimitErrMsg, e.Kind, path, e.Limit)
}

// Unwrap exposes ErrArtifactEntryLimitExceeded for errors.Is checks.
func (e *ArtifactEntryLimitError) Unwrap() error { return ErrArtifactEntryLimitExceeded }

// ScanFailureIsFatal reports whether a scanner error should suppress partial
// results. Cancellation is fatal because the scan did not complete by caller
// intent or deadline. LLM checker failures are fatal because callers explicitly
// requested interpretive analysis and the partial deterministic report would
// otherwise hide that requested analysis failed.
func ScanFailureIsFatal(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return scanErrorContainsChecker(err, LLMCheckerName)
}

//goplint:ignore -- checker names come from the audit Checker.Name() interface.
func scanErrorContainsChecker(err error, checkerName string) bool {
	if err == nil {
		return false
	}

	pending := []error{err}
	for len(pending) > 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]

		if failed, ok := errors.AsType[*CheckerFailedError](current); ok && failed.CheckerName == checkerName {
			return true
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			pending = append(pending, joined.Unwrap()...)
		}
	}
	return false
}
