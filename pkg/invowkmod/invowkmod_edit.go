// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// entrySpan is one top-level {…} entry of the requires list by byte
	// offset: start is the offset of its "{", end the offset just after its
	// matching "}".
	entrySpan struct {
		start int
		end   int
	}

	// requiresBlock is the requires field located by byte offset: field is
	// the offset of the "requires" label, open and close the offsets of the
	// list's "[" and its matching "]".
	requiresBlock struct {
		field   int
		open    int
		close   int
		entries []entrySpan
	}
)

// AddRequirement adds a module requirement to the requires block in invowkmod.cue.
// If no requires block exists, one is appended to the file.
// Returns an error if the file doesn't exist or the requirement is a duplicate
// (same git_url and path).
func AddRequirement(invowkmodPath types.FilesystemPath, req ModuleRef) error {
	if err := req.ValidateDeclaration(); err != nil {
		return fmt.Errorf("invalid requirement declaration: %w", err)
	}

	data, err := os.ReadFile(string(invowkmodPath))
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", invowkmodPath, err)
	}
	text := string(data)

	// Check for duplicate if requires block exists
	block, hasBlock := scanRequiresBlock(text)
	if hasBlock {
		for _, entry := range block.entries {
			entryGitURL, entryPath := entryFields(text, entry)
			if entryGitURL == string(req.GitURL) && entryPath == string(req.Path) {
				identifier := string(req.GitURL)
				if req.Path != "" {
					identifier += "#" + string(req.Path)
				}
				return fmt.Errorf("%w: requirement %s", ErrModuleAlreadyExists, identifier)
			}
		}
	}

	// Build the new entry lines
	entryLines := formatRequiresEntry(req)

	var out string
	switch {
	case hasBlock && strings.TrimSpace(text[lineStart(text, block.close):block.close]) == "":
		// The closing "]" starts its line: insert the entry lines before it.
		at := lineStart(text, block.close)
		out = text[:at] + strings.Join(entryLines, "\n") + "\n" + text[at:]
	case hasBlock:
		// The closing "]" follows other content on its line (a one-line block,
		// or "}]"): split the line at the "]" so the entry lands inside the
		// list, after the existing entries.
		before := strings.TrimRight(text[:block.close], " \t")
		if !strings.HasSuffix(before, ",") && !strings.HasSuffix(before, "[") {
			before += ","
		}
		out = before + "\n" + strings.Join(entryLines, "\n") + "\n" + text[block.close:]
	default:
		// Append requires block at end of file
		// Remove trailing empty lines
		lines := strings.Split(text, "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, "", "requires: [")
		lines = append(lines, entryLines...)
		lines = append(lines, "]", "")
		out = strings.Join(lines, "\n")
	}

	return fspath.AtomicWriteFile(string(invowkmodPath), []byte(out), fspath.DefaultFilePerm)
}

// RemoveRequirement removes a module requirement matching gitURL and subPath
// from the requires block in invowkmod.cue.
// If the requires list becomes empty, the entire block is removed.
// Returns nil if the file doesn't exist or no match is found (idempotent).
//
// RemoveRequirement delegates to RemoveRequirementIfPresent and drops its
// written flag; callers that report whether invowkmod.cue changed call
// RemoveRequirementIfPresent directly.
func RemoveRequirement(invowkmodPath types.FilesystemPath, gitURL GitURL, subPath SubdirectoryPath) error {
	_, err := RemoveRequirementIfPresent(invowkmodPath, gitURL, subPath)
	return err
}

// RemoveRequirementIfPresent removes the requirement exactly as
// RemoveRequirement describes and reports whether it wrote invowkmodPath:
// written is false when the file does not exist, has no requires block or has
// no entry matching gitURL and subPath (the file is left untouched), and true
// only after the atomic write replaced the file with the entry removed. On any
// error (read, write) written is false. The flag is reporting only; it never
// changes when the file is written.
func RemoveRequirementIfPresent(invowkmodPath types.FilesystemPath, gitURL GitURL, subPath SubdirectoryPath) (written bool, err error) {
	pathStr := string(invowkmodPath)
	data, err := os.ReadFile(pathStr)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Idempotent: file doesn't exist
		}
		return false, fmt.Errorf("failed to read %s: %w", invowkmodPath, err)
	}
	text := string(data)

	block, hasBlock := scanRequiresBlock(text)
	if !hasBlock {
		return false, nil // No requires block, nothing to remove
	}

	// Find the matching entry
	matched := -1
	for i, entry := range block.entries {
		entryGitURL, entryPath := entryFields(text, entry)
		if entryGitURL == string(gitURL) && entryPath == string(subPath) {
			matched = i
			break
		}
	}
	if matched < 0 {
		return false, nil // No match found, idempotent
	}

	var out string
	if len(block.entries) == 1 {
		out = removeRequiresField(text, block)
	} else {
		out = removeRequiresEntry(text, block.entries[matched])
	}

	if writeErr := fspath.AtomicWriteFile(pathStr, []byte(out), fspath.DefaultFilePerm); writeErr != nil {
		return false, writeErr
	}
	return true, nil
}

// removeRequiresField removes the whole requires field after its last entry
// is removed. When the field owns its lines (nothing but whitespace before the
// label and after the "]" and an optional comma), those lines go, with one
// blank line after and one before them; otherwise the field is cut out of its
// line.
//
//goplint:ignore -- private source-text rewrite helper operates on raw CUE text.
func removeRequiresField(text string, block requiresBlock) string {
	end := block.close + 1
	if !ownsStart(text, block.field) || !ownsEnd(text, end) {
		cut := skipComma(text, end)
		return text[:block.field] + strings.TrimLeft(text[cut:], " \t")
	}
	lines := strings.Split(text, "\n")
	blockStart := lineIndex(text, block.field)
	blockEnd := lineIndex(text, block.close) + 1

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
	return strings.Join(newLines, "\n")
}

// removeRequiresEntry removes one entry of a list that keeps other entries.
// An entry that owns its lines loses those lines, which keeps the multi-line
// layout module add writes byte-identical. Otherwise the entry is cut with
// the comma after it and the spaces up to the next token; when the line ends
// there, the spaces before the entry go instead. An entry with no comma after
// it (the last one before "]") is cut with the comma and spaces before it.
//
//goplint:ignore -- private source-text rewrite helper operates on raw CUE text.
func removeRequiresEntry(text string, entry entrySpan) string {
	startOwned := ownsStart(text, entry.start)
	endOwned := ownsEnd(text, entry.end)
	if startOwned && endOwned {
		from := lineStart(text, entry.start)
		to := lineEnd(text, entry.end)
		if to < len(text) {
			to++ // the newline
		}
		return text[:from] + text[to:]
	}
	cut := skipComma(text, entry.end)
	if cut == entry.end && !startOwned {
		before := strings.TrimRight(text[:entry.start], " \t")
		before = strings.TrimRight(strings.TrimSuffix(before, ","), " \t")
		return before + text[cut:]
	}
	rest := strings.TrimLeft(text[cut:], " \t")
	if !startOwned && (rest == "" || rest[0] == '\n' || rest[0] == '\r') {
		return strings.TrimRight(text[:entry.start], " \t") + rest
	}
	return text[:entry.start] + rest
}

// scanRequiresBlock finds the top-level requires field, its list and the
// list's top-level entries, skipping string literals and comments.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func scanRequiresBlock(text string) (requiresBlock, bool) {
	field, colon := -1, -1
	depth := 0
	forEachCode(text, 0, len(text), func(i int) bool {
		switch text[i] {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		default:
			if depth == 0 {
				if c, ok := labelAt(text, i, "requires"); ok {
					field, colon = i, c
					return false
				}
			}
		}
		return true
	})
	if field < 0 {
		return requiresBlock{}, false
	}
	open := nextCode(text, colon+1)
	if open >= len(text) || text[open] != '[' {
		return requiresBlock{}, false
	}
	closeAt := -1
	depth = 0
	forEachCode(text, open, len(text), func(i int) bool {
		switch text[i] {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
			if depth == 0 {
				closeAt = i
				return false
			}
		}
		return true
	})
	if closeAt < 0 || text[closeAt] != ']' {
		return requiresBlock{}, false
	}
	return requiresBlock{field: field, open: open, close: closeAt, entries: entrySpans(text, open+1, closeAt)}, true
}

// entrySpans returns the {…} spans at brace depth zero in text[from:to].
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func entrySpans(text string, from, to int) []entrySpan {
	var spans []entrySpan
	depth, start := 0, 0
	forEachCode(text, from, to, func(i int) bool {
		switch text[i] {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 {
				spans = append(spans, entrySpan{start: start, end: i + 1})
			}
		}
		return true
	})
	return spans
}

// entryFields reads the git_url and path string values of one entry: a label
// at the entry's own depth, outside strings and comments, followed by a
// string literal.
//
//goplint:ignore -- private lexical helper returns raw CUE field text.
func entryFields(text string, entry entrySpan) (gitURL, path string) {
	depth := 0
	forEachCode(text, entry.start+1, entry.end-1, func(i int) bool {
		switch text[i] {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		default:
			if depth != 0 {
				return true
			}
			if c, ok := labelAt(text, i, "git_url"); ok {
				gitURL = stringValueAt(text, c+1, entry.end)
			} else if c, ok := labelAt(text, i, "path"); ok {
				path = stringValueAt(text, c+1, entry.end)
			}
		}
		return true
	})
	return gitURL, path
}

// stringValueAt parses the string literal that starts after spaces at from.
//
//goplint:ignore -- private lexical helper returns raw CUE string text.
func stringValueAt(text string, from, limit int) string {
	i := from
	for i < limit && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	if i >= limit || text[i] != '"' {
		return ""
	}
	quoted, err := strconv.QuotedPrefix(text[i:limit])
	if err != nil {
		return ""
	}
	value, err := strconv.Unquote(quoted)
	if err != nil {
		return ""
	}
	return value
}

// labelAt reports whether a field label starts at i: the identifier, a word
// boundary before it, then optional spaces and a colon. It returns the
// colon's offset.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func labelAt(text string, i int, label string) (colon int, ok bool) {
	if !strings.HasPrefix(text[i:], label) || (i > 0 && isIdentByte(text[i-1])) {
		return 0, false
	}
	j := i + len(label)
	if j < len(text) && isIdentByte(text[j]) {
		return 0, false
	}
	for j < len(text) && (text[j] == ' ' || text[j] == '\t') {
		j++
	}
	if j >= len(text) || text[j] != ':' {
		return 0, false
	}
	return j, true
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '#' || c == '$' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// forEachCode calls visit for each byte of text[from:to] outside string
// literals and // comments, until visit returns false.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func forEachCode(text string, from, to int, visit func(i int) bool) {
	for i := skipNonCode(text, from); i < to; i = skipNonCode(text, i+1) {
		if !visit(i) {
			return
		}
	}
}

// nextCode returns the first offset at or after i that holds neither
// whitespace nor a comment nor string-literal text.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func nextCode(text string, i int) int {
	for i = skipNonCode(text, i); i < len(text); i = skipNonCode(text, i+1) {
		if !strings.ContainsRune(" \t\r\n", rune(text[i])) {
			return i
		}
	}
	return len(text)
}

// skipNonCode returns the first offset at or after i that is not inside a
// string literal (double- or single-quoted, or the triple-quoted multi-line
// forms) or a // comment; i must be in code context. A comment ends before
// its newline.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func skipNonCode(text string, i int) int {
	for i < len(text) {
		rest := text[i:]
		switch {
		case strings.HasPrefix(rest, "//"):
			nl := strings.IndexByte(rest, '\n')
			if nl < 0 {
				return len(text)
			}
			i += nl
			return i
		case strings.HasPrefix(rest, `"""`) || strings.HasPrefix(rest, "'''"):
			end := strings.Index(rest[3:], rest[:3])
			if end < 0 {
				return len(text)
			}
			i += 3 + end + 3
		case rest[0] == '"' || rest[0] == '\'':
			i = skipQuoted(text, i)
		default:
			return i
		}
	}
	return i
}

// skipQuoted returns the offset just after the one-line string literal that
// starts at i, or the offset of the newline that ends an unterminated one.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func skipQuoted(text string, i int) int {
	quote := text[i]
	for j := i + 1; j < len(text); j++ {
		switch text[j] {
		case '\\':
			j++
		case quote:
			return j + 1
		case '\n':
			return j
		}
	}
	return len(text)
}

// ownsStart reports whether only whitespace precedes offset on its line.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func ownsStart(text string, offset int) bool {
	return strings.TrimSpace(text[lineStart(text, offset):offset]) == ""
}

// ownsEnd reports whether only whitespace and one optional comma follow
// offset on its line.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func ownsEnd(text string, offset int) bool {
	rest := strings.TrimSpace(text[offset:lineEnd(text, offset)])
	return rest == "" || rest == ","
}

// skipComma returns the offset after the spaces and one comma that follow
// offset, or offset itself when no comma follows on the line.
//
//goplint:ignore -- private lexical helper operates on raw CUE text.
func skipComma(text string, offset int) int {
	j := offset
	for j < len(text) && (text[j] == ' ' || text[j] == '\t') {
		j++
	}
	if j < len(text) && text[j] == ',' {
		return j + 1
	}
	return offset
}

//goplint:ignore -- private lexical helper operates on raw CUE text.
func lineStart(text string, offset int) int {
	return strings.LastIndexByte(text[:offset], '\n') + 1
}

//goplint:ignore -- private lexical helper operates on raw CUE text.
func lineEnd(text string, offset int) int {
	if nl := strings.IndexByte(text[offset:], '\n'); nl >= 0 {
		return offset + nl
	}
	return len(text)
}

//goplint:ignore -- private lexical helper operates on raw CUE text.
func lineIndex(text string, offset int) int {
	return strings.Count(text[:offset], "\n")
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
