// SPDX-License-Identifier: MPL-2.0

package goplint

import "testing"

func TestLoadRunInputsUsesRunConfigPaths(t *testing.T) {
	origConfigPath := configPath
	origBaselinePath := baselinePath
	origConfigExplicit := configPathExplicit
	origBaselineExplicit := baselinePathExplicit
	t.Cleanup(func() {
		configPath = origConfigPath
		baselinePath = origBaselinePath
		configPathExplicit = origConfigExplicit
		baselinePathExplicit = origBaselineExplicit
	})

	// Set globals to strict missing paths. If loadRunInputs still read globals,
	// this test would fail.
	configPath = "/__missing__/config.toml"
	baselinePath = "/__missing__/baseline.toml"
	configPathExplicit = true
	baselinePathExplicit = true

	cfgPath := writeTempFile(t, "goplint-config.toml", `
[[exceptions]]
pattern = "pkg.Type.Field"
reason = "test"
`)
	blPath := writeTempFile(t, "goplint-baseline.toml", `
[primitive]
entries = [
    { id = "id-1", message = "struct field pkg.Type.Field uses primitive type string" },
]
`)

	rc := runConfig{
		configPath:           cfgPath,
		configPathExplicit:   true,
		baselinePath:         blPath,
		baselinePathExplicit: true,
	}

	cfg, bl, err := loadRunInputs(rc)
	if err != nil {
		t.Fatalf("loadRunInputs returned error: %v", err)
	}
	if cfg == nil || bl == nil {
		t.Fatal("loadRunInputs returned nil config/baseline")
	}
	if !cfg.isExcepted("pkg.Type.Field") {
		t.Fatal("expected runConfig-provided config to be loaded")
	}
	if !bl.ContainsFinding(CategoryPrimitive, "id-1", "") {
		t.Fatal("expected runConfig-provided baseline to be loaded")
	}
}
