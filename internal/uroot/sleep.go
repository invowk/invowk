// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// sleepCommand implements the sleep utility.
// It pauses execution for a specified duration with context cancellation support.
type sleepCommand struct {
	name  string
	flags []FlagInfo
}

func init() {
	RegisterDefault(newSleepCommand())
}

// newSleepCommand creates a new sleep command.
func newSleepCommand() *sleepCommand {
	return &sleepCommand{
		name:  "sleep",
		flags: nil, // No flags, only positional arg
	}
}

// Name returns the command name.
func (c *sleepCommand) Name() string { return c.name }

// SupportedFlags returns the flags supported by this command.
func (c *sleepCommand) SupportedFlags() []FlagInfo { return c.flags }

// Run executes the sleep command.
// Usage: sleep DURATION
// Supports plain numbers (seconds), Ns (seconds), Nm (minutes), Nh (hours).
func (c *sleepCommand) Run(ctx context.Context, args []string) error {
	posArgs := args[1:]
	if len(posArgs) == 0 {
		return wrapError(c.name, fmt.Errorf("missing operand"))
	}

	duration, err := parseSleepDuration(posArgs[0])
	if err != nil {
		return wrapError(c.name, err)
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return wrapError(c.name, ctx.Err())
	case <-timer.C:
		return nil
	}
}

// parseSleepDuration parses a sleep duration string.
// Formats: "5" or "5s" (seconds), "5m" (minutes), "5h" (hours).
func parseSleepDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid time interval %q", s)
	}

	// Check for suffix
	suffix := strings.ToLower(s[len(s)-1:])
	switch suffix {
	case "s":
		val, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid time interval %q", s)
		}
		return time.Duration(val * float64(time.Second)), nil
	case "m":
		val, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid time interval %q", s)
		}
		return time.Duration(val * float64(time.Minute)), nil
	case "h":
		val, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid time interval %q", s)
		}
		return time.Duration(val * float64(time.Hour)), nil
	default:
		// Plain number treated as seconds
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid time interval %q", s)
		}
		return time.Duration(val * float64(time.Second)), nil
	}
}
