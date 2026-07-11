// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// newTestScanContext creates a ScanContext for testing with scripts pre-computed.
// This mirrors BuildScanContext's post-construction step.
func newTestScanContext(t *testing.T, invowkfiles []*ScannedInvowkfile, modules []*ScannedModule) *ScanContext {
	t.Helper()

	scripts, err := buildScriptRefs(t.Context(), invowkfiles, modules)
	if err != nil {
		t.Fatalf("buildScriptRefs() error = %v", err)
	}
	return &ScanContext{
		invowkfiles: invowkfiles,
		modules:     modules,
		scripts:     scripts,
	}
}

// newSingleScriptContext creates a ScanContext with one inline script for content analysis tests.
func newSingleScriptContext(t *testing.T, script string) *ScanContext {
	t.Helper()

	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "cmd",
			Implementations: []invowkfile.Implementation{{
				Script:   invowkfile.ImplementationScript{Content: invowkfile.ScriptContent(script)},
				Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtualSh}},
			}},
		}},
	}
	files := []*ScannedInvowkfile{{
		Path:       "test.cue",
		SurfaceID:  "test",
		Invowkfile: inv,
	}}
	return newTestScanContext(t, files, nil)
}

// newModuleOnlyContext creates a ScanContext with only modules (no standalone invowkfiles).
func newModuleOnlyContext(t *testing.T, modules ...*ScannedModule) *ScanContext {
	t.Helper()

	return newTestScanContext(t, nil, modules)
}
