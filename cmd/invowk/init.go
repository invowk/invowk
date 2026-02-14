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
		Use:   "init",
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

func runInit(_ *cobra.Command, args []string, force bool, template string) error {
	filename := "invowkfile.cue"
	if len(args) > 0 {
		filename = args[0]
	}

	// Check if file exists
	if _, err := os.Stat(filename); err == nil && !force {
		return fmt.Errorf("file '%s' already exists. Use --force to overwrite", filename)
	}

	// Generate content based on template
	content := generateInvowkfile(template)

	// Write file
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	absPath, _ := filepath.Abs(filename)
	fmt.Printf("%s Created %s\n", SuccessStyle.Render("✓"), absPath)
	fmt.Println()
	fmt.Println(SubtitleStyle.Render("Next steps:"))
	fmt.Println("  1. Edit the invowkfile to add your commands")
	fmt.Println("  2. Run 'invowk cmd' to see available commands")
	fmt.Println("  3. Run 'invowk cmd <command>' to execute a command")

	return nil
}

func generateInvowkfile(template string) string {
	var inv *invowkfile.Invowkfile

	switch template {
	case "minimal":
		// invowkfile.cue contains only commands - module metadata goes in invowkmod.cue
		inv = &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{
				{
					Name:        "hello",
					Description: "Print a greeting",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "echo 'Hello from invowk!'",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
						},
					},
				},
			},
		}

	case "full":
		// invowkfile.cue contains only commands - module metadata goes in invowkmod.cue
		inv = &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo \"Building $PROJECT_NAME...\"\ngo build -o bin/app ./...",

							Runtimes: []invowkfile.RuntimeConfig{
								{Name: invowkfile.RuntimeNative},
								{Name: invowkfile.RuntimeContainer, Image: "golang:1.26"},
							},
							Platforms: []invowkfile.PlatformConfig{
								{Name: invowkfile.PlatformLinux},
								{Name: invowkfile.PlatformMac},
								{Name: invowkfile.PlatformWindows},
							},
						},
					},
					Env: &invowkfile.EnvConfig{
						Vars: map[string]string{
							"PROJECT_NAME": "myproject",
							"CGO_ENABLED":  "0",
						},
					},
				},
				{
					Name:        "test unit",
					Description: "Run unit tests",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "go test -v ./...",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeVirtual}},
						},
					},
				},
				{
					Name:        "test integration",
					Description: "Run integration tests",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "go test -v -tags=integration ./...",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
						},
					},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Implementations: []invowkfile.Implementation{
						{
							Script: "rm -rf bin/ dist/",

							Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
							Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}},
						},
						{
							Script: "if exist bin rmdir /s /q bin && if exist dist rmdir /s /q dist",

							Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
							Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformWindows}},
						},
					},
				},
				{
					Name:        "docker-build",
					Description: "Build using container runtime",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "go build -o /workspace/bin/app ./...",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "golang:1.26"}},
						},
					},
				},
				{
					Name:        "container hello-invowk",
					Description: "Print a greeting from a container",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "echo \"Hello, Invowk!\"",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
						},
					},
				},
				{
					Name:        "release",
					Description: "Create a release",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo 'Creating release...'",

							Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
							Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}},
						},
					},
					DependsOn: &invowkfile.DependsOn{
						Tools: []invowkfile.ToolDependency{
							{Alternatives: []string{"git"}},
						},
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{"clean"}},
							{Alternatives: []string{"build"}},
							{Alternatives: []string{"test unit"}},
						},
					},
				},
			},
		}

	default: // "default"
		// invowkfile.cue contains only commands - module metadata goes in invowkmod.cue
		inv = &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "echo 'Building...'\n# Add your build commands here",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
						},
					},
				},
				{
					Name:        "test",
					Description: "Run tests",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "echo 'Testing...'\n# Add your test commands here",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
						},
					},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Implementations: []invowkfile.Implementation{
						{
							Script:   "echo 'Cleaning...'\n# Add your clean commands here",
							Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
						},
					},
				},
			},
		}
	}

	return invowkfile.GenerateCUE(inv)
}
