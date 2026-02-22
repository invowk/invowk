// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/discovery"

	"github.com/charmbracelet/lipgloss"
)

// RenderArgumentValidationError creates a styled error message for argument validation failures
func RenderArgumentValidationError(err *ArgumentValidationError) string {
	var sb strings.Builder

	switch err.Type {
	case ArgErrMissingRequired:
		sb.WriteString(renderHeaderStyle.Render("✗ Missing required arguments!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s requires at least %d argument(s), but got %d.\n\n",
			renderCommandStyle.Render("'"+err.CommandName+"'"), err.MinArgs, len(err.ProvidedArgs)))

		sb.WriteString(renderLabelStyle.Render("Expected arguments:"))
		sb.WriteString("\n")
		for _, arg := range err.ArgDefs {
			var reqStr string
			switch {
			case arg.Required:
				reqStr = " (required)"
			case arg.DefaultValue != "":
				reqStr = fmt.Sprintf(" (default: %q)", arg.DefaultValue)
			default:
				reqStr = " (optional)"
			}
			sb.WriteString(renderValueStyle.Render(fmt.Sprintf("  • %s%s - %s\n", arg.Name, reqStr, arg.Description)))
		}

	case ArgErrTooMany:
		sb.WriteString(renderHeaderStyle.Render("✗ Too many arguments!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s accepts at most %d argument(s), but got %d.\n\n",
			renderCommandStyle.Render("'"+err.CommandName+"'"), err.MaxArgs, len(err.ProvidedArgs)))

		sb.WriteString(renderLabelStyle.Render("Expected arguments:"))
		sb.WriteString("\n")
		for _, arg := range err.ArgDefs {
			sb.WriteString(renderValueStyle.Render(fmt.Sprintf("  • %s - %s\n", arg.Name, arg.Description)))
		}
		sb.WriteString("\n")
		sb.WriteString(renderLabelStyle.Render("Provided:"))
		sb.WriteString(renderValueStyle.Render(fmt.Sprintf(" %v", err.ProvidedArgs)))

	case ArgErrInvalidValue:
		sb.WriteString(renderHeaderStyle.Render("✗ Invalid argument value!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s received an invalid value for argument %s.\n\n",
			renderCommandStyle.Render("'"+err.CommandName+"'"), renderCommandStyle.Render("'"+err.InvalidArg+"'")))

		sb.WriteString(renderLabelStyle.Render("Value:  "))
		sb.WriteString(renderValueStyle.Render(fmt.Sprintf("%q", err.InvalidValue)))
		sb.WriteString("\n")
		sb.WriteString(renderLabelStyle.Render("Error:  "))
		sb.WriteString(renderValueStyle.Render(err.ValueError.Error()))
	}

	sb.WriteString("\n\n")
	sb.WriteString(renderHintStyle.Render("Run the command with --help for usage information."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderArgsSubcommandConflictError creates a styled error message when a command
// has both positional arguments and subcommands defined. This is a structural error
// because positional arguments can only be accepted by leaf commands.
func RenderArgsSubcommandConflictError(err *discovery.ArgsSubcommandConflictError) string {
	var sb strings.Builder

	// pathStyle is unique to this render function (not shared across all cards)
	pathStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Italic(true)

	sb.WriteString(renderHeaderStyle.Render("✗ Invalid command structure!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Command %s defines positional arguments but also has subcommands.\n",
		renderCommandStyle.Render("'"+err.CommandName+"'")))
	if err.FilePath != "" {
		sb.WriteString(pathStyle.Render(fmt.Sprintf("  in %s\n", err.FilePath)))
	}
	sb.WriteString("\nPositional arguments can only be defined on leaf commands (commands without subcommands).\n\n")

	sb.WriteString(renderLabelStyle.Render("Defined args:"))
	sb.WriteString("\n")
	for _, arg := range err.Args {
		sb.WriteString(renderValueStyle.Render(fmt.Sprintf("  • %s - %s\n", arg.Name, arg.Description)))
	}

	sb.WriteString("\n")
	sb.WriteString(renderLabelStyle.Render("Subcommands:"))
	sb.WriteString("\n")
	for _, subcmd := range err.Subcommands {
		sb.WriteString(renderValueStyle.Render(fmt.Sprintf("  • %s\n", subcmd)))
	}

	sb.WriteString("\n")
	sb.WriteString(renderHintStyle.Render("Remove either the 'args' field or the subcommands to resolve this conflict."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderDependencyError creates a styled error message for unsatisfied dependencies
func RenderDependencyError(err *DependencyError) string {
	var sb strings.Builder

	// sectionStyle adds MarginTop for spacing between dependency groups
	sectionStyle := renderLabelStyle.MarginTop(1)

	sb.WriteString(renderHeaderStyle.Render("✗ Dependencies not satisfied!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s because some dependencies are missing.\n", renderCommandStyle.Render("'"+err.CommandName+"'")))

	renderSection := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render(label))
		sb.WriteString("\n")
		for _, item := range items {
			sb.WriteString(renderValueStyle.Render(item))
			sb.WriteString("\n")
		}
	}

	renderSection("Missing Tools:", err.MissingTools)
	renderSection("Missing Commands:", err.MissingCommands)
	renderSection("Missing or Inaccessible Files:", err.MissingFilepaths)
	renderSection("Missing Capabilities:", err.MissingCapabilities)
	renderSection("Failed Custom Checks:", err.FailedCustomChecks)
	renderSection("Missing or Invalid Environment Variables:", err.MissingEnvVars)

	sb.WriteString("\n")
	sb.WriteString(renderHintStyle.Render("Fix the missing dependencies and try again, or update your invowkfile to remove unnecessary ones."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderHostNotSupportedError creates a styled error message for unsupported host OS
func RenderHostNotSupportedError(cmdName, currentOS, supportedHosts string) string {
	var sb strings.Builder

	sb.WriteString(renderHeaderStyle.Render("✗ Host not supported!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s on this operating system.\n\n", renderCommandStyle.Render("'"+cmdName+"'")))
	sb.WriteString(renderLabelStyle.Render("Current host:    "))
	sb.WriteString(renderValueStyle.Render(currentOS))
	sb.WriteString("\n")
	sb.WriteString(renderLabelStyle.Render("Supported hosts: "))
	sb.WriteString(renderValueStyle.Render(supportedHosts))
	sb.WriteString("\n\n")
	sb.WriteString(renderHintStyle.Render("Run this command on a supported operating system, or update the 'works_on.hosts' setting in your invowkfile."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderRuntimeNotAllowedError creates a styled error message for invalid runtime selection
func RenderRuntimeNotAllowedError(cmdName, selectedRuntime, allowedRuntimes string) string {
	var sb strings.Builder

	sb.WriteString(renderHeaderStyle.Render("✗ Runtime not allowed!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s with the specified runtime.\n\n", renderCommandStyle.Render("'"+cmdName+"'")))
	sb.WriteString(renderLabelStyle.Render("Selected runtime: "))
	sb.WriteString(renderValueStyle.Render(selectedRuntime))
	sb.WriteString("\n")
	sb.WriteString(renderLabelStyle.Render("Allowed runtimes: "))
	sb.WriteString(renderValueStyle.Render(allowedRuntimes))
	sb.WriteString("\n\n")
	sb.WriteString(renderHintStyle.Render("Use one of the allowed runtimes with --ivk-runtime flag, or update the 'runtimes' setting in your invowkfile."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderSourceNotFoundError creates a styled error message when a specified source doesn't exist.
func RenderSourceNotFoundError(err *SourceNotFoundError) string {
	var sb strings.Builder

	sb.WriteString(renderHeaderStyle.Render("✗ Source not found!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("The source %s does not exist.\n\n", renderCommandStyle.Render("'"+string(err.Source)+"'")))
	sb.WriteString(renderLabelStyle.Render("Available sources: "))
	if len(err.AvailableSources) > 0 {
		var formatted []string
		for _, s := range err.AvailableSources {
			formatted = append(formatted, formatSourceDisplayName(s))
		}
		sb.WriteString(renderValueStyle.Render(strings.Join(formatted, ", ")))
	} else {
		sb.WriteString(renderValueStyle.Render("(none)"))
	}
	sb.WriteString("\n\n")
	sb.WriteString(renderHintStyle.Render("Use @<source> or --ivk-from <source> with a valid source name."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderAmbiguousCommandError creates a styled error message when a command exists in multiple sources.
func RenderAmbiguousCommandError(err *AmbiguousCommandError) string {
	var sb strings.Builder

	sb.WriteString(renderHeaderStyle.Render("✗ Ambiguous command!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("The command %s exists in multiple sources:\n\n", renderCommandStyle.Render("'"+err.CommandName+"'")))

	for _, source := range err.Sources {
		// Show source with @prefix for disambiguation (e.g., "@invowkfile", "@foo")
		sb.WriteString(fmt.Sprintf("  • %s (%s)\n", renderCommandStyle.Render("@"+string(source)), formatSourceDisplayName(source)))
	}

	sb.WriteString("\n")
	sb.WriteString(renderLabelStyle.Render("To run this command, specify the source:\n\n"))

	// Show examples with actual source names
	if len(err.Sources) > 0 {
		firstSource := err.Sources[0]
		sb.WriteString(fmt.Sprintf("  invowk cmd %s %s\n", renderCommandStyle.Render("@"+string(firstSource)), err.CommandName))
		sb.WriteString(fmt.Sprintf("  invowk cmd %s %s %s\n", renderCommandStyle.Render("--ivk-from"), string(firstSource), err.CommandName))
	}

	sb.WriteString("\n")
	sb.WriteString(renderHintStyle.Render("Use 'invowk cmd' to see all commands with their sources."))
	sb.WriteString("\n")

	return sb.String()
}
