// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/invowkfile"

	"github.com/spf13/cobra"
)

// newInitCommand creates the `invowk init` command.
func newInitCommand() *cobra.Command {
	var (
		force    bool
		template string
	)

	cmd := &cobra.Command{
		Use:   "init [filename]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create a new invowkfile in the current directory",
		Long: `Create a new invowkfile in the current directory with example commands.

This command generates a starter invowkfile with sample commands to help
you get started quickly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, args, force, template)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing invowkfile")
	cmd.Flags().StringVarP(&template, "template", "t", "default", "template to use (default, minimal, full)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string, force bool, template string) error {
	filename := "invowkfile.cue"
	if len(args) > 0 {
		filename = args[0]
	}

	// Check if file exists
	if _, err := os.Stat(filename); err == nil && !force {
		return fmt.Errorf("file '%s' already exists. Use --force to overwrite", filename)
	}

	// Validate template name before generating content
	validTemplates := map[string]bool{"default": true, "minimal": true, "full": true}
	if !validTemplates[template] {
		return fmt.Errorf("unknown template %q; valid options are: default, minimal, full", template)
	}

	// Generate content based on template
	content := generateInvowkfile(template)

	// Write file
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s Created %s\n", SuccessStyle.Render("✓"), absPath)
	fmt.Fprintln(w)
	fmt.Fprintln(w, SubtitleStyle.Render("Next steps:"))
	fmt.Fprintln(w, "  1. Run 'invowk cmd hello' to try it out")
	fmt.Fprintln(w, "  2. Run 'invowk cmd hello YourName' to pass an argument")
	fmt.Fprintln(w, "  3. Edit the invowkfile to customize your commands")

	return nil
}

func generateInvowkfile(template string) string {
	var inv *invowkfile.Invowkfile

	switch template {
	case "minimal":
		// Simplest cross-platform template: virtual runtime works identically everywhere
		inv = &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{
				{
					Name:        "hello",
					Description: "Print a greeting",
					Implementations: []invowkfile.Implementation{
						{
							Script:    `echo "Hello, $INVOWK_ARG_NAME!"`,
							Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
							Platforms: invowkfile.AllPlatformConfigs(),
						},
					},
					Args: []invowkfile.Argument{
						{Name: "name", Description: "Who to greet", DefaultValue: "World"},
					},
				},
			},
		}

	case "full":
		// Full template: subcommand, flag, env, and depends_on examples.
		// The parent "hello" is virtual-only (no args) because parent commands
		// with subcommands cannot have args — the CLI parser would interpret
		// positional arguments as subcommand names. Multi-runtime platform-split
		// is already demonstrated by the "default" template.
		inv = &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{
				{
					Name:        "hello",
					Description: "Print a greeting",
					Implementations: []invowkfile.Implementation{
						{
							Script:    `echo "Hello, World!"`,
							Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
							Platforms: invowkfile.AllPlatformConfigs(),
						},
					},
				},
				{
					Name:        "hello formal",
					Description: "Print a formal greeting with a title",
					Implementations: []invowkfile.Implementation{
						{
							Script:    `echo "$INVOWK_FLAG_TITLE $INVOWK_ARG_NAME, welcome!"`,
							Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
							Platforms: invowkfile.AllPlatformConfigs(),
						},
					},
					Flags: []invowkfile.Flag{
						{Name: "title", Description: "Honorific or greeting style", DefaultValue: "Dear"},
					},
					Args: []invowkfile.Argument{
						{Name: "name", Description: "Who to greet", Required: true},
					},
					Env: &invowkfile.EnvConfig{
						Vars: map[string]string{
							"GREETING_STYLE": "formal",
						},
					},
				},
				{
					Name:        "hello all",
					Description: "Run all greeting commands",
					Implementations: []invowkfile.Implementation{
						{
							Script:    `echo "All greetings complete!"`,
							Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
							Platforms: invowkfile.AllPlatformConfigs(),
						},
					},
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []invowkfile.CommandName{"hello"}},
							{Alternatives: []invowkfile.CommandName{"hello formal"}},
						},
					},
				},
			},
		}

	default: // "default"
		// Default template: hello command with all 3 runtimes showing platform-split pattern
		unixPlatforms := []invowkfile.PlatformConfig{
			{Name: invowkfile.PlatformLinux},
			{Name: invowkfile.PlatformMac},
		}
		linuxOnly := []invowkfile.PlatformConfig{
			{Name: invowkfile.PlatformLinux},
		}
		inv = &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{
				helloCommand(unixPlatforms, linuxOnly),
			},
		}
	}

	return invowkfile.GenerateCUE(inv)
}

// helloCommand returns the "hello" command definition used by the "default"
// init template. It demonstrates the platform-split pattern: native has
// separate Unix (parameterized) and Windows (hardcoded) implementations,
// virtual covers all platforms, and container targets Linux only (parameterized).
func helloCommand(unixPlatforms, linuxOnly []invowkfile.PlatformConfig) invowkfile.Command {
	return invowkfile.Command{
		Name:        "hello",
		Description: "Print a greeting",
		Implementations: []invowkfile.Implementation{
			{
				Script:    `echo "Hello, $INVOWK_ARG_NAME!"`,
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
				Platforms: unixPlatforms,
			},
			{
				Script:    `Write-Output "Hello, $($env:INVOWK_ARG_NAME)!"`,
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformWindows}},
			},
			{
				Script:    `echo "Hello, $INVOWK_ARG_NAME!"`,
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
				Platforms: invowkfile.AllPlatformConfigs(),
			},
			{
				Script:    `echo "Hello from container, $INVOWK_ARG_NAME!"`,
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
				Platforms: linuxOnly,
			},
		},
		Args: []invowkfile.Argument{
			{Name: "name", Description: "Who to greet", DefaultValue: "World"},
		},
	}
}
