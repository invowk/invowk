// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestTUIServerBindValidate(t *testing.T) {
	t.Parallel()

	for _, valid := range []TUIServerBind{"", TUIServerBindRestricted, TUIServerBindAllInterfaces} {
		if err := valid.Validate(); err != nil {
			t.Errorf("TUIServerBind(%q).Validate() = %v, want nil", valid, err)
		}
	}
	err := TUIServerBind("everywhere").Validate()
	if !errors.Is(err, ErrInvalidTUIServerBind) {
		t.Fatalf("Validate(everywhere) = %v, want ErrInvalidTUIServerBind", err)
	}
	if !TUIServerBindAllInterfaces.AllowsAllInterfaces() || TUIServerBindRestricted.AllowsAllInterfaces() || TUIServerBind("").AllowsAllInterfaces() {
		t.Fatal("AllowsAllInterfaces() must hold only for all-interfaces")
	}
}

func TestDefaultConfigTUIServerBindIsRestricted(t *testing.T) {
	t.Parallel()

	if got := DefaultConfig().Container.TUIServerBind; got != TUIServerBindRestricted {
		t.Fatalf("DefaultConfig().Container.TUIServerBind = %q, want restricted", got)
	}
}

// TestTUIServerBindFromInvowkConfigSource pins the trust boundary from the
// user-config side: a config file selected through INVOWK_CONFIG sets the
// opt-in, and GenerateCUE round-trips it.
func TestTUIServerBindFromInvowkConfigSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := writeSourceTestFile(t, dir, "env.cue", "container: tui_server_bind: \"all-interfaces\"\n")
	provider := NewProviderWithEnv(envWith(envPath))
	cfg, err := provider.Load(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())})
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Container.TUIServerBind != TUIServerBindAllInterfaces {
		t.Fatalf("TUIServerBind = %q, want all-interfaces", cfg.Container.TUIServerBind)
	}

	generated := GenerateCUE(cfg)
	if !strings.Contains(generated, `tui_server_bind: "all-interfaces"`) {
		t.Fatalf("GenerateCUE() does not carry the opt-in:\n%s", generated)
	}
	roundTrip := writeSourceTestFile(t, dir, "roundtrip.cue", generated)
	cfg2, err := NewProviderWithEnv(envWith(roundTrip)).Load(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())})
	if err != nil {
		t.Fatalf("Load(round trip): %v", err)
	}
	if cfg2.Container.TUIServerBind != TUIServerBindAllInterfaces {
		t.Fatalf("round-trip TUIServerBind = %q, want all-interfaces", cfg2.Container.TUIServerBind)
	}
}

func TestTUIServerBindRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := writeSourceTestFile(t, dir, "env.cue", "container: tui_server_bind: \"everywhere\"\n")
	_, err := NewProviderWithEnv(envWith(envPath)).Load(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(t.TempDir())})
	if err == nil {
		t.Fatal("Load() accepted tui_server_bind: \"everywhere\"")
	}
	if !strings.Contains(err.Error(), "tui_server_bind") {
		t.Fatalf("Load() error = %v, want it to name tui_server_bind", err)
	}
}
