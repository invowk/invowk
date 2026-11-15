// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkmod"
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

type (
	// UsageError marks an error caused by the shape of the command line
	// (unknown command, flag or source, bad flag value, arity, ambiguity).
	// It changes only the process exit status (ExitCodeUsage); the message is the
	// wrapped error's text, so rendering and fang's "Try --help" hint are unchanged.
	// Always create via newUsageError, which keeps the invariant that a UsageError
	// never wraps an ExitError (an explicit exit code always wins).
	UsageError struct {
		Err error
	}

	// SkippedSourcesError is a "command not found" or "source not found" error
	// raised while discovery skipped sources that failed to parse or load and
	// that may have defined the request. It keeps exit status 1 (see
	// notFoundError), and its message names each skipped source and the
	// diagnostic code, so a wrapper can tell why the status is not ExitCodeUsage.
	SkippedSourcesError struct {
		Err     error
		Skipped []discovery.Diagnostic
	}
)

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

// Error returns the not-found message followed by the skipped sources.
func (e *SkippedSourcesError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Err.Error())
	sb.WriteString("; discovery skipped sources that failed to load:")
	for i, diag := range e.Skipped {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, " %s (%s)", skippedSourceLabel(diag), diag.Code())
	}
	return sb.String()
}

// Unwrap returns the not-found error for errors.Is/As chains.
func (e *SkippedSourcesError) Unwrap() error { return e.Err }

// notFoundError classifies a "command not found" or "source not found" error.
// It is a UsageError (exit status ExitCodeUsage) unless discovery reported an
// invowkfile or module that failed to parse or load and that may be the
// requested source: the command may be defined in that file, so the failure is
// not attributed to the command line and keeps exit status 1, like any other
// invalid invowkfile. scope is the request's @source/--ivk-from (empty when
// unscoped). Unscoped, any such skipped source counts, because which file held
// the command cannot be known; scoped, only a skipped source that may carry
// that source ID counts (mayBeSkippedSource). err may already be a UsageError
// (rejectUnmatchedGroupToken); that classification is removed.
func notFoundError(err error, diags []discovery.Diagnostic, scope discovery.SourceID) error {
	var skipped []discovery.Diagnostic
	for _, diag := range diags {
		if isSourceLoadFailure(diag) && mayBeSkippedSource(diag, scope) {
			skipped = append(skipped, diag)
		}
	}
	if len(skipped) == 0 {
		return newUsageError(err)
	}
	if usageErr, ok := errors.AsType[*UsageError](err); ok {
		err = usageErr.Err
	}
	return &SkippedSourcesError{Err: err, Skipped: skipped}
}

// mayBeSkippedSource reports whether the skipped source of diag could be the
// source scope names; an empty scope matches every skipped source. Only the
// default namespaces are derived from the path: a root invowkfile is
// `invowkfile`, and a module directory found by the module scan is its
// directory name. Included and vendored modules can be renamed by an alias or
// a requires namespace, a module's own invowkfile can belong to either, and a
// diagnostic without a path names no source, so those always match.
func mayBeSkippedSource(diag discovery.Diagnostic, scope discovery.SourceID) bool {
	if scope == "" || diag.Path() == "" {
		return true
	}
	if diag.Code() == discovery.CodeInvowkfileParseSkipped {
		if strings.HasSuffix(filepath.Dir(string(diag.Path())), invowkmod.ModuleSuffix) {
			return true
		}
		return scope == discovery.SourceIDInvowkfile
	}
	if diag.Code() == discovery.CodeModuleLoadSkipped {
		return discovery.SourceID(invowkmod.CommandSourceIDFromModulePath(diag.Path())) == scope
	}
	return true
}

// skippedSourceLabel names a skipped source by the last element of its path
// (its module directory, or the invowkfile and the module that holds it). The
// discovery warning printed before the error carries the full path.
//
//plint:render
func skippedSourceLabel(diag discovery.Diagnostic) string {
	path := string(diag.Path())
	if path == "" {
		return "an unnamed source"
	}
	base := filepath.Base(path)
	if parent := filepath.Base(filepath.Dir(path)); strings.HasSuffix(parent, invowkmod.ModuleSuffix) {
		return parent + "/" + base
	}
	return base
}

// isSourceLoadFailure reports whether diag means that discovery found an
// invowkfile or module but skipped it because it failed to parse or load.
// Policy skips (reserved names, symlinks, undeclared vendored modules) do not
// count: those sources are excluded on purpose, not broken.
func isSourceLoadFailure(diag discovery.Diagnostic) bool {
	return slices.Contains([]discovery.DiagnosticCode{
		discovery.CodeInvowkfileParseSkipped,
		discovery.CodeModuleLoadSkipped,
		discovery.CodeIncludeModuleLoadFailed,
		discovery.CodeVendoredModuleLoadSkipped,
	}, diag.Code())
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
//   - bare `invowk` still shows help; an unknown root command keeps Cobra's
//     own check (classified by classifyUnknownRootCommand);
//   - every group without an explicit Args validator rejects unmatched tokens
//     through rejectUnmatchedGroupToken, and a bare group still shows help;
//   - every explicit Args validator's error becomes a UsageError;
//   - on every non-root command, Cobra's required-flag and flag-group checks
//     run inside the Args validator so their errors also become UsageError
//     (Cobra runs them right after Args, outside FlagErrorFunc).
//
// A parent that really accepts positionals MUST declare Args explicitly
// (for example `invowk cmd` and discovered group nodes use cobra.ArbitraryArgs).
func installUsageGuards(root *cobra.Command) {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return newUsageError(err)
	})

	// root.Args stays nil on purpose: Cobra then rejects an unknown first
	// token inside Command.Find, before it handles --help and --version, so
	// `invowk <typo> --help` fails instead of printing the root help.
	if !root.Runnable() {
		root.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	}

	for _, child := range root.Commands() {
		installGroupGuards(child)
	}
}

// classifyUnknownRootCommand classifies Cobra's unknown-root-command error,
// which Command.Find returns before any Args validator or FlagErrorFunc runs,
// as a UsageError. args must be the argv the tree was executed with. Find is
// run again after execution, when Cobra's lazily added help and completion
// commands exist, so it fails exactly when the execution failed in Find.
//
//goplint:ignore -- raw process argv at the Cobra boundary.
func classifyUnknownRootCommand(root *cobra.Command, args []string, err error) error {
	if err == nil || exitCodeForError(err) != 1 {
		return err
	}
	if _, _, findErr := root.Find(args); findErr == nil {
		return err
	}
	return newUsageError(err)
}

// installGroupGuards applies the group and Args rules of installUsageGuards to
// cmd and its descendants.
func installGroupGuards(cmd *cobra.Command) {
	validate := cmd.Args
	if validate == nil && cmd.HasSubCommands() {
		validate = rejectUnmatchedGroupToken
		if !cmd.Runnable() {
			// A bare group is a request to browse: print help and succeed.
			cmd.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
		}
	}
	cmd.Args = usageArgsValidator(validate)

	for _, child := range cmd.Commands() {
		installGroupGuards(child)
	}
}

// usageArgsValidator classifies the errors of a Cobra Args validator (nil
// accepts any tokens, like Cobra's default for subcommands) and of Cobra's
// required-flag and flag-group checks as usage errors, keeping their messages
// unchanged. Cobra repeats those checks after Args; they pass by then.
func usageArgsValidator(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if validate != nil {
			if err := validate(cmd, args); err != nil {
				return newUsageError(err)
			}
		}
		if err := cmd.ValidateRequiredFlags(); err != nil {
			return newUsageError(err)
		}
		return newUsageError(cmd.ValidateFlagGroups())
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
