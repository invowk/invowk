// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/types"
)

// skippedDiag builds a load-failure diagnostic for path (empty for none).
func skippedDiag(t *testing.T, code discovery.DiagnosticCode, path string) discovery.Diagnostic {
	t.Helper()
	var (
		d   discovery.Diagnostic
		err error
	)
	if path == "" {
		d, err = discovery.NewDiagnostic(discovery.SeverityWarning, code, "skipped")
	} else {
		d, err = discovery.NewDiagnosticWithPath(discovery.SeverityWarning, code, "skipped", types.FilesystemPath(path)) //goplint:ignore -- test fixture path
	}
	if err != nil {
		t.Fatalf("NewDiagnostic() error = %v", err)
	}
	return d
}

// TestRenderAndWrapServiceError_ScopedNotFoundAfterSkippedSource verifies
// that a scoped request (@source or --ivk-from) whose source loaded is a
// certain miss (ExitCodeUsage) when a different source failed to load, while
// a skipped source that may be the requested one keeps exit status 1.
func TestRenderAndWrapServiceError_ScopedNotFoundAfterSkippedSource(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	brokenMod := filepath.Join(work, "broken.invowkmod")
	rootFile := filepath.Join(work, "invowkfile.cue")
	modFile := filepath.Join(work, "m.invowkmod", "invowkfile.cue")
	notFoundIn := func(src discovery.SourceID) error {
		return &commandsvc.ClassifiedError{Err: errors.New("command 'nope' not found in source '" + string(src) + "'"), Kind: commandsvc.ErrorKindCommandNotFound}
	}
	sourceNotFound := func(src discovery.SourceID) error {
		return &commandsvc.ClassifiedError{Err: &commandsvc.SourceNotFoundError{Source: src}, Kind: commandsvc.ErrorKindCommandNotFound}
	}

	tests := []struct {
		name  string
		err   error
		scope discovery.SourceID
		diag  discovery.Diagnostic
		want  types.ExitCode
	}{
		{name: "loaded invowkfile, other module broken", err: notFoundIn("invowkfile"), scope: "invowkfile", diag: skippedDiag(t, discovery.CodeModuleLoadSkipped, brokenMod), want: ExitCodeUsage},
		{name: "loaded module, root invowkfile broken", err: notFoundIn("m"), scope: "m", diag: skippedDiag(t, discovery.CodeInvowkfileParseSkipped, rootFile), want: ExitCodeUsage},
		{name: "unknown source, other module broken", err: sourceNotFound("bogus"), scope: "bogus", diag: skippedDiag(t, discovery.CodeModuleLoadSkipped, brokenMod), want: ExitCodeUsage},
		{name: "requested module is the broken one", err: sourceNotFound("broken"), scope: "broken", diag: skippedDiag(t, discovery.CodeModuleLoadSkipped, brokenMod), want: 1},
		{name: "requested invowkfile is the broken one", err: sourceNotFound("invowkfile"), scope: "invowkfile", diag: skippedDiag(t, discovery.CodeInvowkfileParseSkipped, rootFile), want: 1},
		{name: "same-named module loaded elsewhere", err: notFoundIn("broken"), scope: "broken", diag: skippedDiag(t, discovery.CodeModuleLoadSkipped, brokenMod), want: 1},
		{name: "module invowkfile parse skip may be any source", err: notFoundIn("invowkfile"), scope: "invowkfile", diag: skippedDiag(t, discovery.CodeInvowkfileParseSkipped, modFile), want: 1},
		{name: "include failure may carry an alias", err: notFoundIn("invowkfile"), scope: "invowkfile", diag: skippedDiag(t, discovery.CodeIncludeModuleLoadFailed, brokenMod), want: 1},
		{name: "vendored failure may carry a namespace", err: notFoundIn("invowkfile"), scope: "invowkfile", diag: skippedDiag(t, discovery.CodeVendoredModuleLoadSkipped, brokenMod), want: 1},
		{name: "diagnostic without a path", err: notFoundIn("invowkfile"), scope: "invowkfile", diag: skippedDiag(t, discovery.CodeModuleLoadSkipped, ""), want: 1},
		{name: "unscoped, any broken source counts", err: notFoundIn("invowkfile"), scope: "", diag: skippedDiag(t, discovery.CodeModuleLoadSkipped, brokenMod), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := ExecuteRequest{Name: "nope", FromSource: tt.scope}
			wrapped := renderAndWrapServiceError(tt.err, req, []discovery.Diagnostic{tt.diag})
			if got := exitCodeForError(wrapped); got != tt.want {
				t.Fatalf("exitCodeForError() = %d, want %d", got, tt.want)
			}
			_, isSkipped := errors.AsType[*SkippedSourcesError](wrapped)
			if isSkipped != (tt.want == 1) {
				t.Fatalf("SkippedSourcesError in chain = %v, want %v", isSkipped, tt.want == 1)
			}
		})
	}
}

// TestSkippedSourcesErrorMessage verifies that a not-found error kept at exit
// status 1 names every skipped source and its diagnostic code, and still
// unwraps to the not-found error.
func TestSkippedSourcesErrorMessage(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	base := errors.New("command 'nope' not found")
	diags := []discovery.Diagnostic{
		skippedDiag(t, discovery.CodeModuleLoadSkipped, filepath.Join(work, "broken.invowkmod")),
		skippedDiag(t, discovery.CodeInvowkfileParseSkipped, filepath.Join(work, "m.invowkmod", "invowkfile.cue")),
		skippedDiag(t, discovery.CodeModuleLoadSkipped, ""),
		skippedDiag(t, discovery.CodeReservedModuleNameSkipped, filepath.Join(work, "invowkfile.invowkmod")),
	}

	err := notFoundError(newUsageError(base), diags, "")
	if got := exitCodeForError(err); got != 1 {
		t.Fatalf("exitCodeForError() = %d, want 1", got)
	}
	if !errors.Is(err, base) {
		t.Fatal("errors.Is(err, base) = false, want the not-found error in the chain")
	}
	msg := err.Error()
	for _, want := range []string{
		"command 'nope' not found; discovery skipped sources that failed to load:",
		"broken.invowkmod (module_load_skipped)",
		"m.invowkmod/invowkfile.cue (invowkfile_parse_skipped)",
		"an unnamed source (module_load_skipped)",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
	if strings.Contains(msg, "reserved_module_name_skipped") {
		t.Errorf("Error() = %q, must not name a policy skip", msg)
	}
	if strings.Contains(msg, work) {
		t.Errorf("Error() = %q, want labels without the full path", msg)
	}
}
