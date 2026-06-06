// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestParseInvowkmodBytesMutationRequirementPathError(t *testing.T) {
	t.Parallel()

	const path = types.FilesystemPath("/workspace/invowkmod.cue")
	content := []byte(`module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/root.git", version: "^1.0.0"},
	{git_url: "https://github.com/example/tools.git", version: "^2.0.0", path: "../tools.invowkmod"},
]
`)

	_, err := ParseInvowkmodBytes(content, path)
	if err == nil {
		t.Fatal("ParseInvowkmodBytes() error = nil, want indexed requirement path error")
	}
	if !errors.Is(err, ErrInvalidSubdirectoryPath) {
		t.Fatalf("ParseInvowkmodBytes() error = %v, want ErrInvalidSubdirectoryPath", err)
	}
	var pathErr *InvalidSubdirectoryPathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("ParseInvowkmodBytes() error type = %T, want *InvalidSubdirectoryPathError", err)
	}
	if pathErr.Value != "../tools.invowkmod" {
		t.Fatalf("InvalidSubdirectoryPathError.Value = %q, want %q", pathErr.Value, "../tools.invowkmod")
	}
	if pathErr.Reason != "path traversal not allowed" {
		t.Fatalf("InvalidSubdirectoryPathError.Reason = %q, want path traversal not allowed", pathErr.Reason)
	}
	want := `requires[1].path: invalid subdirectory path "../tools.invowkmod": path traversal not allowed in invowkmod at /workspace/invowkmod.cue`
	if err.Error() != want {
		t.Fatalf("ParseInvowkmodBytes() error = %q, want %q", err.Error(), want)
	}
}
