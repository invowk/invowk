// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"invowk-cli/internal/discovery"
	"invowk-cli/internal/sshserver"
	"invowk-cli/pkg/invkfile"

	"github.com/spf13/cobra"
)

const (
	// ArgErrMissingRequired indicates missing required arguments
	ArgErrMissingRequired = iota
	// ArgErrTooMany indicates too many arguments were provided
	ArgErrTooMany
	// ArgErrInvalidValue indicates an argument value failed validation
	ArgErrInvalidValue
)

var (
	// runtimeOverride allows overriding the runtime for a command
	runtimeOverride string
	// fromSource allows specifying the source for disambiguation
	fromSource string
	// forceRebuild forces rebuilding of container images, bypassing cache
	forceRebuild bool
	// sshServerInstance is the global SSH server instance
	sshServerInstance *sshserver.Server
	// sshServerMu protects the SSH server instance
	sshServerMu sync.Mutex
	// listFlag controls whether to list commands
	listFlag bool
	// cmdCmd is the parent command for all discovered commands
	cmdCmd = &cobra.Command{
		Use:   "cmd [command-name]",
		Short: "Execute commands from invkfiles",
		Long: `Execute commands defined in invkfiles and sibling modules.

Commands are discovered from:
  1. Current directory's invkfile.cue (highest priority)
  2. Sibling *.invkmod module directories
  3. ~/.invowk/cmds/
  4. Configured search paths

Commands use their simple names when unique across sources. When a command
name exists in multiple sources, disambiguation is required.

Usage:
  invowk cmd                              List all available commands
  invowk cmd <command-name>               Execute a command (if unambiguous)
  invowk cmd @<source> <command-name>     Disambiguate with @source prefix
  invowk cmd --from <source> <command-name>  Disambiguate with --from flag

Examples:
  invowk cmd build                        Run unique 'build' command
  invowk cmd @invkfile deploy             Run 'deploy' from invkfile
  invowk cmd @foo deploy                  Run 'deploy' from foo.invkmod
  invowk cmd --from invkfile deploy       Same using --from flag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --list flag is set or no arguments, show list
			if listFlag || len(args) == 0 {
				return listCommands()
			}

			// Parse source filter from @prefix or --from flag
			filter, remainingArgs, err := ParseSourceFilter(args, fromSource)
			if err != nil {
				return err
			}

			// If we have a source filter, try to run the disambiguated command
			if filter != nil {
				return runDisambiguatedCommand(filter, remainingArgs)
			}

			// Check if the command is ambiguous (exists in multiple sources)
			// This handles the case where user types `invowk cmd deploy` and deploy is ambiguous.
			// For subcommands like `invowk cmd deploy staging`, we check progressively longer
			// command names to detect ambiguity at the correct hierarchical level.
			if len(args) > 0 {
				if ambigCheckErr := checkAmbiguousCommand(args); ambigCheckErr != nil {
					ambigErr := (*AmbiguousCommandError)(nil)
					if errors.As(ambigCheckErr, &ambigErr) {
						fmt.Fprint(os.Stderr, RenderAmbiguousCommandError(ambigErr))
						cmd.SilenceErrors = true
						cmd.SilenceUsage = true
					}
					return ambigCheckErr
				}
			}

			// No disambiguation needed, run normally (Cobra will handle registered commands)
			err = runCommand(args)
			if err != nil {
				exitErr := (*ExitError)(nil)
				if errors.As(err, &exitErr) {
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
				}
			}
			return err
		},
		ValidArgsFunction: completeCommands,
	}
)

type (
	// DependencyError represents unsatisfied dependencies
	DependencyError struct {
		CommandName         string
		MissingTools        []string
		MissingCommands     []string
		MissingFilepaths    []string
		MissingCapabilities []string
		FailedCustomChecks  []string
		MissingEnvVars      []string
	}

	// ArgErrType represents the type of argument validation error
	ArgErrType int

	// ArgumentValidationError represents an argument validation failure
	ArgumentValidationError struct {
		Type         ArgErrType
		CommandName  string
		ArgDefs      []invkfile.Argument
		ProvidedArgs []string
		MinArgs      int
		MaxArgs      int
		InvalidArg   string
		InvalidValue string
		ValueError   error
	}

	// SourceFilter represents a user-specified source constraint for disambiguation.
	// It is used to filter commands to a specific source when executing ambiguous commands.
	SourceFilter struct {
		// SourceID is the normalized source name (e.g., "foo" not "foo.invkmod")
		SourceID string
		// Raw is the original input (e.g., "@foo.invkmod" or "foo" from --from)
		Raw string
	}

	// SourceNotFoundError is returned when a specified source does not exist.
	SourceNotFoundError struct {
		Source           string
		AvailableSources []string
	}

	// AmbiguousCommandError is returned when trying to execute a command that exists
	// in multiple sources without explicit disambiguation.
	AmbiguousCommandError struct {
		CommandName string
		Sources     []string // SourceIDs where the command exists
	}
)

func init() {
	cmdCmd.Flags().BoolVarP(&listFlag, "list", "l", false, "list all available commands")
	cmdCmd.PersistentFlags().StringVarP(&runtimeOverride, "runtime", "r", "", "override the runtime (must be allowed by the command)")
	cmdCmd.PersistentFlags().StringVar(&fromSource, "from", "", "source to run command from (e.g., 'invkfile' or module name)")
	cmdCmd.PersistentFlags().BoolVar(&forceRebuild, "force-rebuild", false, "force rebuild of container images (container runtime only)")

	// Dynamically add discovered commands
	// This happens at init time to enable shell completion
	registerDiscoveredCommands()
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependencies not satisfied for command '%s'", e.CommandName)
}

func (e *ArgumentValidationError) Error() string {
	switch e.Type {
	case ArgErrMissingRequired:
		return fmt.Sprintf("missing required arguments for command '%s': expected at least %d, got %d", e.CommandName, e.MinArgs, len(e.ProvidedArgs))
	case ArgErrTooMany:
		return fmt.Sprintf("too many arguments for command '%s': expected at most %d, got %d", e.CommandName, e.MaxArgs, len(e.ProvidedArgs))
	case ArgErrInvalidValue:
		return fmt.Sprintf("invalid value for argument '%s': %v", e.InvalidArg, e.ValueError)
	default:
		return fmt.Sprintf("argument validation failed for command '%s'", e.CommandName)
	}
}

func (e *SourceNotFoundError) Error() string {
	return fmt.Sprintf("source '%s' not found", e.Source)
}

func (e *AmbiguousCommandError) Error() string {
	return fmt.Sprintf("command '%s' is ambiguous", e.CommandName)
}

// normalizeSourceName converts various source name formats to a canonical form.
// It accepts: "foo", "foo.invkmod", "invkfile", "invkfile.cue"
// And returns: "foo" or "invkfile"
func normalizeSourceName(raw string) string {
	// Remove @ prefix if present
	name := strings.TrimPrefix(raw, "@")

	// Handle invkfile variants
	if name == "invkfile.cue" || name == discovery.SourceIDInvkfile {
		return discovery.SourceIDInvkfile
	}

	// Handle module variants - strip .invkmod suffix
	if moduleName, found := strings.CutSuffix(name, ".invkmod"); found {
		return moduleName
	}

	return name
}

// ParseSourceFilter extracts source filter from @prefix in args or --from flag.
// Returns the filter (may be nil if no filter specified), remaining args, and any error.
// The @source prefix must be the first argument if present.
func ParseSourceFilter(args []string, fromFlag string) (*SourceFilter, []string, error) {
	// --from flag takes precedence (already parsed by Cobra)
	if fromFlag != "" {
		return &SourceFilter{
			SourceID: normalizeSourceName(fromFlag),
			Raw:      fromFlag,
		}, args, nil
	}

	// Check for @source prefix in first arg
	if len(args) > 0 && strings.HasPrefix(args[0], "@") {
		raw := args[0]
		sourceID := normalizeSourceName(raw)
		return &SourceFilter{
			SourceID: sourceID,
			Raw:      raw,
		}, args[1:], nil
	}

	// No filter specified
	return nil, args, nil
}
