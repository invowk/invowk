// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"path/filepath"
	"strings"
)

// MarkExamplesOutsideCanonical sets OutsideCanonical for examples outside the canonical path.
func MarkExamplesOutsideCanonical(examples []Example, canonicalPath string) []Example {
	if canonicalPath == "" || len(examples) == 0 {
		return examples
	}

	canonicalAbs, err := filepath.Abs(canonicalPath)
	if err != nil {
		return examples
	}
	canonicalAbs = filepath.Clean(canonicalAbs)

	updated := append([]Example(nil), examples...)
	for i := range updated {
		file, _ := parseSourceLocation(updated[i].SourceLocation)
		if file == "" {
			continue
		}
		fileAbs, err := filepath.Abs(file)
		if err != nil {
			continue
		}
		if !isWithinDir(fileAbs, canonicalAbs) {
			updated[i].OutsideCanonical = true
		}
	}

	return updated
}

func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
