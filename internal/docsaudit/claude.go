// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	claudeLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

	claudeFileExtensions = map[string]struct{}{
		".cue":  {},
		".go":   {},
		".json": {},
		".md":   {},
		".mod":  {},
		".sh":   {},
		".sum":  {},
		".toml": {},
		".txt":  {},
		".yaml": {},
		".yml":  {},
	}
)

// AuditClaudeReferences validates references in .claude docs and rules.
func AuditClaudeReferences(ctx context.Context, repoRoot string) ([]Finding, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("repo root is required")
	}

	absRoot, absErr := filepath.Abs(repoRoot)
	if absErr != nil {
		return nil, fmt.Errorf("resolve repo root: %w", absErr)
	}
	repoRoot = absRoot

	claudeDir := filepath.Join(repoRoot, ".claude")
	if _, statErr := os.Stat(claudeDir); statErr != nil {
		if os.IsNotExist(statErr) {
			return []Finding{missingClaudeFinding(".claude")}, nil
		}
		return nil, fmt.Errorf("stat .claude: %w", statErr)
	}

	claudeFile := filepath.Join(claudeDir, "CLAUDE.md")
	if _, statErr := os.Stat(claudeFile); statErr != nil {
		if os.IsNotExist(statErr) {
			return []Finding{missingClaudeFinding(".claude/CLAUDE.md")}, nil
		}
		return nil, fmt.Errorf("stat .claude/CLAUDE.md: %w", statErr)
	}

	rulesDir := filepath.Join(claudeDir, "rules")
	ruleFiles, ruleErr := listClaudeRuleFiles(rulesDir)
	if ruleErr != nil {
		if os.IsNotExist(ruleErr) {
			return []Finding{missingClaudeFinding(".claude/rules")}, nil
		}
		return nil, ruleErr
	}

	files := append([]string{claudeFile}, ruleFiles...)
	var findings []Finding
	for _, file := range files {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return nil, ctxErr
		}

		lines, readErr := readFileLines(file)
		if readErr != nil {
			return nil, readErr
		}
		fileFindings, findErr := detectClaudeReferenceFindings(file, lines, repoRoot)
		if findErr != nil {
			return nil, findErr
		}
		findings = append(findings, fileFindings...)
	}

	indexFindings, indexErr := detectRulesIndexFindings(claudeFile, ruleFiles, repoRoot)
	if indexErr != nil {
		return nil, indexErr
	}
	findings = append(findings, indexFindings...)

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].ID < findings[j].ID
	})

	return findings, nil
}

func listClaudeRuleFiles(rulesDir string) ([]string, error) {
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return nil, fmt.Errorf("read .claude/rules: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		files = append(files, filepath.Join(rulesDir, name))
	}

	sort.Strings(files)
	return files, nil
}

func detectClaudeReferenceFindings(file string, lines []string, repoRoot string) ([]Finding, error) {
	var findings []Finding
	seen := make(map[string]struct{})

	for lineIndex, line := range lines {
		lineNumber := lineIndex + 1
		for _, target := range extractMarkdownLinkTargets(line) {
			ref := normalizeClaudeReference(target)
			if ref == "" {
				continue
			}
			seen[fmt.Sprintf("%s|%d|%s", file, lineNumber, ref)] = struct{}{}
			finding, ok, err := validateClaudeReference(file, lineNumber, ref, repoRoot)
			if err != nil {
				return findings, err
			}
			if ok {
				findings = append(findings, finding)
			}
		}

		lineSansLinks := claudeLinkPattern.ReplaceAllString(line, "")
		for _, token := range extractPathTokens(lineSansLinks) {
			ref := normalizeClaudeReference(token)
			if ref == "" {
				continue
			}
			key := fmt.Sprintf("%s|%d|%s", file, lineNumber, ref)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			finding, ok, err := validateClaudeReference(file, lineNumber, ref, repoRoot)
			if err != nil {
				return findings, err
			}
			if ok {
				findings = append(findings, finding)
			}
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].ID < findings[j].ID
	})

	return findings, nil
}

func extractMarkdownLinkTargets(line string) []string {
	matches := claudeLinkPattern.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil
	}
	targets := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		targets = append(targets, match[1])
	}
	return targets
}

func extractPathTokens(line string) []string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		candidate := strings.Trim(field, "[](){}<>.,;:\"'`|")
		candidate = strings.TrimSuffix(candidate, ":")
		candidate = strings.TrimSuffix(candidate, ";")
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if !looksLikePathToken(candidate) {
			continue
		}
		paths = append(paths, candidate)
	}
	return paths
}

func normalizeClaudeReference(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.Trim(ref, "<>")
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "#") {
		return ""
	}
	if isExternalReference(ref) {
		return ""
	}
	if isIgnoredPath(ref) {
		return ""
	}
	if hashIndex := strings.Index(ref, "#"); hashIndex != -1 {
		ref = ref[:hashIndex]
	}
	return strings.TrimSpace(ref)
}

func validateClaudeReference(file string, lineNumber int, ref, repoRoot string) (Finding, bool, error) {
	resolved := resolveClaudeReferencePath(ref, file, repoRoot)
	if resolved == "" {
		return Finding{}, false, nil
	}
	if _, err := os.Stat(resolved); err != nil {
		if os.IsNotExist(err) {
			return Finding{
				ID:               fmt.Sprintf("claude-ref:%s:%d", sanitizeID(ref), lineNumber),
				MismatchType:     MismatchTypeIncorrect,
				SourceLocation:   fmt.Sprintf("%s:%d", file, lineNumber),
				Summary:          fmt.Sprintf("Broken reference to %s", ref),
				ExpectedBehavior: "Reference should resolve to a repository path.",
				Recommendation:   "Update the path or remove the reference.",
			}, true, nil
		}
		return Finding{}, false, fmt.Errorf("stat %s: %w", resolved, err)
	}
	return Finding{}, false, nil
}

func resolveClaudeReferencePath(ref, file, repoRoot string) string {
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "/") {
		return ""
	}
	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") {
		return filepath.Clean(filepath.Join(filepath.Dir(file), ref))
	}
	return filepath.Clean(filepath.Join(repoRoot, ref))
}

func looksLikePathToken(token string) bool {
	if token == "" {
		return false
	}
	if isExternalReference(token) || isIgnoredPath(token) {
		return false
	}
	if strings.HasPrefix(token, "#") {
		return false
	}
	if len(token) >= 2 && token[1] == ':' {
		return false
	}

	ext := strings.ToLower(filepath.Ext(token))
	if ext == "" {
		return false
	}
	if _, ok := claudeFileExtensions[ext]; !ok {
		return false
	}
	return true
}

func isExternalReference(ref string) bool {
	lower := strings.ToLower(ref)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "ssh://") ||
		strings.HasPrefix(lower, "git@")
}

func isIgnoredPath(ref string) bool {
	if strings.HasPrefix(ref, "~") {
		return true
	}
	if strings.Contains(ref, "$") || strings.Contains(ref, "%") {
		return true
	}
	if strings.Contains(ref, "\\") {
		return true
	}
	return false
}

func detectRulesIndexFindings(claudeFile string, ruleFiles []string, repoRoot string) ([]Finding, error) {
	lines, readErr := readFileLines(claudeFile)
	if readErr != nil {
		return nil, readErr
	}

	indexEntries := extractRulesIndexEntries(lines)
	actualEntries := make(map[string]struct{}, len(ruleFiles))
	for _, file := range ruleFiles {
		rel, err := filepath.Rel(repoRoot, file)
		if err != nil {
			return nil, fmt.Errorf("rel path: %w", err)
		}
		actualEntries[filepath.ToSlash(rel)] = struct{}{}
	}

	var findings []Finding
	for path := range actualEntries {
		if _, ok := indexEntries[path]; ok {
			continue
		}
		findings = append(findings, Finding{
			ID:               fmt.Sprintf("claude-index-missing:%s", sanitizeID(path)),
			MismatchType:     MismatchTypeInconsistent,
			SourceLocation:   claudeFile,
			Summary:          fmt.Sprintf("Rules index missing entry: %s", path),
			ExpectedBehavior: "Index must list every file under .claude/rules.",
			Recommendation:   "Add the missing rule to the index/sync map.",
		})
	}

	for path, line := range indexEntries {
		if _, ok := actualEntries[path]; ok {
			continue
		}
		location := claudeFile
		if line > 0 {
			location = fmt.Sprintf("%s:%d", claudeFile, line)
		}
		findings = append(findings, Finding{
			ID:               fmt.Sprintf("claude-index-extra:%s", sanitizeID(path)),
			MismatchType:     MismatchTypeInconsistent,
			SourceLocation:   location,
			Summary:          fmt.Sprintf("Rules index references missing file: %s", path),
			ExpectedBehavior: "Index should only list files that exist under .claude/rules.",
			Recommendation:   "Remove the entry or restore the referenced rule file.",
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].ID < findings[j].ID
	})

	return findings, nil
}

func extractRulesIndexEntries(lines []string) map[string]int {
	entries := make(map[string]int)
	inIndex := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "**Index / Sync Map") {
			inIndex = true
			continue
		}
		if !inIndex {
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			break
		}
		if !strings.HasPrefix(trimmed, "-") {
			continue
		}
		for _, target := range extractMarkdownLinkTargets(line) {
			ref := normalizeClaudeReference(target)
			if ref == "" {
				continue
			}
			if !strings.Contains(ref, ".claude/rules/") {
				continue
			}
			entries[filepath.ToSlash(ref)] = i + 1
		}
	}
	return entries
}

func missingClaudeFinding(path string) Finding {
	return Finding{
		ID:               fmt.Sprintf("claude-missing:%s", sanitizeID(path)),
		MismatchType:     MismatchTypeMissing,
		SourceLocation:   path,
		Summary:          fmt.Sprintf("Missing .claude path: %s", path),
		ExpectedBehavior: "The .claude documentation set should be present in the repository.",
		Recommendation:   "Restore the missing path or remove references to it.",
	}
}
