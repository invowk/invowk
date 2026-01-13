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
			Version:        "1.0",
			DefaultRuntime: invowkfile.RuntimeNative,
			Commands: []invowkfile.Command{
				{
					Name:        "hello",
					Description: "Print a greeting",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
					Script:      "echo 'Hello from invowk!'",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
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
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative, invowkfile.RuntimeContainer},
					Script:      "echo \"Building $PROJECT_NAME...\"\ngo build -o bin/app ./...",
					Env: map[string]string{
						"CGO_ENABLED": "0",
					},
					WorksOn: invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
				{
					Name:        "test unit",
					Description: "Run unit tests",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative, invowkfile.RuntimeVirtual},
					Script:      "go test -v ./...",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
				{
					Name:        "test integration",
					Description: "Run integration tests",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
					Script:      "go test -v -tags=integration ./...",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
					Script:      "rm -rf bin/ dist/",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac}},
				},
				{
					Name:        "docker-build",
					Description: "Build using container runtime",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeContainer},
					Script:      "go build -o /workspace/bin/app ./...",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
				{
					Name:        "container hello-invowk",
					Description: "Print a greeting from a container",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeContainer},
					Script:      "echo \"Hello, Invowk!\"",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
				{
					Name:        "release",
					Description: "Create a release",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
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
					Script:  "echo 'Creating release...'",
					WorksOn: invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac}},
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
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
					Script:      "echo 'Building...'\n# Add your build commands here",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
				{
					Name:        "test",
					Description: "Run tests",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
					Script:      "echo 'Testing...'\n# Add your test commands here",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
				{
					Name:        "clean",
					Description: "Clean build artifacts",
					Runtimes:    []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
					Script:      "echo 'Cleaning...'\n# Add your clean commands here",
					WorksOn:     invowkfile.WorksOn{Hosts: []invowkfile.HostOS{invowkfile.HostLinux, invowkfile.HostMac, invowkfile.HostWindows}},
				},
			},
		}
	}

	return invowkfile.GenerateCUE(inv)
}
