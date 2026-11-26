// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

const (
	spanHeader   = "module: \"io.example.root\"\nversion: \"1.0.0\"\n"
	spanToolsURL = "https://github.com/user/tools.git"
	spanUtilsURL = "https://github.com/user/utils.git"
	spanNewURL   = "https://github.com/user/new.git"
)

// TestAddRequirementOneLineBlock is the OQ-A regression: add to a requires
// block written on one line, or whose last line is "{…}]", puts the new entry
// inside the list after the existing one, and the file stays valid CUE. On
// the base the entry landed before the "requires:" line.
func TestAddRequirementOneLineBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "whole block on one line", content: spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}]` + "\n"},
		{name: "one-line block with trailing comma", content: spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"},]` + "\n"},
		{name: "last line closes entry and list", content: spanHeader + "requires: [\n\t{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\"}]\n"},
		{name: "multi-line entry closed with }]", content: spanHeader + "requires: [\n\t{\n\t\tgit_url: \"" + spanToolsURL + "\"\n\t\tversion: \"^1.0.0\"\n\t}]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeInvowkmodEditFixture(t, tt.content)
			if err := AddRequirement(types.FilesystemPath(path), ModuleRef{GitURL: spanNewURL, Version: "^2.0.0"}); err != nil {
				t.Fatalf("AddRequirement() error = %v", err)
			}
			assertRequiresParse(t, path, []GitURL{spanToolsURL, spanNewURL})
		})
	}
}

// TestRemoveRequirementOneLineEntries covers removal of entries written on
// one line; every result must parse, with the other entries unchanged.
func TestRemoveRequirementOneLineEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		gitURL      GitURL
		wantURLs    []GitURL
		wantContent string
	}{
		{
			name:        "one-line entry among multi-line entries",
			content:     spanHeader + "requires: [\n\t{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\"},\n\t{\n\t\tgit_url: \"" + spanUtilsURL + "\"\n\t\tversion: \"^2.0.0\"\n\t},\n]\n",
			gitURL:      spanToolsURL,
			wantURLs:    []GitURL{spanUtilsURL},
			wantContent: spanHeader + "requires: [\n\t{\n\t\tgit_url: \"" + spanUtilsURL + "\"\n\t\tversion: \"^2.0.0\"\n\t},\n]\n",
		},
		{
			name:        "first of two entries on one line",
			content:     spanHeader + "requires: [\n\t{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\"}, {git_url: \"" + spanUtilsURL + "\", version: \"^2.0.0\"},\n]\n",
			gitURL:      spanToolsURL,
			wantURLs:    []GitURL{spanUtilsURL},
			wantContent: spanHeader + "requires: [\n\t{git_url: \"" + spanUtilsURL + "\", version: \"^2.0.0\"},\n]\n",
		},
		{
			name:        "second of two entries on one line",
			content:     spanHeader + "requires: [\n\t{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\"}, {git_url: \"" + spanUtilsURL + "\", version: \"^2.0.0\"},\n]\n",
			gitURL:      spanUtilsURL,
			wantURLs:    []GitURL{spanToolsURL},
			wantContent: spanHeader + "requires: [\n\t{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\"},\n]\n",
		},
		{
			name:        "first of two entries in a one-line block",
			content:     spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}, {git_url: "` + spanUtilsURL + `", version: "^2.0.0"}]` + "\n",
			gitURL:      spanToolsURL,
			wantURLs:    []GitURL{spanUtilsURL},
			wantContent: spanHeader + `requires: [{git_url: "` + spanUtilsURL + `", version: "^2.0.0"}]` + "\n",
		},
		{
			name:        "second of two entries in a one-line block",
			content:     spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}, {git_url: "` + spanUtilsURL + `", version: "^2.0.0"}]` + "\n",
			gitURL:      spanUtilsURL,
			wantURLs:    []GitURL{spanToolsURL},
			wantContent: spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}]` + "\n",
		},
		// Like the multi-line layout, the field's lines go with the blank
		// line after them, here the final newline.
		{
			name:        "only entry of a one-line block",
			content:     spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}]` + "\n",
			gitURL:      spanToolsURL,
			wantURLs:    nil,
			wantContent: strings.TrimSuffix(spanHeader, "\n"),
		},
		{
			name:     "only entry of a one-line block followed by a field on its line",
			content:  "module: \"io.example.root\", " + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}], version: "1.0.0"` + "\n",
			gitURL:   spanToolsURL,
			wantURLs: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeInvowkmodEditFixture(t, tt.content)
			written, err := RemoveRequirementIfPresent(types.FilesystemPath(path), tt.gitURL, "")
			if err != nil {
				t.Fatalf("RemoveRequirementIfPresent() error = %v", err)
			}
			if !written {
				t.Fatal("RemoveRequirementIfPresent() written = false, want the one-line entry removed")
			}
			if tt.wantContent != "" {
				assertInvowkmodEditFile(t, path, tt.wantContent)
			}
			assertRequiresParse(t, path, tt.wantURLs)
		})
	}
}

// TestAddRequirementDuplicateOneLineEntry checks that a module already
// required by a one-line entry is reported as a duplicate, with the file
// untouched.
func TestAddRequirementDuplicateOneLineEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		req     ModuleRef
	}{
		{name: "one-line entry", content: spanHeader + "requires: [\n\t{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\"},\n]\n", req: ModuleRef{GitURL: spanToolsURL, Version: "^2.0.0"}},
		{name: "one-line block", content: spanHeader + `requires: [{git_url: "` + spanToolsURL + `", version: "^1.0.0"}]` + "\n", req: ModuleRef{GitURL: spanToolsURL, Version: "^2.0.0"}},
		{name: "one-line entry with path", content: spanHeader + "requires: [\n\t{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\", path: \"pkg/a\"},\n]\n", req: ModuleRef{GitURL: spanToolsURL, Version: "^2.0.0", Path: "pkg/a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeInvowkmodEditFixture(t, tt.content)
			if err := AddRequirement(types.FilesystemPath(path), tt.req); !errors.Is(err, ErrModuleAlreadyExists) {
				t.Fatalf("AddRequirement() error = %v, want ErrModuleAlreadyExists", err)
			}
			assertInvowkmodEditFile(t, path, tt.content)
		})
	}
}

// TestRequiresEditIgnoresStringsAndComments checks that braces, brackets and
// field labels inside string literals and comments are not structure.
func TestRequiresEditIgnoresStringsAndComments(t *testing.T) {
	t.Parallel()

	t.Run("commented git_url is not a duplicate", func(t *testing.T) {
		t.Parallel()

		content := spanHeader + "requires: [\n\t{\n\t\t// git_url: \"" + spanNewURL + "\" }]\n\t\tgit_url: \"" + spanToolsURL + "\"\n\t\tversion: \"^1.0.0\"\n\t},\n]\n"
		path := writeInvowkmodEditFixture(t, content)
		if err := AddRequirement(types.FilesystemPath(path), ModuleRef{GitURL: spanNewURL, Version: "^2.0.0"}); err != nil {
			t.Fatalf("AddRequirement() error = %v", err)
		}
		assertRequiresParse(t, path, []GitURL{spanToolsURL, spanNewURL})
	})

	t.Run("brackets in a string do not close the list", func(t *testing.T) {
		t.Parallel()

		content := "module: \"io.example.root\"\nversion: \"1.0.0\"\ndescription: \"requires: [ {git_url: \\\"" + spanNewURL + "\\\"} ]\"\nrequires: [{git_url: \"" + spanToolsURL + "\", version: \"^1.0.0\"}, // ]}\n]\n"
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

	t.Run("requires label in a comment is not the field", func(t *testing.T) {
		t.Parallel()

		content := spanHeader + "// requires: [{git_url: \"" + spanToolsURL + "\"}]\n"
		path := writeInvowkmodEditFixture(t, content)
		written, err := RemoveRequirementIfPresent(types.FilesystemPath(path), spanToolsURL, "")
		if err != nil || written {
			t.Fatalf("RemoveRequirementIfPresent() = %t, %v, want no change", written, err)
		}
		assertInvowkmodEditFile(t, path, content)
	})
}

// TestRequiresEditMultiLineLayoutUnchanged pins the layout module add writes:
// add then remove of a multi-line entry returns the file byte-identical.
func TestRequiresEditMultiLineLayoutUnchanged(t *testing.T) {
	t.Parallel()

	content := spanHeader + "\nrequires: [\n\t{\n\t\tgit_url: \"" + spanToolsURL + "\"\n\t\tversion: \"^1.0.0\"\n\t},\n]\n\ndescription: \"after\"\n"
	path := writeInvowkmodEditFixture(t, content)
	if err := AddRequirement(types.FilesystemPath(path), ModuleRef{GitURL: spanNewURL, Version: "^2.0.0", Alias: "n", Path: "pkg/n"}); err != nil {
		t.Fatalf("AddRequirement() error = %v", err)
	}
	assertInvowkmodEditFile(t, path, spanHeader+"\nrequires: [\n\t{\n\t\tgit_url: \""+spanToolsURL+"\"\n\t\tversion: \"^1.0.0\"\n\t},\n\t{\n\t\tgit_url: \""+spanNewURL+"\"\n\t\tversion: \"^2.0.0\"\n\t\talias:   \"n\"\n\t\tpath:    \"pkg/n\"\n\t},\n]\n\ndescription: \"after\"\n")
	if err := RemoveRequirement(types.FilesystemPath(path), spanNewURL, "pkg/n"); err != nil {
		t.Fatalf("RemoveRequirement() error = %v", err)
	}
	assertInvowkmodEditFile(t, path, content)
}

// TestScanRequiresBlockBounds pins the scanner's offsets: a commented
// requires label is skipped, a block without its closing bracket is absent,
// and each entry's span runs from its "{" to just after its "}".
func TestScanRequiresBlockBounds(t *testing.T) {
	t.Parallel()

	if block, ok := scanRequiresBlock("// requires: [\nmodule: \"m\"\nrequires: [\n"); ok {
		t.Fatalf("scanRequiresBlock() = %+v, want absent for an unclosed list", block)
	}

	text := "// requires: [\nrequires: [{a: 1}, {b: \"}\"}]\n"
	block, ok := scanRequiresBlock(text)
	if !ok {
		t.Fatal("scanRequiresBlock() found no block")
	}
	if got, want := text[block.field:block.open+1], "requires: ["; got != want {
		t.Fatalf("field..open = %q, want %q", got, want)
	}
	if got := text[block.close]; got != ']' || block.close != len(text)-2 {
		t.Fatalf("close = %d (%q), want the final ]", block.close, got)
	}
	var spans []string
	for _, entry := range block.entries {
		spans = append(spans, text[entry.start:entry.end])
	}
	if want := []string{"{a: 1}", "{b: \"}\"}"}; !slices.Equal(spans, want) {
		t.Fatalf("entries = %q, want %q", spans, want)
	}
}

// assertRequiresParse parses path with ParseInvowkmod and checks the git URLs
// of its requires list, in order.
func assertRequiresParse(t *testing.T, path string, want []GitURL) {
	t.Helper()

	mod, err := ParseInvowkmod(types.FilesystemPath(path))
	if err != nil {
		t.Fatalf("ParseInvowkmod() error = %v, want the edited file to stay valid CUE", err)
	}
	got := make([]GitURL, 0, len(mod.Requires))
	for i := range mod.Requires {
		got = append(got, mod.Requires[i].GitURL)
	}
	if !slices.Equal(got, want) && (len(got) != 0 || len(want) != 0) {
		t.Fatalf("requires git URLs = %v, want %v", got, want)
	}
}
