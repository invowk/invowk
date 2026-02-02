// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	invowkCommandPattern = regexp.MustCompile(`(?i)\binvowk\s+([a-z0-9][a-z0-9_-]*(?:\s+[a-z0-9][a-z0-9_-]*){0,4})`)
	flagPattern          = regexp.MustCompile(`--[a-zA-Z0-9][a-zA-Z0-9_-]*`)
)

type surfaceMatcher struct {
	index int
	terms []string
}

// MatchDocumentation maps documentation references to surfaces and detects docs-only features.
func MatchDocumentation(ctx context.Context, catalog *SourceCatalog, surfaces []UserFacingSurface) ([]UserFacingSurface, []Finding, error) {
	if catalog == nil {
		return nil, nil, errors.New("source catalog is nil")
	}

	updated := append([]UserFacingSurface(nil), surfaces...)
	if len(updated) == 0 {
		return updated, nil, nil
	}

	matchers := buildSurfaceMatchers(updated)
	refSeen := make(map[string]struct{})
	knownCommands := collectCommandNames(updated)
	knownFlags := collectFlagNames(updated)

	var findings []Finding
	for _, file := range catalog.Files {
		if err := checkContext(ctx); err != nil {
			return nil, nil, err
		}

		lines, readErr := readFileLines(file)
		if readErr != nil {
			return nil, nil, readErr
		}

		sourceID := sourceIDForFile(catalog, file)
		for lineIndex, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lineNumber := lineIndex + 1
			for _, matcher := range matchers {
				if !lineMatchesTerms(line, matcher.terms) {
					continue
				}
				surface := &updated[matcher.index]
				refKey := fmt.Sprintf("%s|%s|%d", surface.ID, file, lineNumber)
				if _, ok := refSeen[refKey]; ok {
					continue
				}
				refSeen[refKey] = struct{}{}
				surface.DocumentationRefs = append(surface.DocumentationRefs, DocReference{
					SourceID: sourceID,
					FilePath: file,
					Line:     lineNumber,
					Snippet:  trimSnippet(line, 200),
				})
			}
		}

		findings = append(findings, detectDocsOnlyFindings(file, lines, knownCommands, knownFlags)...)
	}

	return updated, findings, nil
}

func buildSurfaceMatchers(surfaces []UserFacingSurface) []surfaceMatcher {
	flagCounts := countFlagNames(surfaces)
	matchers := make([]surfaceMatcher, 0, len(surfaces))
	for i, surface := range surfaces {
		terms := surfaceMatchTerms(surface, flagCounts)
		if len(terms) == 0 {
			continue
		}
		matchers = append(matchers, surfaceMatcher{index: i, terms: terms})
	}
	return matchers
}

func surfaceMatchTerms(surface UserFacingSurface, flagCounts map[string]int) []string {
	name := strings.TrimSpace(surface.Name)
	if name == "" {
		return nil
	}

	switch surface.Type {
	case SurfaceTypeFlag:
		terms := []string{name}
		flagToken := extractFlagToken(name)
		if flagToken != "" && flagCounts[flagToken] == 1 {
			terms = append(terms, flagToken)
		}
		return terms
	case SurfaceTypeCommand, SurfaceTypeConfigField, SurfaceTypeModule, SurfaceTypeBehavior:
		return []string{name}
	default:
		return []string{name}
	}
}

func lineMatchesTerms(line string, terms []string) bool {
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(line, term) {
			return true
		}
	}
	return false
}

func collectCommandNames(surfaces []UserFacingSurface) map[string]struct{} {
	known := make(map[string]struct{})
	for _, surface := range surfaces {
		if surface.Type != SurfaceTypeCommand {
			continue
		}
		name := normalizeWhitespace(surface.Name)
		if name == "" {
			continue
		}
		known[name] = struct{}{}
	}
	return known
}

func collectFlagNames(surfaces []UserFacingSurface) map[string]struct{} {
	known := make(map[string]struct{})
	for _, surface := range surfaces {
		if surface.Type != SurfaceTypeFlag {
			continue
		}
		flagToken := extractFlagToken(surface.Name)
		if flagToken == "" {
			continue
		}
		known[flagToken] = struct{}{}
	}
	return known
}

func countFlagNames(surfaces []UserFacingSurface) map[string]int {
	counts := make(map[string]int)
	for _, surface := range surfaces {
		if surface.Type != SurfaceTypeFlag {
			continue
		}
		flagToken := extractFlagToken(surface.Name)
		if flagToken == "" {
			continue
		}
		counts[flagToken]++
	}
	return counts
}

func extractFlagToken(value string) string {
	parts := strings.Fields(value)
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "--") {
			return parts[i]
		}
	}
	return ""
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func detectDocsOnlyFindings(file string, lines []string, knownCommands, knownFlags map[string]struct{}) []Finding {
	var findings []Finding
	seen := make(map[string]struct{})

	for lineIndex, line := range lines {
		lineNumber := lineIndex + 1
		matches := invowkCommandPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			candidate := buildCommandCandidate(match[1])
			if candidate == "" {
				continue
			}
			if shouldSkipDocsOnly(candidate, knownCommands) {
				continue
			}

			key := fmt.Sprintf("cmd|%s|%s|%d", candidate, file, lineNumber)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			findings = append(findings, Finding{
				ID:               fmt.Sprintf("docs-only:%s:%d", sanitizeID(candidate), lineNumber),
				MismatchType:     MismatchTypeOutdated,
				SourceLocation:   fmt.Sprintf("%s:%d", file, lineNumber),
				Summary:          fmt.Sprintf("Docs reference command not present: %s", candidate),
				ExpectedBehavior: "Command is not available in the current CLI surface.",
				Recommendation:   "Remove or update the documentation, or implement the referenced command.",
			})
		}

		if !strings.Contains(strings.ToLower(line), "invowk") {
			continue
		}

		for _, flag := range flagPattern.FindAllString(line, -1) {
			if _, ok := knownFlags[flag]; ok {
				continue
			}

			key := fmt.Sprintf("flag|%s|%s|%d", flag, file, lineNumber)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			findings = append(findings, Finding{
				ID:               fmt.Sprintf("docs-only:%s:%d", sanitizeID(flag), lineNumber),
				MismatchType:     MismatchTypeOutdated,
				SourceLocation:   fmt.Sprintf("%s:%d", file, lineNumber),
				Summary:          fmt.Sprintf("Docs reference flag not present: %s", flag),
				ExpectedBehavior: "Flag is not available in the current CLI surface.",
				Recommendation:   "Remove or update the documentation, or implement the referenced flag.",
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].ID < findings[j].ID
	})

	return findings
}

func buildCommandCandidate(candidate string) string {
	value := normalizeWhitespace(candidate)
	if value == "" {
		return ""
	}
	return "invowk " + value
}

func shouldSkipDocsOnly(candidate string, knownCommands map[string]struct{}) bool {
	if candidate == "" {
		return true
	}

	if _, ok := knownCommands[normalizeWhitespace(candidate)]; ok {
		return true
	}

	parts := strings.Fields(candidate)
	if len(parts) < 2 {
		return false
	}

	if strings.EqualFold(parts[1], "cmd") {
		return true
	}

	prefix := strings.Join(parts[:2], " ")
	if _, ok := knownCommands[normalizeWhitespace(prefix)]; ok {
		return false
	}

	return false
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, ":", "-")
	value = strings.ReplaceAll(value, "/", "-")
	return value
}

func sourceIDForFile(catalog *SourceCatalog, file string) string {
	if catalog == nil {
		return ""
	}
	if source, ok := catalog.FileToSource[file]; ok {
		if source.ID != "" {
			return source.ID
		}
		if source.Location != "" {
			return source.Location
		}
	}
	return file
}

func trimSnippet(line string, maxLen int) string {
	trimmed := strings.TrimSpace(line)
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}
	if maxLen <= 3 {
		return trimmed[:maxLen]
	}
	return trimmed[:maxLen-3] + "..."
}
