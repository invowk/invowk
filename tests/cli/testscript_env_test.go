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

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}

		path := filepath.Join("testdata", entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		script := string(data)
		if marker := strings.Index(script, "\n-- "); marker >= 0 {
			script = script[:marker]
		}

		scanner := bufio.NewScanner(strings.NewReader(script))
		for lineNo := 1; scanner.Scan(); lineNo++ {
			line := strings.TrimSpace(scanner.Text())
			if strings.Contains(line, "PATH=") &&
				strings.Contains(line, "$PATH") &&
				!strings.Contains(line, "${:}") {
				t.Errorf("%s:%d prepends PATH using a literal separator; use ${:}", path, lineNo)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("failed to scan %s: %v", path, err)
		}
	}
}
