// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"testing"

	"github.com/invowk/invowk/internal/testutil/alloygolden"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/internal/testutil/mpctree"
)

// modulePathContainmentGoldenFile is the shared ModulePathContainment golden,
// read from pkg/invowkfile so the containment layer binds the real
// validateScriptPathContainment through the export_test.go hook (design D5.2).
const modulePathContainmentGoldenFile = "../../internal/app/moduleops/testdata/formal/module_path_containment_golden.json.gz"

// TestModulePathContainment_ContainmentLayerGolden replays every instance
// against the real validateScriptPathContainment (via ValidateScriptPathContainmentForTest):
// the lexical containment decision must equal the model's dLexContains, binding
// the layer directly even though ScriptFilePath.Validate masks it at every
// exported entry point (design D5.3).
func TestModulePathContainment_ContainmentLayerGolden(t *testing.T) {
	t.Parallel()

	instances := alloygolden.Load(t, modulePathContainmentGoldenFile, "ModulePathContainment")
	cache := fstree.NewCache(t)
	report := mpctree.NewReporter(t)
	for i, inst := range instances {
		switch mpctree.OpKindOf(inst) {
		case mpctree.ScriptRead, mpctree.EnvRead, mpctree.ContainerfileRef:
		case mpctree.WorkdirCwd, mpctree.VendorCopy, mpctree.Hash, mpctree.Unpack:
			continue // the containment check applies to the read family
		}
		c := mpctree.FromInstance(inst)
		b := cache.Get(t, c.StructuralKey(), c.Spec)
		if b == nil {
			continue
		}
		moduleRoot := FilesystemPath(b.Path[c.OwnerAtom])
		resolved := ScriptFilePath(c.RelPath(b)).ResolveFromModule(moduleRoot)
		if gotInside := ValidateScriptPathContainmentForTest(resolved, moduleRoot) == nil; gotInside != c.LexInside {
			report.Errorf("instance %d %s: real containment inside=%v, model dLexContains=%v (path %q)",
				i, c.OpKind, gotInside, c.LexInside, c.RelPath(b))
		}
	}
	cache.LogSkips(t)
	report.Finish("ModulePathContainment containment layer", len(instances))
}
