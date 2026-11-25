// SPDX-License-Identifier: MPL-2.0

package modulelock

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

// PhysicalDir returns dir's physical absolute path: symbolic links resolved,
// and on Windows also junctions, letter case and 8.3 short names, so every
// spelling of one directory gives one path.
func (*FileLocker) PhysicalDir(dir types.FilesystemPath) (types.FilesystemPath, error) {
	if err := dir.Validate(); err != nil {
		return "", fmt.Errorf("directory path: %w", err)
	}
	physical, err := physicalDir(string(dir))
	if err != nil {
		return "", fmt.Errorf("resolve physical path of %s: %w", dir, err)
	}
	result := types.FilesystemPath(physical) //goplint:ignore -- resolved by the OS from a validated path, validated below.
	if err := result.Validate(); err != nil {
		return "", fmt.Errorf("physical path of %s: %w", dir, err)
	}
	return result, nil
}

// trimFinalPathPrefix turns a GetFinalPathNameByHandle result into an
// ordinary path: `\\?\C:\x` becomes `C:\x` and `\\?\UNC\srv\share\x`
// becomes `\\srv\share\x`; a path without the prefix is returned unchanged.
//
//goplint:ignore -- pure string helper over raw Windows path text.
func trimFinalPathPrefix(path string) string {
	if rest, ok := strings.CutPrefix(path, `\\?\UNC\`); ok {
		return `\\` + rest
	}
	if rest, ok := strings.CutPrefix(path, `\\?\`); ok {
		return rest
	}
	return path
}
