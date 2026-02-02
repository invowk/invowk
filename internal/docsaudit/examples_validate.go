// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"
)

// ValidateExamples marks examples as valid or invalid with reasons.
func ValidateExamples(ctx context.Context, examples []Example, surfaces []UserFacingSurface) ([]Example, error) {
	if len(examples) == 0 {
		return examples, nil
	}

	knownCommands := collectCommandNames(surfaces)
	updated := append([]Example(nil), examples...)
	for i := range updated {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}

		example := &updated[i]
		file, line := parseSourceLocation(example.SourceLocation)
		if file == "" {
			example.Status = ExampleStatusInvalid
			example.InvalidReason = "missing example source"
			continue
		}

		if _, err := os.Stat(file); err != nil {
			example.Status = ExampleStatusInvalid
			example.InvalidReason = fmt.Sprintf("example source not found: %v", err)
			continue
		}

		if line > 0 {
			reason, err := validateMarkdownExample(file, line, knownCommands)
			if err != nil {
				return nil, err
			}
			if reason != "" {
				example.Status = ExampleStatusInvalid
				example.InvalidReason = reason
			} else {
				example.Status = ExampleStatusValid
				example.InvalidReason = ""
			}
			continue
		}

		reason, err := validateCueExample(file)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			example.Status = ExampleStatusInvalid
			example.InvalidReason = reason
		} else {
			example.Status = ExampleStatusValid
			example.InvalidReason = ""
		}
	}

	return updated, nil
}

func validateMarkdownExample(file string, line int, knownCommands map[string]struct{}) (string, error) {
	lines, readErr := readFileLines(file)
	if readErr != nil {
		return "", readErr
	}
	if line < 1 || line > len(lines) {
		return "example line is out of range", nil
	}

	block, err := extractFenceBlock(lines, line)
	if err != nil {
		return "", err
	}
	if len(block) == 0 {
		return "empty code block", nil
	}

	for _, blockLine := range block {
		matches := invowkCommandPattern.FindAllStringSubmatch(blockLine, -1)
		for _, match := range matches {
			candidate := buildCommandCandidate(match[1])
			if candidate == "" {
				continue
			}
			if shouldSkipDocsOnly(candidate, knownCommands) {
				continue
			}
			if _, ok := knownCommands[normalizeWhitespace(candidate)]; !ok {
				return fmt.Sprintf("unknown command referenced: %s", candidate), nil
			}
		}
	}

	return "", nil
}

func validateCueExample(file string) (string, error) {
	if strings.HasSuffix(filepath.Base(file), "invkfile.cue") {
		if _, err := invkfile.Parse(file); err != nil {
			return fmt.Sprintf("invalid invkfile.cue: %v", err), nil
		}
		return "", nil
	}

	if strings.HasSuffix(filepath.Base(file), "invkmod.cue") {
		if _, err := invkmod.ParseInvkmod(file); err != nil {
			return fmt.Sprintf("invalid invkmod.cue: %v", err), nil
		}
		return "", nil
	}

	return "", nil
}

func extractFenceBlock(lines []string, startLine int) ([]string, error) {
	startIndex := startLine - 1
	if startIndex < 0 || startIndex >= len(lines) {
		return nil, errors.New("fence start is out of range")
	}

	if !strings.HasPrefix(strings.TrimSpace(lines[startIndex]), "```") {
		return nil, errors.New("fence start line does not contain a code fence")
	}

	var block []string
	for i := startIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "```") {
			return block, nil
		}
		block = append(block, lines[i])
	}

	return nil, errors.New("unterminated code fence")
}

func parseSourceLocation(location string) (file string, line int) {
	location = strings.TrimSpace(location)
	if location == "" {
		return "", 0
	}

	index := strings.LastIndex(location, ":")
	if index == -1 {
		return location, 0
	}

	file = location[:index]
	lineText := location[index+1:]
	parsedLine, parseErr := strconv.Atoi(lineText)
	if parseErr != nil || parsedLine <= 0 {
		return location, 0
	}

	return file, parsedLine
}
