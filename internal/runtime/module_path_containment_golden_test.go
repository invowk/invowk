// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/alloygolden"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/internal/testutil/mpctree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// modulePathContainmentGoldenFile is the shared ModulePathContainment golden,
// read from internal/runtime so the container working-directory layer binds the
// real getContainerWorkDir (design D5.2).
const modulePathContainmentGoldenFile = "../app/moduleops/testdata/formal/module_path_containment_golden.json.gz"

// TestModulePathContainment_ContainerWorkDirGolden replays every WorkdirCwd
// instance against getContainerWorkDir (which resolves the workdir through
// GetEffectiveWorkDir). A declared workdir is an execution working directory
// only, never a read target: the model records dContained = true and
// dAccepts = false for it. The real mapping puts the workdir under /workspace
// exactly when it is lexically inside the invowkfile directory (dLexContains).
func TestModulePathContainment_ContainerWorkDirGolden(t *testing.T) {
	t.Parallel()

	instances := alloygolden.Load(t, modulePathContainmentGoldenFile, "ModulePathContainment")
	cache := fstree.NewCache(t)
	report := mpctree.NewReporter(t)
	rt := &ContainerRuntime{}
	for i, inst := range instances {
		if mpctree.OpKindOf(inst) != mpctree.WorkdirCwd {
			continue
		}
		c := mpctree.FromInstance(inst)
		if c.Accepts || !c.Contained {
			report.Errorf("instance %d WorkdirCwd: model accepts=%v contained=%v, want false/true (a workdir is a cwd, not a read)",
				i, c.Accepts, c.Contained)
			continue
		}
		b := cache.Get(t, c.StructuralKey(), c.Spec)
		if b == nil {
			continue
		}
		invowkDir := b.Path[c.OwnerAtom]
		ctx := &ExecutionContext{
			Invowkfile:   &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(invowkDir, "invowkfile.cue"))},
			SelectedImpl: &invowkfile.Implementation{WorkDir: invowkfile.WorkDir(c.RelPath(b))},
		}
		containerPath := rt.getContainerWorkDir(ctx, invowkDir)
		if mapped := strings.HasPrefix(containerPath, containerWorkspaceRoot); mapped != c.LexInside {
			report.Errorf("instance %d WorkdirCwd: container workdir %q under /workspace=%v, model dLexContains=%v",
				i, containerPath, mapped, c.LexInside)
		}
	}
	cache.LogSkips(t)
	report.Finish("ModulePathContainment container workdir", len(instances))
}
