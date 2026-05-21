// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"io"
	"testing"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	recordingContainerRuntime struct {
		scripts []invowkfile.ImplementationScript
	}

	runtimeProbeScriptCase struct {
		name            string
		script          invowkfile.CustomCheckScript
		wantContent     invowkfile.ScriptContent
		wantInterpreter invowkfile.InterpreterSpec
	}
)

func (r *recordingContainerRuntime) Name() string { return "container" }

func (r *recordingContainerRuntime) Available() bool { return true }

func (r *recordingContainerRuntime) Validate(*runtime.ExecutionContext) error { return nil }

func (r *recordingContainerRuntime) Execute(ctx *runtime.ExecutionContext) *runtime.Result {
	r.scripts = append(r.scripts, ctx.SelectedImpl.Script)
	_, _ = io.WriteString(ctx.IO.Stdout, "ok")
	return &runtime.Result{ExitCode: 0}
}

func TestDependencyRuntimeProbeRunCustomCheckPreservesScriptInterpreter(t *testing.T) {
	t.Parallel()

	tests := []runtimeProbeScriptCase{
		{
			name: "omitted interpreter",
			script: invowkfile.CustomCheckScript{
				Content: "echo ok",
			},
			wantContent: "echo ok",
		},
		{
			name: "shebang interpreter",
			script: invowkfile.CustomCheckScript{
				Content: "#!/usr/bin/env python3\nprint('ok')",
			},
			wantContent: "#!/usr/bin/env python3\nprint('ok')",
		},
		{
			name: "explicit non-shell interpreter",
			script: invowkfile.CustomCheckScript{
				Content:     "print('ok')",
				Interpreter: "python3",
			},
			wantContent:     "print('ok')",
			wantInterpreter: "python3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertRuntimeProbePreservesScriptInterpreter(t, tt)
		})
	}
}

func assertRuntimeProbePreservesScriptInterpreter(t *testing.T, tt runtimeProbeScriptCase) {
	t.Helper()

	probe, containerRT := newRecordingRuntimeProbe(t)
	check := invowkfile.CustomCheck{
		Name:           "container-check",
		Script:         tt.script,
		ExpectedOutput: "^ok$",
	}

	result, err := probe.RunCustomCheck(check)
	if err != nil {
		t.Fatalf("RunCustomCheck() = %v", err)
	}
	if got := result.Output().String(); got != "ok" {
		t.Fatalf("custom check output = %q, want ok", got)
	}
	assertCapturedCustomCheckScript(t, containerRT, tt)
}

func newRecordingRuntimeProbe(t *testing.T) (dependencyRuntimeProbe, *recordingContainerRuntime) {
	t.Helper()

	reg := runtime.NewRegistry()
	containerRT := &recordingContainerRuntime{}
	reg.Register(runtime.RuntimeTypeContainer, containerRT)

	return dependencyRuntimeProbe{
		registry:  reg,
		parentCtx: runtimeContext(t, invowkfile.RuntimeContainer),
	}, containerRT
}

func assertCapturedCustomCheckScript(t *testing.T, containerRT *recordingContainerRuntime, tt runtimeProbeScriptCase) {
	t.Helper()

	if len(containerRT.scripts) != 1 {
		t.Fatalf("captured scripts = %d, want 1", len(containerRT.scripts))
	}
	if got := containerRT.scripts[0].Content; got != tt.wantContent {
		t.Fatalf("container custom check script = %q, want %q", got, tt.wantContent)
	}
	if got := containerRT.scripts[0].Interpreter; got != tt.wantInterpreter {
		t.Fatalf("container custom check interpreter = %q, want %q", got, tt.wantInterpreter)
	}
}
