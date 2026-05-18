// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

func TestParseBytesRuntimePreflightDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runtime     string
		wantField   string
		wantMessage string
	}{
		{
			name:        "native rejects container-only field",
			runtime:     `{name: "native", persistent: {create_if_missing: true}}`,
			wantField:   "cmds[0].implementations[0].runtimes[0].persistent",
			wantMessage: "persistent is only valid for container runtime",
		},
		{
			name:        "virtual rejects interpreter",
			runtime:     `{name: "virtual", interpreter: "python3"}`,
			wantField:   "cmds[0].implementations[0].runtimes[0].interpreter",
			wantMessage: "virtual runtime always uses mvdan/sh",
		},
		{
			name:        "container requires source",
			runtime:     `{name: "container"}`,
			wantField:   "cmds[0].implementations[0].runtimes[0]",
			wantMessage: "container runtime requires either image or containerfile",
		},
		{
			name:        "container rejects duplicate source",
			runtime:     `{name: "container", image: "debian:stable-slim", containerfile: "Containerfile"}`,
			wantField:   "cmds[0].implementations[0].runtimes[0].image",
			wantMessage: "image and containerfile are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseBytes([]byte(invowkfileWithRuntime(tt.runtime)), "runtime-preflight.cue")
			if err == nil {
				t.Fatal("ParseBytes() error = nil, want runtime preflight diagnostic")
			}
			got := err.Error()
			if !strings.Contains(got, tt.wantField) {
				t.Fatalf("ParseBytes() error missing field %q:\n%s", tt.wantField, got)
			}
			if !strings.Contains(got, tt.wantMessage) {
				t.Fatalf("ParseBytes() error missing message %q:\n%s", tt.wantMessage, got)
			}
		})
	}
}

func invowkfileWithRuntime(runtime string) string {
	return `cmds: [{
	name: "test"
	implementations: [{
		script: "echo test"
		runtimes: [` + runtime + `]
		platforms: [{name: "linux"}]
	}]
}]`
}
