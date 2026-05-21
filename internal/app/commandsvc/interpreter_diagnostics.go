// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"log/slog"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func analyzeSelectedImplementationScript(execCtx *runtime.ExecutionContext) (invowkfile.ScriptInterpreterAnalysis, bool) {
	if execCtx == nil || execCtx.SelectedImpl == nil {
		return invowkfile.ScriptInterpreterAnalysis{}, false
	}
	scriptText, err := execCtx.ResolveSelectedScript()
	if err != nil {
		return invowkfile.ScriptInterpreterAnalysis{}, false
	}
	content := invowkfile.ScriptContent(scriptText) //goplint:ignore -- ResolveSelectedScript already validated resolved script content.
	return execCtx.SelectedImpl.Script.AnalyzeInterpreter(content, execCtx.SelectedRuntime), true
}

func appendScriptInterpreterDiagnostics(diags []Diagnostic, analysis invowkfile.ScriptInterpreterAnalysis) []Diagnostic {
	return appendRawScriptInterpreterDiagnostics(diags, analysis.Diagnostics())
}

func appendRawScriptInterpreterDiagnostics(diags []Diagnostic, scriptDiagnostics []invowkfile.ScriptInterpreterDiagnostic) []Diagnostic {
	for i := range scriptDiagnostics {
		scriptDiag := scriptDiagnostics[i]
		diag, err := NewDiagnosticWithCause(
			DiagnosticSeverityWarning,
			DiagnosticCodeScriptInterpreterShebangOverride,
			scriptDiag.Message().String(),
			scriptDiag.Path(),
			nil,
		)
		if err != nil {
			slog.Error("BUG: failed to bridge script interpreter diagnostic",
				"code", scriptDiag.Code(), "error", err)
			continue
		}
		diags = append(diags, diag)
	}
	return diags
}
