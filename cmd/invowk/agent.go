// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/agentcmd"
)

// newAgentCommand creates the `invowk agent` command group.
func newAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Helpers for LLM-assisted invowk authoring",
		Long: `Helpers for LLM-assisted invowk authoring.

These commands expose machine-oriented prompts and generation flows for agents
that create custom invowk commands.`,
	}

	cmd.AddCommand(newAgentCmdCommand())
	return cmd
}

func newAgentCmdCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cmd",
		Short: "Create and inspect custom-command authoring prompts",
	}

	cmd.AddCommand(newAgentCmdPromptCommand())
	cmd.AddCommand(newAgentCmdCreateCommand())
	return cmd
}

func newAgentCmdPromptCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Print the system prompt for custom command authoring",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rendered, err := agentcmd.RenderPrompt(format)
			if err != nil {
				return &ExitError{Code: auditExitError, Err: err}
			}
			fmt.Fprint(cmd.OutOrStdout(), rendered)
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text, json")
	return cmd
}

func newAgentCmdCreateCommand() *cobra.Command {
	var (
		targetPath string
		fromFile   string
		dryRun     bool
		printOnly  bool
		replace    bool
		llmFlags   llmFlagValues
	)

	cmd := &cobra.Command{
		Use:   "create [description...]",
		Short: "Generate and add a custom invowk command with an LLM",
		Long: `Generate one custom invowk command with an LLM, validate the generated CUE,
and add it to invowkfile.cue by default.

Use --dry-run to preview the file patch, or --print to print only the generated
command object.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateLLMFlagSelection(llmFlags); err != nil {
				return &ExitError{Code: auditExitError, Err: err}
			}

			llmOpts := llmClientConfigFromFlags(cmd, llmFlags)
			llmResult, llmErr := buildLLMCompleter(cmd.Context(), cmd, llmFlags, llmOpts)
			if llmErr != nil {
				return llmErr
			}

			result, err := agentcmd.CreateCommand(cmd.Context(), agentcmd.CreateOptions{
				Description: strings.Join(args, " "),
				TargetPath:  targetPath,
				FromFile:    fromFile,
				DryRun:      dryRun,
				PrintOnly:   printOnly,
				Replace:     replace,
				Completer:   llmResult.completer,
			})
			if err != nil {
				return &ExitError{Code: auditExitError, Err: err}
			}

			w := cmd.OutOrStdout()
			switch {
			case printOnly:
				fmt.Fprintln(w, result.CommandCUE)
			case dryRun:
				fmt.Fprint(w, result.Diff)
			default:
				fmt.Fprintf(w, "%s Added command %q to %s\n", SuccessStyle.Render("✓"), result.CommandName, result.TargetPath)
				if result.Summary != "" {
					fmt.Fprintln(w, result.Summary)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&targetPath, "file", "invowkfile.cue", "invowkfile to update")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read the command description from a file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the patch without writing the invowkfile")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the generated command CUE without writing the invowkfile")
	cmd.Flags().BoolVar(&replace, "replace", false, "replace an existing command with the same name")
	bindLLMFlags(cmd, &llmFlags)

	return cmd
}
