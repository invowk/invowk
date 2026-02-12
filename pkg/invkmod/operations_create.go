// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"fmt"
	"os"
	"path/filepath"
)

// Create creates a new module with the given options.
// Returns the path to the created module or an error.
func Create(opts CreateOptions) (string, error) {
	// Validate module name
	if opts.Name == "" {
		return "", fmt.Errorf("module name cannot be empty")
	}

	// Validate the name format
	if !moduleNameRegex.MatchString(opts.Name) {
		return "", fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", opts.Name)
	}

	// Default parent directory to current directory
	parentDir := opts.ParentDir
	if parentDir == "" {
		var err error
		parentDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Resolve absolute path
	absParentDir, err := filepath.Abs(parentDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve parent directory: %w", err)
	}

	// Create module directory
	moduleDirName := opts.Name + ModuleSuffix
	modulePath := filepath.Join(absParentDir, moduleDirName)

	// Check if module already exists
	if _, err := os.Stat(modulePath); err == nil {
		return "", fmt.Errorf("module already exists at %s", modulePath)
	}

	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create module directory: %w", err)
	}

	// Use name as module identifier if not specified
	moduleID := opts.Module
	if moduleID == "" {
		moduleID = opts.Name
	}

	// Create description
	description := opts.Description
	if description == "" {
		description = fmt.Sprintf("Commands from %s module", opts.Name)
	}

	// Create invkmod.cue (module metadata)
	invkmodContent := fmt.Sprintf(`// Invkmod - Module metadata for %s
// See https://github.com/invowk/invowk for documentation

module: %q
version: "1.0.0"
description: %q

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invkmod.git"
//         version: "^1.0.0"
//     },
// ]
`, opts.Name, moduleID, description)

	invkmodPath := filepath.Join(modulePath, "invkmod.cue")
	if err := os.WriteFile(invkmodPath, []byte(invkmodContent), 0o644); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
		return "", fmt.Errorf("failed to create invkmod.cue: %w", err)
	}

	// Create invkfile.cue (command definitions only)
	invkfileContent := fmt.Sprintf(`// Invkfile - Command definitions for %s module
// See https://github.com/invowk/invowk for documentation

cmds: [
	{
		name:        "hello"
		description: "A sample command"
		implementations: [
			{
				script: "echo \"Hello from %s!\""
				runtimes: [
					{name: "native"},
					{name: "virtual"},
				]
			},
		]
	},
]
`, opts.Name, opts.Name)

	invkfilePath := filepath.Join(modulePath, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0o644); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
		return "", fmt.Errorf("failed to create invkfile.cue: %w", err)
	}

	// Optionally create scripts directory
	if opts.CreateScriptsDir {
		scriptsDir := filepath.Join(modulePath, "scripts")
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			// Clean up on failure
			_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
			return "", fmt.Errorf("failed to create scripts directory: %w", err)
		}

		// Create a placeholder .gitkeep file
		gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
		if err := os.WriteFile(gitkeepPath, []byte(""), 0o644); err != nil {
			// Clean up on failure
			_ = os.RemoveAll(modulePath) // Best-effort cleanup on error path
			return "", fmt.Errorf("failed to create .gitkeep: %w", err)
		}
	}

	return modulePath, nil
}
