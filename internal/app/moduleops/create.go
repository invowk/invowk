// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"context"
	"fmt"
	"os"

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

	modulePath, err := invowkmod.Create(opts)
	if err != nil {
		return "", err
	}
	createdPath := types.FilesystemPath(modulePath)
	if err := createdPath.Validate(); err != nil {
		return "", fmt.Errorf("validate created module path: %w", err)
	}
	return createdPath, nil
}
