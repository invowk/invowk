// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

// readScriptFileFacts reads file-based script metadata and contents for content analysis.
// Empty content means the file could not be read or exceeded the scan size cap.
//
//goplint:ignore -- helper resolves raw script path text from invowkfile declarations.
func readScriptFileFacts(ctx context.Context, scriptPath, modulePath string) (scriptFileFacts, error) {
	resolved := strings.TrimSpace(scriptPath)
	if modulePath != "" && !filepath.IsAbs(resolved) {
		resolved = filepath.Join(modulePath, resolved)
	}
	resolvedPath := types.FilesystemPath(resolved) //goplint:ignore -- resolved from validated module/script path inputs.
	facts := scriptFileFacts{Path: resolvedPath}

	// Defense-in-depth: verify the resolved path stays within the module
	// boundary. The invowkfile parser's script path containment check (SC-01)
	// blocks traversal paths at parse time, but the audit scanner should not
	// rely on upstream validation alone.
	if modulePath != "" && !isWithinBoundary(modulePath, resolved) {
		return facts, nil
	}
	if err := scanContextErr(ctx); err != nil {
		return facts, err
	}

	readPath := resolved
	if modulePath != "" {
		evaluated, evalErr := filepath.EvalSymlinks(resolved)
		if evalErr != nil {
			facts.StatErr = evalErr
			return facts, nil //nolint:nilerr // Symlink/path resolution failures are nonfatal scan facts for checkers.
		}
		if !isWithinBoundary(modulePath, evaluated) {
			return facts, nil
		}
		readPath = evaluated
	}

	info, statErr := os.Stat(readPath)
	if statErr != nil {
		facts.StatErr = statErr
		return facts, nil //nolint:nilerr // File stat failures are nonfatal scan facts for checkers.
	}
	facts.Size = info.Size()

	if err := scanContextErr(ctx); err != nil {
		return facts, err
	}
	data, err := os.ReadFile(readPath)
	if err != nil || len(data) > maxScriptFileSize {
		facts.StatErr = err
		return facts, nil //nolint:nilerr // File read failures are nonfatal scan facts for checkers.
	}
	facts.Content = string(data)
	return facts, nil
}

// isWithinBoundary checks whether target resolves to a path within the base
// directory. Used by multiple checkers for module boundary enforcement.
func isWithinBoundary(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
