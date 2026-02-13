// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// seqCommand implements the seq utility.
// It generates a sequence of numbers with configurable start, increment, and separator.
type seqCommand struct {
	name  string
	flags []FlagInfo
}

func init() {
	RegisterDefault(newSeqCommand())
}

// newSeqCommand creates a new seq command.
func newSeqCommand() *seqCommand {
	return &seqCommand{
		name: "seq",
		flags: []FlagInfo{
			{Name: "s", Description: "separator string", TakesValue: true},
			{Name: "w", Description: "equalize width by padding with leading zeroes"},
		},
	}
}

// Name returns the command name.
func (c *seqCommand) Name() string { return c.name }

// SupportedFlags returns the flags supported by this command.
func (c *seqCommand) SupportedFlags() []FlagInfo { return c.flags }

// Run executes the seq command.
// Usage: seq [-w] [-s STRING] [FIRST [INCREMENT]] LAST
func (c *seqCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("seq", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	separator := fs.String("s", "\n", "separator")
	equalWidth := fs.Bool("w", false, "equal width")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	posArgs := fs.Args()
	if len(posArgs) == 0 {
		return wrapError(c.name, fmt.Errorf("missing operand"))
	}

	// Parse positional args: seq LAST, seq FIRST LAST, seq FIRST INCREMENT LAST
	var first, increment, last float64
	var parseErr error

	switch len(posArgs) {
	case 1:
		first = 1
		increment = 1
		last, parseErr = strconv.ParseFloat(posArgs[0], 64)
	case 2:
		first, parseErr = strconv.ParseFloat(posArgs[0], 64)
		if parseErr == nil {
			last, parseErr = strconv.ParseFloat(posArgs[1], 64)
		}
		increment = 1
	default: // 3 or more, use first 3
		first, parseErr = strconv.ParseFloat(posArgs[0], 64)
		if parseErr == nil {
			increment, parseErr = strconv.ParseFloat(posArgs[1], 64)
		}
		if parseErr == nil {
			last, parseErr = strconv.ParseFloat(posArgs[2], 64)
		}
	}

	if parseErr != nil {
		return wrapError(c.name, fmt.Errorf("invalid floating point argument: %w", parseErr))
	}

	if increment == 0 {
		return wrapError(c.name, fmt.Errorf("increment must not be zero"))
	}

	// Calculate width for -w padding by examining first and last values
	width := 0
	if *equalWidth {
		width = seqFormatWidth(first)
		if w := seqFormatWidth(last); w > width {
			width = w
		}
	}

	// Stream the sequence directly to stdout. This avoids unbounded memory
	// allocation for large ranges (e.g., "seq 1 1000000000" would OOM if
	// accumulated into a slice). The epsilon tolerance (1e-9) in the loop
	// bound compensates for floating point accumulation drift — without it,
	// sequences like "seq 0 0.1 1" miss the terminal value because repeated
	// 0.1 addition drifts past 1.0.
	count := 0
	for n := first; (increment > 0 && n <= last+1e-9) || (increment < 0 && n >= last-1e-9); n += increment {
		select {
		case <-ctx.Done():
			return wrapError(c.name, ctx.Err())
		default:
		}

		// Round to avoid floating point drift artifacts
		rounded := math.Round(n*1e9) / 1e9

		var formatted string
		if *equalWidth {
			formatted = seqFormatPadded(rounded, width)
		} else {
			formatted = seqFormat(rounded)
		}

		if count > 0 {
			fmt.Fprint(hc.Stdout, *separator)
		}
		fmt.Fprint(hc.Stdout, formatted)
		count++
	}
	if count > 0 {
		fmt.Fprintln(hc.Stdout)
	}

	return nil
}

// seqFormat formats a number, using integer format when the value is integral.
func seqFormat(n float64) string {
	if n == math.Trunc(n) && !math.IsInf(n, 0) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'g', -1, 64)
}

// seqFormatPadded formats a number with leading zeros to the given width.
func seqFormatPadded(n float64, width int) string {
	if n == math.Trunc(n) && !math.IsInf(n, 0) {
		return fmt.Sprintf("%0*d", width, int64(n))
	}
	s := strconv.FormatFloat(n, 'g', -1, 64)
	if len(s) < width {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return s
}

// seqFormatWidth returns the string length of a number formatted as integer.
func seqFormatWidth(n float64) int {
	if n == math.Trunc(n) && !math.IsInf(n, 0) {
		return len(strconv.FormatInt(int64(n), 10))
	}
	return len(strconv.FormatFloat(n, 'g', -1, 64))
}
