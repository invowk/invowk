// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// spanMultiLineTools is the tools entry in the multi-line layout module add
// writes.
const spanMultiLineTools = "requires: [\n\t{\n\t\tgit_url: \"" + spanToolsURL + "\"\n\t\tversion: \"^1.0.0\"\n\t},\n]\n"

// TestRequiresEditSkipsRawStrings checks that quotes, brackets and braces
// inside CUE's #-delimited strings are text, not structure: remove finds the
// real block and add puts the entry inside it, and both results parse.
func TestRequiresEditSkipsRawStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
	}{
		{name: "one-line raw string", description: `description: #"a "[" b"#` + "\n"},
		{name: "raw string with two hashes", description: `description: ##"a "# ]} b"##` + "\n"},
		{name: "multi-line raw string", description: "description: #\"\"\"\n\tsays \"\"\" and [ {\n\t\"\"\"#\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := spanHeader + tt.description + spanMultiLineTools
			path := writeInvowkmodEditFixture(t, content)
			if err := AddRequirement(types.FilesystemPath(path), ModuleRef{GitURL: spanNewURL, Version: "^2.0.0"}); err != nil {
				t.Fatalf("AddRequirement() error = %v", err)
			}
			assertRequiresParse(t, path, []GitURL{spanToolsURL, spanNewURL})

			written, err := RemoveRequirementIfPresent(types.FilesystemPath(path), spanToolsURL, "")
			if err != nil || !written {
				t.Fatalf("RemoveRequirementIfPresent() = %t, %v, want the tools entry removed", written, err)
			}
			assertRequiresParse(t, path, []GitURL{spanNewURL})
		})
	}
}

// TestRemoveRequirementSkipsUnreadablePath checks that a path value Go
// cannot unquote (CUE's \/ escape) is never read as an absent path: removing
// the path-less entry removes that one and keeps the a/b entry.
func TestRemoveRequirementSkipsUnreadablePath(t *testing.T) {
	t.Parallel()

	content := spanHeader + "requires: [\n" +
		"\t{\n\t\tgit_url: \"" + spanToolsURL + "\"\n\t\tversion: \"^1.0.0\"\n\t\tpath:    \"a\\/b\"\n\t},\n" +
		"\t{\n\t\tgit_url: \"" + spanToolsURL + "\"\n\t\tversion: \"^1.0.0\"\n\t},\n]\n"
	path := writeInvowkmodEditFixture(t, content)
	written, err := RemoveRequirementIfPresent(types.FilesystemPath(path), spanToolsURL, "")
	if err != nil || !written {
		t.Fatalf("RemoveRequirementIfPresent() = %t, %v, want the path-less entry removed", written, err)
	}
	mod, err := ParseInvowkmod(types.FilesystemPath(path))
	if err != nil {
		t.Fatalf("ParseInvowkmod() error = %v", err)
	}
	if len(mod.Requires) != 1 || mod.Requires[0].Path != "a/b" {
		t.Fatalf("requires = %+v, want only the a/b entry", mod.Requires)
	}
}

// TestAddRequirementRefusesEditThatBreaksCUE checks the guard: a layout the
// lexer does not read (the fields inside a top-level struct embedding) gets
// an error and an unchanged file, not a second, conflicting requires field.
func TestAddRequirementRefusesEditThatBreaksCUE(t *testing.T) {
	t.Parallel()

	content := "{\n\tmodule: \"io.example.root\"\n\tversion: \"1.0.0\"\n\t" + strings.ReplaceAll(strings.TrimSuffix(spanMultiLineTools, "\n"), "\n", "\n\t") + "\n}\n"
	path := writeInvowkmodEditFixture(t, content)
	if _, err := ParseInvowkmod(types.FilesystemPath(path)); err != nil {
		t.Fatalf("fixture ParseInvowkmod() error = %v, want a valid wrapped file", err)
	}
	err := AddRequirement(types.FilesystemPath(path), ModuleRef{GitURL: spanNewURL, Version: "^2.0.0"})
	if err == nil || !strings.Contains(err.Error(), "would leave invalid CUE") {
		t.Fatalf("AddRequirement() error = %v, want the invalid-CUE refusal", err)
	}
	assertInvowkmodEditFile(t, path, content)
}

// TestRequiresEditCRLF covers the line endings Git for Windows checks
// invowkmod.cue out with: add to a one-line block and to a "}]" block, and
// remove from a one-line block, all with CRLF, give files that parse.
func TestRequiresEditCRLF(t *testing.T) {
	t.Parallel()

	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	addCases := map[string]string{
		"one-line block": crlf(spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}]` + "\n"),
		"}] block":       crlf(spanHeader + "requires: [\n\t{\n\t\tgit_url: \"" + spanToolsURL + "\"\n\t\tversion: \"^1.0.0\"\n\t}]\n"),
		"multi-line":     crlf(spanHeader + spanMultiLineTools),
	}
	for name, content := range addCases {
		t.Run("add "+name, func(t *testing.T) {
			t.Parallel()
			path := writeInvowkmodEditFixture(t, content)
			if err := AddRequirement(types.FilesystemPath(path), ModuleRef{GitURL: spanNewURL, Version: "^2.0.0"}); err != nil {
				t.Fatalf("AddRequirement() error = %v", err)
			}
			assertRequiresParse(t, path, []GitURL{spanToolsURL, spanNewURL})
		})
	}

	t.Run("remove from one-line block", func(t *testing.T) {
		t.Parallel()
		content := crlf(spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}, {git_url: "` + spanUtilsURL + `", version: "^1.0.0"}]` + "\n")
		path := writeInvowkmodEditFixture(t, content)
		written, err := RemoveRequirementIfPresent(types.FilesystemPath(path), spanToolsURL, "")
		if err != nil || !written {
			t.Fatalf("RemoveRequirementIfPresent() = %t, %v, want the tools entry removed", written, err)
		}
		assertRequiresParse(t, path, []GitURL{spanUtilsURL})
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !strings.HasSuffix(string(data), "\r\n") {
			t.Fatalf("file = %q, want its CRLF line end kept", data)
		}
	})
}
