// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// ExitCodeUsage is the process exit status for a rejected command line
	// (unknown command, flag or source, bad flag value, arity, ambiguity).
	// It follows the common convention of Go's flag package, POSIX utilities,
	// argparse and clap; the exit-code contract is documented in the CLI reference.
	ExitCodeUsage types.ExitCode = 2

	// defaultSuggestionsMinimumDistance mirrors Cobra's default Levenshtein
	// distance for "Did you mean this?" suggestions (cobra.Command.findSuggestions).
	defaultSuggestionsMinimumDistance = 2
)

// UsageError marks an error caused by the shape of the command line
// (unknown command, flag or source, bad flag value, arity, ambiguity).
// It changes only the process exit status (ExitCodeUsage); the message is the
// wrapped error's text, so rendering and fang's "Try --help" hint are unchanged.
// Always create via newUsageError, which keeps the invariant that a UsageError
// never wraps an ExitError (an explicit exit code always wins).
type UsageError struct {
	Err error
}

// Error returns the wrapped error's message unchanged.
func (e *UsageError) Error() string { return e.Err.Error() }

// Unwrap returns the wrapped error for errors.Is/As chains.
func (e *UsageError) Unwrap() error { return e.Err }

// newUsageError classifies err as a command-line rejection. It returns nil for
// nil, and returns err unchanged when it already carries an *ExitError, so the
// explicit exit code is never masked by the usage classification.
func newUsageError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*ExitError](err); ok { //nolint:errcheck // type match only; error is handled via ok
		return err
	}
	if _, ok := errors.AsType[*UsageError](err); ok { //nolint:errcheck // type match only; error is handled via ok
		return err
	}
	return &UsageError{Err: err}
}

// exitCodeForError maps the error returned by the Cobra tree to the process
// exit status: nil is 0, an *ExitError carries its own code, a *UsageError is
// ExitCodeUsage, and every other error is 1.
func exitCodeForError(err error) types.ExitCode {
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*ExitError](err); ok {
		return exitErr.Code
	}
	if _, ok := errors.AsType[*UsageError](err); ok { //nolint:errcheck // type match only; error is handled via ok
		return ExitCodeUsage
	}
	return 1
}

// installUsageGuards wires usage-error classification into the whole Cobra
// tree rooted at root. It must run after every subcommand has been added.
//   - pflag parse errors at any level become UsageError (children inherit the
//     root FlagErrorFunc through Cobra's parent walk);
//   - an unknown root command becomes a UsageError with Cobra's message, while
//     bare `invowk` still shows help;
//   - every group without an explicit Args validator rejects unmatched tokens
//     through rejectUnmatchedGroupToken, and a bare group still shows help;
//   - every explicit Args validator's error becomes a UsageError.
//
// A parent that really accepts positionals MUST declare Args explicitly
// (for example `invowk cmd` and discovered group nodes use cobra.ArbitraryArgs).
func installUsageGuards(root *cobra.Command) {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return newUsageError(err)
	})

	root.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return newUsageError(unknownCommandError(cmd, args[0]))
	}
	if !root.Runnable() {
		root.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	}

	for _, child := range root.Commands() {
		installGroupGuards(child)
	}
}

// installGroupGuards applies the group and Args rules of installUsageGuards to
// cmd and its descendants.
func installGroupGuards(cmd *cobra.Command) {
	switch {
	case cmd.Args != nil:
		cmd.Args = wrapArgsValidator(cmd.Args)
	case cmd.HasSubCommands():
		cmd.Args = rejectUnmatchedGroupToken
		if !cmd.Runnable() {
			// A bare group is a request to browse: print help and succeed.
			cmd.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
		}
	}

	for _, child := range cmd.Commands() {
		installGroupGuards(child)
	}
}

// wrapArgsValidator classifies the errors of an explicit Cobra Args validator
// as usage errors, keeping their messages unchanged.
func wrapArgsValidator(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return newUsageError(validate(cmd, args))
	}
}

// rejectUnmatchedGroupToken rejects positional tokens given to a command group
// that did not resolve to a subcommand. It writes the group's visible
// subcommands to stderr (stdout stays empty) and returns a UsageError with
// Cobra's unknown-command message and "Did you mean this?" suggestions.
// With no tokens it returns nil, so a bare group still shows help.
//
//goplint:ignore -- raw positional tokens from the Cobra boundary (cobra.PositionalArgs signature).
func rejectUnmatchedGroupToken(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	writeAvailableSubcommands(cmd.ErrOrStderr(), cmd)
	return newUsageError(unknownCommandError(cmd, args[0]))
}

// writeAvailableSubcommands lists cmd's visible subcommands with their short
// descriptions, in Cobra's sorted order.
//
//plint:render
func writeAvailableSubcommands(w io.Writer, cmd *cobra.Command) {
	children := cmd.Commands()
	width := 0
	for _, child := range children {
		if child.IsAvailableCommand() {
			width = max(width, len(child.Name()))
		}
	}
	if width == 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString("Available commands:\n")
	for _, child := range children {
		if !child.IsAvailableCommand() {
			continue
		}
		fmt.Fprintf(&sb, "  %-*s  %s\n", width, child.Name(), child.Short)
	}
	sb.WriteString("\n")
	fmt.Fprint(w, sb.String())
}

// unknownCommandError reproduces Cobra's unknown-command message (legacyArgs),
// including its "Did you mean this?" suffix.
//
//goplint:ignore -- raw positional token from the Cobra boundary.
func unknownCommandError(cmd *cobra.Command, token string) error {
	return fmt.Errorf("unknown command %q for %q%s", token, cmd.CommandPath(), commandSuggestions(cmd, token))
}

// commandSuggestions mirrors Cobra's unexported findSuggestions so built-in
// and discovered groups format suggestions exactly like the root command.
//
//plint:render
//goplint:ignore -- raw positional token from the Cobra boundary.
func commandSuggestions(cmd *cobra.Command, token string) string {
	if cmd.DisableSuggestions {
		return ""
	}
	if cmd.SuggestionsMinimumDistance <= 0 {
		cmd.SuggestionsMinimumDistance = defaultSuggestionsMinimumDistance
	}
	suggestions := cmd.SuggestionsFor(token)
	if len(suggestions) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\nDid you mean this?\n")
	for _, s := range suggestions {
		fmt.Fprintf(&sb, "\t%v\n", s)
	}
	return sb.String()
}
