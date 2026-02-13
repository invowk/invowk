// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"strings"

	"invowk-cli/internal/discovery"

	"github.com/charmbracelet/lipgloss"
)

// RenderArgumentValidationError creates a styled error message for argument validation failures
func RenderArgumentValidationError(err *ArgumentValidationError) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	switch err.Type {
	case ArgErrMissingRequired:
		sb.WriteString(headerStyle.Render("✗ Missing required arguments!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s requires at least %d argument(s), but got %d.\n\n",
			commandStyle.Render("'"+err.CommandName+"'"), err.MinArgs, len(err.ProvidedArgs)))

		sb.WriteString(labelStyle.Render("Expected arguments:"))
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
			sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s%s - %s\n", arg.Name, reqStr, arg.Description)))
		}

	case ArgErrTooMany:
		sb.WriteString(headerStyle.Render("✗ Too many arguments!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s accepts at most %d argument(s), but got %d.\n\n",
			commandStyle.Render("'"+err.CommandName+"'"), err.MaxArgs, len(err.ProvidedArgs)))

		sb.WriteString(labelStyle.Render("Expected arguments:"))
		sb.WriteString("\n")
		for _, arg := range err.ArgDefs {
			sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s - %s\n", arg.Name, arg.Description)))
		}
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render("Provided:"))
		sb.WriteString(valueStyle.Render(fmt.Sprintf(" %v", err.ProvidedArgs)))

	case ArgErrInvalidValue:
		sb.WriteString(headerStyle.Render("✗ Invalid argument value!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s received an invalid value for argument %s.\n\n",
			commandStyle.Render("'"+err.CommandName+"'"), commandStyle.Render("'"+err.InvalidArg+"'")))

		sb.WriteString(labelStyle.Render("Value:  "))
		sb.WriteString(valueStyle.Render(fmt.Sprintf("%q", err.InvalidValue)))
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render("Error:  "))
		sb.WriteString(valueStyle.Render(err.ValueError.Error()))
	}

	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Run the command with --help for usage information."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderArgsSubcommandConflictError creates a styled error message when a command
// has both positional arguments and subcommands defined. This is a structural error
// because positional arguments can only be accepted by leaf commands.
func RenderArgsSubcommandConflictError(err *discovery.ArgsSubcommandConflictError) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")). // Red for error
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Invalid command structure!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Command %s defines positional arguments but also has subcommands.\n",
		commandStyle.Render("'"+err.CommandName+"'")))
	if err.FilePath != "" {
		sb.WriteString(pathStyle.Render(fmt.Sprintf("  in %s\n", err.FilePath)))
	}
	sb.WriteString("\nPositional arguments can only be defined on leaf commands (commands without subcommands).\n\n")

	sb.WriteString(labelStyle.Render("Defined args:"))
	sb.WriteString("\n")
	for _, arg := range err.Args {
		sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s - %s\n", arg.Name, arg.Description)))
	}

	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Subcommands:"))
	sb.WriteString("\n")
	for _, subcmd := range err.Subcommands {
		sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s\n", subcmd)))
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Remove either the 'args' field or the subcommands to resolve this conflict."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderDependencyError creates a styled error message for unsatisfied dependencies
func RenderDependencyError(err *DependencyError) string {
	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		MarginTop(1)

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Dependencies not satisfied!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s because some dependencies are missing.\n", commandStyle.Render("'"+err.CommandName+"'")))

	if len(err.MissingTools) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing Tools:"))
		sb.WriteString("\n")
		for _, tool := range err.MissingTools {
			sb.WriteString(itemStyle.Render(tool))
			sb.WriteString("\n")
		}
	}

	if len(err.MissingCommands) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing Commands:"))
		sb.WriteString("\n")
		for _, cmd := range err.MissingCommands {
			sb.WriteString(itemStyle.Render(cmd))
			sb.WriteString("\n")
		}
	}

	if len(err.MissingFilepaths) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing or Inaccessible Files:"))
		sb.WriteString("\n")
		for _, fp := range err.MissingFilepaths {
			sb.WriteString(itemStyle.Render(fp))
			sb.WriteString("\n")
		}
	}

	if len(err.MissingCapabilities) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing Capabilities:"))
		sb.WriteString("\n")
		for _, cap := range err.MissingCapabilities {
			sb.WriteString(itemStyle.Render(cap))
			sb.WriteString("\n")
		}
	}

	if len(err.FailedCustomChecks) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Failed Custom Checks:"))
		sb.WriteString("\n")
		for _, check := range err.FailedCustomChecks {
			sb.WriteString(itemStyle.Render(check))
			sb.WriteString("\n")
		}
	}

	if len(err.MissingEnvVars) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing or Invalid Environment Variables:"))
		sb.WriteString("\n")
		for _, envVar := range err.MissingEnvVars {
			sb.WriteString(itemStyle.Render(envVar))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Fix the missing dependencies and try again, or update your invowkfile to remove unnecessary ones."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderHostNotSupportedError creates a styled error message for unsupported host OS
func RenderHostNotSupportedError(cmdName, currentOS, supportedHosts string) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Host not supported!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s on this operating system.\n\n", commandStyle.Render("'"+cmdName+"'")))
	sb.WriteString(labelStyle.Render("Current host:    "))
	sb.WriteString(valueStyle.Render(currentOS))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Supported hosts: "))
	sb.WriteString(valueStyle.Render(supportedHosts))
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Run this command on a supported operating system, or update the 'works_on.hosts' setting in your invowkfile."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderRuntimeNotAllowedError creates a styled error message for invalid runtime selection
func RenderRuntimeNotAllowedError(cmdName, selectedRuntime, allowedRuntimes string) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Runtime not allowed!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s with the specified runtime.\n\n", commandStyle.Render("'"+cmdName+"'")))
	sb.WriteString(labelStyle.Render("Selected runtime: "))
	sb.WriteString(valueStyle.Render(selectedRuntime))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Allowed runtimes: "))
	sb.WriteString(valueStyle.Render(allowedRuntimes))
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Use one of the allowed runtimes with --ivk-runtime flag, or update the 'runtimes' setting in your invowkfile."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderSourceNotFoundError creates a styled error message when a specified source doesn't exist.
func RenderSourceNotFoundError(err *SourceNotFoundError) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	sourceStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Source not found!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("The source %s does not exist.\n\n", sourceStyle.Render("'"+err.Source+"'")))
	sb.WriteString(labelStyle.Render("Available sources: "))
	if len(err.AvailableSources) > 0 {
		var formatted []string
		for _, s := range err.AvailableSources {
			formatted = append(formatted, formatSourceDisplayName(s))
		}
		sb.WriteString(valueStyle.Render(strings.Join(formatted, ", ")))
	} else {
		sb.WriteString(valueStyle.Render("(none)"))
	}
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Use @<source> or --ivk-from <source> with a valid source name."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderAmbiguousCommandError creates a styled error message when a command exists in multiple sources.
func RenderAmbiguousCommandError(err *AmbiguousCommandError) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	sourceStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	sb.WriteString(headerStyle.Render("✗ Ambiguous command!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("The command %s exists in multiple sources:\n\n", commandStyle.Render("'"+err.CommandName+"'")))

	for _, source := range err.Sources {
		// Show source with @prefix for disambiguation (e.g., "@invowkfile", "@foo")
		sb.WriteString(fmt.Sprintf("  • %s (%s)\n", sourceStyle.Render("@"+source), formatSourceDisplayName(source)))
	}

	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("To run this command, specify the source:\n\n"))

	// Show examples with actual source names
	if len(err.Sources) > 0 {
		firstSource := err.Sources[0]
		sb.WriteString(fmt.Sprintf("  invowk cmd %s %s\n", sourceStyle.Render("@"+firstSource), err.CommandName))
		sb.WriteString(fmt.Sprintf("  invowk cmd %s %s %s\n", sourceStyle.Render("--ivk-from"), firstSource, err.CommandName))
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Use 'invowk cmd' to see all commands with their sources."))
	sb.WriteString("\n")

	return sb.String()
}
