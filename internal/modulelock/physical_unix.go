// SPDX-License-Identifier: MPL-2.0

//go:build !windows && !darwin

package modulelock

import (
	"fmt"
	"path/filepath"
)

//goplint:ignore -- private helper over raw OS path text.
func physicalDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	physical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve links: %w", err)
	}
	return physical, nil
}
