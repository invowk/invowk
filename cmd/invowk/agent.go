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
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	agentInvowkfileName      = "invowkfile.cue"
	agentInvowkfileFlagUsage = "invowkfile to update"
	agentFromFileFlagName    = "from-file"
	agentDryRunFlagName      = "dry-run"
	agentCmdDryRunFlagUsage  = "print the patch without writing the invowkfile"
)

// newAgentCommand creates the `invowk agent` command group.
func newAgentCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Helpers for LLM-assisted invowk authoring",
		Long: `Helpers for LLM-assisted invowk authoring.

These commands expose machine-oriented prompts and generation flows for agents
that create custom invowk commands and local modules.`,
	}

	cmd.AddCommand(newAgentCmdCommand(app, rootFlags))
	cmd.AddCommand(newAgentModCommand(app, rootFlags))
	return cmd
}

func newAgentCmdCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cmd",
		Short: "Create, change, remove, and inspect custom-command authoring prompts",
	}

	cmd.AddCommand(newAgentCmdPromptCommand())
	cmd.AddCommand(newAgentCmdCreateCommand(app, rootFlags))
	cmd.AddCommand(newAgentCmdChangeCommand(app, rootFlags))
	cmd.AddCommand(newAgentCmdRemoveCommand())
	return cmd
}

func newAgentCmdPromptCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "prompt [operation]",
		Short: "Print the system prompt for custom command authoring",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			operation := ""
			if len(args) > 0 {
				operation = args[0]
			}
			rendered, err := agentcmd.RenderCommandPrompt(format, operation)
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
		verify     bool
		llmFlags   llmFlagValues
	)

	cmd := &cobra.Command{
		Use:   "create <name> [description...]",
		Short: "Generate and add a custom invowk command with an LLM",
		Long: `Generate one custom invowk command with an LLM, validate the generated CUE,
and add it to invowkfile.cue by default.

Use --dry-run to preview the file patch, or --print to print only the generated
command object.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentCmdCreate(cmd, app, rootFlags, agentcmd.CreateOptions{
				Name:        invowkfile.CommandName(args[0]),
				Description: strings.Join(args[1:], " "),
				TargetPath:  targetPath,
				FromFile:    fromFile,
				DryRun:      dryRun,
				PrintOnly:   printOnly,
			}, verify, llmFlags)
		},
	}

	cmd.Flags().StringVar(&targetPath, "file", agentInvowkfileName, agentInvowkfileFlagUsage)
	cmd.Flags().StringVar(&fromFile, agentFromFileFlagName, "", "read the command description from a file")
	cmd.Flags().BoolVar(&dryRun, agentDryRunFlagName, false, agentCmdDryRunFlagUsage)
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the generated command CUE without writing the invowkfile")
	cmd.Flags().BoolVar(&verify, "verify", false, "after writing, resolve the generated command with a dry-run execution plan")
	bindLLMFlags(cmd, &llmFlags)

	return cmd
}

func newAgentCmdChangeCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	var (
		targetPath string
		fromFile   string
		dryRun     bool
		printOnly  bool
		verify     bool
		llmFlags   llmFlagValues
	)

	cmd := &cobra.Command{
		Use:   "change <name> [description...]",
		Short: "Generate and replace an existing custom invowk command with an LLM",
		Long: `Generate a replacement for one existing custom invowk command with an LLM,
validate the generated CUE, and update invowkfile.cue by default.

Use --dry-run to preview the file patch, or --print to print only the generated
command object.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentCmdChange(cmd, app, rootFlags, agentcmd.ChangeOptions{
				Name:        invowkfile.CommandName(args[0]),
				Description: strings.Join(args[1:], " "),
				TargetPath:  targetPath,
				FromFile:    fromFile,
				DryRun:      dryRun,
				PrintOnly:   printOnly,
			}, verify, llmFlags)
		},
	}

	cmd.Flags().StringVar(&targetPath, "file", agentInvowkfileName, agentInvowkfileFlagUsage)
	cmd.Flags().StringVar(&fromFile, agentFromFileFlagName, "", "read the command change description from a file")
	cmd.Flags().BoolVar(&dryRun, agentDryRunFlagName, false, agentCmdDryRunFlagUsage)
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the generated command CUE without writing the invowkfile")
	cmd.Flags().BoolVar(&verify, "verify", false, "after writing, resolve the changed command with a dry-run execution plan")
	bindLLMFlags(cmd, &llmFlags)

	return cmd
}

func newAgentCmdRemoveCommand() *cobra.Command {
	var (
		targetPath string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom invowk command from an invowkfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := agentcmd.RemoveCommand(cmd.Context(), agentcmd.RemoveOptions{
				Name:       invowkfile.CommandName(args[0]),
				TargetPath: targetPath,
				DryRun:     dryRun,
			})
			if err != nil {
				return &ExitError{Code: auditExitError, Err: err}
			}
			renderAgentCmdRemoveResult(cmd, result, dryRun)
			return nil
		},
	}

	cmd.Flags().StringVar(&targetPath, "file", agentInvowkfileName, agentInvowkfileFlagUsage)
	cmd.Flags().BoolVar(&dryRun, agentDryRunFlagName, false, agentCmdDryRunFlagUsage)
	return cmd
}

func newAgentModCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Create, change, remove, and inspect local-module authoring prompts",
	}

	cmd.AddCommand(newAgentModPromptCommand())
	cmd.AddCommand(newAgentModCreateCommand(app, rootFlags))
	cmd.AddCommand(newAgentModChangeCommand(app, rootFlags))
	cmd.AddCommand(newAgentModRemoveCommand())
	return cmd
}

func newAgentModPromptCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "prompt [operation]",
		Short: "Print the system prompt for local module authoring",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			operation := ""
			if len(args) > 0 {
				operation = args[0]
			}
			rendered, err := agentcmd.RenderModulePrompt(format, operation)
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

func newAgentModCreateCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	var (
		fromFile  string
		dryRun    bool
		printOnly bool
		verify    bool
		scripts   bool
		llmFlags  llmFlagValues
	)

	cmd := &cobra.Command{
		Use:   "create <module-id> [description...]",
		Short: "Generate and add a local invowk module with an LLM",
		Long: `Generate invowkmod.cue and invowkfile.cue for one local module with an LLM,
validate the generated CUE, and create <module-id>.invowkmod by default.

Use --dry-run to preview the file patches, or --print to print only the
generated module file bundle.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentModCreate(cmd, app, rootFlags, agentcmd.ModuleCreateOptions{
				ModuleID:         invowkmod.ModuleID(args[0]),
				Description:      strings.Join(args[1:], " "),
				FromFile:         fromFile,
				DryRun:           dryRun,
				PrintOnly:        printOnly,
				Verify:           verify,
				CreateScriptsDir: scripts,
			}, llmFlags)
		},
	}

	cmd.Flags().StringVar(&fromFile, agentFromFileFlagName, "", "read the module description from a file")
	cmd.Flags().BoolVar(&dryRun, agentDryRunFlagName, false, "print the patch without writing the module")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the generated module file bundle without writing")
	cmd.Flags().BoolVar(&verify, "verify", false, "after writing, validate the generated module")
	cmd.Flags().BoolVar(&scripts, "scripts", false, "create an empty scripts/ directory with .gitkeep")
	bindLLMFlags(cmd, &llmFlags)

	return cmd
}

func newAgentModChangeCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	var (
		fromFile  string
		dryRun    bool
		printOnly bool
		verify    bool
		llmFlags  llmFlagValues
	)

	cmd := &cobra.Command{
		Use:   "change <module-id-or-path> [description...]",
		Short: "Generate and update local invowk module files with an LLM",
		Long: `Generate replacements for invowkmod.cue and invowkfile.cue for one existing
local module with an LLM. The operation updates only those two files.

Use --dry-run to preview the file patches, or --print to print only the
generated module file bundle.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentModChange(cmd, app, rootFlags, agentcmd.ModuleChangeOptions{
				Target:      args[0],
				Description: strings.Join(args[1:], " "),
				FromFile:    fromFile,
				DryRun:      dryRun,
				PrintOnly:   printOnly,
				Verify:      verify,
			}, llmFlags)
		},
	}

	cmd.Flags().StringVar(&fromFile, agentFromFileFlagName, "", "read the module change description from a file")
	cmd.Flags().BoolVar(&dryRun, agentDryRunFlagName, false, "print the patch without writing the module")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the generated module file bundle without writing")
	cmd.Flags().BoolVar(&verify, "verify", false, "after writing, validate the changed module")
	bindLLMFlags(cmd, &llmFlags)

	return cmd
}

func newAgentModRemoveCommand() *cobra.Command {
	var (
		dryRun bool
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "remove <module-id-or-path>",
		Short: "Remove a validated local invowk module directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := agentcmd.RemoveModule(cmd.Context(), agentcmd.ModuleRemoveOptions{
				Target: args[0],
				DryRun: dryRun,
				Force:  force,
			})
			if err != nil {
				return &ExitError{Code: auditExitError, Err: err}
			}
			renderAgentModRemoveResult(cmd, result, dryRun)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, agentDryRunFlagName, false, "print the deletion plan without removing the module")
	cmd.Flags().BoolVar(&force, "force", false, "delete the validated module directory")
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
	llmResult, llmErr := resolveAgentAuthoringCompleter(cmd, app, rootFlags, llmFlags)
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
	if err := validateAgentAuthoringModes(createOpts.DryRun, createOpts.PrintOnly, verify); err != nil {
		return err
	}
	if err := validateCommandAuthoringInput(createOpts.Name, createOpts.Description, createOpts.FromFile); err != nil {
		return err
	}
	return validateLLMFlagSelection(llmFlags)
}

func runAgentCmdChange(
	cmd *cobra.Command,
	app *App,
	rootFlags *rootFlagValues,
	changeOpts agentcmd.ChangeOptions,
	verify bool,
	llmFlags llmFlagValues,
) error {
	if err := validateAgentCmdChangeOptions(changeOpts, verify, llmFlags); err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}
	llmResult, llmErr := resolveAgentAuthoringCompleter(cmd, app, rootFlags, llmFlags)
	if llmErr != nil {
		return llmErr
	}

	changeOpts.Completer = llmResult.completer
	result, err := agentcmd.ChangeCommand(cmd.Context(), changeOpts)
	if err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}

	if verify {
		if err := verifyGeneratedCommand(cmd, app, rootFlags, result); err != nil {
			return &ExitError{Code: auditExitError, Err: err}
		}
	}

	renderAgentCmdChangeResult(cmd, result, changeOpts, verify)
	return nil
}

func validateAgentCmdChangeOptions(changeOpts agentcmd.ChangeOptions, verify bool, llmFlags llmFlagValues) error {
	if err := validateAgentAuthoringModes(changeOpts.DryRun, changeOpts.PrintOnly, verify); err != nil {
		return err
	}
	if err := validateCommandAuthoringInput(changeOpts.Name, changeOpts.Description, changeOpts.FromFile); err != nil {
		return err
	}
	return validateLLMFlagSelection(llmFlags)
}

func runAgentModCreate(
	cmd *cobra.Command,
	app *App,
	rootFlags *rootFlagValues,
	createOpts agentcmd.ModuleCreateOptions,
	llmFlags llmFlagValues,
) error {
	if err := validateAgentModCreateOptions(createOpts, llmFlags); err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}
	llmResult, llmErr := resolveAgentAuthoringCompleter(cmd, app, rootFlags, llmFlags)
	if llmErr != nil {
		return llmErr
	}

	createOpts.Completer = llmResult.completer
	result, err := agentcmd.CreateModule(cmd.Context(), createOpts)
	if err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}

	renderAgentModCreateResult(cmd, result, createOpts)
	return nil
}

func validateAgentModCreateOptions(createOpts agentcmd.ModuleCreateOptions, llmFlags llmFlagValues) error {
	if err := validateAgentAuthoringModes(createOpts.DryRun, createOpts.PrintOnly, createOpts.Verify); err != nil {
		return err
	}
	if err := validateModuleAuthoringInput(createOpts.ModuleID, createOpts.Description, createOpts.FromFile); err != nil {
		return err
	}
	return validateLLMFlagSelection(llmFlags)
}

func runAgentModChange(
	cmd *cobra.Command,
	app *App,
	rootFlags *rootFlagValues,
	changeOpts agentcmd.ModuleChangeOptions,
	llmFlags llmFlagValues,
) error {
	if err := validateAgentModChangeOptions(changeOpts, llmFlags); err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}
	llmResult, llmErr := resolveAgentAuthoringCompleter(cmd, app, rootFlags, llmFlags)
	if llmErr != nil {
		return llmErr
	}

	changeOpts.Completer = llmResult.completer
	result, err := agentcmd.ChangeModule(cmd.Context(), changeOpts)
	if err != nil {
		return &ExitError{Code: auditExitError, Err: err}
	}

	renderAgentModChangeResult(cmd, result, changeOpts)
	return nil
}

func validateAgentModChangeOptions(changeOpts agentcmd.ModuleChangeOptions, llmFlags llmFlagValues) error {
	if err := validateAgentAuthoringModes(changeOpts.DryRun, changeOpts.PrintOnly, changeOpts.Verify); err != nil {
		return err
	}
	if strings.TrimSpace(changeOpts.Target) == "" {
		return errors.New("module target is required")
	}
	if err := validateAuthoringDescription(changeOpts.Description, changeOpts.FromFile, "module"); err != nil {
		return err
	}
	return validateLLMFlagSelection(llmFlags)
}

func resolveAgentAuthoringCompleter(
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

func renderAgentCmdChangeResult(
	cmd *cobra.Command,
	result *agentcmd.ChangeResult,
	changeOpts agentcmd.ChangeOptions,
	verify bool,
) {
	w := cmd.OutOrStdout()
	switch {
	case changeOpts.PrintOnly:
		fmt.Fprintln(w, result.CommandCUE)
	case changeOpts.DryRun:
		fmt.Fprint(w, result.Diff)
	default:
		renderAgentCmdChangeWriteResult(w, result, verify)
	}
}

func renderAgentCmdChangeWriteResult(w io.Writer, result *agentcmd.ChangeResult, verify bool) {
	fmt.Fprintf(w, "%s Updated command %q in %s\n", SuccessStyle.Render("✓"), result.CommandName, result.TargetPath)
	if verify {
		fmt.Fprintf(w, "%s Verified command %q with a dry-run execution plan\n", SuccessStyle.Render("✓"), result.CommandName)
	}
	if result.Summary != "" {
		fmt.Fprintln(w, result.Summary)
	}
}

func renderAgentCmdRemoveResult(cmd *cobra.Command, result *agentcmd.RemoveResult, dryRun bool) {
	w := cmd.OutOrStdout()
	if dryRun {
		fmt.Fprint(w, result.Diff)
		return
	}
	fmt.Fprintf(w, "%s Removed command %q from %s\n", SuccessStyle.Render("✓"), result.CommandName, result.TargetPath)
}

func renderAgentModCreateResult(cmd *cobra.Command, result *agentcmd.ModuleResult, createOpts agentcmd.ModuleCreateOptions) {
	w := cmd.OutOrStdout()
	switch {
	case createOpts.PrintOnly:
		renderAgentModPrintResult(w, result)
	case createOpts.DryRun:
		fmt.Fprint(w, result.Diff)
	default:
		renderAgentModWriteResult(w, "Created", result)
	}
}

func renderAgentModChangeResult(cmd *cobra.Command, result *agentcmd.ModuleResult, changeOpts agentcmd.ModuleChangeOptions) {
	w := cmd.OutOrStdout()
	switch {
	case changeOpts.PrintOnly:
		renderAgentModPrintResult(w, result)
	case changeOpts.DryRun:
		fmt.Fprint(w, result.Diff)
	default:
		renderAgentModWriteResult(w, "Updated", result)
	}
}

func renderAgentModPrintResult(w io.Writer, result *agentcmd.ModuleResult) {
	rendered, err := result.PrintJSON()
	if err != nil {
		fmt.Fprintf(w, "module print rendering failed: %v\n", err)
		return
	}
	fmt.Fprint(w, rendered)
}

func renderAgentModWriteResult(w io.Writer, verb string, result *agentcmd.ModuleResult) {
	fmt.Fprintf(w, "%s %s module %q at %s\n", SuccessStyle.Render("✓"), verb, result.ModuleID, result.ModulePath)
	if result.Verified {
		fmt.Fprintf(w, "%s Verified module %q\n", SuccessStyle.Render("✓"), result.ModuleID)
	}
	if result.Summary != "" {
		fmt.Fprintln(w, result.Summary)
	}
}

func renderAgentModRemoveResult(cmd *cobra.Command, result *agentcmd.ModuleResult, dryRun bool) {
	w := cmd.OutOrStdout()
	if dryRun {
		fmt.Fprint(w, result.Diff)
		return
	}
	fmt.Fprintf(w, "%s Removed module %q at %s\n", SuccessStyle.Render("✓"), result.ModuleID, result.ModulePath)
}

func validateAgentCmdCreateModes(dryRun, printOnly, verify bool) error {
	return validateAgentAuthoringModes(dryRun, printOnly, verify)
}

func validateAgentAuthoringModes(dryRun, printOnly, verify bool) error {
	if dryRun && printOnly {
		return errors.New("--dry-run and --print are mutually exclusive")
	}
	if verify && (dryRun || printOnly) {
		return errors.New("--verify requires writing the invowkfile and cannot be used with --dry-run or --print")
	}
	return nil
}

func validateCommandAuthoringInput(name invowkfile.CommandName, description, fromFile string) error {
	if err := name.Validate(); err != nil {
		return err
	}
	return validateAuthoringDescription(description, fromFile, "command")
}

func validateModuleAuthoringInput(moduleID invowkmod.ModuleID, description, fromFile string) error {
	if err := moduleID.Validate(); err != nil {
		return err
	}
	return validateAuthoringDescription(description, fromFile, "module")
}

func validateAuthoringDescription(description, fromFile, target string) error {
	if strings.TrimSpace(description) == "" && fromFile == "" {
		return fmt.Errorf("%s description is required", target)
	}
	if strings.TrimSpace(description) != "" && fromFile != "" {
		return errors.New("description arguments and --from-file are mutually exclusive")
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
	expected := filepath.Join(cwd, agentInvowkfileName)
	if filepath.Clean(targetAbs) != filepath.Clean(expected) {
		return fmt.Errorf("--verify supports only the current directory invowkfile.cue target; got %s", targetPath)
	}
	return nil
}
