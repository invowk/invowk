// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"io"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestCustomCheckScriptFileInterpreterWarnings(t *testing.T) {
	t.Parallel()

	expectedCode := types.ExitCode(0)
	scriptFile := invowkfile.ScriptFilePath("scripts/check.sh")
	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Name: "file-check",
			Script: invowkfile.CustomCheckScript{
				File:        &scriptFile,
				Interpreter: "python3",
			},
			ExpectedCode:   &expectedCode,
			ExpectedOutput: "^ok$",
		}},
	}

	t.Run("host", func(t *testing.T) {
		t.Parallel()

		stderr := &strings.Builder{}
		var diagnostics []invowkfile.ScriptInterpreterDiagnostic
		ctx := customCheckFileContext(t, "#!/bin/sh\nprint('ok')\n")
		ctx.IO.Stderr = stderr
		ctx.ReportScriptInterpreter = func(diag invowkfile.ScriptInterpreterDiagnostic) {
			diagnostics = append(diagnostics, diag)
		}
		probe := &recordingHostProbe{
			checkResults: map[invowkfile.CheckName]CustomCheckResult{
				"file-check": mustCustomCheckResult(t, "ok", 0),
			},
		}

		if err := CheckHostCustomCheckDependenciesWithProbe(deps, ctx, probe); err != nil {
			t.Fatalf("CheckHostCustomCheckDependenciesWithProbe() = %v", err)
		}
		if stderr.String() != "" {
			t.Fatalf("stderr = %q, want no direct dependency rendering", stderr.String())
		}
		assertCustomCheckInterpreterDiagnostic(t, diagnostics)
	})

	t.Run("container", func(t *testing.T) {
		t.Parallel()

		stderr := &strings.Builder{}
		var diagnostics []invowkfile.ScriptInterpreterDiagnostic
		ctx := customCheckFileContext(t, "#!/bin/sh\nprint('ok')\n")
		ctx.IO.Stderr = stderr
		ctx.ReportScriptInterpreter = func(diag invowkfile.ScriptInterpreterDiagnostic) {
			diagnostics = append(diagnostics, diag)
		}
		stub := newFilepathStubRuntime(t,
			func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				_, _ = io.WriteString(ctx.IO.Stdout, "ok\n")
				return &runtimepkg.Result{ExitCode: 0}
			})

		if err := CheckCustomCheckDependenciesInContainer(deps, stub, ctx); err != nil {
			t.Fatalf("CheckCustomCheckDependenciesInContainer() = %v", err)
		}
		if stderr.String() != "" {
			t.Fatalf("stderr = %q, want no direct dependency rendering", stderr.String())
		}
		assertCustomCheckInterpreterDiagnostic(t, diagnostics)
	})
}

func assertCustomCheckInterpreterDiagnostic(t testing.TB, diagnostics []invowkfile.ScriptInterpreterDiagnostic) {
	t.Helper()
	if len(diagnostics) != 1 {
		t.Fatalf("len(diagnostics) = %d, want 1", len(diagnostics))
	}
	output := diagnostics[0].Message().String()
	for _, want := range []string{
		"scripts/check.sh",
		"python3",
		"/bin/sh",
		"script.interpreter takes precedence",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("warning output = %q, want containing %q", output, want)
		}
	}
}
