// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"strings"
)

// entryBounds represents the line range of a single entry within the requires block.
type entryBounds struct {
	start int // First line (the "{" line), absolute index in file
	end   int // Last line (the "}," or "}" line), absolute index in file
}

// AddRequirement adds a module requirement to the requires block in invowkmod.cue.
// If no requires block exists, one is appended to the file.
// Returns an error if the file doesn't exist or the requirement is a duplicate
// (same git_url and path).
func AddRequirement(invowkmodPath string, req ModuleRef) error {
	data, err := os.ReadFile(invowkmodPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", invowkmodPath, err)
	}

	lines := strings.Split(string(data), "\n")

	// Check for duplicate if requires block exists
	startLine, endLine, hasBlock := findRequiresBlock(lines)
	if hasBlock {
		entries := findEntryBounds(lines, startLine, endLine)
		for _, entry := range entries {
			entryGitURL, entryPath := parseRequiresEntryFields(lines[entry.start : entry.end+1])
			if entryGitURL == string(req.GitURL) && entryPath == string(req.Path) {
				identifier := string(req.GitURL)
				if req.Path != "" {
					identifier += "#" + string(req.Path)
				}
				return fmt.Errorf("requirement already exists: %s", identifier)
			}
		}
	}

	// Build the new entry lines
	entryLines := formatRequiresEntry(req)

	if hasBlock {
		// Insert new entry before the closing "]"
		newLines := make([]string, 0, len(lines)+len(entryLines))
		newLines = append(newLines, lines[:endLine]...)
		newLines = append(newLines, entryLines...)
		newLines = append(newLines, lines[endLine:]...)
		lines = newLines
	} else {
		// Append requires block at end of file
		// Remove trailing empty lines
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, "", "requires: [")
		lines = append(lines, entryLines...)
		lines = append(lines, "]", "")
	}

	return atomicWriteFile(invowkmodPath, []byte(strings.Join(lines, "\n")))
}

// RemoveRequirement removes a module requirement matching gitURL and subPath
// from the requires block in invowkmod.cue.
// If the requires list becomes empty, the entire block is removed.
// Returns nil if the file doesn't exist or no match is found (idempotent).
func RemoveRequirement(invowkmodPath, gitURL, subPath string) error {
	data, err := os.ReadFile(invowkmodPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Idempotent: file doesn't exist
		}
		return fmt.Errorf("failed to read %s: %w", invowkmodPath, err)
	}

	lines := strings.Split(string(data), "\n")

	startLine, endLine, hasBlock := findRequiresBlock(lines)
	if !hasBlock {
		return nil // No requires block, nothing to remove
	}

	// Find entries within the requires block
	entries := findEntryBounds(lines, startLine, endLine)

	// Find the matching entry
	removeIdx := -1
	for i, entry := range entries {
		entryGitURL, entryPath := parseRequiresEntryFields(lines[entry.start : entry.end+1])
		if entryGitURL == gitURL && entryPath == subPath {
			removeIdx = i
			break
		}
	}

	if removeIdx < 0 {
		return nil // No match found, idempotent
	}

	if len(entries) == 1 {
		// Last entry: remove entire requires block including surrounding blank lines
		blockStart := startLine
		blockEnd := endLine + 1

		// Skip trailing blank line after the block
		if blockEnd < len(lines) && strings.TrimSpace(lines[blockEnd]) == "" {
			blockEnd++
		}
		// Skip preceding blank line before the block
		if blockStart > 0 && strings.TrimSpace(lines[blockStart-1]) == "" {
			blockStart--
		}

		newLines := make([]string, 0, len(lines))
		newLines = append(newLines, lines[:blockStart]...)
		newLines = append(newLines, lines[blockEnd:]...)
		lines = newLines
	} else {
		// Remove just the matching entry
		entry := entries[removeIdx]
		newLines := make([]string, 0, len(lines))
		newLines = append(newLines, lines[:entry.start]...)
		newLines = append(newLines, lines[entry.end+1:]...)
		lines = newLines
	}

	return atomicWriteFile(invowkmodPath, []byte(strings.Join(lines, "\n")))
}

// findRequiresBlock locates the requires block boundaries in the file lines.
// Returns the line index of "requires:", the line index of the closing "]",
// and whether the block was found.
func findRequiresBlock(lines []string) (startLine, endLine int, found bool) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if strings.HasPrefix(trimmed, "requires:") {
			startLine = i
			// Find matching closing bracket
			bracketDepth := 0
			for j := i; j < len(lines); j++ {
				for _, ch := range lines[j] {
					switch ch {
					case '[':
						bracketDepth++
					case ']':
						bracketDepth--
						if bracketDepth == 0 {
							return startLine, j, true
						}
					}
				}
			}
			break
		}
	}
	return 0, 0, false
}

// findEntryBounds finds the line ranges of each {} entry within the requires block.
// Uses absolute line indices from the original file.
func findEntryBounds(lines []string, blockStart, blockEnd int) []entryBounds {
	var entries []entryBounds
	braceDepth := 0
	var currentStart int

	for i := blockStart; i <= blockEnd; i++ {
		for _, ch := range lines[i] {
			switch ch {
			case '{':
				if braceDepth == 0 {
					currentStart = i
				}
				braceDepth++
			case '}':
				braceDepth--
				if braceDepth == 0 {
					entries = append(entries, entryBounds{start: currentStart, end: i})
				}
			}
		}
	}

	return entries
}

// parseRequiresEntryFields extracts git_url and path values from a single entry's lines.
// Reuses parseStringValue from lockfile.go since both parse CUE field: "value" lines.
func parseRequiresEntryFields(entryLines []string) (gitURL, path string) {
	for _, line := range entryLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "git_url:") {
			gitURL = parseStringValue(trimmed)
		} else if strings.HasPrefix(trimmed, "path:") {
			path = parseStringValue(trimmed)
		}
	}
	return
}

// formatRequiresEntry formats a ModuleRef as CUE lines for insertion into a requires block.
// Uses tab indentation matching the sample module style.
func formatRequiresEntry(req ModuleRef) []string {
	lines := []string{
		"\t{",
		fmt.Sprintf("\t\tgit_url: %q", req.GitURL),
		fmt.Sprintf("\t\tversion: %q", req.Version),
	}
	if req.Alias != "" {
		lines = append(lines, fmt.Sprintf("\t\talias:   %q", req.Alias))
	}
	if req.Path != "" {
		lines = append(lines, fmt.Sprintf("\t\tpath:    %q", req.Path))
	}
	lines = append(lines, "\t},")
	return lines
}

// atomicWriteFile writes data to a file atomically using temp file + rename.
// Same pattern as lockfile.go Save method.
func atomicWriteFile(path string, data []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Best-effort cleanup
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}
	return nil
}
