// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/agentcmd"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/types"
)

// newAgentCommand creates the `invowk agent` command group.
func newAgentCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Helpers for LLM-assisted invowk authoring",
		Long: `Helpers for LLM-assisted invowk authoring.

These commands expose machine-oriented prompts and generation flows for agents
that create custom invowk commands.`,
	}

	cmd.AddCommand(newAgentCmdCommand(app, rootFlags))
	return cmd
}

func newAgentCmdCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cmd",
		Short: "Create and inspect custom-command authoring prompts",
	}

	cmd.AddCommand(newAgentCmdPromptCommand())
	cmd.AddCommand(newAgentCmdCreateCommand(app, rootFlags))
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

func newAgentCmdCreateCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	var (
		targetPath string
		fromFile   string
		dryRun     bool
		printOnly  bool
		replace    bool
		verify     bool
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
			return runAgentCmdCreate(cmd, app, rootFlags, agentcmd.CreateOptions{
				Description: strings.Join(args, " "),
				TargetPath:  targetPath,
				FromFile:    fromFile,
				DryRun:      dryRun,
				PrintOnly:   printOnly,
				Replace:     replace,
			}, verify, llmFlags)
		},
	}

	cmd.Flags().StringVar(&targetPath, "file", "invowkfile.cue", "invowkfile to update")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read the command description from a file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the patch without writing the invowkfile")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the generated command CUE without writing the invowkfile")
	cmd.Flags().BoolVar(&replace, "replace", false, "replace an existing command with the same name")
	cmd.Flags().BoolVar(&verify, "verify", false, "after writing, resolve the generated command with a dry-run execution plan")
	bindLLMFlags(cmd, &llmFlags)

	return cmd
}

func runAgentCmdCreate(
	cmd *cobra.Command,
	app *App,
	rootFlags *rootFlagValues,
	createOpts agentcmd.CreateOptions,
	verify bool,
	llmFlags llmFlagValues,
) error {
	if err := validateAgentCmdCreateOptions(createOpts, verify, llmFlags); err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}
	llmResult, llmErr := resolveAgentCmdCreateCompleter(cmd, app, rootFlags, llmFlags)
	if llmErr != nil {
		return llmErr
	}

	createOpts.Completer = llmResult.completer
	result, err := agentcmd.CreateCommand(cmd.Context(), createOpts)
	if err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}

	if verify {
		if err := verifyGeneratedCommand(cmd, app, rootFlags, result); err != nil {
			return &ExitError{Code: auditExitError, Err: err}
		}
	}

	renderAgentCmdCreateResult(cmd, result, createOpts, verify)
	return nil
}

func validateAgentCmdCreateOptions(createOpts agentcmd.CreateOptions, verify bool, llmFlags llmFlagValues) error {
	if err := validateAgentCmdCreateModes(createOpts.DryRun, createOpts.PrintOnly, verify); err != nil {
		return err
	}
	return validateLLMFlagSelection(llmFlags)
}

func resolveAgentCmdCreateCompleter(
	cmd *cobra.Command,
	app *App,
	rootFlags *rootFlagValues,
	llmFlags llmFlagValues,
) (*llmCompleterResult, *ExitError) {
	resolved, llmErr := resolveLLMForCommand(
		cmd.Context(),
		cmd,
		app.Config,
		types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- root flag value is validated by config provider.
		llmFlags,
		true,
	)
	if llmErr != nil {
		return nil, llmErr
	}
	return buildLLMCompleter(cmd.Context(), cmd, resolved)
}

func renderAgentCmdCreateResult(
	cmd *cobra.Command,
	result *agentcmd.CreateResult,
	createOpts agentcmd.CreateOptions,
	verify bool,
) {
	w := cmd.OutOrStdout()
	switch {
	case createOpts.PrintOnly:
		fmt.Fprintln(w, result.CommandCUE)
	case createOpts.DryRun:
		fmt.Fprint(w, result.Diff)
	default:
		renderAgentCmdCreateWriteResult(w, result, verify)
	}
}

func renderAgentCmdCreateWriteResult(w io.Writer, result *agentcmd.CreateResult, verify bool) {
	fmt.Fprintf(w, "%s Added command %q to %s\n", SuccessStyle.Render("✓"), result.CommandName, result.TargetPath)
	if verify {
		fmt.Fprintf(w, "%s Verified command %q with a dry-run execution plan\n", SuccessStyle.Render("✓"), result.CommandName)
	}
	if result.Summary != "" {
		fmt.Fprintln(w, result.Summary)
	}
}

func validateAgentCmdCreateModes(dryRun, printOnly, verify bool) error {
	if dryRun && printOnly {
		return errors.New("--dry-run and --print are mutually exclusive")
	}
	if verify && (dryRun || printOnly) {
		return errors.New("--verify requires writing the invowkfile and cannot be used with --dry-run or --print")
	}
	return nil
}

func verifyGeneratedCommand(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, result *agentcmd.CreateResult) error {
	if result == nil {
		return errors.New("generated command result is required for verification")
	}
	if err := ensureCurrentInvowkfileTarget(result.TargetPath); err != nil {
		return err
	}

	req := ExecuteRequest{
		Name:       result.CommandName.String(),
		FromSource: discovery.SourceIDInvowkfile,
		ConfigPath: types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- root flag value is validated by config provider.
		DryRun:     true,
	}
	if err := executeRequest(cmd, app, req); err != nil {
		return fmt.Errorf("verify generated command: %w", err)
	}
	return nil
}

func ensureCurrentInvowkfileTarget(targetPath string) error {
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolve generated invowkfile path: %w", err)
	}
	cwd, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}
	expected := filepath.Join(cwd, "invowkfile.cue")
	if filepath.Clean(targetAbs) != filepath.Clean(expected) {
		return fmt.Errorf("--verify supports only the current directory invowkfile.cue target; got %s", targetPath)
	}
	return nil
}
