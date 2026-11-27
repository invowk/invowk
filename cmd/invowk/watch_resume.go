// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
)

const (
	// Protocol limits enforced by terminals accepting OSC 7577 (matching x/ansi).
	watchResumeMaxBytes = 2048
	watchResumeMaxArgs  = 64
)

type (
	watchResumeOptions struct {
		rawArgs    []string
		executable func() (string, error)
		getwd      func() (string, error)
		getpid     func() int
		now        func() time.Time
	}

	watchResumeReporter struct {
		enabled     bool
		reported    bool
		token       string
		pid         int
		writer      io.Writer
		cleanup     func()
		closeOnce   sync.Once
		lastReport  time.Time
		now         func() time.Time
		buildResume func() ansi.ProgramResume
	}
)

func newWatchResumeReporter(
	ctx context.Context,
	app *App,
	cmd *cobra.Command,
	rootFlags *rootFlagValues,
	opts watchResumeOptions,
) *watchResumeReporter {
	opts = normalizeWatchResumeOptions(opts)
	if hasWatchEnvVarOverride(cmd, opts) {
		return &watchResumeReporter{}
	}

	getenv := appGetenv(app)
	token := getenv(ansi.ProgramResumeTokenEnv)
	if token == "" {
		return &watchResumeReporter{}
	}

	writer, cleanup, ok := openAppWatchTerminal(app)
	if !ok || writer == nil {
		if cleanup != nil {
			cleanup()
		}
		return &watchResumeReporter{}
	}

	pid := opts.getpid()
	if pid <= 0 {
		if cleanup != nil {
			cleanup()
		}
		return &watchResumeReporter{}
	}

	var configPath string
	var noCwdFlag bool
	if rootFlags != nil {
		configPath = rootFlags.configPath
		noCwdFlag = rootFlags.noCwd
	}

	var configFromEnv string
	if configPath == "" {
		configFromEnv = getenv(config.EnvConfigPath)
	}
	materializeNoCwd := noCwdFromContext(ctx) && !noCwdFlag

	rawArgs := slices.Clone(opts.rawArgs)
	return &watchResumeReporter{
		enabled: true,
		token:   token,
		pid:     pid,
		writer:  writer,
		cleanup: cleanup,
		now:     opts.now,
		buildResume: func() ansi.ProgramResume {
			exePath, err := opts.executable()
			if err != nil || exePath == "" {
				return ansi.ProgramResume{}
			}
			cwd, err := opts.getwd()
			if err != nil || cwd == "" {
				return ansi.ProgramResume{}
			}
			argv := buildWatchResumeArgs(rawArgs, exePath, configPath, configFromEnv, materializeNoCwd)
			if len(argv) == 0 {
				return ansi.ProgramResume{}
			}
			return ansi.ProgramResume{Args: argv, Dir: cwd}
		},
	}
}

func (r *watchResumeReporter) Report() {
	if r == nil || !r.enabled {
		return
	}

	pr := r.buildResume()
	if len(pr.Args) == 0 || len(pr.Args) > watchResumeMaxArgs {
		if r.reported {
			r.Clear()
		}
		return
	}

	now := r.nextTimestamp()
	seq := ansi.SetProgramResume(r.token, r.pid, now, pr)
	if seq == "" || len(seq) > watchResumeMaxBytes || strings.Contains(seq, ":clear=1:") {
		if r.reported {
			r.Clear()
		}
		return
	}

	if _, err := io.WriteString(r.writer, seq); err == nil {
		r.reported = true
		r.lastReport = now
	}
}

func (r *watchResumeReporter) Clear() {
	if r == nil || !r.enabled || !r.reported {
		return
	}

	now := r.nextTimestamp()
	seq := ansi.ClearProgramResume(r.token, r.pid, now)
	if seq != "" {
		if _, err := io.WriteString(r.writer, seq); err == nil {
			r.lastReport = now
		}
	}
	r.reported = false
}

func (r *watchResumeReporter) Close() {
	if r == nil || r.cleanup == nil {
		return
	}
	r.closeOnce.Do(r.cleanup)
}

func (r *watchResumeReporter) nextTimestamp() time.Time {
	now := r.now()
	if !r.lastReport.IsZero() && now.Unix() < r.lastReport.Unix() {
		return r.lastReport
	}
	return now
}

func normalizeWatchResumeOptions(opts watchResumeOptions) watchResumeOptions {
	if opts.rawArgs == nil {
		if len(os.Args) > 1 {
			opts.rawArgs = os.Args[1:]
		} else {
			opts.rawArgs = []string{}
		}
	}
	if opts.executable == nil {
		opts.executable = os.Executable
	}
	if opts.getwd == nil {
		opts.getwd = os.Getwd
	}
	if opts.getpid == nil {
		opts.getpid = os.Getpid
	}
	if opts.now == nil {
		opts.now = time.Now
	}
	return opts
}

func appGetenv(app *App) func(string) string {
	if app != nil && app.Getenv != nil {
		return app.Getenv
	}
	return os.Getenv
}

func openAppWatchTerminal(app *App) (io.Writer, func(), bool) {
	if app == nil {
		return nil, func() {}, false
	}
	if app.WatchTerminal != nil {
		return app.WatchTerminal.OpenWatchTerminal()
	}
	return defaultWatchTerminalOpener{stdout: app.stdout}.OpenWatchTerminal()
}

func hasWatchEnvVarOverride(cmd *cobra.Command, opts watchResumeOptions) bool {
	if cmd != nil {
		if len(stringArrayFlagValue(cmd, "ivk-env-var")) > 0 {
			return true
		}
		if flag := cmd.Flags().Lookup("ivk-env-var"); flag != nil && cmd.Flags().Changed("ivk-env-var") {
			return true
		}
	}
	for _, arg := range opts.rawArgs {
		if arg == "--" {
			break
		}
		if arg == "--ivk-env-var" || arg == "-E" ||
			strings.HasPrefix(arg, "--ivk-env-var=") || strings.HasPrefix(arg, "-E=") {
			return true
		}
	}
	return false
}

func buildWatchResumeArgs(rawArgs []string, exePath, configPath, configFromEnv string, materializeNoCwd bool) []string {
	if exePath == "" {
		return nil
	}

	cmdIdx := -1
	dashIdx := -1
	hasConfig := configPath != "" || configFromEnv == ""
	hasNoCwd := !materializeNoCwd
	skipNext := false
	for i, arg := range rawArgs {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--" {
			dashIdx = i
			break
		}
		if cmdIdx < 0 && arg == "cmd" {
			cmdIdx = i
		}
		switch {
		case arg == "--ivk-config" || arg == "-c":
			hasConfig = true
			skipNext = true
		case strings.HasPrefix(arg, "--ivk-config=") || (strings.HasPrefix(arg, "-c") && len(arg) > 2):
			hasConfig = true
		case arg == longNoCwdFlag || strings.HasPrefix(arg, longNoCwdFlagInline):
			hasNoCwd = true
		}
	}

	extra := make([]string, 0, 2)
	if !hasConfig {
		extra = append(extra, "--ivk-config="+configFromEnv)
	}
	if !hasNoCwd {
		extra = append(extra, longNoCwdFlag)
	}

	args := make([]string, 0, 1+len(rawArgs)+len(extra))
	args = append(args, exePath)
	if len(extra) == 0 {
		return append(args, rawArgs...)
	}

	insertAt := 0
	if cmdIdx >= 0 {
		insertAt = cmdIdx + 1
	} else if dashIdx >= 0 {
		insertAt = dashIdx
	}
	args = append(args, rawArgs[:insertAt]...)
	args = append(args, extra...)
	args = append(args, rawArgs[insertAt:]...)
	return args
}

func isInteractiveWatchRun(cmd *cobra.Command, rootFlags *rootFlagValues) bool {
	if rootFlags != nil && rootFlags.interactive {
		return true
	}
	if cmd != nil {
		if v, err := cmd.Flags().GetBool("ivk-interactive"); err == nil && v {
			return true
		}
	}
	return false
}
