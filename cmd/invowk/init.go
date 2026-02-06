// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"invowk-cli/pkg/invkfile"

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
		Short: "Create a new invkfile in the current directory",
		Long: `Create a new invkfile in the current directory with example commands.

This command generates a starter invkfile with sample commands to help
you get started quickly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, args, force, template)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing invkfile")
	cmd.Flags().StringVarP(&template, "template", "t", "default", "template to use (default, minimal, full)")

	return cmd
}

func runInit(_ *cobra.Command, args []string, force bool, template string) error {
	filename := "invkfile.cue"
	if len(args) > 0 {
		filename = args[0]
	}

	// Check if file exists
	if _, err := os.Stat(filename); err == nil && !force {
		return fmt.Errorf("file '%s' already exists. Use --force to overwrite", filename)
	}

	// Generate content based on template
	content := generateInvkfile(template)

	// Write file
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	absPath, _ := filepath.Abs(filename)
	fmt.Printf("%s Created %s\n", SuccessStyle.Render("✓"), absPath)
	fmt.Println()
	fmt.Println(SubtitleStyle.Render("Next steps:"))
	fmt.Println("  1. Edit the invkfile to add your commands")
	fmt.Println("  2. Run 'invowk cmd' to see available commands")
	fmt.Println("  3. Run 'invowk cmd <command>' to execute a command")

	return nil
}

func generateInvkfile(template string) string {
	var inv *invkfile.Invkfile

	switch template {
	case "minimal":
		// invkfile.cue contains only commands - module metadata goes in invkmod.cue
		inv = &invkfile.Invkfile{
			Commands: []invkfile.Command{
				{
					Name:        "hello",
					Description: "Print a greeting",
					Implementations: []invkfile.Implementation{
						{
							Script:   "echo 'Hello from invowk!'",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						},
					},
				},
			},
		}

	case "full":
		// invkfile.cue contains only commands - module metadata goes in invkmod.cue
		inv = &invkfile.Invkfile{
			Commands: []invkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Implementations: []invkfile.Implementation{
						{
							Script: "echo \"Building $PROJECT_NAME...\"\ngo build -o bin/app ./...",

							Runtimes: []invkfile.RuntimeConfig{
								{Name: invkfile.RuntimeNative},
								{Name: invkfile.RuntimeContainer, Image: "golang:1.21"},
							},
							Platforms: []invkfile.PlatformConfig{
								{Name: invkfile.PlatformLinux},
								{Name: invkfile.PlatformMac},
								{Name: invkfile.PlatformWindows},
							},
						},
					},
					Env: &invkfile.EnvConfig{
						Vars: map[string]string{
							"PROJECT_NAME": "myproject",
							"CGO_ENABLED":  "0",
						},
					},
				},
				{
					Name:        "test unit",
					Description: "Run unit tests",
					Implementations: []invkfile.Implementation{
						{
							Script:   "go test -v ./...",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}, {Name: invkfile.RuntimeVirtual}},
						},
					},
				},
				{
					Name:        "test integration",
					Description: "Run integration tests",
					Implementations: []invkfile.Implementation{
						{
							Script:   "go test -v -tags=integration ./...",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						},
					},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Implementations: []invkfile.Implementation{
						{
							Script: "rm -rf bin/ dist/",

							Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
							Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}},
						},
						{
							Script: "if exist bin rmdir /s /q bin && if exist dist rmdir /s /q dist",

							Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
							Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformWindows}},
						},
					},
				},
				{
					Name:        "docker-build",
					Description: "Build using container runtime",
					Implementations: []invkfile.Implementation{
						{
							Script:   "go build -o /workspace/bin/app ./...",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer, Image: "golang:1.21"}},
						},
					},
				},
				{
					Name:        "container hello-invowk",
					Description: "Print a greeting from a container",
					Implementations: []invkfile.Implementation{
						{
							Script:   "echo \"Hello, Invowk!\"",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer, Image: "debian:stable-slim"}},
						},
					},
				},
				{
					Name:        "release",
					Description: "Create a release",
					Implementations: []invkfile.Implementation{
						{
							Script: "echo 'Creating release...'",

							Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
							Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}},
						},
					},
					DependsOn: &invkfile.DependsOn{
						Tools: []invkfile.ToolDependency{
							{Alternatives: []string{"git"}},
						},
						Commands: []invkfile.CommandDependency{
							{Alternatives: []string{"clean"}},
							{Alternatives: []string{"build"}},
							{Alternatives: []string{"test unit"}},
						},
					},
				},
			},
		}

	default: // "default"
		// invkfile.cue contains only commands - module metadata goes in invkmod.cue
		inv = &invkfile.Invkfile{
			Commands: []invkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Implementations: []invkfile.Implementation{
						{
							Script:   "echo 'Building...'\n# Add your build commands here",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						},
					},
				},
				{
					Name:        "test",
					Description: "Run tests",
					Implementations: []invkfile.Implementation{
						{
							Script:   "echo 'Testing...'\n# Add your test commands here",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						},
					},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Implementations: []invkfile.Implementation{
						{
							Script:   "echo 'Cleaning...'\n# Add your clean commands here",
							Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
						},
					},
				},
			},
		}
	}

	return invkfile.GenerateCUE(inv)
}
