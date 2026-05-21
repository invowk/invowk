// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestscriptPATHUsesPortableSeparator(t *testing.T) {
	t.Parallel()

	for _, path := range testscriptArchivePaths(t) {
		assertTestscriptPATHUsesPortableSeparator(t, path)
	}
}

func testscriptArchivePaths(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata directory: %v", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}

		paths = append(paths, filepath.Join("testdata", entry.Name()))
	}
	return paths
}

func assertTestscriptPATHUsesPortableSeparator(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(testscriptPreamble(string(data))))
	for lineNo := 1; scanner.Scan(); lineNo++ {
		if testscriptLineUsesLiteralPATHSeparator(scanner.Text()) {
			t.Errorf("%s:%d prepends PATH using a literal separator; use ${:}", path, lineNo)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan %s: %v", path, err)
	}
}

func testscriptPreamble(script string) string {
	if preamble, _, found := strings.Cut(script, "\n-- "); found {
		return preamble
	}
	return script
}

func testscriptLineUsesLiteralPATHSeparator(line string) bool {
	line = strings.TrimSpace(line)
	return strings.Contains(line, "PATH=") &&
		strings.Contains(line, "$PATH") &&
		!strings.Contains(line, "${:}")
}
