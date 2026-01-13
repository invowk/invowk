package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"invowk-cli/pkg/invowkfile"
)

// initCmd creates a new invowkfile
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new invowkfile in the current directory",
	Long: `Create a new invowkfile in the current directory with example commands.

This command generates a starter invowkfile with sample commands to help
you get started quickly.`,
	RunE: runInit,
}

var (
	initForce    bool
	initTemplate string
)

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing invowkfile")
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "default", "template to use (default, minimal, full)")
}

func runInit(cmd *cobra.Command, args []string) error {
	filename := "invowkfile.cue"
	if len(args) > 0 {
		filename = args[0]
	}

	// Check if file exists
	if _, err := os.Stat(filename); err == nil && !initForce {
		return fmt.Errorf("file '%s' already exists. Use --force to overwrite", filename)
	}

	// Generate content based on template
	content := generateInvowkfile(initTemplate)

	// Write file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	absPath, _ := filepath.Abs(filename)
	fmt.Printf("%s Created %s\n", successStyle.Render("✓"), absPath)
	fmt.Println()
	fmt.Println(subtitleStyle.Render("Next steps:"))
	fmt.Println("  1. Edit the invowkfile to add your commands")
	fmt.Println("  2. Run 'invowk cmd list' to see available commands")
	fmt.Println("  3. Run 'invowk cmd <command>' to execute a command")

	return nil
}

func generateInvowkfile(template string) string {
	var inv *invowkfile.Invowkfile

	switch template {
	case "minimal":
		inv = &invowkfile.Invowkfile{
			Version: "1.0",
			Commands: []invowkfile.Command{
				{
					Name:        "hello",
					Description: "Print a greeting",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo 'Hello from invowk!'",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}},
						},
					},
				},
			},
		}

	case "full":
		inv = &invowkfile.Invowkfile{
			Version:     "1.0",
			Description: "Full example project commands",
			Commands: []invowkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo \"Building $PROJECT_NAME...\"\ngo build -o bin/app ./...",
							Target: invowkfile.Target{
								Runtimes: []invowkfile.RuntimeConfig{
									{Name: invowkfile.RuntimeNative},
									{Name: invowkfile.RuntimeContainer, Image: "golang:1.21"},
								},
								Platforms: []invowkfile.PlatformConfig{
									{Name: invowkfile.PlatformLinux, Env: map[string]string{"PROJECT_NAME": "myproject"}},
									{Name: invowkfile.PlatformMac, Env: map[string]string{"PROJECT_NAME": "myproject"}},
									{Name: invowkfile.PlatformWindows, Env: map[string]string{"PROJECT_NAME": "myproject"}},
								},
							},
						},
					},
					Env: map[string]string{
						"CGO_ENABLED": "0",
					},
				},
				{
					Name:        "test unit",
					Description: "Run unit tests",
					Implementations: []invowkfile.Implementation{
						{
							Script: "go test -v ./...",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeVirtual}}},
						},
					},
				},
				{
					Name:        "test integration",
					Description: "Run integration tests",
					Implementations: []invowkfile.Implementation{
						{
							Script: "go test -v -tags=integration ./...",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}},
						},
					},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Implementations: []invowkfile.Implementation{
						{
							Script: "rm -rf bin/ dist/",
							Target: invowkfile.Target{
								Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
								Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}},
							},
						},
						{
							Script: "if exist bin rmdir /s /q bin && if exist dist rmdir /s /q dist",
							Target: invowkfile.Target{
								Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
								Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformWindows}},
							},
						},
					},
				},
				{
					Name:        "docker-build",
					Description: "Build using container runtime",
					Implementations: []invowkfile.Implementation{
						{
							Script: "go build -o /workspace/bin/app ./...",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "golang:1.21"}}},
						},
					},
				},
				{
					Name:        "container hello-invowk",
					Description: "Print a greeting from a container",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo \"Hello, Invowk!\"",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "alpine:latest"}}},
						},
					},
				},
				{
					Name:        "release",
					Description: "Create a release",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo 'Creating release...'",
							Target: invowkfile.Target{
								Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
								Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}},
							},
						},
					},
					DependsOn: &invowkfile.DependsOn{
						Tools: []invowkfile.ToolDependency{
							{Name: "git"},
						},
						Commands: []invowkfile.CommandDependency{
							{Name: "clean"},
							{Name: "build"},
							{Name: "test unit"},
						},
					},
				},
			},
		}

	default: // "default"
		inv = &invowkfile.Invowkfile{
			Version:     "1.0",
			Description: "Project commands",
			Commands: []invowkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo 'Building...'\n# Add your build commands here",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}},
						},
					},
				},
				{
					Name:        "test",
					Description: "Run tests",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo 'Testing...'\n# Add your test commands here",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}},
						},
					},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Implementations: []invowkfile.Implementation{
						{
							Script: "echo 'Cleaning...'\n# Add your clean commands here",
							Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}},
						},
					},
				},
			},
		}
	}

	return invowkfile.GenerateCUE(inv)
}
