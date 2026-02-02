// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ExtractExamples extracts documentation and sample examples.
func ExtractExamples(ctx context.Context, catalog *SourceCatalog) ([]Example, error) {
	if catalog == nil {
		return nil, errors.New("source catalog is nil")
	}

	var examples []Example
	for _, file := range catalog.Files {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}

		ext := strings.ToLower(filepath.Ext(file))
		switch ext {
		case ".md", ".mdx":
			fileExamples, err := extractMarkdownExamples(file)
			if err != nil {
				return nil, err
			}
			examples = append(examples, fileExamples...)
		case ".cue":
			examples = append(examples, Example{
				ID:             exampleID(file, 1),
				SourceLocation: file,
			})
		}
	}

	return examples, nil
}

func extractMarkdownExamples(file string) ([]Example, error) {
	lines, err := ReadFileLines(file)
	if err != nil {
		return nil, err
	}

	var examples []Example
	insideFence := false
	fenceStart := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if insideFence {
				examples = append(examples, Example{
					ID:             exampleID(file, fenceStart),
					SourceLocation: fmt.Sprintf("%s:%d", file, fenceStart),
				})
				insideFence = false
				fenceStart = 0
				continue
			}

			insideFence = true
			fenceStart = i + 1
		}
	}

	return examples, nil
}

func exampleID(file string, line int) string {
	path := filepath.ToSlash(file)
	if line <= 0 {
		return fmt.Sprintf("example:%s", path)
	}
	return fmt.Sprintf("example:%s:%d", path, line)
}
