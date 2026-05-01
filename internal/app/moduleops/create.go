// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// CreateModule creates a new module using application-owned path defaults.
func CreateModule(ctx context.Context, opts invowkmod.CreateOptions) (types.FilesystemPath, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("create module: %w", err)
	}

	if opts.ParentDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current directory: %w", err)
		}
		parentDir := types.FilesystemPath(wd)
		if err := parentDir.Validate(); err != nil {
			return "", fmt.Errorf("validate current directory: %w", err)
		}
		opts.ParentDir = parentDir
	}

	scaffold, err := invowkmod.NewModuleScaffold(opts)
	if err != nil {
		return "", err
	}

	absParentDir, err := filepath.Abs(string(opts.ParentDir))
	if err != nil {
		return "", fmt.Errorf("resolve parent directory: %w", err)
	}
	modulePath := filepath.Join(absParentDir, scaffold.DirectoryName().String())
	if _, err := os.Stat(modulePath); err == nil {
		return "", fmt.Errorf("%w at %s", invowkmod.ErrModuleAlreadyExists, modulePath)
	}
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		return "", fmt.Errorf("create module directory: %w", err)
	}
	createdPath := types.FilesystemPath(modulePath) //goplint:ignore -- validated before file writes.
	if err := createdPath.Validate(); err != nil {
		_ = os.RemoveAll(modulePath)
		return "", fmt.Errorf("validate created module path: %w", err)
	}
	if err := writeModuleScaffold(createdPath, scaffold); err != nil {
		_ = os.RemoveAll(modulePath)
		return "", err
	}

	return createdPath, nil
}

func writeModuleScaffold(modulePath types.FilesystemPath, scaffold invowkmod.ModuleScaffold) error {
	modulePathStr := string(modulePath)
	invowkmodPath := filepath.Join(modulePathStr, "invowkmod.cue")
	if err := os.WriteFile(invowkmodPath, []byte(scaffold.InvowkmodContent().String()), 0o644); err != nil {
		return fmt.Errorf("create invowkmod.cue: %w", err)
	}

	invowkfilePath := filepath.Join(modulePathStr, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(scaffold.InvowkfileContent().String()), 0o644); err != nil {
		return fmt.Errorf("create invowkfile.cue: %w", err)
	}

	if !scaffold.CreateScriptsDir() {
		return nil
	}

	scriptsDir := filepath.Join(modulePathStr, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		return fmt.Errorf("create scripts directory: %w", err)
	}
	gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
	if err := os.WriteFile(gitkeepPath, nil, 0o644); err != nil {
		return fmt.Errorf("create .gitkeep: %w", err)
	}
	return nil
}
