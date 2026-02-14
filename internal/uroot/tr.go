// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// trCommand implements the tr utility.
type trCommand struct {
	name  string
	flags []FlagInfo
}

// newTrCommand creates a new tr command.
func newTrCommand() *trCommand {
	return &trCommand{
		name: "tr",
		flags: []FlagInfo{
			{Name: "d", Description: "delete characters in SET1"},
			{Name: "s", Description: "squeeze repeated characters"},
			{Name: "c", Description: "use complement of SET1"},
			{Name: "C", Description: "use complement of SET1 (same as -c)"},
		},
	}
}

// Name returns the command name.
func (c *trCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *trCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the tr command.
func (c *trCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("tr", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	deleteMode := fs.Bool("d", false, "delete")
	squeezeMode := fs.Bool("s", false, "squeeze")
	complement := fs.Bool("c", false, "complement")
	complementC := fs.Bool("C", false, "complement")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	remaining := fs.Args()
	if len(remaining) == 0 {
		return wrapError(c.name, fmt.Errorf("missing operand"))
	}

	useComplement := *complement || *complementC
	set1 := expandSet(remaining[0])

	var set2 string
	if len(remaining) > 1 {
		set2 = expandSet(remaining[1])
	}

	// Validation
	if *deleteMode && !*squeezeMode && len(remaining) > 1 {
		// Delete mode with only set1
		set2 = ""
	}

	if !*deleteMode && !*squeezeMode && set2 == "" {
		return wrapError(c.name, fmt.Errorf("missing operand after '%s'", remaining[0]))
	}

	// Build translation table
	reader := bufio.NewReader(hc.Stdin)
	writer := bufio.NewWriter(hc.Stdout)

	var lastRune rune
	firstChar := true

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return wrapError(c.name, err)
		}

		inSet1 := strings.ContainsRune(set1, r)
		if useComplement {
			inSet1 = !inSet1
		}

		if *deleteMode {
			if inSet1 {
				continue // Delete the character
			}
			if *squeezeMode && strings.ContainsRune(set2, r) {
				if !firstChar && r == lastRune {
					continue // Squeeze
				}
			}
			if _, err := writer.WriteRune(r); err != nil {
				return wrapError(c.name, err)
			}
			lastRune = r
			firstChar = false
			continue
		}

		if *squeezeMode && set2 == "" {
			// Squeeze mode without translation
			if inSet1 {
				if !firstChar && r == lastRune {
					continue // Squeeze
				}
			}
			if _, err := writer.WriteRune(r); err != nil {
				return wrapError(c.name, err)
			}
			lastRune = r
			firstChar = false
			continue
		}

		// Translation mode
		outputRune := r
		if inSet1 {
			idx := strings.IndexRune(set1, r)
			if idx >= 0 && idx < len([]rune(set2)) {
				outputRune = []rune(set2)[idx]
			} else if set2 != "" {
				// If set2 is shorter, use last char of set2
				outputRune = []rune(set2)[len([]rune(set2))-1]
			}
		}

		if *squeezeMode {
			// Squeeze after translation
			toSqueeze := set2
			if toSqueeze == "" {
				toSqueeze = set1
			}
			if strings.ContainsRune(toSqueeze, outputRune) {
				if !firstChar && outputRune == lastRune {
					continue // Squeeze
				}
			}
		}

		if _, err := writer.WriteRune(outputRune); err != nil {
			return wrapError(c.name, err)
		}
		lastRune = outputRune
		firstChar = false
	}

	if err := writer.Flush(); err != nil {
		return wrapError(c.name, err)
	}
	return nil
}

// expandSet expands a character set specification.
// Handles ranges like a-z, A-Z, 0-9, and escape sequences.
func expandSet(s string) string {
	var result strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		if i+2 < len(runes) && runes[i+1] == '-' { //nolint:gocritic // ifElseChain: conditions check different patterns
			// Range: a-z
			start := runes[i]
			end := runes[i+2]
			if start <= end {
				for c := start; c <= end; c++ {
					result.WriteRune(c)
				}
			} else {
				for c := start; c >= end; c-- {
					result.WriteRune(c)
				}
			}
			i += 2
		} else if runes[i] == '\\' && i+1 < len(runes) {
			// Escape sequence
			i++
			switch runes[i] {
			case 'n':
				result.WriteRune('\n')
			case 't':
				result.WriteRune('\t')
			case 'r':
				result.WriteRune('\r')
			case '\\':
				result.WriteRune('\\')
			default:
				result.WriteRune(runes[i])
			}
		} else {
			result.WriteRune(runes[i])
		}
	}

	return result.String()
}
