// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

// TestInvowkfileCannotSetTUIServerBind pins the trust boundary from the
// repository side: container.tui_server_bind is user configuration only, so an
// invowkfile that tries to set it in a container runtime block is rejected
// instead of widening the TUI server bind.
func TestInvowkfileCannotSetTUIServerBind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  string
	}{
		{
			name: "container runtime block",
			cue: `
cmds: [{
	name: "probe"
	implementations: [{
		script: {content: "true"}
		runtimes: [{name: "container", image: "debian:stable-slim", tui_server_bind: "all-interfaces"}]
		platforms: [{name: "linux"}]
	}]
}]`,
		},
		{
			name: "invowkfile root",
			cue: `
tui_server_bind: "all-interfaces"
cmds: [{
	name: "probe"
	implementations: [{script: {content: "true"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}]}]
}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseBytes([]byte(tt.cue), "invowkfile.cue"); err == nil {
				t.Fatal("ParseBytes() accepted tui_server_bind in an invowkfile")
			}
			// Control: the same invowkfile without the key parses, so the
			// rejection above is the closed schema, not a broken fixture.
			control := strings.NewReplacer(`, tui_server_bind: "all-interfaces"`, "", `tui_server_bind: "all-interfaces"`, "").Replace(tt.cue)
			if _, err := ParseBytes([]byte(control), "invowkfile.cue"); err != nil {
				t.Fatalf("control fixture does not parse: %v", err)
			}
		})
	}
}

// TestInvowkmodCannotSetTUIServerBind covers the module manifest side of the
// trust boundary: invowkmod.cue is closed, so tui_server_bind is rejected.
func TestInvowkmodCannotSetTUIServerBind(t *testing.T) {
	t.Parallel()

	control := "module: \"com.example.tuibind\"\nversion: \"0.1.0\"\n"
	if _, err := ParseInvowkmodBytes([]byte(control), "invowkmod.cue"); err != nil {
		t.Fatalf("control manifest does not parse: %v", err)
	}
	for _, extra := range []string{
		"tui_server_bind: \"all-interfaces\"\n",
		"container: tui_server_bind: \"all-interfaces\"\n",
	} {
		if _, err := ParseInvowkmodBytes([]byte(control+extra), "invowkmod.cue"); err == nil {
			t.Fatalf("ParseInvowkmodBytes() accepted %q in a module manifest", extra)
		}
	}
}
