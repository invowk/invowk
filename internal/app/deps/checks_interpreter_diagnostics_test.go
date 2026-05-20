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
	scriptFile := invowkfile.FilesystemPath("scripts/check.sh")
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
		ctx := customCheckFileContext(t, "#!/bin/sh\nprint('ok')\n")
		ctx.IO.Stderr = stderr
		probe := &recordingHostProbe{
			checkResults: map[invowkfile.CheckName]CustomCheckResult{
				"file-check": mustCustomCheckResult(t, "ok", 0),
			},
		}

		if err := CheckHostCustomCheckDependenciesWithProbe(deps, ctx, probe); err != nil {
			t.Fatalf("CheckHostCustomCheckDependenciesWithProbe() = %v", err)
		}
		assertCustomCheckInterpreterWarning(t, stderr.String())
	})

	t.Run("container", func(t *testing.T) {
		t.Parallel()

		stderr := &strings.Builder{}
		ctx := customCheckFileContext(t, "#!/bin/sh\nprint('ok')\n")
		ctx.IO.Stderr = stderr
		stub := &filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				_, _ = io.WriteString(ctx.IO.Stdout, "ok\n")
				return &runtimepkg.Result{ExitCode: 0}
			},
		}

		if err := CheckCustomCheckDependenciesInContainer(deps, stub, ctx); err != nil {
			t.Fatalf("CheckCustomCheckDependenciesInContainer() = %v", err)
		}
		assertCustomCheckInterpreterWarning(t, stderr.String())
	})
}

func assertCustomCheckInterpreterWarning(t testing.TB, output string) {
	t.Helper()
	for _, want := range []string{
		"warning:",
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
