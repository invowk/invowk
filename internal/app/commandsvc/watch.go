// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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
	watchInfraErrorCount int

	// WatchInfraErrorLimit is the number of consecutive infrastructure errors
	// that cause watch mode to abort.
	WatchInfraErrorLimit int

	// WatchPlanOption configures watch plan construction.
	WatchPlanOption func(*watchPlanOptions)

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

	// WatchExecutionOutcome separates command exit status from execution
	// infrastructure failures during watch re-execution.
	//
	//goplint:ignore -- watch execution result is a small callback DTO with no constructor invariants.
	WatchExecutionOutcome struct {
		ExitCode types.ExitCode
		Err      error
	}

	// WatchExecutionFunc executes the watched command once.
	//
	//goplint:ignore -- filesystem watcher adapters provide changed paths as raw OS-native strings.
	WatchExecutionFunc func(context.Context, []string) WatchExecutionOutcome

	// WatchSession owns watch-mode re-execution policy; adapters own
	// filesystem watching and rendering.
	WatchSession struct {
		plan                   WatchPlan
		execute                WatchExecutionFunc
		consecutiveInfraErrors watchInfraErrorCount
	}

	// InvalidWatchPlanError reports invalid watch configuration on a command.
	InvalidWatchPlanError struct {
		Err error
	}

	watchPlanOptions struct {
		workdirOverride invowkfile.WorkDir
		selectedRuntime *invowkfile.RuntimeMode
		selectedImpl    *invowkfile.Implementation
	}
)

func (c watchInfraErrorCount) String() string { return fmt.Sprintf("%d", c) }

func (c watchInfraErrorCount) Validate() error {
	if c < 0 {
		return errors.New("watch infrastructure error count must not be negative")
	}
	return nil
}

// String returns the decimal string representation of the WatchInfraErrorLimit.
func (l WatchInfraErrorLimit) String() string { return fmt.Sprintf("%d", l) }

// Validate returns nil when the watch infrastructure error limit is positive.
func (l WatchInfraErrorLimit) Validate() error {
	if l <= 0 {
		return errors.New("watch infrastructure error limit must be positive")
	}
	return nil
}

// Validate returns nil when the watch execution outcome is well-formed.
func (o WatchExecutionOutcome) Validate() error {
	return o.ExitCode.Validate()
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

// WithWatchWorkdirOverride applies a request-level workdir override to the watch base directory.
func WithWatchWorkdirOverride(workdir invowkfile.WorkDir) WatchPlanOption {
	return func(opts *watchPlanOptions) {
		opts.workdirOverride = workdir
	}
}

// WithWatchExecution applies the selected execution runtime and implementation
// to host watch-root planning.
func WithWatchExecution(runtime invowkfile.RuntimeMode, impl *invowkfile.Implementation) WatchPlanOption {
	return func(opts *watchPlanOptions) {
		opts.selectedRuntime = &runtime
		opts.selectedImpl = impl
	}
}

// Validate returns nil when watch plan options contain valid typed fields.
func (o watchPlanOptions) Validate() error {
	if o.workdirOverride != "" {
		if err := o.workdirOverride.Validate(); err != nil {
			return err
		}
	}
	if o.selectedRuntime != nil {
		if err := o.selectedRuntime.Validate(); err != nil {
			return err
		}
	}
	if o.selectedImpl != nil {
		if err := o.selectedImpl.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// NewWatchPlan builds the app-owned watch plan for a resolved command.
func NewWatchPlan(cmdInfo *discovery.CommandInfo, opts ...WatchPlanOption) (WatchPlan, error) {
	options := watchPlanOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if err := options.Validate(); err != nil {
		return WatchPlan{}, &InvalidWatchPlanError{Err: err}
	}
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

	baseDir := watchBaseDir(cmdInfo, options)
	plan.BaseDir = baseDir
	if err := plan.Validate(); err != nil {
		return WatchPlan{}, &InvalidWatchPlanError{Err: err}
	}
	return plan, nil
}

func watchBaseDir(cmdInfo *discovery.CommandInfo, options watchPlanOptions) types.FilesystemPath {
	invowkfileDir := types.FilesystemPath(filepath.Dir(string(cmdInfo.FilePath))) //goplint:ignore -- derived from validated command source path.
	baseDir := types.FilesystemPath(cmdInfo.Command.WorkDir)
	if options.workdirOverride != "" {
		baseDir = types.FilesystemPath(options.workdirOverride)
	}
	if options.selectedImpl != nil && cmdInfo.Invowkfile != nil {
		effective := cmdInfo.Invowkfile.GetEffectiveWorkDir(cmdInfo.Command, options.selectedImpl, options.workdirOverride)
		baseDir = effective
	}
	if baseDir == "" {
		return ""
	}
	if options.selectedRuntime != nil && *options.selectedRuntime == invowkfile.RuntimeContainer && strings.HasPrefix(string(baseDir), "/") {
		if isPathInside(invowkfileDir, baseDir) {
			return baseDir
		}
		return invowkfileDir
	}
	if !filepath.IsAbs(string(baseDir)) {
		return types.FilesystemPath(filepath.Join(string(invowkfileDir), string(baseDir))) //goplint:ignore -- joined from validated command source and workdir paths.
	}
	return baseDir
}

func isPathInside(baseDir, candidate types.FilesystemPath) bool {
	rel, err := filepath.Rel(string(baseDir), string(candidate))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// NewWatchSession creates watch-mode execution policy for a validated plan.
func NewWatchSession(plan WatchPlan, execute WatchExecutionFunc) (*WatchSession, error) {
	if err := plan.Validate(); err != nil {
		return nil, &InvalidWatchPlanError{Err: err}
	}
	if execute == nil {
		return nil, errors.New("watch execution function is required")
	}
	session := &WatchSession{plan: plan, execute: execute}
	if err := session.Validate(); err != nil {
		return nil, err
	}
	return session, nil
}

// Validate returns nil when the session has executable watch policy.
func (s *WatchSession) Validate() error {
	if s == nil {
		return errors.New("watch session is required")
	}
	if err := s.plan.Validate(); err != nil {
		return err
	}
	if s.execute == nil {
		return errors.New("watch execution function is required")
	}
	return s.consecutiveInfraErrors.Validate()
}

// InitialExecution executes the watched command once before the filesystem
// watcher starts.
func (s *WatchSession) InitialExecution(ctx context.Context) (WatchExecutionOutcome, error) {
	outcome := s.execute(ctx, nil)
	if outcome.Err == nil && outcome.ExitCode == 0 {
		return outcome, nil
	}
	if ctx.Err() != nil {
		return outcome, fmt.Errorf("initial execution cancelled: %w", ctx.Err())
	}
	if outcome.Err != nil {
		return outcome, fmt.Errorf("cannot start watch mode: %w", outcome.Err)
	}
	return outcome, nil
}

// HandleChange applies watch-mode re-execution policy after a filesystem change.
//
//goplint:ignore -- filesystem watcher adapters provide changed paths as raw OS-native strings.
func (s *WatchSession) HandleChange(ctx context.Context, changed []string) (WatchExecutionOutcome, error) {
	outcome := s.execute(ctx, changed)
	if outcome.Err == nil && outcome.ExitCode == 0 {
		s.consecutiveInfraErrors = 0
		return outcome, nil
	}
	if ctx.Err() != nil {
		return outcome, fmt.Errorf("execution cancelled: %w", ctx.Err())
	}
	if outcome.Err == nil {
		s.consecutiveInfraErrors = 0
		return outcome, nil
	}

	s.consecutiveInfraErrors++
	if s.consecutiveInfraErrors >= watchInfraErrorCount(s.plan.InfraErrorAbortLimit) {
		return outcome, fmt.Errorf("aborting watch: %d consecutive infrastructure failures (last: %w)", s.consecutiveInfraErrors, outcome.Err)
	}
	return outcome, nil
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
