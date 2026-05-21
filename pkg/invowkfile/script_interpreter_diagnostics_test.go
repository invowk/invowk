// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

type scriptInterpreterDiagnosticCase struct {
	name           string
	source         ScriptInterpreterSource
	interpreter    InterpreterSpec
	script         ScriptContent
	runtime        RuntimeMode
	wantProvenance ScriptInterpreterProvenance
	wantEffective  string
	wantDiag       bool
}

func TestAnalyzeScriptInterpreterDiagnostics(t *testing.T) {
	t.Parallel()

	fileSource := newFileScriptInterpreterSource("scripts/build", "")
	inlineSource := newInlineScriptInterpreterSource("inline script")

	tests := []scriptInterpreterDiagnosticCase{
		{
			name:           "explicit interpreter overrides file shebang",
			source:         fileSource,
			interpreter:    "python3",
			script:         "#!/bin/sh\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "python3",
			wantDiag:       true,
		},
		{
			name:           "omitted interpreter uses shebang without warning",
			source:         inlineSource,
			script:         "#!/usr/bin/env python3\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceShebang,
			wantEffective:  "python3",
		},
		{
			name:           "auto interpreter uses shebang without warning",
			source:         inlineSource,
			interpreter:    "auto",
			script:         "#!/bin/bash\necho ok",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceShebang,
			wantEffective:  "/bin/bash",
		},
		{
			name:           "equivalent env shebang does not warn",
			source:         inlineSource,
			interpreter:    "python3",
			script:         "#!/usr/bin/env python3\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "python3",
		},
		{
			name:           "argument difference warns",
			source:         inlineSource,
			interpreter:    "python3",
			script:         "#!/usr/bin/env -S python3 -u\nprint('ok')",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "python3",
			wantDiag:       true,
		},
		{
			name:           "virtual shell-compatible selections do not warn",
			source:         inlineSource,
			interpreter:    "bash",
			script:         "#!/bin/sh\necho ok",
			runtime:        RuntimeVirtualSh,
			wantProvenance: ScriptInterpreterProvenanceExplicit,
			wantEffective:  "bash",
		},
		{
			name:           "default shell without shebang",
			source:         inlineSource,
			script:         "echo ok",
			runtime:        RuntimeNative,
			wantProvenance: ScriptInterpreterProvenanceDefaultShell,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertScriptInterpreterDiagnostic(t, tt)
		})
	}
}

func assertScriptInterpreterDiagnostic(t *testing.T, tt scriptInterpreterDiagnosticCase) {
	t.Helper()

	got := AnalyzeScriptInterpreter(tt.source, tt.interpreter, tt.script, tt.runtime)
	if got.Provenance() != tt.wantProvenance {
		t.Fatalf("Provenance() = %q, want %q", got.Provenance(), tt.wantProvenance)
	}
	if got.Effective().CommandString() != tt.wantEffective {
		t.Fatalf("Effective() = %q, want %q", got.Effective().CommandString(), tt.wantEffective)
	}
	diagnostics := got.Diagnostics()
	if (len(diagnostics) > 0) != tt.wantDiag {
		t.Fatalf("Diagnostics() len = %d, want diagnostic=%v", len(diagnostics), tt.wantDiag)
	}
	if tt.wantDiag {
		assertInterpreterDiagnosticMessage(t, diagnostics[0])
	}
}

func assertInterpreterDiagnosticMessage(t *testing.T, diagnostic ScriptInterpreterDiagnostic) {
	t.Helper()

	message := diagnostic.Message().String()
	for _, token := range []string{"script.interpreter", "overrides shebang", "takes precedence"} {
		if !strings.Contains(message, token) {
			t.Fatalf("diagnostic message %q missing %q", message, token)
		}
	}
}
