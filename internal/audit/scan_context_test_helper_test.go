// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"github.com/invowk/invowk/pkg/invowkfile"
)

// newTestScanContext creates a ScanContext for testing with scripts pre-computed.
// This mirrors BuildScanContext's post-construction step.
func newTestScanContext(invowkfiles []*ScannedInvowkfile, modules []*ScannedModule) *ScanContext {
	return &ScanContext{
		invowkfiles: invowkfiles,
		modules:     modules,
		scripts:     buildScriptRefs(invowkfiles, modules),
	}
}

// newSingleScriptContext creates a ScanContext with one inline script for content analysis tests.
func newSingleScriptContext(script string) *ScanContext {
	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "cmd",
			Implementations: []invowkfile.Implementation{{
				Script:   invowkfile.ScriptContent(script),
				Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
			}},
		}},
	}
	files := []*ScannedInvowkfile{{
		Path:       "test.cue",
		SurfaceID:  "test",
		Invowkfile: inv,
	}}
	return newTestScanContext(files, nil)
}

// newModuleOnlyContext creates a ScanContext with only modules (no standalone invowkfiles).
func newModuleOnlyContext(modules ...*ScannedModule) *ScanContext {
	return newTestScanContext(nil, modules)
}
