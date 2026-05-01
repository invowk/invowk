// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	defaultWatchPattern         invowkfile.GlobPattern = "**/*"
	defaultWatchInfraErrorLimit WatchInfraErrorLimit   = 3
)

type (
	// WatchInfraErrorLimit is the number of consecutive infrastructure errors
	// that cause watch mode to abort.
	WatchInfraErrorLimit int

	// WatchPlan contains command watch configuration derived from the resolved
	// command definition. Adapters own filesystem watching and rendering.
	//
	//goplint:mutable
	//
	// WatchPlan is a service-to-adapter DTO assembled at the command boundary.
	WatchPlan struct {
		Patterns             []invowkfile.GlobPattern
		Ignore               []invowkfile.GlobPattern
		Debounce             time.Duration
		BaseDir              types.FilesystemPath
		ClearScreen          bool
		InfraErrorAbortLimit WatchInfraErrorLimit
	}

	// InvalidWatchPlanError reports invalid watch configuration on a command.
	InvalidWatchPlanError struct {
		Err error
	}
)

// String returns the decimal string representation of the WatchInfraErrorLimit.
func (l WatchInfraErrorLimit) String() string { return fmt.Sprintf("%d", l) }

// Validate returns nil when the watch infrastructure error limit is positive.
func (l WatchInfraErrorLimit) Validate() error {
	if l <= 0 {
		return errors.New("watch infrastructure error limit must be positive")
	}
	return nil
}

// Error implements error.
func (e *InvalidWatchPlanError) Error() string {
	if e == nil || e.Err == nil {
		return "invalid watch plan"
	}
	return fmt.Sprintf("invalid watch plan: %v", e.Err)
}

// Unwrap returns the underlying watch-plan error.
func (e *InvalidWatchPlanError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewWatchPlan builds the app-owned watch plan for a resolved command.
func NewWatchPlan(cmdInfo *discovery.CommandInfo) (WatchPlan, error) {
	plan := WatchPlan{
		Patterns:             []invowkfile.GlobPattern{defaultWatchPattern},
		InfraErrorAbortLimit: defaultWatchInfraErrorLimit,
	}
	if cmdInfo == nil || cmdInfo.Command == nil {
		if err := plan.Validate(); err != nil {
			return WatchPlan{}, &InvalidWatchPlanError{Err: err}
		}
		return plan, nil
	}

	if watchCfg := cmdInfo.Command.Watch; watchCfg != nil {
		if len(watchCfg.Patterns) > 0 {
			plan.Patterns = append([]invowkfile.GlobPattern(nil), watchCfg.Patterns...)
		}
		plan.Ignore = append([]invowkfile.GlobPattern(nil), watchCfg.Ignore...)
		plan.ClearScreen = watchCfg.ClearScreen
		if watchCfg.Debounce != "" {
			debounce, err := watchCfg.ParseDebounce()
			if err != nil {
				return WatchPlan{}, &InvalidWatchPlanError{Err: err}
			}
			plan.Debounce = debounce
		}
	}

	baseDir := string(cmdInfo.Command.WorkDir)
	if baseDir != "" && !filepath.IsAbs(baseDir) {
		baseDir = filepath.Join(filepath.Dir(string(cmdInfo.FilePath)), baseDir)
	}
	plan.BaseDir = types.FilesystemPath(baseDir) //goplint:ignore -- from invowkfile directory resolution
	if err := plan.Validate(); err != nil {
		return WatchPlan{}, &InvalidWatchPlanError{Err: err}
	}
	return plan, nil
}

// Validate returns nil when the watch plan's typed fields are valid.
func (p WatchPlan) Validate() error {
	var errs []error
	for _, pattern := range p.Patterns {
		if err := pattern.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, ignore := range p.Ignore {
		if err := ignore.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.BaseDir != "" {
		if err := p.BaseDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := p.InfraErrorAbortLimit.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
