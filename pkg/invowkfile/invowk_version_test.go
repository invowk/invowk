// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestParseBytes_InvowkVersionAndHint(t *testing.T) { //nolint:paralleltest // Mutates global buildVersionOverride and slog default logger.
	//nolint:paralleltest // subtests mutate global buildVersionOverride and slog default logger.
	t.Run("satisfied invowk_version parses cleanly", func(t *testing.T) {
		defer invowkmod.OverrideBuildVersionForTest("0.18.0")()

		data := []byte(`
invowk_version: ">=0.18.0"
cmds: [{
	name: "build"
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`)
		inv, err := ParseBytes(data, "invowkfile.cue")
		if err != nil {
			t.Fatalf("ParseBytes() error = %v", err)
		}
		if inv.InvowkVersion != ">=0.18.0" {
			t.Fatalf("InvowkVersion = %q, want >=0.18.0", inv.InvowkVersion)
		}
	})

	//nolint:paralleltest // subtests mutate global buildVersionOverride.
	t.Run("unsatisfied invowk_version fails before unknown fields and runtime preflight", func(t *testing.T) {
		defer invowkmod.OverrideBuildVersionForTest("0.18.0")()

		data := []byte(`
invowk_version: ">=0.19.0"
future_top_level: true
cmds: [{
	name: "build"
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "native", future_runtime_field: true}]
		platforms: [{name: "linux"}]
	}]
}]
`)
		_, err := ParseBytes(data, "invowkfile.cue")
		if !errors.Is(err, ErrUnsupportedInvowkVersion) {
			t.Fatalf("error = %v, want ErrUnsupportedInvowkVersion", err)
		}
		if strings.Contains(err.Error(), "field not allowed") {
			t.Fatalf("error %q must fail on invowk_version before strict schema decoding", err)
		}
		var verErr *UnsupportedInvowkVersionError
		if !errors.As(err, &verErr) {
			t.Fatalf("error type = %T, want *UnsupportedInvowkVersionError", err)
		}
		if verErr.Required != ">=0.19.0" || verErr.CurrentVersion != "0.18.0" {
			t.Fatalf("UnsupportedInvowkVersionError = %+v", verErr)
		}
	})

	//nolint:paralleltest // subtests mutate global buildVersionOverride.
	t.Run("field not allowed appends upgrade hint", func(t *testing.T) {
		defer invowkmod.OverrideBuildVersionForTest("0.18.0")()

		data := []byte(`
cmds: [{
	name: "build"
	future_field: true
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}]
	}]
}]
`)
		_, err := ParseBytes(data, "invowkfile.cue")
		if err == nil {
			t.Fatal("expected schema error")
		}
		if !strings.Contains(err.Error(), "field not allowed") {
			t.Fatalf("error = %q, want field not allowed", err)
		}
		if !strings.Contains(err.Error(), "hint: this file may require a newer version of invowk") {
			t.Fatalf("error = %q, want upgrade hint", err)
		}
	})

	//nolint:paralleltest // subtests mutate global buildVersionOverride and slog default logger.
	t.Run("true dev build warns and proceeds", func(t *testing.T) {
		defer invowkmod.OverrideBuildVersionForTest("dev")()

		var logBuf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
		defer slog.SetDefault(prev)

		data := []byte(`
invowk_version: ">=99.0.0"
cmds: [{
	name: "build"
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}]
	}]
}]
`)
		inv, err := ParseBytes(data, "invowkfile.cue")
		if err != nil {
			t.Fatalf("ParseBytes() in dev build error = %v", err)
		}
		if inv.InvowkVersion != ">=99.0.0" {
			t.Fatalf("InvowkVersion = %q, want >=99.0.0", inv.InvowkVersion)
		}
		if !strings.Contains(logBuf.String(), "invowk_version") {
			t.Fatalf("expected slog warning mentioning invowk_version, got %q", logBuf.String())
		}
	})
}
