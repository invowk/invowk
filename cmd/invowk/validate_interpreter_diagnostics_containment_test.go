// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestCollectInvowkfileInterpreterDiagnosticsNeverReadsOutsideModule: when a
// module's script.file and a custom-check file escape through an
// invowk_modules/ link, `invowk validate`'s interpreter diagnostics never ask
// the reader for them (SC-01, F13), while a contained script is still read.
func TestCollectInvowkfileInterpreterDiagnosticsNeverReadsOutsideModule(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	mod := filepath.Join(base, "com.example.tools.invowkmod")
	if err := os.MkdirAll(filepath.Join(mod, "invowk_modules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(base, "outside.sh")
	if err := os.WriteFile(outside, []byte("#!/bin/sh\necho outside\n"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mod, "inside.sh"), []byte("#!/bin/sh\necho inside\n"), 0o644); err != nil {
		t.Fatalf("write inside: %v", err)
	}
	fstree.MustSymlink(t, outside, filepath.Join(mod, "invowk_modules", "x"))

	escaping := invowkfile.ScriptFilePath("invowk_modules/x")
	inside := invowkfile.ScriptFilePath("inside.sh")
	inv := &invowkfile.Invowkfile{
		ModulePath: invowkfile.FilesystemPath(mod),
		DependsOn: &invowkfile.DependsOn{CustomChecks: []invowkfile.CustomCheckDependency{{
			Name:   "escaping-check",
			Script: invowkfile.CustomCheckScript{File: &escaping},
		}}},
		Commands: []invowkfile.Command{{
			Name: "demo",
			Implementations: []invowkfile.Implementation{
				{
					Script:    invowkfile.ImplementationScript{File: &escaping},
					Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
					Platforms: invowkfile.AllPlatformConfigs(),
				},
				{
					Script:    invowkfile.ImplementationScript{File: &inside},
					Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
					Platforms: invowkfile.AllPlatformConfigs(),
				},
			},
		}},
	}

	var reads []string
	collectInvowkfileInterpreterDiagnostics(inv, func(p string) ([]byte, error) {
		reads = append(reads, p)
		return os.ReadFile(p)
	})
	for _, p := range reads {
		if filepath.Base(p) == "x" {
			t.Errorf("reader asked for the escaping path %s", p)
		}
	}
	if len(reads) != 1 || filepath.Base(reads[0]) != "inside.sh" {
		t.Errorf("reads = %v, want only the contained inside.sh", reads)
	}
}
