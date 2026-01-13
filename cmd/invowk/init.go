package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
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
	filename := "invowkfile.toml"
	if len(args) > 0 {
		filename = args[0]
	}

	// Check if file exists
	if _, err := os.Stat(filename); err == nil && !initForce {
		return fmt.Errorf("file '%s' already exists. Use --force to overwrite", filename)
	}

	// Generate content based on template
	content, err := generateInvowkfile(initTemplate)
	if err != nil {
		return err
	}

	// Write file
	if err := os.WriteFile(filename, content, 0644); err != nil {
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

func generateInvowkfile(template string) ([]byte, error) {
	var inv *invowkfile.Invowkfile

	switch template {
	case "minimal":
		inv = &invowkfile.Invowkfile{
			Version:        "1.0",
			DefaultRuntime: invowkfile.RuntimeNative,
			Commands: []invowkfile.Command{
				{
					Name:        "hello",
					Description: "Print a greeting",
					Script:      "echo 'Hello from invowk!'",
				},
			},
		}

	case "full":
		inv = &invowkfile.Invowkfile{
			Version:        "1.0",
			Description:    "Full example project commands",
			DefaultRuntime: invowkfile.RuntimeNative,
			DefaultShell:   "",
			Container: invowkfile.ContainerConfig{
				Dockerfile: "Dockerfile",
				Image:      "alpine:latest",
			},
			Env: map[string]string{
				"PROJECT_NAME": "myproject",
			},
			Commands: []invowkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Script:      "echo \"Building $PROJECT_NAME...\"\ngo build -o bin/app ./...",
					Env: map[string]string{
						"CGO_ENABLED": "0",
					},
				},
				{
					Name:        "test unit",
					Description: "Run unit tests",
					Script:      "go test -v ./...",
				},
				{
					Name:        "test integration",
					Description: "Run integration tests",
					Script:      "go test -v -tags=integration ./...",
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Script:      "rm -rf bin/ dist/",
				},
				{
					Name:        "docker-build",
					Description: "Build using container runtime",
					Runtime:     invowkfile.RuntimeContainer,
					Script:      "go build -o /workspace/bin/app ./...",
				},
				{
					Name:        "container hello-invowk",
					Description: "Print a greeting from a container",
					Runtime:     invowkfile.RuntimeContainer,
					Script:      "echo \"Hello, Invowk!\"",
				},
				{
					Name:        "release",
					Description: "Create a release",
					DependsOn:   []string{"clean", "build", "test unit"},
					Script:      "echo 'Creating release...'",
				},
			},
		}

	default: // "default"
		inv = &invowkfile.Invowkfile{
			Version:        "1.0",
			Description:    "Project commands",
			DefaultRuntime: invowkfile.RuntimeNative,
			Commands: []invowkfile.Command{
				{
					Name:        "build",
					Description: "Build the project",
					Script:      "echo 'Building...'\n# Add your build commands here",
				},
				{
					Name:        "test",
					Description: "Run tests",
					Script:      "echo 'Testing...'\n# Add your test commands here",
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Script:      "echo 'Cleaning...'\n# Add your clean commands here",
				},
			},
		}
	}

	header := []byte(`# Invowkfile - Command definitions for invowk
# See https://github.com/invowk/invowk for documentation
#
# Available runtimes:
#   - native: Use system shell (default)
#   - virtual: Use built-in sh interpreter
#   - container: Run in Docker/Podman container
#
# Script can be:
#   - Inline shell commands (single or multi-line using ''' in TOML)
#   - A path to a script file (e.g., ./scripts/build.sh)
#
# Example command with inline script:
#   [[commands]]
#   name = "build"
#   description = "Build the project"
#   script = '''
#   echo "Building..."
#   go build ./...
#   '''
#
# Example command with script file:
#   [[commands]]
#   name = "deploy"
#   script = "./scripts/deploy.sh"
#
# Use spaces in names for subcommand-like behavior:
#   [[commands]]
#   name = "test unit"
#   script = "go test ./..."

`)

	content, err := toml.Marshal(inv)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invowkfile: %w", err)
	}

	return append(header, content...), nil
}
